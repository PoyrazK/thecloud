package httphandlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockEventService struct {
	mock.Mock
}

func (m *mockEventService) RecordEvent(ctx context.Context, action, resourceID, resourceType string, metadata map[string]interface{}) error {
	args := m.Called(ctx, action, resourceID, resourceType, metadata)
	return args.Error(0)
}

func (m *mockEventService) ListEvents(ctx context.Context, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Event), args.Error(1)
}

func TestEventHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockEventService)
	handler := NewEventHandler(svc)

	r := gin.New()
	r.GET("/events", handler.List)

	events := []*domain.Event{{ID: uuid.New(), Action: "test"}}
	svc.On("ListEvents", mock.Anything, 50).Return(events, nil)

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
