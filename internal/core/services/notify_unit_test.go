package services_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testTopicName = "my-topic"

func TestNotifyServiceUnit(t *testing.T) {
	t.Run("CRUD", testNotifyServiceUnitCRUD)
	t.Run("RBACErrors", testNotifyServiceUnitRbacErrors)
	t.Run("RepoErrors", testNotifyServiceUnitRepoErrors)
	t.Run("PublishErrors", testNotifyServiceUnitPublishErrors)
}

func testNotifyServiceUnitCRUD(t *testing.T) {
	mockRepo := new(MockNotifyRepo)
	mockQueueSvc := new(MockQueueService)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewNotifyService(services.NotifyServiceParams{
		Repo:     mockRepo,
		RBACSvc:  rbacSvc,
		QueueSvc: mockQueueSvc,
		EventSvc: mockEventSvc,
		AuditSvc: mockAuditSvc,
		Logger:   slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	t.Run("CreateTopic", func(t *testing.T) {
		mockRepo.On("GetTopicByName", mock.Anything, testTopicName, userID).Return(nil, nil).Once()
		mockRepo.On("CreateTopic", mock.Anything, mock.Anything).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "TOPIC_CREATED", mock.Anything, "TOPIC", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "notify.topic_create", "topic", mock.Anything, mock.Anything).Return(nil).Once()

		topic, err := svc.CreateTopic(ctx, testTopicName)
		require.NoError(t, err)
		assert.NotNil(t, topic)
		assert.Equal(t, testTopicName, topic.Name)
	})

	t.Run("ListTopics", func(t *testing.T) {
		mockRepo.On("ListTopics", mock.Anything, userID).Return([]*domain.Topic{{ID: uuid.New(), Name: "topic1"}}, nil).Once()

		topics, err := svc.ListTopics(ctx)
		require.NoError(t, err)
		assert.Len(t, topics, 1)
	})

	t.Run("DeleteTopic", func(t *testing.T) {
		topicID := uuid.New()
		topic := &domain.Topic{ID: topicID, UserID: userID, Name: "to-delete"}
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(topic, nil).Once()
		mockRepo.On("DeleteTopic", mock.Anything, topicID).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "TOPIC_DELETED", mock.Anything, "TOPIC", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "notify.topic_delete", "topic", mock.Anything, mock.Anything).Return(nil).Once()

		err := svc.DeleteTopic(ctx, topicID)
		require.NoError(t, err)
	})

	t.Run("Subscribe", func(t *testing.T) {
		topicID := uuid.New()
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(&domain.Topic{ID: topicID}, nil).Once()
		mockRepo.On("CreateSubscription", mock.Anything, mock.Anything).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "SUBSCRIPTION_CREATED", mock.Anything, "SUBSCRIPTION", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "notify.subscribe", "subscription", mock.Anything, mock.Anything).Return(nil).Once()

		sub, err := svc.Subscribe(ctx, topicID, domain.ProtocolWebhook, "https://example.com/hook")
		require.NoError(t, err)
		assert.NotNil(t, sub)
	})

	t.Run("ListSubscriptions", func(t *testing.T) {
		topicID := uuid.New()
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(&domain.Topic{ID: topicID}, nil).Once()
		mockRepo.On("ListSubscriptions", mock.Anything, topicID).Return([]*domain.Subscription{{ID: uuid.New()}}, nil).Once()

		subs, err := svc.ListSubscriptions(ctx, topicID)
		require.NoError(t, err)
		assert.Len(t, subs, 1)
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		subID := uuid.New()
		topicID := uuid.New()
		sub := &domain.Subscription{ID: subID, UserID: userID, TopicID: topicID}
		mockRepo.On("GetSubscriptionByID", mock.Anything, subID, userID).Return(sub, nil).Once()
		mockRepo.On("DeleteSubscription", mock.Anything, subID).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "SUBSCRIPTION_DELETED", mock.Anything, "SUBSCRIPTION", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "notify.unsubscribe", "subscription", mock.Anything, mock.Anything).Return(nil).Once()

		err := svc.Unsubscribe(ctx, subID)
		require.NoError(t, err)
	})

	t.Run("Publish", func(t *testing.T) {
		done := make(chan struct{})
		topicID := uuid.New()
		topic := &domain.Topic{ID: topicID, UserID: userID}
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(topic, nil).Once()
		mockRepo.On("SaveMessage", mock.Anything, mock.Anything).Return(nil).Once()
		mockRepo.On("ListSubscriptions", mock.Anything, topicID).Return([]*domain.Subscription{}, nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "TOPIC_PUBLISHED", mock.Anything, "TOPIC", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "notify.publish", "topic", topicID.String(), mock.Anything).Return(nil).Run(func(mock.Arguments) { close(done) }).Once()

		err := svc.Publish(ctx, topicID, "hello")
		require.NoError(t, err)
		<-done
	})
}

