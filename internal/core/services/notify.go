// Package services implements core business workflows.
package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

var webhookHTTPClient = &http.Client{Timeout: 15 * time.Second}

// NotifyServiceParams defines the dependencies for NotifyService.
type NotifyServiceParams struct {
	Repo     ports.NotifyRepository
	RBACSvc  ports.RBACService
	QueueSvc ports.QueueService
	EventSvc ports.EventService
	AuditSvc ports.AuditService
	Logger   *slog.Logger
}

// NotifyService manages topics, subscriptions, and message delivery.
type NotifyService struct {
	repo     ports.NotifyRepository
	rbacSvc  ports.RBACService
	queueSvc ports.QueueService
	eventSvc ports.EventService
	auditSvc ports.AuditService
	logger   *slog.Logger
}

// NewNotifyService constructs a NotifyService with its dependencies.
func NewNotifyService(params NotifyServiceParams) ports.NotifyService {
	return &NotifyService{
		repo:     params.Repo,
		rbacSvc:  params.RBACSvc,
		queueSvc: params.QueueSvc,
		eventSvc: params.EventSvc,
		auditSvc: params.AuditSvc,
		logger:   params.Logger,
	}
}

func (s *NotifyService) CreateTopic(ctx context.Context, name string) (*domain.Topic, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionNotifyCreate, "*"); err != nil {
		return nil, err
	}

	existing, _ := s.repo.GetTopicByName(ctx, name, userID)
	if existing != nil {
		return nil, fmt.Errorf("topic with name %s already exists", name)
	}

	id := uuid.New()
	topic := &domain.Topic{
		ID:        id,
		UserID:    userID,
		Name:      name,
		ARN:       fmt.Sprintf("arn:thecloud:notify:local:%s:topic/%s", userID, name),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.CreateTopic(ctx, topic); err != nil {
		return nil, err
	}

	_ = s.eventSvc.RecordEvent(ctx, "TOPIC_CREATED", topic.ID.String(), "TOPIC", nil)

	_ = s.auditSvc.Log(ctx, topic.UserID, "notify.topic_create", "topic", topic.ID.String(), map[string]interface{}{
		"name": topic.Name,
	})

	return topic, nil
}

func (s *NotifyService) ListTopics(ctx context.Context) ([]*domain.Topic, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionNotifyRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListTopics(ctx, userID)
}

func (s *NotifyService) DeleteTopic(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionNotifyDelete, id.String()); err != nil {
		return err
	}

	topic, err := s.repo.GetTopicByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if topic == nil {
		return fmt.Errorf("topic not found")
	}

	if err := s.repo.DeleteTopic(ctx, id); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "TOPIC_DELETED", id.String(), "TOPIC", nil)

	_ = s.auditSvc.Log(ctx, topic.UserID, "notify.topic_delete", "topic", topic.ID.String(), map[string]interface{}{
		"name": topic.Name,
	})

	return nil
}

func (s *NotifyService) Subscribe(ctx context.Context, topicID uuid.UUID, protocol domain.SubscriptionProtocol, endpoint string) (*domain.Subscription, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionNotifyWrite, topicID.String()); err != nil {
		return nil, err
	}

	// Verify topic exists and belongs to user
	topic, err := s.repo.GetTopicByID(ctx, topicID, userID)
	if err != nil {
		return nil, err
	}

	sub := &domain.Subscription{
		ID:        uuid.New(),
		UserID:    userID,
		TopicID:   topic.ID,
		Protocol:  protocol,
		Endpoint:  endpoint,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.CreateSubscription(ctx, sub); err != nil {
		return nil, err
	}

	_ = s.eventSvc.RecordEvent(ctx, "SUBSCRIPTION_CREATED", sub.ID.String(), "SUBSCRIPTION", map[string]interface{}{"topic_id": topicID})

	_ = s.auditSvc.Log(ctx, sub.UserID, "notify.subscribe", "subscription", sub.ID.String(), map[string]interface{}{
		"topic_id": topicID.String(),
		"protocol": protocol,
		"endpoint": endpoint,
	})

	return sub, nil
}

