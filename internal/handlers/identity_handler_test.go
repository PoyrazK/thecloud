package httphandlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	authKeysPath = "/auth/keys"
	testKeyName  = "Test Key"
)

func setupIdentityHandlerTest(userID uuid.UUID) (*mockIdentityService, *IdentityHandler, *gin.Engine) {
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
	t.Parallel()
	userID := uuid.New()
	svc, handler, r := setupIdentityHandlerTest(userID)
	defer svc.AssertExpectations(t)

	r.POST(authKeysPath, handler.CreateKey)

	key := &domain.APIKey{Key: "sk_test_123", Name: testKeyName}
	svc.On("CreateKey", mock.Anything, userID, testKeyName).Return(key, nil)

	body, err := json.Marshal(map[string]string{"name": testKeyName})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", authKeysPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "sk_test_123")
}

func TestIdentityHandlerListKeys(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	svc, handler, r := setupIdentityHandlerTest(userID)
	defer svc.AssertExpectations(t)

	r.GET(authKeysPath, handler.ListKeys)

	keys := []*domain.APIKey{{ID: uuid.New(), Name: "K1"}, {ID: uuid.New(), Name: "K2"}}
	svc.On("ListKeys", mock.Anything, userID).Return(keys, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", authKeysPath, nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "K1")
	assert.Contains(t, w.Body.String(), "K2")
}

func TestIdentityHandlerRevokeKey(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	svc, handler, r := setupIdentityHandlerTest(userID)
	defer svc.AssertExpectations(t)

	keyID := uuid.New()
	r.DELETE(authKeysPath+"/:id", handler.RevokeKey)

	svc.On("RevokeKey", mock.Anything, userID, keyID).Return(nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", authKeysPath+"/"+keyID.String(), nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestIdentityHandlerRotateKey(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	svc, handler, r := setupIdentityHandlerTest(userID)
	defer svc.AssertExpectations(t)

	keyID := uuid.New()
	r.POST(authKeysPath+"/:id/rotate", handler.RotateKey)

	newKey := &domain.APIKey{ID: uuid.New(), Key: "new_key", Name: "Rotated"}
	svc.On("RotateKey", mock.Anything, userID, keyID).Return(newKey, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", authKeysPath+"/"+keyID.String()+"/rotate", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "new_key")
}

func TestIdentityHandlerRegenerateKey(t *testing.T) {
	t.Parallel()
	userID := uuid.New()
	svc, handler, r := setupIdentityHandlerTest(userID)
	defer svc.AssertExpectations(t)

	keyID := uuid.New()
	r.POST(authKeysPath+"/:id/regenerate", handler.RegenerateKey)

	newKey := &domain.APIKey{ID: uuid.New(), Key: "new_key_gen", Name: "Regenerated"}
	svc.On("RotateKey", mock.Anything, userID, keyID).Return(newKey, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", authKeysPath+"/"+keyID.String()+"/regenerate", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "new_key_gen")
}
