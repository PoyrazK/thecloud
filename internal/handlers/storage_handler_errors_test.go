package httphandlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	internalerrors "github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
)

func TestStorageHandlerErrors(t *testing.T) { //nolint:gocyclo
	t.Parallel()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		path       string
		setupMock  func(*mockStorageService)
		setupCtx   func(*gin.Context)
		body       io.Reader
		checkCode  int
		checkError string
	}{
		{
			name:   "Upload Service Error",
			method: http.MethodPut,
			path:   "/storage/b1/key1",
			setupMock: func(m *mockStorageService) {
				m.On("Upload", mock.Anything, "b1", "key1", mock.Anything, "").
					Return(nil, internalerrors.New(internalerrors.Internal, "upload failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
		{
			name:   "Download Service Error",
			method: http.MethodGet,
			path:   "/storage/b1/key1",
			setupMock: func(m *mockStorageService) {
				m.On("Download", mock.Anything, "b1", "key1").
					Return(nil, nil, internalerrors.New(internalerrors.Internal, "download failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
		{
			name:   "ListBuckets Service Error",
			method: http.MethodGet,
			path:   "/storage/buckets",
			setupMock: func(m *mockStorageService) {
				m.On("ListBuckets", mock.Anything).
					Return(nil, internalerrors.New(internalerrors.Internal, "list failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
		{
			name:   "DeleteBucket Missing Name",
			method: http.MethodDelete,
			path:   "/storage/buckets/",
			setupMock: func(m *mockStorageService) {
				// No mock call expected
			},
			checkCode: http.StatusBadRequest,
		},
		{
			name:   "DeleteBucket Service Error",
			method: http.MethodDelete,
			path:   "/storage/buckets/b1",
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{{Key: "bucket", Value: "b1"}}
			},
			setupMock: func(m *mockStorageService) {
				m.On("DeleteBucket", mock.Anything, "b1", false).
					Return(internalerrors.New(internalerrors.Internal, "delete failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
		{
			name:   "InitiateMultipartUpload Service Error",
			method: http.MethodPost,
			path:   "/storage/b1/key1/multipart",
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{
					{Key: "bucket", Value: "b1"},
					{Key: "key", Value: "/key1"},
				}
			},
			setupMock: func(m *mockStorageService) {
				m.On("CreateMultipartUpload", mock.Anything, "b1", "/key1").
					Return(nil, internalerrors.New(internalerrors.Internal, "init failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
		{
			name:   "UploadPart Invalid UUID",
			method: http.MethodPut,
			path:   "/storage/multipart/invalid-uuid/parts",
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{{Key: "id", Value: "invalid-uuid"}}
			},
			setupMock: func(m *mockStorageService) {},
			checkCode: http.StatusBadRequest,
		},
		{
			name:   "UploadPart Service Error",
			method: http.MethodPut,
			path:   "/storage/multipart/" + uuid.New().String() + "/parts",
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{{Key: "id", Value: uuid.New().String()}}
				c.Request.URL.RawQuery = "part=1"
			},
			setupMock: func(m *mockStorageService) {
				m.On("UploadPart", mock.Anything, mock.Anything, 1, mock.Anything, "").
					Return(nil, internalerrors.New(internalerrors.Internal, "upload part failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
		{
			name:   "CompleteMultipartUpload Invalid UUID",
			method: http.MethodPost,
			path:   "/storage/multipart/invalid-uuid/complete",
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{{Key: "id", Value: "invalid-uuid"}}
			},
			setupMock: func(m *mockStorageService) {},
			checkCode: http.StatusBadRequest,
		},
		{
			name:   "CompleteMultipartUpload Service Error",
			method: http.MethodPost,
			path:   "/storage/multipart/" + uuid.New().String() + "/complete",
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{{Key: "id", Value: uuid.New().String()}}
			},
			setupMock: func(m *mockStorageService) {
				m.On("CompleteMultipartUpload", mock.Anything, mock.Anything).
					Return(nil, internalerrors.New(internalerrors.Internal, "complete failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
		{
			name:   "AbortMultipartUpload Invalid UUID",
			method: http.MethodDelete,
			path:   "/storage/multipart/invalid-uuid",
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{{Key: "id", Value: "invalid-uuid"}}
			},
			setupMock: func(m *mockStorageService) {},
			checkCode: http.StatusBadRequest,
		},
		{
			name:   "AbortMultipartUpload Service Error",
			method: http.MethodDelete,
			path:   "/storage/multipart/" + uuid.New().String(),
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{{Key: "id", Value: uuid.New().String()}}
			},
			setupMock: func(m *mockStorageService) {
				m.On("AbortMultipartUpload", mock.Anything, mock.Anything).
					Return(internalerrors.New(internalerrors.Internal, "abort failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
		{
			name:   "SetBucketVersioning Invalid JSON",
			method: http.MethodPatch,
			path:   "/storage/buckets/b1/versioning",
			body:   strings.NewReader("invalid json"),
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{{Key: "bucket", Value: "b1"}}
			},
			setupMock: func(m *mockStorageService) {},
			checkCode: http.StatusBadRequest,
		},
		{
			name:   "SetBucketVersioning Service Error",
			method: http.MethodPatch,
			path:   "/storage/buckets/b1/versioning",
			body:   strings.NewReader(`{"enabled": true}`),
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{{Key: "bucket", Value: "b1"}}
			},
			setupMock: func(m *mockStorageService) {
				m.On("SetBucketVersioning", mock.Anything, "b1", true).
					Return(internalerrors.New(internalerrors.Internal, "versioning failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
		{
			name:   "ListVersions Service Error",
			method: http.MethodGet,
			path:   "/storage/versions/b1/key1",
			setupMock: func(m *mockStorageService) {
				m.On("ListVersions", mock.Anything, "b1", "key1").
					Return(nil, internalerrors.New(internalerrors.Internal, "list versions failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
		{
			name:   "GeneratePresignedURL Invalid JSON",
			method: http.MethodPost,
			path:   "/storage/b1/key1/presigned",
			body:   strings.NewReader("invalid json"),
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{
					{Key: "bucket", Value: "b1"},
					{Key: "key", Value: "/key1"},
				}
			},
			setupMock: func(m *mockStorageService) {},
			checkCode: http.StatusBadRequest,
		},
		{
			name:   "GeneratePresignedURL Service Error",
			method: http.MethodPost,
			path:   "/storage/b1/key1/presigned",
			body:   strings.NewReader(`{"method": "GET"}`),
			setupCtx: func(c *gin.Context) {
				c.Params = []gin.Param{
					{Key: "bucket", Value: "b1"},
					{Key: "key", Value: "/key1"},
				}
			},
			setupMock: func(m *mockStorageService) {
				m.On("GeneratePresignedURL", mock.Anything, "b1", "/key1", "GET", 900*time.Second).
					Return(nil, internalerrors.New(internalerrors.Internal, "presign failed"))
			},
			checkCode: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mockSvc := new(mockStorageService)
			if tc.setupMock != nil {
				tc.setupMock(mockSvc)
			}

			handler := NewStorageHandler(mockSvc, &platform.Config{StorageSecret: "secret"})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Request = httptest.NewRequest(tc.method, tc.path, tc.body)
			if tc.setupCtx != nil {
				// setupCtx also responsible for setting params if needed
				tc.setupCtx(c)
			} else {
				// Default param extraction simulation logic
				parts := strings.Split(tc.path, "/")
				if len(parts) > 2 && parts[1] == "storage" {
					if parts[2] != "buckets" && parts[2] != "multipart" && parts[2] != "versions" {
						c.Params = append(c.Params, gin.Param{Key: "bucket", Value: parts[2]})
						if len(parts) > 3 {
							c.Params = append(c.Params, gin.Param{Key: "key", Value: "/" + strings.Join(parts[3:], "/")})
						}
					} else if parts[2] == "versions" && len(parts) > 4 {
						c.Params = append(c.Params, gin.Param{Key: "bucket", Value: parts[3]})
						c.Params = append(c.Params, gin.Param{Key: "key", Value: "/" + strings.Join(parts[4:], "/")})
					}
				}
			}

			// Route dispatch logic
			switch {
			case strings.Contains(tc.path, "/presigned") && !strings.Contains(tc.path, "Serve"):
				handler.GeneratePresignedURL(c)
			case strings.HasSuffix(tc.path, "/versioning"):
				handler.SetBucketVersioning(c)
			case strings.Contains(tc.path, "/versions/"):
				handler.ListVersions(c)
			case strings.Contains(tc.path, "/multipart") && strings.HasSuffix(tc.path, "/parts") && tc.method == http.MethodPut:
				handler.UploadPart(c)
			case strings.Contains(tc.path, "/multipart") && strings.HasSuffix(tc.path, "/complete"):
				handler.CompleteMultipartUpload(c)
			case strings.Contains(tc.path, "/multipart") && strings.Contains(tc.path, "/init"):
				handler.InitiateMultipartUpload(c)
			case strings.Contains(tc.path, "/multipart") && tc.method == http.MethodDelete && !strings.HasSuffix(tc.path, "/abort"):
				handler.AbortMultipartUpload(c)
			case strings.Contains(tc.path, "/multipart") && tc.method == http.MethodPost:
				if strings.HasSuffix(tc.path, "/multipart") {
					handler.InitiateMultipartUpload(c)
				}
			case strings.Contains(tc.path, "/buckets") && tc.method == http.MethodGet:
				handler.ListBuckets(c)
			case strings.Contains(tc.path, "/buckets") && tc.method == http.MethodDelete:
				handler.DeleteBucket(c)
			case tc.method == http.MethodPut:
				handler.Upload(c)
			case tc.method == http.MethodGet:
				handler.Download(c)
			}

			assert.Equal(t, tc.checkCode, w.Code)
		})
	}
}
