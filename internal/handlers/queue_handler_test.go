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

const (
	queuesPath    = "/queues"
	testQueueName = "q-1"
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}

func (m *mockQueueService) DeleteMessage(ctx context.Context, queueID uuid.UUID, receiptHandle string) error {
	args := m.Called(ctx, queueID, receiptHandle)
	return args.Error(0)
}

func (m *mockQueueService) PurgeQueue(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func setupQueueHandlerTest(t *testing.T) (*mockQueueService, *QueueHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockQueueService)
	handler := NewQueueHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestQueueHandlerCreate(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(queuesPath, handler.Create)

	q := &domain.Queue{ID: uuid.New(), Name: testQueueName}
	// Expect opts to have nils for optional fields as per request
	svc.On("CreateQueue", mock.Anything, testQueueName, mock.MatchedBy(func(opts *ports.CreateQueueOptions) bool {
		return opts.VisibilityTimeout == nil && opts.RetentionDays == nil && opts.MaxMessageSize == nil
	})).Return(q, nil)

	body, err := json.Marshal(map[string]interface{}{"name": testQueueName})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", queuesPath, bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestQueueHandlerList(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(queuesPath, handler.List)

	queues := []*domain.Queue{{ID: uuid.New(), Name: testQueueName}}
	svc.On("ListQueues", mock.Anything).Return(queues, nil)

	req, err := http.NewRequest(http.MethodGet, queuesPath, nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQueueHandlerGet(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(queuesPath+"/:id", handler.Get)

	id := uuid.New()
	q := &domain.Queue{ID: id, Name: testQueueName}
	svc.On("GetQueue", mock.Anything, id).Return(q, nil)

	req, err := http.NewRequest(http.MethodGet, queuesPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQueueHandlerDelete(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(queuesPath+"/:id", handler.Delete)

	id := uuid.New()
	svc.On("DeleteQueue", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, queuesPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestQueueHandlerSendMessage(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(queuesPath+"/:id/messages", handler.SendMessage)

	id := uuid.New()
	msg := &domain.Message{ID: uuid.New(), Body: "hello"}
	svc.On("SendMessage", mock.Anything, id, "hello").Return(msg, nil)

	body, err := json.Marshal(map[string]interface{}{"body": "hello"})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", queuesPath+"/"+id.String()+"/messages", bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestQueueHandlerReceiveMessages(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(queuesPath+"/:id/messages", handler.ReceiveMessages)

	id := uuid.New()
	msgs := []*domain.Message{{ID: uuid.New(), Body: "hello"}}
	svc.On("ReceiveMessages", mock.Anything, id, 10).Return(msgs, nil)

	req, err := http.NewRequest(http.MethodGet, queuesPath+"/"+id.String()+"/messages?max_messages=10", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQueueHandlerDeleteMessage(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(queuesPath+"/:id/messages/:handle", handler.DeleteMessage)

	id := uuid.New()
	handle := "handle123"
	svc.On("DeleteMessage", mock.Anything, id, handle).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, queuesPath+"/"+id.String()+"/messages/"+handle, nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestQueueHandlerPurge(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(queuesPath+"/:id/purge", handler.Purge)

	id := uuid.New()
	svc.On("PurgeQueue", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodPost, queuesPath+"/"+id.String()+"/purge", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestQueueHandlerReceiveMessages_Defaults(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	r.GET(queuesPath+"/:id/messages", handler.ReceiveMessages)

	id := uuid.New()
	svc.On("ReceiveMessages", mock.Anything, id, 1).Return([]*domain.Message{}, nil)

	req, _ := http.NewRequest(http.MethodGet, queuesPath+"/"+id.String()+"/messages?max_messages=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQueueHandler_Errors(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	r.POST(queuesPath, handler.Create)
	r.GET(queuesPath+"/:id", handler.Get)
	r.DELETE(queuesPath+"/:id", handler.Delete)
	r.POST(queuesPath+"/:id/messages", handler.SendMessage)
	r.GET(queuesPath+"/:id/messages", handler.ReceiveMessages)
	r.DELETE(queuesPath+"/:id/messages/:handle", handler.DeleteMessage)
	r.POST(queuesPath+"/:id/purge", handler.Purge)

	id := uuid.New()

	t.Run("CreateError", func(t *testing.T) {
		svc.On("CreateQueue", mock.Anything, "err-q", mock.Anything).Return(nil, assert.AnError)
		body, _ := json.Marshal(map[string]string{"name": "err-q"})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", queuesPath, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetError", func(t *testing.T) {
		svc.On("GetQueue", mock.Anything, id).Return(nil, assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", queuesPath+"/"+id.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("DeleteError", func(t *testing.T) {
		svc.On("DeleteQueue", mock.Anything, id).Return(assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", queuesPath+"/"+id.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("SendMessageError", func(t *testing.T) {
		svc.On("SendMessage", mock.Anything, id, "body").Return(nil, assert.AnError)
		body, _ := json.Marshal(map[string]string{"body": "body"})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", queuesPath+"/"+id.String()+"/messages", bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ReceiveMessagesError", func(t *testing.T) {
		svc.On("ReceiveMessages", mock.Anything, id, 1).Return(nil, assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", queuesPath+"/"+id.String()+"/messages", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("DeleteMessageError", func(t *testing.T) {
		svc.On("DeleteMessage", mock.Anything, id, "handle").Return(assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", queuesPath+"/"+id.String()+"/messages/handle", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("PurgeError", func(t *testing.T) {
		svc.On("PurgeQueue", mock.Anything, id).Return(assert.AnError)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", queuesPath+"/"+id.String()+"/purge", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestQueueHandler_InvalidUUIDs(t *testing.T) {
	_, handler, r := setupQueueHandlerTest(t)
	r.GET(queuesPath+"/:id", handler.Get)
	r.DELETE(queuesPath+"/:id", handler.Delete)
	r.POST(queuesPath+"/:id/messages", handler.SendMessage)
	r.GET(queuesPath+"/:id/messages", handler.ReceiveMessages)
	r.DELETE(queuesPath+"/:id/messages/:handle", handler.DeleteMessage)
	r.POST(queuesPath+"/:id/purge", handler.Purge)

	tests := []struct {
		method string
		path   string
	}{
		{"GET", queuesPath + "/invalid"},
		{"DELETE", queuesPath + "/invalid"},
		{"POST", queuesPath + "/invalid/messages"},
		{"GET", queuesPath + "/invalid/messages"},
		{"POST", queuesPath + "/invalid/purge"},
		{"DELETE", queuesPath + "/invalid/messages/h"},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(tt.method, tt.path, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	}
}

func TestQueueHandlerDeleteMessage_NoHandle(t *testing.T) {
	_, handler, r := setupQueueHandlerTest(t)
	r.DELETE(queuesPath+"/:id/messages", handler.DeleteMessage) // Note: handle param is empty here

	id := uuid.New()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, queuesPath+"/"+id.String()+"/messages", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestQueueHandlerList_Error(t *testing.T) {
	svc, handler, r := setupQueueHandlerTest(t)
	r.GET(queuesPath, handler.List)

	svc.On("ListQueues", mock.Anything).Return(nil, assert.AnError)

	req, _ := http.NewRequest(http.MethodGet, queuesPath, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestQueueHandler_InvalidJSON(t *testing.T) {
	_, handler, r := setupQueueHandlerTest(t)
	r.POST(queuesPath, handler.Create)
	r.POST(queuesPath+"/:id/messages", handler.SendMessage)

	id := uuid.New()
	
	t.Run("Create", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", queuesPath, bytes.NewBufferString("{invalid}"))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("SendMessage", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", queuesPath+"/"+id.String()+"/messages", bytes.NewBufferString("{invalid}"))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
