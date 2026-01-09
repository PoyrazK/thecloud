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
	secretsPath    = "/secrets"
	testSecretName = "sec-1"
)

type mockSecretService struct {
	mock.Mock
}

func (m *mockSecretService) CreateSecret(ctx context.Context, name, value, description string) (*domain.Secret, error) {
	args := m.Called(ctx, name, value, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Secret), args.Error(1)
}

func (m *mockSecretService) ListSecrets(ctx context.Context) ([]*domain.Secret, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Secret), args.Error(1)
}

func (m *mockSecretService) GetSecret(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Secret), args.Error(1)
}

func (m *mockSecretService) GetSecretByName(ctx context.Context, name string) (*domain.Secret, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Secret), args.Error(1)
}

func (m *mockSecretService) DeleteSecret(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func setupSecretHandlerTest(t *testing.T) (*mockSecretService, *SecretHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSecretService)
	handler := NewSecretHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestSecretHandlerCreate(t *testing.T) {
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(secretsPath, handler.Create)

	secret := &domain.Secret{ID: uuid.New(), Name: testSecretName}
	svc.On("CreateSecret", mock.Anything, testSecretName, "value", "desc").Return(secret, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":        testSecretName,
		"value":       "value",
		"description": "desc",
	})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", secretsPath, bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestSecretHandlerList(t *testing.T) {
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(secretsPath, handler.List)

	secrets := []*domain.Secret{{ID: uuid.New(), Name: testSecretName}}
	svc.On("ListSecrets", mock.Anything).Return(secrets, nil)

	req, err := http.NewRequest(http.MethodGet, secretsPath, nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandlerGetByID(t *testing.T) {
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(secretsPath+"/:id", handler.Get)

	id := uuid.New()
	secret := &domain.Secret{ID: id, Name: testSecretName}
	svc.On("GetSecret", mock.Anything, id).Return(secret, nil)

	req, err := http.NewRequest(http.MethodGet, secretsPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandlerGetByName(t *testing.T) {
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(secretsPath+"/:id", handler.Get)

	secret := &domain.Secret{ID: uuid.New(), Name: testSecretName}
	svc.On("GetSecretByName", mock.Anything, testSecretName).Return(secret, nil)

	req, err := http.NewRequest(http.MethodGet, secretsPath+"/"+testSecretName, nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandlerDelete(t *testing.T) {
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(secretsPath+"/:id", handler.Delete)

	id := uuid.New()
	svc.On("DeleteSecret", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, secretsPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
