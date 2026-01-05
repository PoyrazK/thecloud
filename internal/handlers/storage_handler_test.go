package httphandlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockStorageService struct {
	mock.Mock
}

func (m *mockStorageService) CreateBucket(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *mockStorageService) ListBuckets(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockStorageService) Upload(ctx context.Context, bucket, key string, content io.Reader) (*domain.Object, error) {
	args := m.Called(ctx, bucket, key, content)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Object), args.Error(1)
}

func (m *mockStorageService) Download(ctx context.Context, bucket, key string) (io.ReadCloser, *domain.Object, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(io.ReadCloser), args.Get(1).(*domain.Object), args.Error(2)
}

func (m *mockStorageService) ListObjects(ctx context.Context, bucket string) ([]*domain.Object, error) {
	args := m.Called(ctx, bucket)
	return args.Get(0).([]*domain.Object), args.Error(1)
}

func (m *mockStorageService) DeleteObject(ctx context.Context, bucket, key string) error {
	args := m.Called(ctx, bucket, key)
	return args.Error(0)
}

func (m *mockStorageService) GetObject(ctx context.Context, bucket, key string) (*domain.Object, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Object), args.Error(1)
}

func TestStorageHandler_Upload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockStorageService)
	handler := NewStorageHandler(svc)

	r := gin.New()
	r.PUT("/storage/:bucket/:key", handler.Upload)

	obj := &domain.Object{Key: "test.txt", SizeBytes: 4}
	svc.On("Upload", mock.Anything, "b1", "test.txt", mock.Anything).Return(obj, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/storage/b1/test.txt", strings.NewReader("data"))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestStorageHandler_Download(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockStorageService)
	handler := NewStorageHandler(svc)

	r := gin.New()
	r.GET("/storage/:bucket/:key", handler.Download)

	content := io.NopCloser(bytes.NewBufferString("hello"))
	obj := &domain.Object{Key: "test.txt", SizeBytes: 5, ContentType: "text/plain"}
	svc.On("Download", mock.Anything, "b1", "test.txt").Return(content, obj, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/storage/b1/test.txt", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello", w.Body.String())
}
