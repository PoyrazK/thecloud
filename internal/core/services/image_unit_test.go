package services_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestImageService_Unit(t *testing.T) {
	repo := new(MockImageRepo)
	fileStore := new(MockFileStore)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	svc := services.NewImageService(services.ImageServiceParams{
		Repo:      repo,
		RBACSvc:   rbacSvc,
		FileStore: fileStore,
		Logger:    slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	t.Run("RegisterImage", func(t *testing.T) {
		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		img, err := svc.RegisterImage(ctx, "ubuntu-custom", "desc", "linux", "22.04", false)
		require.NoError(t, err)
		assert.NotNil(t, img)
		assert.Equal(t, "ubuntu-custom", img.Name)
	})

	t.Run("UploadImage", func(t *testing.T) {
		imgID := uuid.New()
		img := &domain.Image{ID: imgID, UserID: userID}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()
		fileStore.On("Write", mock.Anything, "images", mock.Anything, mock.Anything).Return(int64(1024*1024*1024), nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusActive && i.SizeGB == 1
		})).Return(nil).Once()

		err := svc.UploadImage(ctx, imgID, bytes.NewReader([]byte("dummy content")))
		require.NoError(t, err)
	})

	t.Run("UploadImage_NotFound", func(t *testing.T) {
		imgID := uuid.New()
		repo.On("GetByID", mock.Anything, imgID).Return(nil, fmt.Errorf("not found")).Once()

		err := svc.UploadImage(ctx, imgID, bytes.NewReader([]byte("foo")))
		require.Error(t, err)
	})

	t.Run("ListImages", func(t *testing.T) {
		repo.On("List", mock.Anything, userID, true).Return([]*domain.Image{{ID: uuid.New()}}, nil).Once()
		res, err := svc.ListImages(ctx, userID, true)
		require.NoError(t, err)
		assert.Len(t, res, 1)
	})

	t.Run("DeleteImage", func(t *testing.T) {
		imgID := uuid.New()
		img := &domain.Image{ID: imgID, UserID: userID, FilePath: "img-path"}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()
		repo.On("Delete", mock.Anything, imgID).Return(nil).Once()
		fileStore.On("Delete", mock.Anything, "images", "img-path").Return(nil).Once()

		err := svc.DeleteImage(ctx, imgID)
		require.NoError(t, err)
	})

	t.Run("GetImage_Success", func(t *testing.T) {
		imgID := uuid.New()
		tenantID := uuid.New()
		tenantCtx := appcontext.WithTenantID(ctx, tenantID)
		img := &domain.Image{ID: imgID, UserID: userID, TenantID: &tenantID}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()

		res, err := svc.GetImage(tenantCtx, imgID)
		require.NoError(t, err)
		assert.Equal(t, imgID, res.ID)
	})

	t.Run("GetImage_RepoError", func(t *testing.T) {
		imgID := uuid.New()
		repo.On("GetByID", mock.Anything, imgID).Return(nil, fmt.Errorf("db error")).Once()

		_, err := svc.GetImage(ctx, imgID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("GetImage_TenantMismatch", func(t *testing.T) {
		imgID := uuid.New()
		tenantID := uuid.New()
		otherTenantID := uuid.New()
		tenantCtx := appcontext.WithTenantID(ctx, tenantID)
		img := &domain.Image{ID: imgID, UserID: userID, TenantID: &otherTenantID}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()

		_, err := svc.GetImage(tenantCtx, imgID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("RegisterImage_RepoError", func(t *testing.T) {
		repo.On("Create", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Once()

		_, err := svc.RegisterImage(ctx, "ubuntu-custom", "desc", "linux", "22.04", false)
		require.Error(t, err)
	})

	t.Run("DeleteImage_RepoGetError", func(t *testing.T) {
		imgID := uuid.New()
		repo.On("GetByID", mock.Anything, imgID).Return(nil, fmt.Errorf("db error")).Once()

		err := svc.DeleteImage(ctx, imgID)
		require.Error(t, err)
	})

	t.Run("ListImages_Success", func(t *testing.T) {
		images := []*domain.Image{
			{ID: uuid.New(), UserID: userID},
			{ID: uuid.New(), UserID: userID},
		}
		repo.On("List", mock.Anything, userID, true).Return(images, nil).Once()

		res, err := svc.ListImages(ctx, userID, true)
		require.NoError(t, err)
		assert.Len(t, res, 2)
	})

	t.Run("ImportImage_Success", func(t *testing.T) {
		// QCOW2 magic bytes + padding
		qcow2Magic := []byte{0x51, 0x46, 0x44, 0xbf}
		payload := make([]byte, 1024*1024)
		copy(payload, qcow2Magic)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		}))
		defer server.Close()

		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusActive
		})).Return(nil).Once()

		fileStore.On("Write", mock.Anything, "images", mock.Anything, mock.Anything).Return(int64(1024*1024), nil).Once()

		img, err := svc.ImportImage(ctx, "my-image", server.URL, "desc", "linux", "22.04", false)
		require.NoError(t, err)
		assert.Equal(t, "my-image", img.Name)
		assert.Equal(t, server.URL, img.SourceURL)
		assert.Equal(t, domain.ImageStatusActive, img.Status)
	})

	t.Run("ImportImage_StoreError", func(t *testing.T) {
		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		// importFromURL fails → first Update with Error status
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusError
		})).Return(nil).Once()

		fileStore.On("Write", mock.Anything, "images", mock.Anything, mock.Anything).Return(int64(0), fmt.Errorf("store error")).Once()

		_, err := svc.ImportImage(ctx, "my-image", "https://example.com/image.qcow2", "desc", "linux", "22.04", false)
		require.Error(t, err)
	})

	t.Run("ImportImage_ServerError", func(t *testing.T) {
		// Non-200 response
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusError
		})).Return(nil).Once()

		_, err := svc.ImportImage(ctx, "fail-img", server.URL, "desc", "linux", "1.0", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})
}

