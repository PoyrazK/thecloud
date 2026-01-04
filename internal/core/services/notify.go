package services

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type NotifyService struct {
	repo     ports.NotifyRepository
	queueSvc ports.QueueService
	eventSvc ports.EventService
}

func NewNotifyService(repo ports.NotifyRepository, queueSvc ports.QueueService, eventSvc ports.EventService) ports.NotifyService {
	return &NotifyService{
		repo:     repo,
		queueSvc: queueSvc,
		eventSvc: eventSvc,
	}
}

func (s *NotifyService) CreateTopic(ctx context.Context, name string) (*domain.Topic, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, fmt.Errorf("unauthorized")
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

	return topic, nil
}

func (s *NotifyService) ListTopics(ctx context.Context) ([]*domain.Topic, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, fmt.Errorf("unauthorized")
	}
	return s.repo.ListTopics(ctx, userID)
}

func (s *NotifyService) DeleteTopic(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return fmt.Errorf("unauthorized")
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
	return nil
}

func (s *NotifyService) Subscribe(ctx context.Context, topicID uuid.UUID, protocol domain.SubscriptionProtocol, endpoint string) (*domain.Subscription, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, fmt.Errorf("unauthorized")
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

	return sub, nil
}

func (s *NotifyService) ListSubscriptions(ctx context.Context, topicID uuid.UUID) ([]*domain.Subscription, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, fmt.Errorf("unauthorized")
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
	if userID == uuid.Nil {
		return fmt.Errorf("unauthorized")
	}

	sub, err := s.repo.GetSubscriptionByID(ctx, id, userID)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteSubscription(ctx, sub.ID); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "SUBSCRIPTION_DELETED", id.String(), "SUBSCRIPTION", nil)
	return nil
}

func (s *NotifyService) Publish(ctx context.Context, topicID uuid.UUID, body string) error {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return fmt.Errorf("unauthorized")
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
		go s.deliver(context.Background(), sub, body)
	}

	_ = s.eventSvc.RecordEvent(ctx, "TOPIC_PUBLISHED", topic.ID.String(), "TOPIC", map[string]interface{}{"message_id": msg.ID})

	return nil
}

func (s *NotifyService) deliver(ctx context.Context, sub *domain.Subscription, body string) {
	switch sub.Protocol {
	case domain.ProtocolQueue:
		// Endpoint is Queue ARN or ID. Let's assume ID for simplicity or parse ARN.
		// For now let's assume endpoint is the Queue UUID string.
		qID, err := uuid.Parse(sub.Endpoint)
		if err != nil {
			fmt.Printf("Invalid queue ID in subscription: %v\n", err)
			return
		}
		// We need to bypass user check or use sub.UserID context
		deliveryCtx := appcontext.WithUserID(ctx, sub.UserID)
		_, err = s.queueSvc.SendMessage(deliveryCtx, qID, body)
		if err != nil {
			fmt.Printf("Failed to deliver to queue %s: %v\n", qID, err)
		}

	case domain.ProtocolWebhook:
		req, _ := http.NewRequestWithContext(ctx, "POST", sub.Endpoint, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("Failed to deliver to webhook %s: %v\n", sub.Endpoint, err)
			return
		}
		_ = resp.Body.Close()
	}
}