func testNotifyServiceUnitRbacErrors(t *testing.T) {
	mockRepo := new(MockNotifyRepo)
	mockQueueSvc := new(MockQueueService)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)

	svc := services.NewNotifyService(services.NotifyServiceParams{
		Repo:     mockRepo,
		RBACSvc:  rbacSvc,
		QueueSvc: mockQueueSvc,
		EventSvc: mockEventSvc,
		AuditSvc: mockAuditSvc,
		Logger:   slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	type rbacCase struct {
		name       string
		permission domain.Permission
		resourceID string
		invoke     func(id string) error
	}

	cases := []rbacCase{
		{
			name:       "CreateTopic_Unauthorized",
			permission: domain.PermissionNotifyCreate,
			resourceID: "*",
			invoke: func(id string) error {
				_, err := svc.CreateTopic(ctx, "my-topic")
				return err
			},
		},
		{
			name:       "ListTopics_Unauthorized",
			permission: domain.PermissionNotifyRead,
			resourceID: "*",
			invoke: func(id string) error {
				_, err := svc.ListTopics(ctx)
				return err
			},
		},
		{
			name:       "DeleteTopic_Unauthorized",
			permission: domain.PermissionNotifyDelete,
			resourceID: uuid.New().String(),
			invoke: func(id string) error {
				return svc.DeleteTopic(ctx, uuid.MustParse(id))
			},
		},
		{
			name:       "Subscribe_Unauthorized",
			permission: domain.PermissionNotifyWrite,
			resourceID: uuid.New().String(),
			invoke: func(id string) error {
				_, err := svc.Subscribe(ctx, uuid.MustParse(id), domain.ProtocolWebhook, "https://example.com/hook")
				return err
			},
		},
		{
			name:       "ListSubscriptions_Unauthorized",
			permission: domain.PermissionNotifyRead,
			resourceID: uuid.New().String(),
			invoke: func(id string) error {
				_, err := svc.ListSubscriptions(ctx, uuid.MustParse(id))
				return err
			},
		},
		{
			name:       "Unsubscribe_Unauthorized",
			permission: domain.PermissionNotifyDelete,
			resourceID: uuid.New().String(),
			invoke: func(id string) error {
				return svc.Unsubscribe(ctx, uuid.MustParse(id))
			},
		},
		{
			name:       "Publish_Unauthorized",
			permission: domain.PermissionNotifyWrite,
			resourceID: uuid.New().String(),
			invoke: func(id string) error {
				return svc.Publish(ctx, uuid.MustParse(id), "hello")
			},
		},
	}

	authErr := errors.New(errors.Forbidden, "permission denied")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rbacSvc.On("Authorize", mock.Anything, userID, tenantID, c.permission, c.resourceID).Return(authErr).Once()
			err := c.invoke(c.resourceID)
			require.Error(t, err)
			assert.True(t, errors.Is(err, errors.Forbidden))
		})
	}
}

