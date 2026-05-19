package httphandlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockStorageService struct {
	mock.Mock
}

func (m *mockStorageService) Upload(ctx context.Context, bucket, key string, r io.Reader, providedChecksum string) (*domain.Object, error) {
	args := m.Called(ctx, bucket, key, r, providedChecksum)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Object)
	return r0, args.Error(1)
}

func (m *mockStorageService) Download(ctx context.Context, bucket, key string) (io.ReadCloser, *domain.Object, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	r0, _ := args.Get(0).(io.ReadCloser)
	r1, _ := args.Get(1).(*domain.Object)
	return r0, r1, args.Error(2)
}

func (m *mockStorageService) ListObjects(ctx context.Context, bucket string) ([]*domain.Object, error) {
	args := m.Called(ctx, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Object)
	return r0, args.Error(1)
}

func (m *mockStorageService) DeleteObject(ctx context.Context, bucket, key string) error {
	return m.Called(ctx, bucket, key).Error(0)
}

func (m *mockStorageService) CreateBucket(ctx context.Context, name string, isPublic bool) (*domain.Bucket, error) {
	args := m.Called(ctx, name, isPublic)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Bucket)
	return r0, args.Error(1)
}

func (m *mockStorageService) ListBuckets(ctx context.Context) ([]*domain.Bucket, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Bucket)
	return r0, args.Error(1)
}

func (m *mockStorageService) DeleteBucket(ctx context.Context, name string, force bool) error {
	return m.Called(ctx, name, force).Error(0)
}

func (m *mockStorageService) GetClusterStatus(ctx context.Context) (*domain.StorageCluster, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.StorageCluster)
	return r0, args.Error(1)
}

func (m *mockStorageService) ListVersions(ctx context.Context, bucket, key string) ([]*domain.Object, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Object)
	return r0, args.Error(1)
}

func (m *mockStorageService) DownloadVersion(ctx context.Context, bucket, key, versionID string) (io.ReadCloser, *domain.Object, error) {
	args := m.Called(ctx, bucket, key, versionID)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	r0, _ := args.Get(0).(io.ReadCloser)
	r1, _ := args.Get(1).(*domain.Object)
	return r0, r1, args.Error(2)
}

func (m *mockStorageService) DeleteVersion(ctx context.Context, bucket, key, versionID string) error {
	return m.Called(ctx, bucket, key, versionID).Error(0)
}

func (m *mockStorageService) SetBucketVersioning(ctx context.Context, name string, enabled bool) error {
	return m.Called(ctx, name, enabled).Error(0)
}

func (m *mockStorageService) CreateMultipartUpload(ctx context.Context, bucket, key string) (*domain.MultipartUpload, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.MultipartUpload)
	return r0, args.Error(1)
}

func (m *mockStorageService) UploadPart(ctx context.Context, uploadID uuid.UUID, partNumber int, r io.Reader, providedChecksum string) (*domain.Part, error) {
	args := m.Called(ctx, uploadID, partNumber, r, providedChecksum)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Part)
	return r0, args.Error(1)
}

func (m *mockStorageService) CompleteMultipartUpload(ctx context.Context, uploadID uuid.UUID) (*domain.Object, error) {
	args := m.Called(ctx, uploadID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Object)
	return r0, args.Error(1)
}

func (m *mockStorageService) AbortMultipartUpload(ctx context.Context, uploadID uuid.UUID) error {
	return m.Called(ctx, uploadID).Error(0)
}

func (m *mockStorageService) CleanupDeleted(ctx context.Context, limit int) (int, error) {
	args := m.Called(ctx, limit)
	return args.Int(0), args.Error(1)
}

func (m *mockStorageService) CleanupPendingUploads(ctx context.Context, olderThan time.Duration, limit int) (int, error) {
	args := m.Called(ctx, olderThan, limit)
	return args.Int(0), args.Error(1)
}

func (m *mockStorageService) GeneratePresignedURL(ctx context.Context, bucket, key, method string, expiry time.Duration) (*domain.PresignedURL, error) {
	args := m.Called(ctx, bucket, key, method, expiry)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.PresignedURL)
	return r0, args.Error(1)
}

