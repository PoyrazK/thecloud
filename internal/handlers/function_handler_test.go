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

const (
	functionsPath    = "/functions"
	testFunctionName = "fn-1"
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

func TestFunctionHandlerCreate(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(functionsPath, handler.Create)

	fn := &domain.Function{ID: uuid.New(), Name: testFunctionName}
	svc.On("CreateFunction", mock.Anything, testFunctionName, "nodejs18", "index.handler", []byte("code")).Return(fn, nil)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	err := writer.WriteField("name", testFunctionName)
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

	req, err := http.NewRequest(http.MethodPost, functionsPath, body)
	assert.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestFunctionHandlerList(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(functionsPath, handler.List)

	fns := []*domain.Function{{ID: uuid.New(), Name: testFunctionName}}
	svc.On("ListFunctions", mock.Anything).Return(fns, nil)

	req, err := http.NewRequest(http.MethodGet, functionsPath, nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFunctionHandlerGet(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(functionsPath+"/:id", handler.Get)

	id := uuid.New()
	fn := &domain.Function{ID: id, Name: testFunctionName}
	svc.On("GetFunction", mock.Anything, id).Return(fn, nil)

	req, err := http.NewRequest(http.MethodGet, functionsPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFunctionHandlerDelete(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(functionsPath+"/:id", handler.Delete)

	id := uuid.New()
	svc.On("DeleteFunction", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, functionsPath+"/"+id.String(), nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFunctionHandlerInvoke(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(functionsPath+"/:id/invoke", handler.Invoke)

	id := uuid.New()
	inv := &domain.Invocation{ID: uuid.New(), Status: "completed"}
	svc.On("InvokeFunction", mock.Anything, id, []byte("{}"), false).Return(inv, nil)

	req, err := http.NewRequest(http.MethodPost, functionsPath+"/"+id.String()+"/invoke", strings.NewReader("{}"))
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFunctionHandlerGetLogs(t *testing.T) {
	svc, handler, r := setupFunctionHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(functionsPath+"/:id/logs", handler.GetLogs)

	id := uuid.New()
	svc.On("GetFunctionLogs", mock.Anything, id, 100).Return([]*domain.Invocation{}, nil)

	req, err := http.NewRequest(http.MethodGet, functionsPath+"/"+id.String()+"/logs", nil)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
