package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockQueueService struct {
	mock.Mock
}

func (m *mockQueueService) CreateQueue(ctx context.Context, name string, opts *ports.CreateQueueOptions) (*domain.Queue, error) {
	args := m.Called(ctx, name, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Queue), args.Error(1)
}

func (m *mockQueueService) ListQueues(ctx context.Context) ([]*domain.Queue, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Queue), args.Error(1)
}

func (m *mockQueueService) GetQueue(ctx context.Context, id uuid.UUID) (*domain.Queue, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Queue), args.Error(1)
}

func (m *mockQueueService) DeleteQueue(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockQueueService) SendMessage(ctx context.Context, queueID uuid.UUID, body string) (*domain.Message, error) {
	args := m.Called(ctx, queueID, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}

func (m *mockQueueService) ReceiveMessages(ctx context.Context, queueID uuid.UUID, max int) ([]*domain.Message, error) {
	args := m.Called(ctx, queueID, max)
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *mockQueueService) DeleteMessage(ctx context.Context, queueID uuid.UUID, receiptHandle string) error {
	args := m.Called(ctx, queueID, receiptHandle)
	return args.Error(0)
}

func (m *mockQueueService) PurgeQueue(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestQueueHandler_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockQueueService)
	handler := NewQueueHandler(svc)

	r := gin.New()
	r.POST("/queues", handler.Create)

	q := &domain.Queue{ID: uuid.New(), Name: "q-1"}
	// Expect opts to have nils for optional fields as per request
	svc.On("CreateQueue", mock.Anything, "q-1", mock.MatchedBy(func(opts *ports.CreateQueueOptions) bool {
		return opts.VisibilityTimeout == nil && opts.RetentionDays == nil && opts.MaxMessageSize == nil
	})).Return(q, nil)

	body, _ := json.Marshal(map[string]interface{}{"name": "q-1"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/queues", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestQueueHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockQueueService)
	handler := NewQueueHandler(svc)

	r := gin.New()
	r.GET("/queues", handler.List)

	queues := []*domain.Queue{{ID: uuid.New(), Name: "q-1"}}
	svc.On("ListQueues", mock.Anything).Return(queues, nil)

	req := httptest.NewRequest(http.MethodGet, "/queues", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQueueHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockQueueService)
	handler := NewQueueHandler(svc)

	r := gin.New()
	r.GET("/queues/:id", handler.Get)

	id := uuid.New()
	q := &domain.Queue{ID: id, Name: "q-1"}
	svc.On("GetQueue", mock.Anything, id).Return(q, nil)

	req := httptest.NewRequest(http.MethodGet, "/queues/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQueueHandler_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockQueueService)
	handler := NewQueueHandler(svc)

	r := gin.New()
	r.DELETE("/queues/:id", handler.Delete)

	id := uuid.New()
	svc.On("DeleteQueue", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/queues/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestQueueHandler_SendMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockQueueService)
	handler := NewQueueHandler(svc)

	r := gin.New()
	r.POST("/queues/:id/messages", handler.SendMessage)

	id := uuid.New()
	msg := &domain.Message{ID: uuid.New(), Body: "hello"}
	svc.On("SendMessage", mock.Anything, id, "hello").Return(msg, nil)

	body, _ := json.Marshal(map[string]interface{}{"body": "hello"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/queues/"+id.String()+"/messages", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestQueueHandler_ReceiveMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockQueueService)
	handler := NewQueueHandler(svc)

	r := gin.New()
	r.GET("/queues/:id/messages", handler.ReceiveMessages)

	id := uuid.New()
	msgs := []*domain.Message{{ID: uuid.New(), Body: "hello"}}
	svc.On("ReceiveMessages", mock.Anything, id, 10).Return(msgs, nil)

	req := httptest.NewRequest(http.MethodGet, "/queues/"+id.String()+"/messages?max_messages=10", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQueueHandler_DeleteMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockQueueService)
	handler := NewQueueHandler(svc)

	r := gin.New()
	r.DELETE("/queues/:id/messages/:handle", handler.DeleteMessage)

	id := uuid.New()
	handle := "handle123"
	svc.On("DeleteMessage", mock.Anything, id, handle).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/queues/"+id.String()+"/messages/"+handle, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestQueueHandler_Purge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockQueueService)
	handler := NewQueueHandler(svc)

	r := gin.New()
	r.POST("/queues/:id/purge", handler.Purge)

	id := uuid.New()
	svc.On("PurgeQueue", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/queues/"+id.String()+"/purge", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