func (m *mockStorageService) GetBucket(ctx context.Context, name string) (*domain.Bucket, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Bucket)
	return r0, args.Error(1)
}

func (m *mockStorageService) UploadStream(ctx context.Context, bucket, key string, r io.Reader) (*domain.Object, error) {
	return nil, nil
}

const (
	bucketsPath        = "/storage/buckets"
	bucketPath         = "/storage/:bucket"
	bucketKeyPath      = "/storage/:bucket/*key"
	testTxtKey         = "test.txt"
	testTxtPath        = "/test.txt"
	testTxtFullURL     = "/storage/b1/test.txt"
	clusterStatusPath  = "/storage/cluster/status"
	multipartInitPath  = "/storage/multipart/init/:bucket/*key"
	multipartPartsPath = "/storage/multipart/upload/:id/parts"
	multipartComplPath = "/storage/multipart/complete/:id"
	multipartAbortPath = "/storage/multipart/abort/:id"
	versioningPath     = "/storage/buckets/:bucket/versioning"
	versionsPath       = "/storage/versions/:bucket/*key"
	presignPath        = "/storage/presign/:bucket/*key"
	presignedPath      = "/storage/presigned/:bucket/*key"
	presignTestURL     = "/storage/presign/b1/test.txt"
	contentTypeJSON    = "application/json"
)

func setupStorageHandlerTest() (*mockStorageService, *StorageHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(mockStorageService)
	handler := NewStorageHandler(mockSvc, &platform.Config{})
	r := gin.New()
	return mockSvc, handler, r
}

