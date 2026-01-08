package httphandlers

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockFunctionService struct {
	mock.Mock
}

func (m *mockFunctionService) CreateFunction(ctx context.Context, name, runtime, handler string, code []byte) (*domain.Function, error) {
	args := m.Called(ctx, name, runtime, handler, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Function), args.Error(1)
}

func (m *mockFunctionService) ListFunctions(ctx context.Context) ([]*domain.Function, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Function), args.Error(1)
}

func (m *mockFunctionService) GetFunction(ctx context.Context, id uuid.UUID) (*domain.Function, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Function), args.Error(1)
}

func (m *mockFunctionService) DeleteFunction(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockFunctionService) InvokeFunction(ctx context.Context, id uuid.UUID, payload []byte, async bool) (*domain.Invocation, error) {
	args := m.Called(ctx, id, payload, async)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Invocation), args.Error(1)
}

func (m *mockFunctionService) GetFunctionLogs(ctx context.Context, id uuid.UUID, limit int) ([]*domain.Invocation, error) {
	args := m.Called(ctx, id, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Invocation), args.Error(1)
}

func setupFunctionHandlerTest(t *testing.T) (*mockFunctionService, *FunctionHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockFunctionService)
	handler := NewFunctionHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestFunctionHandler_Create(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST("/functions", handler.Create)

	fn := &domain.Function{ID: uuid.New(), Name: "fn-1"}
	svc.On("CreateFunction", mock.Anything, "fn-1", "nodejs18", "index.handler", []byte("code")).Return(fn, nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	err := writer.WriteField("name", "fn-1")
	assert.NoError(t, err)
	err = writer.WriteField("runtime", "nodejs18")
	assert.NoError(t, err)
	err = writer.WriteField("handler", "index.handler")
	assert.NoError(t, err)
	part, err := writer.CreateFormFile("code", "index.js")
	assert.NoError(t, err)
	_, err = part.Write([]byte("code"))
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "/functions", body)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestFunctionHandler_List(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET("/functions", handler.List)

	fns := []*domain.Function{{ID: uuid.New(), Name: "fn-1"}}
	svc.On("ListFunctions", mock.Anything).Return(fns, nil)

	req, err := http.NewRequest(http.MethodGet, "/functions", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFunctionHandler_Get(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET("/functions/:id", handler.Get)

	id := uuid.New()
	fn := &domain.Function{ID: id, Name: "fn-1"}
	svc.On("GetFunction", mock.Anything, id).Return(fn, nil)

	req, err := http.NewRequest(http.MethodGet, "/functions/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFunctionHandler_Delete(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE("/functions/:id", handler.Delete)

	id := uuid.New()
	svc.On("DeleteFunction", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, "/functions/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFunctionHandler_Invoke(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST("/functions/:id/invoke", handler.Invoke)

	id := uuid.New()
	inv := &domain.Invocation{ID: uuid.New(), Status: "completed"}
	svc.On("InvokeFunction", mock.Anything, id, []byte("{}"), false).Return(inv, nil)

	req, err := http.NewRequest(http.MethodPost, "/functions/"+id.String()+"/invoke", strings.NewReader("{}"))
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFunctionHandler_GetLogs(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET("/functions/:id/logs", handler.GetLogs)

	id := uuid.New()
	svc.On("GetFunctionLogs", mock.Anything, id, 100).Return([]*domain.Invocation{}, nil)

	req, err := http.NewRequest(http.MethodGet, "/functions/"+id.String()+"/logs", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
