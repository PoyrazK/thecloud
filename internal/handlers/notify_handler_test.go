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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockNotifyService struct {
	mock.Mock
}

func (m *mockNotifyService) CreateTopic(ctx context.Context, name string) (*domain.Topic, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Topic), args.Error(1)
}

func (m *mockNotifyService) ListTopics(ctx context.Context) ([]*domain.Topic, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Topic), args.Error(1)
}

func (m *mockNotifyService) DeleteTopic(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockNotifyService) Subscribe(ctx context.Context, topicID uuid.UUID, protocol domain.SubscriptionProtocol, endpoint string) (*domain.Subscription, error) {
	args := m.Called(ctx, topicID, protocol, endpoint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}

func (m *mockNotifyService) ListSubscriptions(ctx context.Context, topicID uuid.UUID) ([]*domain.Subscription, error) {
	args := m.Called(ctx, topicID)
	return args.Get(0).([]*domain.Subscription), args.Error(1)
}

func (m *mockNotifyService) Unsubscribe(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockNotifyService) Publish(ctx context.Context, topicID uuid.UUID, body string) error {
	args := m.Called(ctx, topicID, body)
	return args.Error(0)
}

func TestNotifyHandler_CreateTopic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockNotifyService)
	handler := NewNotifyHandler(svc)

	r := gin.New()
	r.POST("/notify/topics", handler.CreateTopic)

	topic := &domain.Topic{ID: uuid.New(), Name: "topic-1"}
	svc.On("CreateTopic", mock.Anything, "topic-1").Return(topic, nil)

	body, _ := json.Marshal(map[string]interface{}{"name": "topic-1"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/notify/topics", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestNotifyHandler_ListTopics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockNotifyService)
	handler := NewNotifyHandler(svc)

	r := gin.New()
	r.GET("/notify/topics", handler.ListTopics)

	topics := []*domain.Topic{{ID: uuid.New(), Name: "topic-1"}}
	svc.On("ListTopics", mock.Anything).Return(topics, nil)

	req := httptest.NewRequest(http.MethodGet, "/notify/topics", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNotifyHandler_DeleteTopic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockNotifyService)
	handler := NewNotifyHandler(svc)

	r := gin.New()
	r.DELETE("/notify/topics/:id", handler.DeleteTopic)

	id := uuid.New()
	svc.On("DeleteTopic", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/notify/topics/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNotifyHandler_Subscribe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockNotifyService)
	handler := NewNotifyHandler(svc)

	r := gin.New()
	r.POST("/notify/topics/:id/subscriptions", handler.Subscribe)

	id := uuid.New()
	sub := &domain.Subscription{ID: uuid.New(), TopicID: id, Endpoint: "http://example.com"}
	svc.On("Subscribe", mock.Anything, id, domain.SubscriptionProtocol("http"), "http://example.com").Return(sub, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"protocol": "http",
		"endpoint": "http://example.com",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/notify/topics/"+id.String()+"/subscriptions", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestNotifyHandler_ListSubscriptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockNotifyService)
	handler := NewNotifyHandler(svc)

	r := gin.New()
	r.GET("/notify/topics/:id/subscriptions", handler.ListSubscriptions)

	id := uuid.New()
	subs := []*domain.Subscription{{ID: uuid.New(), TopicID: id}}
	svc.On("ListSubscriptions", mock.Anything, id).Return(subs, nil)

	req := httptest.NewRequest(http.MethodGet, "/notify/topics/"+id.String()+"/subscriptions", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNotifyHandler_Unsubscribe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockNotifyService)
	handler := NewNotifyHandler(svc)

	r := gin.New()
	r.DELETE("/notify/subscriptions/:id", handler.Unsubscribe)

	id := uuid.New()
	svc.On("Unsubscribe", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/notify/subscriptions/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNotifyHandler_Publish(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockNotifyService)
	handler := NewNotifyHandler(svc)

	r := gin.New()
	r.POST("/notify/topics/:id/publish", handler.Publish)

	id := uuid.New()
	svc.On("Publish", mock.Anything, id, "hello").Return(nil)

	body, _ := json.Marshal(map[string]interface{}{"message": "hello"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/notify/topics/"+id.String()+"/publish", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