func TestStorageHandlerCreateBucket(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.POST(bucketsPath, handler.CreateBucket)

	mockSvc.On("CreateBucket", mock.Anything, "b1", false).Return(&domain.Bucket{Name: "b1"}, nil)

	body := `{"name":"b1"}`
	req := httptest.NewRequest(http.MethodPost, bucketsPath, strings.NewReader(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestStorageHandlerCreateBucketError(t *testing.T) {
	t.Parallel()
	_, handler, r := setupStorageHandlerTest()
	r.POST(bucketsPath, handler.CreateBucket)

	body := `{"invalid":json}`
	req := httptest.NewRequest(http.MethodPost, bucketsPath, strings.NewReader(body))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStorageHandlerListBuckets(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.GET(bucketsPath, handler.ListBuckets)

	buckets := []*domain.Bucket{{Name: "b1"}}
	mockSvc.On("ListBuckets", mock.Anything).Return(buckets, nil)

	req := httptest.NewRequest(http.MethodGet, bucketsPath, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStorageHandlerUpload(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.PUT(bucketKeyPath, handler.Upload)

	t.Run("Success without Checksum", func(t *testing.T) {
		obj := &domain.Object{Key: testTxtKey}
		mockSvc.On("Upload", mock.Anything, "b1", testTxtKey, mock.Anything, "").Return(obj, nil).Once()

		req := httptest.NewRequest(http.MethodPut, testTxtFullURL, strings.NewReader("hello"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("Success with Checksum", func(t *testing.T) {
		checksum := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		obj := &domain.Object{Key: testTxtKey, Checksum: checksum}
		mockSvc.On("Upload", mock.Anything, "b1", testTxtKey, mock.Anything, checksum).Return(obj, nil).Once()

		req := httptest.NewRequest(http.MethodPut, testTxtFullURL, strings.NewReader("hello"))
		req.Header.Set("X-Content-Sha256", checksum)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

func TestStorageHandlerList(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.GET(bucketPath, handler.List)

	objects := []*domain.Object{{Key: testTxtKey}}
	mockSvc.On("ListObjects", mock.Anything, "b1").Return(objects, nil)

	req := httptest.NewRequest(http.MethodGet, "/storage/b1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStorageHandlerDeleteBucket(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.DELETE(bucketsPath+"/:bucket", handler.DeleteBucket)

	t.Run("Normal Delete", func(t *testing.T) {
		mockSvc.On("DeleteBucket", mock.Anything, "b1", false).Return(nil).Once()
		req := httptest.NewRequest(http.MethodDelete, bucketsPath+"/b1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("Force Delete", func(t *testing.T) {
		mockSvc.On("DeleteBucket", mock.Anything, "b2", true).Return(nil).Once()
		req := httptest.NewRequest(http.MethodDelete, bucketsPath+"/b2?force=true", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("Service Error", func(t *testing.T) {
		mockSvc.On("DeleteBucket", mock.Anything, "b3", false).Return(errors.New(errors.Internal, "fail")).Once()
		req := httptest.NewRequest(http.MethodDelete, bucketsPath+"/b3", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestStorageHandlerDelete(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.DELETE(bucketKeyPath, handler.Delete)

	mockSvc.On("DeleteObject", mock.Anything, "b1", testTxtKey).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, testTxtFullURL, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestStorageHandlerDownload(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.GET(bucketKeyPath, handler.Download)

	content := "hello world"
	reader := io.NopCloser(strings.NewReader(content))
	obj := &domain.Object{Key: testTxtKey, SizeBytes: int64(len(content)), ContentType: "text/plain"}

	mockSvc.On("Download", mock.Anything, "b1", testTxtKey).Return(reader, obj, nil)

	req := httptest.NewRequest(http.MethodGet, testTxtFullURL, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, content, w.Body.String())
}

func TestStorageHandlerGetClusterStatus(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.GET(clusterStatusPath, handler.GetClusterStatus)

	status := &domain.StorageCluster{Nodes: []domain.StorageNode{{ID: "n1"}}}
	mockSvc.On("GetClusterStatus", mock.Anything).Return(status, nil)

	req := httptest.NewRequest(http.MethodGet, clusterStatusPath, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStorageHandlerMultipartUpload(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.POST("/storage/multipart/init/:bucket/*key", handler.InitiateMultipartUpload)
	r.PUT("/storage/multipart/upload/:id/parts", handler.UploadPart)
	r.POST("/storage/multipart/complete/:id", handler.CompleteMultipartUpload)
	r.DELETE("/storage/multipart/abort/:id", handler.AbortMultipartUpload)

	uploadID := uuid.New()

	// Initiate
	mockSvc.On("CreateMultipartUpload", mock.Anything, "b1", testTxtPath).Return(&domain.MultipartUpload{ID: uploadID}, nil)
	req := httptest.NewRequest(http.MethodPost, "/storage/multipart/init/b1/test.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Upload Part
	mockSvc.On("UploadPart", mock.Anything, uploadID, 1, mock.Anything, "").Return(&domain.Part{PartNumber: 1}, nil)
	req = httptest.NewRequest(http.MethodPut, "/storage/multipart/upload/"+uploadID.String()+"/parts?part=1", strings.NewReader("data"))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Complete
	mockSvc.On("CompleteMultipartUpload", mock.Anything, uploadID).Return(&domain.Object{Key: testTxtKey}, nil)
	req = httptest.NewRequest(http.MethodPost, "/storage/multipart/complete/"+uploadID.String(), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Abort
	mockSvc.On("AbortMultipartUpload", mock.Anything, uploadID).Return(nil)
	req = httptest.NewRequest(http.MethodDelete, "/storage/multipart/abort/"+uploadID.String(), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestStorageHandlerVersioning(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.PATCH("/storage/buckets/:bucket/versioning", handler.SetBucketVersioning)
	r.GET("/storage/versions/:bucket/*key", handler.ListVersions)

	// Set Versioning
	mockSvc.On("SetBucketVersioning", mock.Anything, "b1", true).Return(nil)
	body := `{"enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/storage/buckets/b1/versioning", strings.NewReader(body))
	req.Header.Set("Content-Type", contentTypeJSON)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// List Versions
	versions := []*domain.Object{{Key: testTxtKey, VersionID: "v1"}}
	mockSvc.On("ListVersions", mock.Anything, "b1", testTxtKey).Return(versions, nil)
	req = httptest.NewRequest(http.MethodGet, "/storage/versions/b1/test.txt", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStorageHandlerPresigned(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	// Set secret for signed URL verification
	handler.cfg.StorageSecret = "secret"

	r.POST(presignPath, handler.GeneratePresignedURL)
	r.GET(presignedPath, handler.ServePresignedDownload)

	// Generate
	mockSvc.On("GeneratePresignedURL", mock.Anything, "b1", testTxtPath, "GET", mock.Anything).Return(&domain.PresignedURL{URL: "http://test.com"}, nil)
	body := `{"method":"GET"}`
	req := httptest.NewRequest(http.MethodPost, presignTestURL, strings.NewReader(body))
	req.Header.Set("Content-Type", contentTypeJSON)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Serve (Download)

	signedURL, _ := crypto.SignURL("secret", "http://localhost", "GET", "b1", testTxtKey, time.Now().Add(time.Hour))
	u, _ := url.Parse(signedURL)

	content := "presigned content"
	reader := io.NopCloser(strings.NewReader(content))
	obj := &domain.Object{Key: testTxtKey, SizeBytes: int64(len(content)), ContentType: "text/plain"}
	mockSvc.On("Download", mock.Anything, "b1", testTxtPath).Return(reader, obj, nil)

	req = httptest.NewRequest(http.MethodGet, u.String(), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, content, w.Body.String())
}

func TestStorageHandlerPresignedUpload(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	handler.cfg.StorageSecret = "secret"
	r.PUT(presignedPath, handler.ServePresignedUpload)

	signedURL, _ := crypto.SignURL("secret", "http://localhost", "PUT", "b1", testTxtKey, time.Now().Add(time.Hour))
	u, _ := url.Parse(signedURL)

	obj := &domain.Object{Key: testTxtKey}
	mockSvc.On("Upload", mock.Anything, "b1", testTxtPath, mock.Anything, "").Return(obj, nil)

	req := httptest.NewRequest(http.MethodPut, u.String(), strings.NewReader("uploaded content"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestStorageHandlerUploadError(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.PUT(bucketKeyPath, handler.Upload)

	mockSvc.On("Upload", mock.Anything, "b1", testTxtKey, mock.Anything, "").Return(nil, errors.New(errors.Internal, "upload failed"))

	req := httptest.NewRequest(http.MethodPut, testTxtFullURL, strings.NewReader("hello"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStorageHandlerListError(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.GET(bucketPath, handler.List)

	mockSvc.On("ListObjects", mock.Anything, "b1").Return(nil, errors.New(errors.Internal, "list failed"))

	req := httptest.NewRequest(http.MethodGet, "/storage/b1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStorageHandlerDeleteError(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.DELETE(bucketKeyPath, handler.Delete)

	mockSvc.On("DeleteObject", mock.Anything, "b1", testTxtKey).Return(errors.New(errors.Internal, "delete failed"))

	req := httptest.NewRequest(http.MethodDelete, testTxtFullURL, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStorageHandlerListBucketsError(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.GET(bucketsPath, handler.ListBuckets)

	mockSvc.On("ListBuckets", mock.Anything).Return(nil, errors.New(errors.Internal, "list buckets failed"))

	req := httptest.NewRequest(http.MethodGet, bucketsPath, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStorageHandlerDeleteBucketErrorCase(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.DELETE(bucketsPath+"/:bucket", handler.DeleteBucket)

	mockSvc.On("DeleteBucket", mock.Anything, "b1", false).Return(errors.New(errors.Internal, "delete bucket failed"))

	req := httptest.NewRequest(http.MethodDelete, bucketsPath+"/b1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStorageHandlerGetClusterStatusError(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.GET(clusterStatusPath, handler.GetClusterStatus)

	mockSvc.On("GetClusterStatus", mock.Anything).Return(nil, errors.New(errors.Internal, "cluster status failed"))

	req := httptest.NewRequest(http.MethodGet, clusterStatusPath, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStorageHandlerMultipartErrors(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.POST(multipartInitPath, handler.InitiateMultipartUpload)
	r.PUT(multipartPartsPath, handler.UploadPart)
	r.POST(multipartComplPath, handler.CompleteMultipartUpload)
	r.DELETE(multipartAbortPath, handler.AbortMultipartUpload)

	uploadID := uuid.New()

	// Init Error
	mockSvc.On("CreateMultipartUpload", mock.Anything, "b1", testTxtPath).Return(nil, errors.New(errors.Internal, "init failed"))
	req := httptest.NewRequest(http.MethodPost, "/storage/multipart/init/b1/test.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Upload Part Error
	mockSvc.On("UploadPart", mock.Anything, uploadID, 1, mock.Anything, "").Return(nil, errors.New(errors.Internal, "part failed"))
	req = httptest.NewRequest(http.MethodPut, "/storage/multipart/upload/"+uploadID.String()+"/parts?part=1", strings.NewReader("data"))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Complete Error
	mockSvc.On("CompleteMultipartUpload", mock.Anything, uploadID).Return(nil, errors.New(errors.Internal, "complete failed"))
	req = httptest.NewRequest(http.MethodPost, "/storage/multipart/complete/"+uploadID.String(), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Abort Error
	mockSvc.On("AbortMultipartUpload", mock.Anything, uploadID).Return(errors.New(errors.Internal, "abort failed"))
	req = httptest.NewRequest(http.MethodDelete, "/storage/multipart/abort/"+uploadID.String(), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStorageHandlerVersioningErrors(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	r.PATCH("/storage/buckets/:bucket/versioning", handler.SetBucketVersioning)
	r.GET("/storage/versions/:bucket/*key", handler.ListVersions)

	// Set Error
	mockSvc.On("SetBucketVersioning", mock.Anything, "b1", true).Return(errors.New(errors.Internal, "set versioning failed"))
	body := `{"enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/storage/buckets/b1/versioning", strings.NewReader(body))
	req.Header.Set("Content-Type", contentTypeJSON)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// List Error
	mockSvc.On("ListVersions", mock.Anything, "b1", testTxtKey).Return(nil, errors.New(errors.Internal, "list versions failed"))
	req = httptest.NewRequest(http.MethodGet, "/storage/versions/b1/test.txt", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestStorageHandlerPresignedErrors(t *testing.T) {
	t.Parallel()
	mockSvc, handler, r := setupStorageHandlerTest()
	handler.cfg.StorageSecret = "secret"

	r.POST(presignPath, handler.GeneratePresignedURL)
	r.GET(presignedPath, handler.ServePresignedDownload)
	r.PUT(presignedPath, handler.ServePresignedUpload)

	// Generate Error - Bind
	req := httptest.NewRequest(http.MethodPost, presignTestURL, strings.NewReader("invalid"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Generate Error - Svc
	mockSvc.On("GeneratePresignedURL", mock.Anything, "b1", testTxtPath, "GET", mock.Anything).Return(nil, errors.New(errors.Internal, "presign failed"))
	body := `{"method":"GET"}`
	req = httptest.NewRequest(http.MethodPost, presignTestURL, strings.NewReader(body))
	req.Header.Set("Content-Type", contentTypeJSON)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Serve Download Error - Signature
	req = httptest.NewRequest(http.MethodGet, "/storage/presigned/b1/test.txt?signature=invalid&expires=123", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Serve Upload Error - Signature
	req = httptest.NewRequest(http.MethodPut, "/storage/presigned/b1/test.txt?signature=invalid&expires=123", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestStorageHandlerPresignedMissingSecret(t *testing.T) {
	t.Parallel()
	_, handler, r := setupStorageHandlerTest()
	// Ensure secret is empty (default in new config)
	handler.cfg.StorageSecret = ""

	r.GET(presignedPath, handler.ServePresignedDownload)
	r.PUT(presignedPath, handler.ServePresignedUpload)

	// Download - Missing Secret
	req := httptest.NewRequest(http.MethodGet, "/storage/presigned/b1/test.txt?signature=sig&expires=123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Upload - Missing Secret
	req = httptest.NewRequest(http.MethodPut, "/storage/presigned/b1/test.txt?signature=sig&expires=123", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
