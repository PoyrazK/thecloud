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
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	secretsPath       = "/secrets"
	testSecretName    = "sec-1"
	errSecretNotFound = "not found"
)

type mockSecretService struct {
	mock.Mock
}

func (m *mockSecretService) CreateSecret(ctx context.Context, name, value, description string) (*domain.Secret, error) {
	args := m.Called(ctx, name, value, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Secret)
	return r0, args.Error(1)
}

func (m *mockSecretService) ListSecrets(ctx context.Context) ([]*domain.Secret, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Secret)
	return r0, args.Error(1)
}

func (m *mockSecretService) GetSecret(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Secret)
	return r0, args.Error(1)
}

func (m *mockSecretService) GetSecretByName(ctx context.Context, name string) (*domain.Secret, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Secret)
	return r0, args.Error(1)
}

func (m *mockSecretService) DeleteSecret(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockSecretService) Encrypt(ctx context.Context, userID uuid.UUID, plainText string) (string, error) {
	args := m.Called(ctx, userID, plainText)
	return args.String(0), args.Error(1)
}

func (m *mockSecretService) Decrypt(ctx context.Context, userID uuid.UUID, cipherText string) (string, error) {
	args := m.Called(ctx, userID, cipherText)
	return args.String(0), args.Error(1)
}

