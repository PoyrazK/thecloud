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

type mockIdentityService struct {
	mock.Mock
}

func (m *mockIdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *mockIdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func TestIdentityHandler_CreateKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)
	handler := NewIdentityHandler(svc)

	r := gin.New()
	r.POST("/auth/keys", handler.CreateKey)

	key := &domain.APIKey{Key: "sk_test_123", Name: "Test Key"}
	svc.On("CreateKey", mock.Anything, uuid.Nil, "Test Key").Return(key, nil)

	body, _ := json.Marshal(map[string]string{"name": "Test Key"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/keys", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "sk_test_123")
}