func testNotifyServiceUnitRepoErrors(t *testing.T) {
	mockRepo := new(MockNotifyRepo)
	mockQueueSvc := new(MockQueueService)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewNotifyService(services.NotifyServiceParams{
		Repo:     mockRepo,
		RBACSvc:  rbacSvc,
		QueueSvc: mockQueueSvc,
		EventSvc: mockEventSvc,
		AuditSvc: mockAuditSvc,
		Logger:   slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	t.Run("CreateTopic_DuplicateName", func(t *testing.T) {
		existing := &domain.Topic{ID: uuid.New(), Name: testTopicName}
		mockRepo.On("GetTopicByName", mock.Anything, testTopicName, userID).Return(existing, nil).Once()

		_, err := svc.CreateTopic(ctx, testTopicName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("CreateTopic_RepoError", func(t *testing.T) {
		mockRepo.On("GetTopicByName", mock.Anything, testTopicName, userID).Return(nil, nil).Once()
		mockRepo.On("CreateTopic", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Once()

		_, err := svc.CreateTopic(ctx, testTopicName)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("DeleteTopic_NotFound", func(t *testing.T) {
		topicID := uuid.New()
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(nil, nil).Once()

		err := svc.DeleteTopic(ctx, topicID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("DeleteTopic_RepoError", func(t *testing.T) {
		topicID := uuid.New()
		topic := &domain.Topic{ID: topicID, UserID: userID, Name: "test"}
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(topic, nil).Once()
		mockRepo.On("DeleteTopic", mock.Anything, topicID).Return(fmt.Errorf("db error")).Once()

		err := svc.DeleteTopic(ctx, topicID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("Subscribe_TopicNotFound", func(t *testing.T) {
		topicID := uuid.New()
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		_, err := svc.Subscribe(ctx, topicID, domain.ProtocolWebhook, "https://example.com/hook")
		require.Error(t, err)
	})

	t.Run("Subscribe_RepoError", func(t *testing.T) {
		topicID := uuid.New()
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(&domain.Topic{ID: topicID}, nil).Once()
		mockRepo.On("CreateSubscription", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Once()

		_, err := svc.Subscribe(ctx, topicID, domain.ProtocolWebhook, "https://example.com/hook")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("Unsubscribe_NotFound", func(t *testing.T) {
		subID := uuid.New()
		mockRepo.On("GetSubscriptionByID", mock.Anything, subID, userID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		err := svc.Unsubscribe(ctx, subID)
		require.Error(t, err)
	})

	t.Run("Unsubscribe_RepoError", func(t *testing.T) {
		subID := uuid.New()
		topicID := uuid.New()
		sub := &domain.Subscription{ID: subID, UserID: userID, TopicID: topicID}
		mockRepo.On("GetSubscriptionByID", mock.Anything, subID, userID).Return(sub, nil).Once()
		mockRepo.On("DeleteSubscription", mock.Anything, subID).Return(fmt.Errorf("db error")).Once()

		err := svc.Unsubscribe(ctx, subID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("ListSubscriptions_TopicNotFound", func(t *testing.T) {
		topicID := uuid.New()
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		_, err := svc.ListSubscriptions(ctx, topicID)
		require.Error(t, err)
	})

	t.Run("Publish_TopicNotFound", func(t *testing.T) {
		topicID := uuid.New()
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		err := svc.Publish(ctx, topicID, "hello")
		require.Error(t, err)
	})

	t.Run("Publish_SaveMessageError", func(t *testing.T) {
		topicID := uuid.New()
		topic := &domain.Topic{ID: topicID, UserID: userID}
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(topic, nil).Once()
		mockRepo.On("SaveMessage", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Once()

		err := svc.Publish(ctx, topicID, "hello")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

func testNotifyServiceUnitPublishErrors(t *testing.T) {
	mockRepo := new(MockNotifyRepo)
	mockQueueSvc := new(MockQueueService)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewNotifyService(services.NotifyServiceParams{
		Repo:     mockRepo,
		RBACSvc:  rbacSvc,
		QueueSvc: mockQueueSvc,
		EventSvc: mockEventSvc,
		AuditSvc: mockAuditSvc,
		Logger:   slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	t.Run("Publish_WebhookDeliveryError", func(t *testing.T) {
		done := make(chan struct{})
		topicID := uuid.New()
		subID := uuid.New()
		topic := &domain.Topic{ID: topicID, UserID: userID}
		sub := &domain.Subscription{
			ID:       subID,
			UserID:   userID,
			TopicID:  topicID,
			Protocol: domain.ProtocolWebhook,
			Endpoint: "http://localhost:9999/nonexistent",
		}
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(topic, nil).Once()
		mockRepo.On("SaveMessage", mock.Anything, mock.Anything).Return(nil).Once()
		mockRepo.On("ListSubscriptions", mock.Anything, topicID).Return([]*domain.Subscription{sub}, nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "TOPIC_PUBLISHED", mock.Anything, "TOPIC", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "notify.publish", "topic", topicID.String(), mock.Anything).Return(nil).Run(func(mock.Arguments) { close(done) }).Once()

		err := svc.Publish(ctx, topicID, "hello")
		require.NoError(t, err)
		<-done
	})

	t.Run("Publish_WebhookNon2xxStatus", func(t *testing.T) {
		// Issue #338: webhook delivery must surface non-2xx HTTP responses.
		var (
			mu       sync.Mutex
			received bool
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			mu.Lock()
			received = true
			mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		var logBuf bytes.Buffer
		var logMu sync.Mutex
		capturingLogger := slog.New(slog.NewTextHandler(&lockedWriter{w: &logBuf, mu: &logMu}, &slog.HandlerOptions{Level: slog.LevelDebug}))

		mockRepo2 := new(MockNotifyRepo)
		mockQueueSvc2 := new(MockQueueService)
		mockEventSvc2 := new(MockEventService)
		mockAuditSvc2 := new(MockAuditService)
		rbacSvc2 := new(MockRBACService)
		rbacSvc2.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		svc2 := services.NewNotifyService(services.NotifyServiceParams{
			Repo:     mockRepo2,
			RBACSvc:  rbacSvc2,
			QueueSvc: mockQueueSvc2,
			EventSvc: mockEventSvc2,
			AuditSvc: mockAuditSvc2,
			Logger:   capturingLogger,
		})

		done := make(chan struct{})
		topicID := uuid.New()
		topic := &domain.Topic{ID: topicID, UserID: userID}
		sub := &domain.Subscription{
			ID:       uuid.New(),
			UserID:   userID,
			TopicID:  topicID,
			Protocol: domain.ProtocolWebhook,
			Endpoint: server.URL,
		}
		mockRepo2.On("GetTopicByID", mock.Anything, topicID, userID).Return(topic, nil).Once()
		mockRepo2.On("SaveMessage", mock.Anything, mock.Anything).Return(nil).Once()
		mockRepo2.On("ListSubscriptions", mock.Anything, topicID).Return([]*domain.Subscription{sub}, nil).Once()
		mockEventSvc2.On("RecordEvent", mock.Anything, "TOPIC_PUBLISHED", mock.Anything, "TOPIC", mock.Anything).Return(nil).Once()
		mockAuditSvc2.On("Log", mock.Anything, userID, "notify.publish", "topic", topicID.String(), mock.Anything).Return(nil).Run(func(mock.Arguments) { close(done) }).Once()

		err := svc2.Publish(ctx, topicID, "hello")
		require.NoError(t, err)
		<-done

		// Allow the async webhook goroutine a moment to fire and log.
		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return received
		}, 2*time.Second, 10*time.Millisecond, "webhook server never received request")

		require.Eventually(t, func() bool {
			logMu.Lock()
			defer logMu.Unlock()
			return strings.Contains(logBuf.String(), "webhook delivery failed") &&
				strings.Contains(logBuf.String(), "status=500")
		}, 2*time.Second, 10*time.Millisecond, "expected webhook delivery failure log with status=500")
	})

	t.Run("Publish_QueueInvalidUUID", func(t *testing.T) {
		done := make(chan struct{})
		topicID := uuid.New()
		subID := uuid.New()
		topic := &domain.Topic{ID: topicID, UserID: userID}
		sub := &domain.Subscription{
			ID:       subID,
			UserID:   userID,
			TopicID:  topicID,
			Protocol: domain.ProtocolQueue,
			Endpoint: "not-a-valid-uuid",
		}
		mockRepo.On("GetTopicByID", mock.Anything, topicID, userID).Return(topic, nil).Once()
		mockRepo.On("SaveMessage", mock.Anything, mock.Anything).Return(nil).Once()
		mockRepo.On("ListSubscriptions", mock.Anything, topicID).Return([]*domain.Subscription{sub}, nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "TOPIC_PUBLISHED", mock.Anything, "TOPIC", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "notify.publish", "topic", topicID.String(), mock.Anything).Return(nil).Run(func(mock.Arguments) { close(done) }).Once()

		err := svc.Publish(ctx, topicID, "hello")
		require.NoError(t, err)
		<-done
	})
}

type lockedWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
