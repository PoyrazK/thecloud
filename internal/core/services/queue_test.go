package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestQueueService_CreateQueue(t *testing.T) {
	queueRepo := new(MockQueueRepo)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)

	svc := services.NewQueueService(queueRepo, eventSvc, auditSvc)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)

	t.Run("Success", func(t *testing.T) {
		queueName := "test-queue"

		queueRepo.On("GetByName", ctx, queueName, userID).Return(nil, assert.AnError).Once()
		queueRepo.On("Create", ctx, mock.MatchedBy(func(q *domain.Queue) bool {
			return q.Name == queueName && q.UserID == userID && q.Status == domain.QueueStatusActive
		})).Return(nil).Once()
		eventSvc.On("RecordEvent", ctx, "QUEUE_CREATED", mock.Anything, "QUEUE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", ctx, userID, "queue.create", "queue", mock.Anything, mock.Anything).Return(nil).Once()

		queue, err := svc.CreateQueue(ctx, queueName, nil)
		require.NoError(t, err)
		assert.Equal(t, queueName, queue.Name)
		assert.Equal(t, userID, queue.UserID)
		assert.Equal(t, domain.QueueStatusActive, queue.Status)
		assert.Equal(t, 30, queue.VisibilityTimeout)

		queueRepo.AssertExpectations(t)
		eventSvc.AssertExpectations(t)
		auditSvc.AssertExpectations(t)
	})

	t.Run("WithOptions", func(t *testing.T) {
		queueName := "queue-with-opts"
		visTimeout := 60
		retention := 7
		maxSize := 512000

		opts := &ports.CreateQueueOptions{
			VisibilityTimeout: &visTimeout,
			RetentionDays:     &retention,
			MaxMessageSize:    &maxSize,
		}

		queueRepo.On("GetByName", ctx, queueName, userID).Return(nil, assert.AnError).Once()
		queueRepo.On("Create", ctx, mock.MatchedBy(func(q *domain.Queue) bool {
			return q.VisibilityTimeout == 60 && q.RetentionDays == 7 && q.MaxMessageSize == 512000
		})).Return(nil).Once()
		eventSvc.On("RecordEvent", ctx, "QUEUE_CREATED", mock.Anything, "QUEUE", mock.Anything).Return(nil).Once()
		auditSvc.On("Log", ctx, userID, "queue.create", "queue", mock.Anything, mock.Anything).Return(nil).Once()

		queue, err := svc.CreateQueue(ctx, queueName, opts)
		require.NoError(t, err)
		assert.Equal(t, 60, queue.VisibilityTimeout)
		assert.Equal(t, 7, queue.RetentionDays)

		queueRepo.AssertExpectations(t)
	})

	t.Run("QueueAlreadyExists", func(t *testing.T) {
		existingQueue := &domain.Queue{ID: uuid.New(), Name: "existing-queue"}
		queueRepo.On("GetByName", ctx, "existing-queue", userID).Return(existingQueue, nil).Once()

		_, err := svc.CreateQueue(ctx, "existing-queue", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")

		queueRepo.AssertExpectations(t)
	})
}

func TestQueueService_SendMessage(t *testing.T) {
	queueRepo := new(MockQueueRepo)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)

	svc := services.NewQueueService(queueRepo, eventSvc, auditSvc)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)
	queueID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		queue := &domain.Queue{ID: queueID, UserID: userID, Name: "test-queue", Status: domain.QueueStatusActive}
		messageBody := "test message"

		queueRepo.On("GetByID", ctx, queueID, userID).Return(queue, nil).Once()
		queueRepo.On("EnqueueMessage", ctx, mock.MatchedBy(func(m *domain.Message) bool {
			return m.QueueID == queueID && m.Body == messageBody
		})).Return(nil).Once()
		eventSvc.On("RecordEvent", ctx, "MESSAGE_SENT", mock.Anything, "MESSAGE", mock.Anything).Return(nil).Once()

		msg, err := svc.SendMessage(ctx, queueID, messageBody)
		require.NoError(t, err)
		assert.Equal(t, queueID, msg.QueueID)
		assert.Equal(t, messageBody, msg.Body)

		queueRepo.AssertExpectations(t)
		eventSvc.AssertExpectations(t)
	})
}

func TestQueueService_ReceiveMessages(t *testing.T) {
	queueRepo := new(MockQueueRepo)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)

	svc := services.NewQueueService(queueRepo, eventSvc, auditSvc)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)
	queueID := uuid.New()

	queue := &domain.Queue{ID: queueID, UserID: userID, Name: "test-queue", VisibilityTimeout: 30}
	messages := []*domain.Message{
		{ID: uuid.New(), QueueID: queueID, Body: "message 1"},
		{ID: uuid.New(), QueueID: queueID, Body: "message 2"},
	}

	queueRepo.On("GetByID", ctx, queueID, userID).Return(queue, nil).Once()
	queueRepo.On("ReceiveMessages", ctx, queueID, 10, 30).Return(messages, nil).Once()
	eventSvc.On("RecordEvent", ctx, "MESSAGE_RECEIVED", mock.Anything, "MESSAGE", mock.Anything).Return(nil).Times(2)

	result, err := svc.ReceiveMessages(ctx, queueID, 10)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	queueRepo.AssertExpectations(t)
}

func TestQueueService_DeleteMessage(t *testing.T) {
	queueRepo := new(MockQueueRepo)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)

	svc := services.NewQueueService(queueRepo, eventSvc, auditSvc)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)
	queueID := uuid.New()
	receiptHandle := "test-receipt-handle"

	queue := &domain.Queue{ID: queueID, UserID: userID}

	queueRepo.On("GetByID", ctx, queueID, userID).Return(queue, nil).Once()
	queueRepo.On("DeleteMessage", ctx, receiptHandle).Return(nil).Once()
	eventSvc.On("RecordEvent", ctx, "MESSAGE_DELETED", receiptHandle, "MESSAGE", mock.Anything).Return(nil).Once()

	err := svc.DeleteMessage(ctx, queueID, receiptHandle)
	require.NoError(t, err)

	queueRepo.AssertExpectations(t)
	eventSvc.AssertExpectations(t)
}

func TestQueueService_DeleteQueue(t *testing.T) {
	queueRepo := new(MockQueueRepo)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)

	svc := services.NewQueueService(queueRepo, eventSvc, auditSvc)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)
	queueID := uuid.New()

	queue := &domain.Queue{ID: queueID, UserID: userID, Name: "test-queue"}

	queueRepo.On("GetByID", ctx, queueID, userID).Return(queue, nil).Once()
	queueRepo.On("Delete", ctx, queueID).Return(nil).Once()
	eventSvc.On("RecordEvent", ctx, "QUEUE_DELETED", queueID.String(), "QUEUE", mock.Anything).Return(nil).Once()
	auditSvc.On("Log", ctx, userID, "queue.delete", "queue", queueID.String(), mock.Anything).Return(nil).Once()

	err := svc.DeleteQueue(ctx, queueID)
	require.NoError(t, err)

	queueRepo.AssertExpectations(t)
	eventSvc.AssertExpectations(t)
	auditSvc.AssertExpectations(t)
}
