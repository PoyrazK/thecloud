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

const (
	authKeysPath = "/auth/keys"
	testKeyName  = "Test Key"
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
func (m *mockIdentityService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}
func (m *mockIdentityService) RevokeKey(ctx context.Context, userID, id uuid.UUID) error {
	args := m.Called(ctx, userID, id)
	return args.Error(0)
}
func (m *mockIdentityService) RotateKey(ctx context.Context, userID, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func setupIdentityHandlerTest(t *testing.T, userID uuid.UUID) (*mockIdentityService, *IdentityHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockIdentityService)
	handler := NewIdentityHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("user_id", userID) // Some tests might use user_id
		c.Next()
	})
	return svc, handler, r
}

func TestIdentityHandlerCreateKey(t *testing.T) {
	userID := uuid.New()
	svc, handler, r := setupIdentityHandlerTest(t, userID)
	defer svc.AssertExpectations(t)

	r.POST(authKeysPath, handler.CreateKey)

	key := &domain.APIKey{Key: "sk_test_123", Name: testKeyName}
	svc.On("CreateKey", mock.Anything, userID, testKeyName).Return(key, nil)

	body, err := json.Marshal(map[string]string{"name": testKeyName})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", authKeysPath, bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "sk_test_123")
}

func TestIdentityHandlerListKeys(t *testing.T) {
	userID := uuid.New()
	svc, handler, r := setupIdentityHandlerTest(t, userID)
	defer svc.AssertExpectations(t)

	r.GET(authKeysPath, handler.ListKeys)

	keys := []*domain.APIKey{{ID: uuid.New(), Name: "K1"}, {ID: uuid.New(), Name: "K2"}}
	svc.On("ListKeys", mock.Anything, userID).Return(keys, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", authKeysPath, nil)
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "K1")
	assert.Contains(t, w.Body.String(), "K2")
}

func TestIdentityHandlerRevokeKey(t *testing.T) {
	userID := uuid.New()
	svc, handler, r := setupIdentityHandlerTest(t, userID)
	defer svc.AssertExpectations(t)

	keyID := uuid.New()
	r.DELETE(authKeysPath+"/:id", handler.RevokeKey)

	svc.On("RevokeKey", mock.Anything, userID, keyID).Return(nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", authKeysPath+"/"+keyID.String(), nil)
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestIdentityHandlerRotateKey(t *testing.T) {
	userID := uuid.New()
	svc, handler, r := setupIdentityHandlerTest(t, userID)
	defer svc.AssertExpectations(t)

	keyID := uuid.New()
	r.POST(authKeysPath+"/:id/rotate", handler.RotateKey)

	newKey := &domain.APIKey{ID: uuid.New(), Key: "new_key", Name: "Rotated"}
	svc.On("RotateKey", mock.Anything, userID, keyID).Return(newKey, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", authKeysPath+"/"+keyID.String()+"/rotate", nil)
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "new_key")
}