func (s *NotifyService) ListSubscriptions(ctx context.Context, topicID uuid.UUID) ([]*domain.Subscription, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionNotifyRead, topicID.String()); err != nil {
		return nil, err
	}

	// Verify topic ownership
	_, err := s.repo.GetTopicByID(ctx, topicID, userID)
	if err != nil {
		return nil, err
	}

	return s.repo.ListSubscriptions(ctx, topicID)
}

func (s *NotifyService) Unsubscribe(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionNotifyDelete, id.String()); err != nil {
		return err
	}

	sub, err := s.repo.GetSubscriptionByID(ctx, id, userID)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteSubscription(ctx, sub.ID); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "SUBSCRIPTION_DELETED", id.String(), "SUBSCRIPTION", nil)

	_ = s.auditSvc.Log(ctx, sub.UserID, "notify.unsubscribe", "subscription", sub.ID.String(), map[string]interface{}{
		"topic_id": sub.TopicID.String(),
	})

	return nil
}

func (s *NotifyService) Publish(ctx context.Context, topicID uuid.UUID, body string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionNotifyWrite, topicID.String()); err != nil {
		return err
	}

	topic, err := s.repo.GetTopicByID(ctx, topicID, userID)
	if err != nil {
		return err
	}

	msg := &domain.NotifyMessage{
		ID:        uuid.New(),
		TopicID:   topic.ID,
		Body:      body,
		CreatedAt: time.Now(),
	}

	if err := s.repo.SaveMessage(ctx, msg); err != nil {
		return err
	}

	subs, err := s.repo.ListSubscriptions(ctx, topicID)
	if err != nil {
		return err
	}

	// Delivery logic
	for _, sub := range subs {
		// Create a background context for async delivery to avoid request cancellation
		// but keep it separate for each subscriber to avoid shared timeout issues
		go func(c context.Context, sub *domain.Subscription) {
			deliveryCtx, cancel := context.WithTimeout(c, 30*time.Second)
			defer cancel()

			// Carry over potential trace IDs or other metadata if needed
			// (Simplified for now, but avoids using request-scoped ctx)
			s.deliver(deliveryCtx, sub, body)
		}(ctx, sub)
	}

	if err := s.eventSvc.RecordEvent(ctx, "TOPIC_PUBLISHED", topic.ID.String(), "TOPIC", map[string]interface{}{"message_id": msg.ID}); err != nil {
		s.logger.Warn("failed to record topic publish event", "topic_id", topic.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, topic.UserID, "notify.publish", "topic", topic.ID.String(), map[string]interface{}{
		"message_id": msg.ID.String(),
	}); err != nil {
		s.logger.Warn("failed to log topic publish audit event", "topic_id", topic.ID, "error", err)
	}

	return nil
}

func (s *NotifyService) deliver(ctx context.Context, sub *domain.Subscription, body string) {
	switch sub.Protocol {
	case domain.ProtocolQueue:
		s.deliverToQueue(ctx, sub, body)
	case domain.ProtocolWebhook:
		s.deliverToWebhook(ctx, sub, body)
	}
}

func (s *NotifyService) deliverToQueue(ctx context.Context, sub *domain.Subscription, body string) {
	// Endpoint is Queue ARN or ID. Let's assume ID for simplicity or parse ARN.
	// For now let's assume endpoint is the Queue UUID string.
	qID, err := uuid.Parse(sub.Endpoint)
	if err != nil {
		s.logger.Warn("invalid queue ID in subscription", "endpoint", sub.Endpoint, "error", err)
		return
	}
	// We need to bypass user check or use sub.UserID context
	deliveryCtx := appcontext.WithUserID(ctx, sub.UserID)
	if _, err = s.queueSvc.SendMessage(deliveryCtx, qID, body); err != nil {
		s.logger.Warn("failed to deliver to queue", "queue_id", qID, "error", err)
	}
}

func (s *NotifyService) deliverToWebhook(ctx context.Context, sub *domain.Subscription, body string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewBufferString(body))
	if err != nil {
		s.logger.Warn("failed to build webhook request", "endpoint", sub.Endpoint, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		s.logger.Warn("failed to deliver to webhook", "endpoint", sub.Endpoint, "error", err)
		return
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		s.logger.Warn("webhook delivery failed",
			"endpoint", sub.Endpoint,
			"subscription_id", sub.ID,
			"status", resp.StatusCode)
		return
	}
}