func TestImageService_Unit_RBACErrors(t *testing.T) {
	// Each test gets its own service with a mock that returns errors on Authorize
	for _, tc := range []struct {
		name string
		test func(*testing.T, ports.ImageService, *MockImageRepo, *MockFileStore, *MockRBACService)
	}{
		{"RegisterImage_RBACDenied", testRegisterImageRBACDenied},
		{"UploadImage_RBACDenied", testUploadImageRBACDenied},
		{"GetImage_RBACDenied", testGetImageRBACDenied},
		{"ListImages_RBACDenied", testListImagesRBACDenied},
		{"DeleteImage_RBACDenied", testDeleteImageRBACDenied},
		{"ImportImage_RBACDenied", testImportImageRBACDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := new(MockImageRepo)
			fileStore := new(MockFileStore)
			rbacSvc := new(MockRBACService)
			svc := services.NewImageService(services.ImageServiceParams{
				Repo:      repo,
				RBACSvc:   rbacSvc,
				FileStore: fileStore,
				Logger:    slog.Default(),
			})
			tc.test(t, svc, repo, fileStore, rbacSvc)
		})
	}
}

func testRegisterImageRBACDenied(t *testing.T, svc ports.ImageService, repo *MockImageRepo, fs *MockFileStore, rbacSvc *MockRBACService) {
	t.Helper()
	ctx := context.Background()
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, domain.PermissionImageCreate, "*").
		Return(errors.New(errors.Forbidden, "denied")).Once()
	_, err := svc.RegisterImage(ctx, "name", "desc", "linux", "22.04", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func testUploadImageRBACDenied(t *testing.T, svc ports.ImageService, repo *MockImageRepo, fs *MockFileStore, rbacSvc *MockRBACService) {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, domain.PermissionImageCreate, id.String()).
		Return(errors.New(errors.Forbidden, "denied")).Once()
	err := svc.UploadImage(ctx, id, bytes.NewReader(nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func testGetImageRBACDenied(t *testing.T, svc ports.ImageService, repo *MockImageRepo, fs *MockFileStore, rbacSvc *MockRBACService) {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, domain.PermissionImageRead, id.String()).
		Return(errors.New(errors.Forbidden, "denied")).Once()
	_, err := svc.GetImage(ctx, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func testListImagesRBACDenied(t *testing.T, svc ports.ImageService, repo *MockImageRepo, fs *MockFileStore, rbacSvc *MockRBACService) {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, domain.PermissionImageRead, "*").
		Return(errors.New(errors.Forbidden, "denied")).Once()
	_, err := svc.ListImages(ctx, userID, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func testDeleteImageRBACDenied(t *testing.T, svc ports.ImageService, repo *MockImageRepo, fs *MockFileStore, rbacSvc *MockRBACService) {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, domain.PermissionImageDelete, id.String()).
		Return(errors.New(errors.Forbidden, "denied")).Once()
	err := svc.DeleteImage(ctx, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func testImportImageRBACDenied(t *testing.T, svc ports.ImageService, repo *MockImageRepo, fs *MockFileStore, rbacSvc *MockRBACService) {
	t.Helper()
	ctx := context.Background()
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, domain.PermissionImageCreate, "*").
		Return(errors.New(errors.Forbidden, "denied")).Once()
	_, err := svc.ImportImage(ctx, "name", "https://example.com/img.qcow2", "desc", "linux", "1.0", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "denied")
}

func TestImageService_Unit_ErrorPaths(t *testing.T) {
	repo := new(MockImageRepo)
	fileStore := new(MockFileStore)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	svc := services.NewImageService(services.ImageServiceParams{
		Repo:      repo,
		RBACSvc:   rbacSvc,
		FileStore: fileStore,
		Logger:    slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	t.Run("UploadImage_NotOwner", func(t *testing.T) {
		imgID := uuid.New()
		otherUser := uuid.New()
		img := &domain.Image{ID: imgID, UserID: otherUser}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()
		err := svc.UploadImage(ctx, imgID, bytes.NewReader(nil))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "someone else's")
	})

	t.Run("UploadImage_TenantMismatch", func(t *testing.T) {
		imgID := uuid.New()
		tenantID := uuid.New()
		otherTenantID := uuid.New()
		tenantCtx := appcontext.WithTenantID(ctx, tenantID)
		img := &domain.Image{ID: imgID, UserID: userID, TenantID: &otherTenantID}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()
		err := svc.UploadImage(tenantCtx, imgID, bytes.NewReader(nil))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("DeleteImage_NotOwner", func(t *testing.T) {
		imgID := uuid.New()
		otherUser := uuid.New()
		img := &domain.Image{ID: imgID, UserID: otherUser}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()
		err := svc.DeleteImage(ctx, imgID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "someone else's")
	})

	t.Run("DeleteImage_FileDeleteError", func(t *testing.T) {
		imgID := uuid.New()
		img := &domain.Image{ID: imgID, UserID: userID, FilePath: "some/path"}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()
		fileStore.On("Delete", mock.Anything, "images", "some/path").Return(fmt.Errorf("delete failed")).Once()
		err := svc.DeleteImage(ctx, imgID)
		require.Error(t, err)
	})

	t.Run("ImportImage_CreateFails", func(t *testing.T) {
		repo.On("Create", mock.Anything, mock.Anything).Return(fmt.Errorf("db error")).Maybe()
		_, err := svc.ImportImage(ctx, "name", "https://example.com/img.qcow2", "desc", "linux", "1.0", false)
		require.Error(t, err)
	})

	t.Run("ImportImage_InvalidURL", func(t *testing.T) {
		// http.NewRequestWithContext returns error for an invalid URL
		_, err := svc.ImportImage(ctx, "name", "not-a-valid-url", "desc", "linux", "1.0", false)
		require.Error(t, err)
	})

	t.Run("ImportImage_InvalidScheme", func(t *testing.T) {
		// url.Parse succeeds for ftp:// but scheme is not http/https
		_, err := svc.ImportImage(ctx, "name", "ftp://example.com/img.qcow2", "desc", "linux", "1.0", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid URL scheme")
	})

	t.Run("UploadImage_UpdateFails", func(t *testing.T) {
		imgID := uuid.New()
		img := &domain.Image{ID: imgID, UserID: userID}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()
		fileStore.On("Write", mock.Anything, "images", mock.Anything, mock.Anything).Return(int64(1024*1024*1024), nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusActive && i.SizeGB == 1
		})).Return(fmt.Errorf("db error")).Once()

		err := svc.UploadImage(ctx, imgID, bytes.NewReader([]byte("content")))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("DeleteImage_EmptyFilePath", func(t *testing.T) {
		imgID := uuid.New()
		img := &domain.Image{ID: imgID, UserID: userID, FilePath: ""}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()
		repo.On("Delete", mock.Anything, imgID).Return(nil).Once()

		err := svc.DeleteImage(ctx, imgID)
		require.NoError(t, err)
	})

	t.Run("DeleteImage_DeleteFails", func(t *testing.T) {
		imgID := uuid.New()
		img := &domain.Image{ID: imgID, UserID: userID, FilePath: "some/path"}
		repo.On("GetByID", mock.Anything, imgID).Return(img, nil).Once()
		fileStore.On("Delete", mock.Anything, "images", "some/path").Return(nil).Once()
		repo.On("Delete", mock.Anything, imgID).Return(fmt.Errorf("db error")).Once()

		err := svc.DeleteImage(ctx, imgID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}

func TestImageService_Unit_ImportImage(t *testing.T) {
	repo := new(MockImageRepo)
	fileStore := new(MockFileStore)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	svc := services.NewImageService(services.ImageServiceParams{
		Repo:      repo,
		RBACSvc:   rbacSvc,
		FileStore: fileStore,
		Logger:    slog.Default(),
	})

	ctx := context.Background()
	ctx = appcontext.WithUserID(ctx, uuid.New())

	t.Run("FormatIMG", func(t *testing.T) {
		// .img format has no magic bytes - any data is valid
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bytes.Repeat([]byte("x"), 1024))
		}))
		defer server.Close()

		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusActive
		})).Return(nil).Once()
		fileStore.On("Write", mock.Anything, "images", mock.Anything, mock.Anything).Return(int64(1024*1024), nil).Once()

		img, err := svc.ImportImage(ctx, "img", strings.TrimSuffix(server.URL, "/")+"/ubuntu.img", "desc", "linux", "1.0", false)
		require.NoError(t, err)
		assert.Equal(t, "img", img.Format)
	})

	t.Run("FormatRaw", func(t *testing.T) {
		// .raw format has no magic bytes - any data is valid
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bytes.Repeat([]byte("x"), 1024))
		}))
		defer server.Close()

		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusActive
		})).Return(nil).Once()
		fileStore.On("Write", mock.Anything, "images", mock.Anything, mock.Anything).Return(int64(1024*1024), nil).Once()

		img, err := svc.ImportImage(ctx, "raw", strings.TrimSuffix(server.URL, "/")+"/disk.raw", "desc", "linux", "1.0", false)
		require.NoError(t, err)
		assert.Equal(t, "raw", img.Format)
	})

	t.Run("FormatISO", func(t *testing.T) {
		// CD-ROM magic bytes
		isoMagic := []byte{0x43, 0x44, 0x30, 0x30, 0x31}
		payload := make([]byte, 1024)
		copy(payload, isoMagic)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-iso9660-image")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		}))
		defer server.Close()

		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusActive
		})).Return(nil).Once()
		fileStore.On("Write", mock.Anything, "images", mock.Anything, mock.Anything).Return(int64(1024*1024), nil).Once()

		img, err := svc.ImportImage(ctx, "iso", strings.TrimSuffix(server.URL, "/")+"/installer.iso", "desc", "linux", "1.0", false)
		require.NoError(t, err)
		assert.Equal(t, "iso", img.Format)
	})

	t.Run("UpdateToActiveFails", func(t *testing.T) {
		qcow2Magic := []byte{0x51, 0x46, 0x44, 0xbf}
		payload := make([]byte, 1024*1024)
		copy(payload, qcow2Magic)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		}))
		defer server.Close()

		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusActive
		})).Return(fmt.Errorf("db error")).Once()
		fileStore.On("Write", mock.Anything, "images", mock.Anything, mock.Anything).Return(int64(1024*1024), nil).Once()

		_, err := svc.ImportImage(ctx, "my-image", server.URL, "desc", "linux", "1.0", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("NetworkError", func(t *testing.T) {
		deadlineCtx, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Hour))
		defer cancel()

		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusError
		})).Return(nil).Once()

		_, err := svc.ImportImage(deadlineCtx, "my-image", "http://localhost:1/image.qcow2", "desc", "linux", "1.0", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch image")
	})

	t.Run("ImportImage_ExceedsMaxSize", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", "99999999999999") // >> 10GB
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusError
		})).Return(nil).Once()

		_, err := svc.ImportImage(ctx, "big-img", server.URL+"/image.qcow2", "desc", "linux", "1.0", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds max size")
	})

	t.Run("ImportImage_InvalidContentType", func(t *testing.T) {
		qcow2Magic := []byte{0x51, 0x46, 0x44, 0xbf}
		payload := make([]byte, 1024*1024)
		copy(payload, qcow2Magic)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html") // not allowed
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		}))
		defer server.Close()

		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusError
		})).Return(nil).Once()

		_, err := svc.ImportImage(ctx, "bad-ct", server.URL+"/image.qcow2", "desc", "linux", "1.0", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid content-type")
	})

	t.Run("ImportImage_WrongMagicBytes", func(t *testing.T) {
		// JPEG magic bytes for a .qcow2 URL — format mismatch
		jpegMagic := []byte{0xff, 0xd8, 0xff, 0xe0}
		payload := make([]byte, 1024*1024)
		copy(payload, jpegMagic)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		}))
		defer server.Close()

		repo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Image) bool {
			return i.Status == domain.ImageStatusError
		})).Return(nil).Once()

		_, err := svc.ImportImage(ctx, "spoofed", server.URL+"/image.qcow2", "desc", "linux", "1.0", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid magic bytes")
	})
}

func TestImageService_Unit_ListImages_OtherUserNoReadAll(t *testing.T) {
	repo := new(MockImageRepo)
	fileStore := new(MockFileStore)
	rbacSvc := new(MockRBACService)
	svc := services.NewImageService(services.ImageServiceParams{
		Repo:      repo,
		RBACSvc:   rbacSvc,
		FileStore: fileStore,
		Logger:    slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	otherUser := uuid.New()
	userCtx := appcontext.WithUserID(ctx, userID)

	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, domain.PermissionImageRead, "*").
		Return(nil).Once()
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, domain.PermissionImageReadAll, "*").
		Return(errors.New(errors.Forbidden, "cannot list for another user")).Once()
	_, err := svc.ListImages(userCtx, otherUser, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "another user")
}
