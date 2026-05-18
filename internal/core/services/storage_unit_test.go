package services_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}

func TestStorageServiceUnit(t *testing.T) {
	mockRepo := new(MockStorageRepo)
	mockStore := new(MockFileStore)
	mockAuditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cfg := &platform.Config{StorageSecret: "test-secret-key-32-chars-long-!!!"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewStorageService(services.StorageServiceParams{
		Repo:       mockRepo,
		RBACSvc:    rbacSvc,
		Store:      mockStore,
		AuditSvc:   mockAuditSvc,
		EncryptSvc: nil,
		Config:     cfg,
		Logger:     logger,
	})

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	t.Run("CreateBucket", func(t *testing.T) {
		mockRepo.On("CreateBucket", mock.Anything, mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "storage.bucket_create", "bucket", mock.Anything, mock.Anything).Return(nil).Once()

		bucket, err := svc.CreateBucket(ctx, "my-bucket", false)
		require.NoError(t, err)
		assert.NotNil(t, bucket)
		assert.Equal(t, "my-bucket", bucket.Name)
		mockRepo.AssertExpectations(t)
	})

	t.Run("CreateBucket Invalid Names", func(t *testing.T) {
		invalidNames := []string{"a", "ab", "Invalid_Name", "-start-hyphen", "end-dot.", "two..dots"}
		for _, name := range invalidNames {
			_, err := svc.CreateBucket(ctx, name, false)
			assert.Error(t, err, "expected error for bucket name: %s", name)
		}
	})

	t.Run("GetBucket", func(t *testing.T) {
		bucket := &domain.Bucket{Name: "my-bucket"}
		mockRepo.On("GetBucket", mock.Anything, "my-bucket").Return(bucket, nil).Once()

		res, err := svc.GetBucket(ctx, "my-bucket")
		require.NoError(t, err)
		assert.Equal(t, bucket, res)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DeleteBucket", func(t *testing.T) {
		mockRepo.On("List", mock.Anything, "my-bucket").Return([]*domain.Object{}, nil).Once()
		mockRepo.On("DeleteBucket", mock.Anything, "my-bucket").Return(nil).Once()

		err := svc.DeleteBucket(ctx, "my-bucket", false)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DeleteBucket Not Empty", func(t *testing.T) {
		mockRepo.On("List", mock.Anything, "my-bucket").Return([]*domain.Object{{Key: "k1"}}, nil).Once()

		err := svc.DeleteBucket(ctx, "my-bucket", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bucket is not empty")
		mockRepo.AssertExpectations(t)
	})

	t.Run("DeleteBucket Force", func(t *testing.T) {
		mockRepo.On("List", mock.Anything, "my-bucket").Return([]*domain.Object{{Key: "k1"}}, nil).Once()
		mockRepo.On("SoftDelete", mock.Anything, "my-bucket", "k1").Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "storage.object_delete", "storage", mock.Anything, mock.Anything).Return(nil).Once()
		mockRepo.On("DeleteBucket", mock.Anything, "my-bucket").Return(nil).Once()

		err := svc.DeleteBucket(ctx, "my-bucket", true)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("ListBuckets", func(t *testing.T) {
		buckets := []*domain.Bucket{{Name: "b1"}, {Name: "b2"}}
		mockRepo.On("ListBuckets", mock.Anything, userID.String()).Return(buckets, nil).Once()

		res, err := svc.ListBuckets(ctx)
		require.NoError(t, err)
		assert.Equal(t, buckets, res)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SetBucketVersioning", func(t *testing.T) {
		mockRepo.On("SetBucketVersioning", mock.Anything, "my-bucket", true).Return(nil).Once()

		err := svc.SetBucketVersioning(ctx, "my-bucket", true)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Upload", func(t *testing.T) {
		bucket := &domain.Bucket{Name: "my-bucket", VersioningEnabled: false}
		mockRepo.On("GetBucket", mock.Anything, "my-bucket").Return(bucket, nil).Once()
		mockStore.On("Write", mock.Anything, "my-bucket", "test.txt", mock.Anything).Return(int64(12), nil).Once()

		// First SaveMeta call for PENDING status
		mockRepo.On("SaveMeta", mock.Anything, mock.MatchedBy(func(obj *domain.Object) bool {
			return obj.UploadStatus == domain.UploadStatusPending && obj.SizeBytes == 0
		})).Return(nil).Once()

		// Second SaveMeta call for AVAILABLE status
		mockRepo.On("SaveMeta", mock.Anything, mock.MatchedBy(func(obj *domain.Object) bool {
			return obj.UploadStatus == domain.UploadStatusAvailable && obj.SizeBytes == 12
		})).Return(nil).Once()

		mockAuditSvc.On("Log", mock.Anything, userID, "storage.object_upload", "storage", mock.Anything, mock.Anything).Return(nil).Once()

		obj, err := svc.Upload(ctx, "my-bucket", "test.txt", strings.NewReader("hello world!"), "")
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.Equal(t, int64(12), obj.SizeBytes)
		assert.Equal(t, "text/plain; charset=utf-8", obj.ContentType)
		assert.NotEmpty(t, obj.Checksum)

		mockRepo.AssertExpectations(t)
		mockStore.AssertExpectations(t)
		mockAuditSvc.AssertExpectations(t)
	})

	t.Run("Upload Checksum Mismatch", func(t *testing.T) {
		bucket := &domain.Bucket{Name: "my-bucket", VersioningEnabled: false}
		mockRepo.On("GetBucket", mock.Anything, "my-bucket").Return(bucket, nil).Once()
		// SaveMeta for PENDING
		mockRepo.On("SaveMeta", mock.Anything, mock.MatchedBy(func(obj *domain.Object) bool {
			return obj.UploadStatus == domain.UploadStatusPending
		})).Return(nil).Once()

		mockStore.On("Write", mock.Anything, "my-bucket", "test.txt", mock.Anything).Return(int64(12), nil).Once()
		mockStore.On("Delete", mock.Anything, "my-bucket", "test.txt").Return(nil).Once()

		providedChecksum := "invalid-checksum"
		_, err := svc.Upload(ctx, "my-bucket", "test.txt", strings.NewReader("hello world!"), providedChecksum)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "data integrity check failed")

		mockRepo.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("Download", func(t *testing.T) {
		obj := &domain.Object{Bucket: "my-bucket", Key: "test.txt", VersionID: "", SizeBytes: 12}
		mockRepo.On("GetMeta", mock.Anything, "my-bucket", "test.txt").Return(obj, nil).Once()
		mockStore.On("Read", mock.Anything, "my-bucket", "test.txt").Return(io.NopCloser(strings.NewReader("hello world!")), nil).Once()
		mockRepo.On("GetBucket", mock.Anything, "my-bucket").Return(&domain.Bucket{Name: "my-bucket"}, nil).Once()

		reader, meta, err := svc.Download(ctx, "my-bucket", "test.txt")
		require.NoError(t, err)
		assert.NotNil(t, reader)
		assert.Equal(t, obj, meta)

		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, "hello world!", string(data))
		mockRepo.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("Download Bucket Lookup Failure Closes Reader", func(t *testing.T) {
		obj := &domain.Object{Bucket: "my-bucket", Key: "test.txt", VersionID: ""}
		reader := &trackingReadCloser{Reader: strings.NewReader("hello world!")}
		mockRepo.On("GetMeta", mock.Anything, "my-bucket", "test.txt").Return(obj, nil).Once()
		mockStore.On("Read", mock.Anything, "my-bucket", "test.txt").Return(reader, nil).Once()
		mockRepo.On("GetBucket", mock.Anything, "my-bucket").Return(nil, fmt.Errorf("bucket error")).Once()

		gotReader, meta, err := svc.Download(ctx, "my-bucket", "test.txt")
		require.Error(t, err)
		assert.Nil(t, gotReader)
		assert.Nil(t, meta)
		assert.Contains(t, err.Error(), "failed to get bucket")
		assert.True(t, reader.closed)
		mockRepo.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("ListObjects", func(t *testing.T) {
		objects := []*domain.Object{{Key: "test.txt"}}
		mockRepo.On("List", mock.Anything, "my-bucket").Return(objects, nil).Once()

		res, err := svc.ListObjects(ctx, "my-bucket")
		require.NoError(t, err)
		assert.Equal(t, objects, res)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DeleteObject", func(t *testing.T) {
		mockRepo.On("SoftDelete", mock.Anything, "my-bucket", "test.txt").Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "storage.object_delete", "storage", mock.Anything, mock.Anything).Return(nil).Once()

		err := svc.DeleteObject(ctx, "my-bucket", "test.txt")
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DownloadVersion", func(t *testing.T) {
		obj := &domain.Object{Bucket: "my-bucket", Key: "test.txt", VersionID: "v1", SizeBytes: 12}
		mockRepo.On("GetMetaByVersion", mock.Anything, "my-bucket", "test.txt", "v1").Return(obj, nil).Once()
		mockStore.On("Read", mock.Anything, "my-bucket", "test.txt?versionId=v1").Return(io.NopCloser(strings.NewReader("hello v1")), nil).Once()

		reader, meta, err := svc.DownloadVersion(ctx, "my-bucket", "test.txt", "v1")
		require.NoError(t, err)
		assert.NotNil(t, reader)
		assert.Equal(t, obj, meta)

		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		assert.Equal(t, "hello v1", string(data))
		mockRepo.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("ListVersions", func(t *testing.T) {
		versions := []*domain.Object{{Key: "test.txt", VersionID: "v1"}, {Key: "test.txt", VersionID: "v2"}}
		mockRepo.On("ListVersions", mock.Anything, "my-bucket", "test.txt").Return(versions, nil).Once()

		res, err := svc.ListVersions(ctx, "my-bucket", "test.txt")
		require.NoError(t, err)
		assert.Equal(t, versions, res)
		mockRepo.AssertExpectations(t)
	})

	t.Run("DeleteVersion", func(t *testing.T) {
		obj := &domain.Object{Bucket: "my-bucket", Key: "test.txt", VersionID: "v1"}
		mockRepo.On("GetMetaByVersion", mock.Anything, "my-bucket", "test.txt", "v1").Return(obj, nil).Once()
		mockStore.On("Delete", mock.Anything, "my-bucket", "test.txt?versionId=v1").Return(nil).Once()
		mockRepo.On("DeleteVersion", mock.Anything, "my-bucket", "test.txt", "v1").Return(nil).Once()

		err := svc.DeleteVersion(ctx, "my-bucket", "test.txt", "v1")
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("GetClusterStatus", func(t *testing.T) {
		status := &domain.StorageCluster{Nodes: []domain.StorageNode{{ID: "n1"}}}
		mockStore.On("GetClusterStatus", mock.Anything).Return(status, nil).Once()

		res, err := svc.GetClusterStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, status, res)
		mockStore.AssertExpectations(t)
	})

	t.Run("CleanupDeleted", func(t *testing.T) {
		deleted := []*domain.Object{{Bucket: "b1", Key: "k1", VersionID: ""}}
		mockRepo.On("ListDeleted", mock.Anything, 10).Return(deleted, nil).Once()
		mockStore.On("Delete", mock.Anything, "b1", "k1").Return(nil).Once()
		mockRepo.On("HardDelete", mock.Anything, "b1", "k1", "").Return(nil).Once()

		count, err := svc.CleanupDeleted(ctx, 10)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockRepo.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("CleanupPendingUploads", func(t *testing.T) {
		pending := []*domain.Object{{Bucket: "b1", Key: "k1", VersionID: ""}}
		mockRepo.On("ListPending", mock.Anything, mock.Anything, 10).Return(pending, nil).Once()
		mockStore.On("Delete", mock.Anything, "b1", "k1").Return(nil).Once()
		mockRepo.On("HardDelete", mock.Anything, "b1", "k1", "").Return(nil).Once()

		count, err := svc.CleanupPendingUploads(ctx, time.Hour, 10)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
		mockRepo.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("Multipart Lifecycle", func(t *testing.T) {
		uploadID := uuid.New()
		bucket := "my-bucket"
		key := "large.file"

		mockRepo.On("GetBucket", mock.Anything, bucket).Return(&domain.Bucket{Name: bucket}, nil).Once()
		mockRepo.On("SaveMultipartUpload", mock.Anything, mock.MatchedBy(func(u *domain.MultipartUpload) bool {
			return u.Bucket == bucket && u.Key == key
		})).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "storage.multipart_init", "storage", mock.Anything, mock.Anything).Return(nil).Once()

		mu, err := svc.CreateMultipartUpload(ctx, bucket, key)
		require.NoError(t, err)
		assert.NotNil(t, mu)

		mockRepo.On("GetMultipartUpload", mock.Anything, uploadID).Return(&domain.MultipartUpload{ID: uploadID, Bucket: bucket, Key: key}, nil).Once()
		mockStore.On("Write", mock.Anything, bucket, mock.Anything, mock.Anything).Return(int64(100), nil).Once()
		mockRepo.On("SavePart", mock.Anything, mock.MatchedBy(func(p *domain.Part) bool {
			return p.UploadID == uploadID && p.PartNumber == 1
		})).Return(nil).Once()

		part, err := svc.UploadPart(ctx, uploadID, 1, strings.NewReader("part data"), "")
		require.NoError(t, err)
		assert.NotNil(t, part)

		mockRepo.On("GetMultipartUpload", mock.Anything, uploadID).Return(&domain.MultipartUpload{ID: uploadID, Bucket: bucket, Key: key, UserID: userID}, nil).Once()
		parts := []*domain.Part{{PartNumber: 1, SizeBytes: 100}}
		mockRepo.On("ListParts", mock.Anything, uploadID).Return(parts, nil).Once()
		mockRepo.On("GetBucket", mock.Anything, bucket).Return(&domain.Bucket{Name: bucket}, nil).Once()
		mockStore.On("Assemble", mock.Anything, bucket, key, mock.Anything).Return(int64(100), nil).Once()
		mockStore.On("Read", mock.Anything, bucket, key).Return(io.NopCloser(strings.NewReader("part data")), nil).Once()
		mockRepo.On("SaveMeta", mock.Anything, mock.MatchedBy(func(obj *domain.Object) bool {
			return obj.UploadStatus == domain.UploadStatusAvailable && obj.SizeBytes == 100
		})).Return(nil).Once()
		mockRepo.On("DeleteMultipartUpload", mock.Anything, uploadID).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "storage.multipart_complete", "storage", mock.Anything, mock.Anything).Return(nil).Once()

		obj, err := svc.CompleteMultipartUpload(ctx, uploadID)
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.Equal(t, int64(100), obj.SizeBytes)

		mockRepo.AssertExpectations(t)
		mockStore.AssertExpectations(t)
		mockAuditSvc.AssertExpectations(t)
	})

	t.Run("AbortMultipartUpload", func(t *testing.T) {
		uploadID := uuid.New()
		bucket := "my-bucket"
		mockRepo.On("GetMultipartUpload", mock.Anything, uploadID).Return(&domain.MultipartUpload{ID: uploadID, Bucket: bucket}, nil).Once()
		parts := []*domain.Part{{PartNumber: 1}}
		mockRepo.On("ListParts", mock.Anything, uploadID).Return(parts, nil).Once()
		mockStore.On("Delete", mock.Anything, bucket, mock.Anything).Return(nil).Once()
		mockRepo.On("DeleteMultipartUpload", mock.Anything, uploadID).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "storage.multipart_abort", "storage", uploadID.String(), mock.Anything).Return(nil).Once()

		err := svc.AbortMultipartUpload(ctx, uploadID)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
		mockStore.AssertExpectations(t)
		mockAuditSvc.AssertExpectations(t)
	})

	t.Run("GeneratePresignedURL", func(t *testing.T) {
		bucket := "my-bucket"
		key := "file.txt"
		mockRepo.On("GetBucket", mock.Anything, bucket).Return(&domain.Bucket{Name: bucket}, nil).Once()

		res, err := svc.GeneratePresignedURL(ctx, bucket, key, "GET", time.Hour)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.URL, "/storage/presigned/my-bucket/file.txt")
		mockRepo.AssertExpectations(t)
	})

	t.Run("GetBucket_RepoError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		mockRepo.On("GetBucket", mock.Anything, "missing").Return(nil, fmt.Errorf("not found")).Once()
		_, err := svc.GetBucket(ctx, "missing")
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SetBucketVersioning_RepoError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		mockRepo.On("SetBucketVersioning", mock.Anything, "b", true).Return(fmt.Errorf("db error")).Once()
		err := svc.SetBucketVersioning(ctx, "b", true)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("GetClusterStatus_StoreError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		mockStore.On("GetClusterStatus", mock.Anything).Return(nil, fmt.Errorf("cluster unreachable")).Once()
		_, err := svc.GetClusterStatus(ctx)
		require.Error(t, err)
		mockStore.AssertExpectations(t)
	})

	t.Run("CleanupDeleted_ListDeletedError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		mockRepo.On("ListDeleted", mock.Anything, 10).Return(nil, fmt.Errorf("db error")).Once()
		_, err := svc.CleanupDeleted(ctx, 10)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("CleanupDeleted_HardDeleteError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		deleted := []*domain.Object{{Bucket: "b", Key: "k", VersionID: ""}}
		mockRepo.On("ListDeleted", mock.Anything, 10).Return(deleted, nil).Once()
		mockStore.On("Delete", mock.Anything, "b", "k").Return(nil).Once()
		mockRepo.On("HardDelete", mock.Anything, "b", "k", "").Return(fmt.Errorf("hard delete failed")).Once()
		_, err := svc.CleanupDeleted(ctx, 10)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "hard delete failed")
		mockRepo.AssertExpectations(t)
	})

	t.Run("CleanupPendingUploads_ListPendingError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		mockRepo.On("ListPending", mock.Anything, mock.Anything, 10).Return(nil, fmt.Errorf("db error")).Once()
		_, err := svc.CleanupPendingUploads(ctx, time.Hour, 10)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("CreateMultipartUpload_BucketNotFound", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		mockRepo.On("GetBucket", mock.Anything, "nonexistent").Return(nil, fmt.Errorf("not found")).Once()
		_, err := svc.CreateMultipartUpload(ctx, "nonexistent", "key")
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("UploadPart_NotFound", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		fakeID := uuid.New()
		mockRepo.On("GetMultipartUpload", mock.Anything, fakeID).Return(nil, fmt.Errorf("not found")).Once()
		_, err := svc.UploadPart(ctx, fakeID, 1, strings.NewReader("data"), "")
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("UploadPart_ChecksumMismatch", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		upload := &domain.MultipartUpload{ID: uuid.New(), Bucket: "b", Key: "k"}
		mockRepo.On("GetMultipartUpload", mock.Anything, upload.ID).Return(upload, nil).Once()
		mockStore.On("Write", mock.Anything, "b", mock.Anything, mock.Anything).Return(int64(5), nil).Once()
		mockStore.On("Delete", mock.Anything, "b", mock.Anything).Return(nil).Once()
		_, err := svc.UploadPart(ctx, upload.ID, 1, strings.NewReader("data"), "wrong-checksum")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "checksum")
		mockRepo.AssertExpectations(t)
	})

	t.Run("CompleteMultipartUpload_NotFound", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		fakeID := uuid.New()
		mockRepo.On("GetMultipartUpload", mock.Anything, fakeID).Return(nil, fmt.Errorf("not found")).Once()
		_, err := svc.CompleteMultipartUpload(ctx, fakeID)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("CompleteMultipartUpload_NoParts", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		upload := &domain.MultipartUpload{ID: uuid.New(), Bucket: "b", Key: "k", UserID: userID}
		mockRepo.On("GetMultipartUpload", mock.Anything, upload.ID).Return(upload, nil).Once()
		mockRepo.On("ListParts", mock.Anything, upload.ID).Return([]*domain.Part{}, nil).Once()
		_, err := svc.CompleteMultipartUpload(ctx, upload.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no parts")
		mockRepo.AssertExpectations(t)
	})

	t.Run("CompleteMultipartUpload_BucketNotFound", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		upload := &domain.MultipartUpload{ID: uuid.New(), Bucket: "b", Key: "k", UserID: userID}
		parts := []*domain.Part{{PartNumber: 1, SizeBytes: 100}}
		mockRepo.On("GetMultipartUpload", mock.Anything, upload.ID).Return(upload, nil).Once()
		mockRepo.On("ListParts", mock.Anything, upload.ID).Return(parts, nil).Once()
		mockRepo.On("GetBucket", mock.Anything, "b").Return(nil, fmt.Errorf("not found")).Once()
		_, err := svc.CompleteMultipartUpload(ctx, upload.ID)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("AbortMultipartUpload_DeleteError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		upload := &domain.MultipartUpload{ID: uuid.New(), Bucket: "b", Key: "k", UserID: userID}
		mockRepo.On("GetMultipartUpload", mock.Anything, upload.ID).Return(upload, nil).Once()
		mockRepo.On("ListParts", mock.Anything, upload.ID).Return([]*domain.Part{{PartNumber: 1}}, nil).Once()
		mockStore.On("Delete", mock.Anything, "b", mock.Anything).Return(nil).Once()
		mockRepo.On("DeleteMultipartUpload", mock.Anything, upload.ID).Return(fmt.Errorf("db error")).Once()
		err := svc.AbortMultipartUpload(ctx, upload.ID)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("GeneratePresignedURL_BucketNotFound", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		mockRepo.On("GetBucket", mock.Anything, "nonexistent").Return(nil, fmt.Errorf("not found")).Once()
		_, err := svc.GeneratePresignedURL(ctx, "nonexistent", "key", "GET", time.Hour)
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("GeneratePresignedURL_MissingSecret", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		cfgNoSecret := &platform.Config{StorageSecret: ""}
		svcNoSecret := services.NewStorageService(services.StorageServiceParams{
			Repo: mockRepo, RBACSvc: rbacSvc, Store: mockStore,
			AuditSvc: mockAuditSvc, EncryptSvc: nil, Config: cfgNoSecret, Logger: logger,
		})
		mockRepo.On("GetBucket", mock.Anything, "b").Return(&domain.Bucket{Name: "b"}, nil).Once()
		_, err := svcNoSecret.GeneratePresignedURL(ctx, "b", "key", "GET", time.Hour)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("DeleteVersion_StoreError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		obj := &domain.Object{Bucket: "b", Key: "k", VersionID: "v1"}
		mockRepo.On("GetMetaByVersion", mock.Anything, "b", "k", "v1").Return(obj, nil).Once()
		mockStore.On("Delete", mock.Anything, "b", "k?versionId=v1").Return(fmt.Errorf("disk error")).Once()
		err := svc.DeleteVersion(ctx, "b", "k", "v1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "disk error")
		mockRepo.AssertExpectations(t)
	})

	t.Run("ListObjects_RepoError", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil
		mockRepo.On("List", mock.Anything, "b").Return(nil, fmt.Errorf("db error")).Once()
		_, err := svc.ListObjects(ctx, "b")
		require.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("RBAC_AuthorizationFailures", func(t *testing.T) {
		denyRbac := new(MockRBACService)
		denyRbac.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(fmt.Errorf("permission denied")).Maybe()
		svcDeny := services.NewStorageService(services.StorageServiceParams{
			Repo: mockRepo, RBACSvc: denyRbac, Store: mockStore,
			AuditSvc: mockAuditSvc, EncryptSvc: nil, Config: cfg, Logger: logger,
		})

		_, err := svcDeny.CreateBucket(ctx, "b", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")

		_, _, err = svcDeny.Download(ctx, "b", "k")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")

		_, err = svcDeny.ListObjects(ctx, "b")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")

		denyRbac.AssertExpectations(t)
	})

	t.Run("Additional Error Paths", func(t *testing.T) {
		mockRepo.ExpectedCalls = nil
		mockStore.ExpectedCalls = nil

		mockRepo.On("GetMeta", mock.Anything, "b", "k").Return(&domain.Object{Bucket: "b", Key: "k", VersionID: ""}, nil).Once()
		mockStore.On("Read", mock.Anything, "b", "k").Return(io.NopCloser(strings.NewReader("data")), nil).Once()
		mockRepo.On("GetBucket", mock.Anything, "b").Return(nil, fmt.Errorf("bucket error")).Once()
		_, _, err := svc.Download(ctx, "b", "k")
		require.Error(t, err)

		mockRepo.On("GetBucket", mock.Anything, "non-existent").Return(nil, fmt.Errorf("not found")).Once()
		_, err = svc.CreateMultipartUpload(ctx, "non-existent", "k")
		assert.Error(t, err)
	})
}