func setupSecretHandlerTest(_ *testing.T) (*mockSecretService, *SecretHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSecretService)
	handler := NewSecretHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestSecretHandlerCreate(t *testing.T) {
	t.Parallel()
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
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", secretsPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestSecretHandlerList(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(secretsPath, handler.List)

	secrets := []*domain.Secret{{ID: uuid.New(), Name: testSecretName}}
	svc.On("ListSecrets", mock.Anything).Return(secrets, nil)

	req, err := http.NewRequest(http.MethodGet, secretsPath, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandlerGetByID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(secretsPath+"/:id", handler.Get)

	id := uuid.New()
	secret := &domain.Secret{ID: id, Name: testSecretName}
	svc.On("GetSecret", mock.Anything, id).Return(secret, nil)

	req, err := http.NewRequest(http.MethodGet, secretsPath+"/"+id.String(), nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandlerGetByName(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(secretsPath+"/:id", handler.Get)

	id := uuid.New()
	secret := &domain.Secret{ID: id, Name: testSecretName}
	svc.On("GetSecretByName", mock.Anything, testSecretName).Return(secret, nil)
	svc.On("GetSecret", mock.Anything, id).Return(secret, nil)

	req, err := http.NewRequest(http.MethodGet, secretsPath+"/"+testSecretName, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandlerGetByShortID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(secretsPath+"/:id", handler.Get)

	id := uuid.New()
	secret := &domain.Secret{ID: id, Name: testSecretName}
	shortPrefix := id.String()[:8]
	svc.On("GetSecretByName", mock.Anything, shortPrefix).Return(nil, errors.New(errors.NotFound, "not found"))
	svc.On("ListSecrets", mock.Anything).Return([]*domain.Secret{secret}, nil)
	svc.On("GetSecret", mock.Anything, id).Return(secret, nil)

	req, err := http.NewRequest(http.MethodGet, secretsPath+"/"+shortPrefix, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandlerGetByShortID_Ambiguous(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(secretsPath+"/:id", handler.Get)

	idA := uuid.MustParse("11111111-0000-0000-0000-000000000001")
	idB := uuid.MustParse("11111111-0000-0000-0000-000000000002")
	secretA := &domain.Secret{ID: idA, Name: "sec-a"}
	secretB := &domain.Secret{ID: idB, Name: "sec-b"}
	shortPrefix := idA.String()[:8]

	svc.On("GetSecretByName", mock.Anything, shortPrefix).Return(nil, errors.New(errors.NotFound, "not found"))
	svc.On("ListSecrets", mock.Anything).Return([]*domain.Secret{secretA, secretB}, nil)

	req, err := http.NewRequest(http.MethodGet, secretsPath+"/"+shortPrefix, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ambiguous")
}

func TestSecretHandlerDeleteByShortID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(secretsPath+"/:id", handler.Delete)

	id := uuid.New()
	secret := &domain.Secret{ID: id, Name: testSecretName}
	shortPrefix := id.String()[:8]
	svc.On("GetSecretByName", mock.Anything, shortPrefix).Return(nil, errors.New(errors.NotFound, "not found"))
	svc.On("ListSecrets", mock.Anything).Return([]*domain.Secret{secret}, nil)
	svc.On("DeleteSecret", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, secretsPath+"/"+shortPrefix, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSecretHandlerDeleteByShortID_Ambiguous(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupSecretHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(secretsPath+"/:id", handler.Delete)

	idA := uuid.MustParse("22222222-0000-0000-0000-000000000001")
	idB := uuid.MustParse("22222222-0000-0000-0000-000000000002")
	secretA := &domain.Secret{ID: idA, Name: "sec-a"}
	secretB := &domain.Secret{ID: idB, Name: "sec-b"}
	shortPrefix := idA.String()[:8]

	svc.On("GetSecretByName", mock.Anything, shortPrefix).Return(nil, errors.New(errors.NotFound, "not found"))
	svc.On("ListSecrets", mock.Anything).Return([]*domain.Secret{secretA, secretB}, nil)

	req, err := http.NewRequest(http.MethodDelete, secretsPath+"/"+shortPrefix, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "ambiguous")
}

func TestSecretHandlerDelete(t *testing.T) {
	t.Parallel()
	t.Run("SuccessByID", func(t *testing.T) {
		svc, handler, r := setupSecretHandlerTest(t)
		r.DELETE(secretsPath+"/:id", handler.Delete)
		id := uuid.New()
		svc.On("DeleteSecret", mock.Anything, id).Return(nil)
		req, _ := http.NewRequest(http.MethodDelete, secretsPath+"/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("SuccessByName", func(t *testing.T) {
		svc, handler, r := setupSecretHandlerTest(t)
		r.DELETE(secretsPath+"/:id", handler.Delete)
		id := uuid.New()
		svc.On("GetSecretByName", mock.Anything, testSecretName).Return(&domain.Secret{ID: id}, nil)
		svc.On("DeleteSecret", mock.Anything, id).Return(nil)
		req, _ := http.NewRequest(http.MethodDelete, secretsPath+"/"+testSecretName, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("GetByNameError", func(t *testing.T) {
		svc, handler, r := setupSecretHandlerTest(t)
		r.DELETE(secretsPath+"/:id", handler.Delete)
		svc.On("GetSecretByName", mock.Anything, testSecretName).Return(nil, errors.New(errors.NotFound, errSecretNotFound))
		svc.On("ListSecrets", mock.Anything).Return(nil, errors.New(errors.NotFound, errSecretNotFound))
		req, _ := http.NewRequest(http.MethodDelete, secretsPath+"/"+testSecretName, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("DeleteError", func(t *testing.T) {
		svc, handler, r := setupSecretHandlerTest(t)
		r.DELETE(secretsPath+"/:id", handler.Delete)
		id := uuid.New()
		svc.On("DeleteSecret", mock.Anything, id).Return(errors.New(errors.Internal, "internal error"))
		req, _ := http.NewRequest(http.MethodDelete, secretsPath+"/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestSecretHandlerCreateError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidJSON", func(t *testing.T) {
		_, handler, r := setupSecretHandlerTest(t)
		r.POST(secretsPath, handler.Create)
		req, _ := http.NewRequest("POST", secretsPath, bytes.NewBufferString("invalid"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupSecretHandlerTest(t)
		r.POST(secretsPath, handler.Create)
		svc.On("CreateSecret", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New(errors.Internal, "error"))
		body, _ := json.Marshal(map[string]interface{}{"name": "n", "value": "v"})
		req, _ := http.NewRequest("POST", secretsPath, bytes.NewBuffer(body))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestSecretHandlerListError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupSecretHandlerTest(t)
	r.GET(secretsPath, handler.List)
	svc.On("ListSecrets", mock.Anything).Return(nil, errors.New(errors.Internal, "error"))
	req, _ := http.NewRequest(http.MethodGet, secretsPath, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestSecretHandlerGetError(t *testing.T) {
	t.Parallel()
	t.Run("NotFoundByID", func(t *testing.T) {
		svc, handler, r := setupSecretHandlerTest(t)
		r.GET(secretsPath+"/:id", handler.Get)
		id := uuid.New()
		svc.On("GetSecret", mock.Anything, id).Return(nil, errors.New(errors.NotFound, errSecretNotFound))
		req, _ := http.NewRequest(http.MethodGet, secretsPath+"/"+id.String(), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("NotFoundByName", func(t *testing.T) {
		svc, handler, r := setupSecretHandlerTest(t)
		r.GET(secretsPath+"/:id", handler.Get)
		svc.On("GetSecretByName", mock.Anything, "name").Return(nil, errors.New(errors.NotFound, errSecretNotFound))
		svc.On("ListSecrets", mock.Anything).Return(nil, errors.New(errors.NotFound, errSecretNotFound))
		req, _ := http.NewRequest(http.MethodGet, secretsPath+"/name", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
		svc.AssertExpectations(t)
	})
}
