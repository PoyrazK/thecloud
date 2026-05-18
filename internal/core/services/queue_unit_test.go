package services_test

import (
	"context"
	apierrors "github.com/poyrazk/thecloud/internal/errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestQueueServiceUnit(t *testing.T) {
	mockRepo := new(MockQueueRepository)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewQueueService(mockRepo, rbacSvc, mockEventSvc, mockAuditSvc, slog.Default())

	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)

	t.Run("CreateQueue", func(t *testing.T) {
		mockRepo.On("GetByName", mock.Anything, "my-queue", mock.Anything).Return(nil, nil).Once()
		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "QUEUE_CREATED", mock.Anything, "QUEUE", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "queue.create", "queue", mock.Anything, mock.Anything).Return(nil).Once()

		q, err := svc.CreateQueue(ctx, "my-queue", nil)
		require.NoError(t, err)
		assert.NotNil(t, q)
		assert.Equal(t, "my-queue", q.Name)
		mockRepo.AssertExpectations(t)
	})

	t.Run("CreateQueue error types", func(t *testing.T) {
		t.Run("duplicate returns Conflict", func(t *testing.T) {
			mockRepo.On("GetByName", mock.Anything, "existing-queue", mock.Anything).Return(&domain.Queue{ID: uuid.New()}, nil).Once()

			_, err := svc.CreateQueue(ctx, "existing-queue", nil)
			require.Error(t, err)

			var appErr apierrors.Error
			if apierrors.As(err, &appErr) {
				assert.Equal(t, apierrors.Conflict, appErr.Type)
			} else {
				t.Fatalf("expected apierrors.Error, got %T", err)
			}
		})
	})

	t.Run("GetQueue error types", func(t *testing.T) {
		t.Run("not found returns NotFound", func(t *testing.T) {
			qID := uuid.New()
			mockRepo.On("GetByID", mock.Anything, qID, mock.Anything).Return(nil, nil).Once()

			_, err := svc.GetQueue(ctx, qID)
			require.Error(t, err)

			var appErr apierrors.Error
			if apierrors.As(err, &appErr) {
				assert.Equal(t, apierrors.NotFound, appErr.Type)
			} else {
				t.Fatalf("expected apierrors.Error, got %T", err)
			}
		})
	})

	t.Run("DeleteQueue error types", func(t *testing.T) {
		t.Run("not found returns NotFound", func(t *testing.T) {
			qID := uuid.New()
			mockRepo.On("GetByID", mock.Anything, qID, mock.Anything).Return(nil, nil).Once()

			err := svc.DeleteQueue(ctx, qID)
			require.Error(t, err)

			var appErr apierrors.Error
			if apierrors.As(err, &appErr) {
				assert.Equal(t, apierrors.NotFound, appErr.Type)
			} else {
				t.Fatalf("expected apierrors.Error, got %T", err)
			}
		})
	})

	t.Run("SendMessage error types", func(t *testing.T) {
		t.Run("queue not found returns NotFound", func(t *testing.T) {
			qID := uuid.New()
			mockRepo.On("GetByID", mock.Anything, qID, mock.Anything).Return(nil, nil).Once()

			_, err := svc.SendMessage(ctx, qID, "hello")
			require.Error(t, err)

			var appErr apierrors.Error
			if apierrors.As(err, &appErr) {
				assert.Equal(t, apierrors.NotFound, appErr.Type)
			} else {
				t.Fatalf("expected apierrors.Error, got %T", err)
			}
		})

		t.Run("message too large returns InvalidInput", func(t *testing.T) {
			qID := uuid.New()
			mockRepo.On("GetByID", mock.Anything, qID, mock.Anything).Return(&domain.Queue{ID: qID, MaxMessageSize: 5}, nil).Once()

			_, err := svc.SendMessage(ctx, qID, "this message is too long")
			require.Error(t, err)

			var appErr apierrors.Error
			if apierrors.As(err, &appErr) {
				assert.Equal(t, apierrors.InvalidInput, appErr.Type)
			} else {
				t.Fatalf("expected apierrors.Error, got %T", err)
			}
		})
	})

	t.Run("SendMessage", func(t *testing.T) {
		qID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, qID, mock.Anything).Return(&domain.Queue{ID: qID, MaxMessageSize: 100}, nil).Once()
		mockRepo.On("SendMessage", mock.Anything, qID, "hello").Return(&domain.Message{ID: uuid.New()}, nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "MESSAGE_SENT", mock.Anything, "MESSAGE", mock.Anything).Return(nil).Once()

		msg, err := svc.SendMessage(ctx, qID, "hello")
		require.NoError(t, err)
		assert.NotNil(t, msg)
		mockRepo.AssertExpectations(t)
	})

	t.Run("ReceiveMessages", func(t *testing.T) {
		qID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, qID, mock.Anything).Return(&domain.Queue{ID: qID, VisibilityTimeout: 30}, nil).Once()
		mockRepo.On("ReceiveMessages", mock.Anything, qID, 5, 30).Return([]*domain.Message{{ID: uuid.New()}}, nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "MESSAGE_RECEIVED", mock.Anything, "MESSAGE", mock.Anything).Return(nil).Once()

		msgs, err := svc.ReceiveMessages(ctx, qID, 5)
		require.NoError(t, err)
		assert.Len(t, msgs, 1)
	})
}
