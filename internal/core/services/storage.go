// Package services implements core business workflows.
package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	go_errors "errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/pkg/crypto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerNameStorage = "storage-service"

const (
	errMultipartNotFound = "multipart upload not found"
	partPathFormat       = ".uploads/%s/part-%d"
	versionQueryFormat   = "%s?versionId=%s"
	versionEpochBit      = 1 << 62
	sniffLen             = 512
	maxPartSize          = 5 * 1024 * 1024 * 1024 // 5 GB per part
)

var validBucketNameRe = regexp.MustCompile(`^[a-z0-9.-]+$`)

// boundedReader reads from r but returns ObjectTooLarge if more than limit bytes
// are consumed, ensuring oversized uploads fail with an explicit error rather than
// silent truncation. It uses a 1-byte probe to distinguish "object exactly at
// limit" (underlying EOF on next Read) from "object exceeds limit" (extra data).
type boundedReader struct {
	r         io.Reader
	limit     int64
	count     int64
	cleanupFn func() // cleanup partial object on oversize; may be nil
}

func (b *boundedReader) Read(p []byte) (n int, err error) {
	// Already exceeded limit — propagate error immediately.
	if b.count > b.limit {
		if b.cleanupFn != nil {
			b.cleanupFn()
		}
		return 0, errors.New(errors.ObjectTooLarge, "object exceeds maximum size")
	}

	remaining := b.limit - b.count
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}

	// Read up to 'remaining' bytes from underlying.
	n, err = b.r.Read(p)
	b.count += int64(n)

	// Detect overflow: we read past the limit.
	if b.count > b.limit {
		if b.cleanupFn != nil {
			b.cleanupFn()
		}
		// Return 0 to signal no valid data transferred; error conveys cause.
		return 0, errors.New(errors.ObjectTooLarge, "object exceeds maximum size")
	}

	// count == limit — signal limit reached.
	// If underlying error is already EOF, return it directly.
	// Otherwise probe to distinguish "exact limit + underlying done" (EOF)
	// from "exact limit + underlying has more" (overflow).
	if b.count == b.limit {
		if err == io.EOF {
			return 0, io.EOF
		}
		// Underlying may have more data; probe with 1 byte.
		// Note: the probe reads from the already-encrypted stream (boundedReader
		// sits above the encryption layer in Upload), so it does not interfere
		// with decryption state.
		probe := [1]byte{}
		_, probeErr := b.r.Read(probe[:])
		if probeErr == io.EOF {
			// Underlying exhausted at exactly the limit — clean EOF.
			return 0, io.EOF
		}
		// Extra data exists beyond the limit — overflow.
		if b.cleanupFn != nil {
			b.cleanupFn()
		}
		return 0, errors.New(errors.ObjectTooLarge, "object exceeds maximum size")
	}

	return n, err
}

// generateVersionID generates a timestamp-based version ID (reverse chronological).
func generateVersionID() string {
	return fmt.Sprintf("%d", versionEpochBit-time.Now().UnixNano())
}

// versionedStoreKey returns the store key with version ID suffix.
func versionedStoreKey(key, versionID string) string {
	return fmt.Sprintf(versionQueryFormat, key, versionID)
}

// StorageServiceParams defines the dependencies for StorageService.
type StorageServiceParams struct {
	Repo       ports.StorageRepository
	RBACSvc    ports.RBACService
	Store      ports.FileStore
	AuditSvc   ports.AuditService
	EncryptSvc ports.EncryptionService
	Config     *platform.Config
	Logger     *slog.Logger
}

// StorageService manages object storage metadata and files.
type StorageService struct {
	repo       ports.StorageRepository
	rbacSvc    ports.RBACService
	store      ports.FileStore
	auditSvc   ports.AuditService
	encryptSvc ports.EncryptionService
	cfg        *platform.Config
	logger     *slog.Logger
}

// NewStorageService constructs a StorageService with its dependencies.
func NewStorageService(params StorageServiceParams) *StorageService {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &StorageService{
		repo:       params.Repo,
		rbacSvc:    params.RBACSvc,
		store:      params.Store,
		auditSvc:   params.AuditSvc,
		encryptSvc: params.EncryptSvc,
		cfg:        params.Config,
		logger:     logger,
	}
}

func (s *StorageService) Upload(ctx context.Context, bucketName, key string, r io.Reader, providedChecksum string) (*domain.Object, error) {
	tracer := otel.Tracer(tracerNameStorage)
	_, span := tracer.Start(ctx, "StorageService.Upload",
		trace.WithAttributes(
			attribute.String("storage.bucket", bucketName),
			attribute.String("storage.key", key),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageWrite, "*"); err != nil {
		span.RecordError(err)
		return nil, err
	}

	// 1. Check bucket versioning status
	bucket, err := s.repo.GetBucket(ctx, bucketName)
	if err != nil {
		return nil, err
	}

	versionID := "" // Default version ID when versioning is disabled
	if bucket.VersioningEnabled {
		versionID = generateVersionID()
	}

	// 2. Sniff content-type (first 512 bytes)
	sniffBuf := make([]byte, sniffLen)
	n, err := io.ReadFull(r, sniffBuf)
	if err != nil && !go_errors.Is(err, io.EOF) && !go_errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, errors.Wrap(errors.Internal, "failed to read for MIME sniffing", err)
	}
	sniffBuf = sniffBuf[:n]
	contentType := http.DetectContentType(sniffBuf)

	// Combine back for full streaming
	fullReader := io.MultiReader(bytes.NewReader(sniffBuf), r)

	// 3. Prepare initial metadata (PENDING status)
	obj := &domain.Object{
		ID:           uuid.New(),
		UserID:       userID,
		TenantID:     tenantID,
		Bucket:       bucketName,
		Key:          key,
		VersionID:    versionID,
		IsLatest:     true,
		SizeBytes:    0,
		ContentType:  contentType,
		UploadStatus: domain.UploadStatusPending,
		CreatedAt:    time.Now(),
	}

	// Generate ARN
	obj.ARN = fmt.Sprintf("arn:thecloud:storage:local:default:object/%s/%s", bucketName, key)
	if bucket.VersioningEnabled {
		obj.ARN += fmt.Sprintf("?versionId=%s", versionID)
	}

	// Save preliminary metadata
	if err := s.repo.SaveMeta(ctx, obj); err != nil {
		return nil, err
	}

	// 4. Setup checksum calculation while streaming
	hash := sha256.New()
	teeReader := io.TeeReader(fullReader, hash)

	// 5. Write file to store
	storeKey := key
	if bucket.VersioningEnabled {
		storeKey = versionedStoreKey(key, versionID)
	}

	dataStream := teeReader

	// Encryption (Optional)
	if bucket.EncryptionEnabled && s.encryptSvc != nil {
		encryptedReader, err := s.encryptSvc.Encrypt(ctx, bucketName, teeReader)
		if err != nil {
			return nil, errors.Wrap(errors.Internal, "encryption failed", err)
		}
		dataStream = encryptedReader
	}

	// Defense-in-depth: error on oversized upload rather than silent truncation.
	// boundedReader tears down any partial object when the limit is exceeded.
	br := &boundedReader{
		r:     dataStream,
		limit: maxPartSize,
		cleanupFn: func() {
			_ = s.store.Delete(ctx, bucketName, storeKey)
		},
	}
	size, err := s.store.Write(ctx, bucketName, storeKey, br)
	if err != nil {
		// boundedReader already deleted the partial object on ObjectTooLarge.
		// Preserve the error type so the HTTP layer returns 413.
		var appErr errors.Error
		if errors.As(err, &appErr) && appErr.Type == errors.ObjectTooLarge {
			span.RecordError(err)
			return nil, err
		}
		// We leave the record as PENDING. The garbage collector will clean it up.
		return nil, errors.Wrap(errors.Internal, "failed to write to store", err)
	}

	// 6. Update metadata to AVAILABLE and set final size/checksum
	calculatedChecksum := hex.EncodeToString(hash.Sum(nil))

	// Verify integrity if checksum was provided
	if providedChecksum != "" && !strings.EqualFold(providedChecksum, calculatedChecksum) {
		// Cleanup physical file immediately on mismatch
		_ = s.store.Delete(ctx, bucketName, storeKey)
		return nil, errors.New(errors.Conflict, fmt.Sprintf("data integrity check failed: checksum mismatch (provided: %s, calculated: %s)", providedChecksum, calculatedChecksum))
	}

	obj.SizeBytes = size
	obj.Checksum = calculatedChecksum
	obj.UploadStatus = domain.UploadStatusAvailable

	if err := s.repo.SaveMeta(ctx, obj); err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, obj.UserID, "storage.object_upload", "storage", obj.ARN, map[string]interface{}{
		"bucket": bucketName,
		"key":    key,
		"size":   size,
	}); err != nil {
		s.logger.Warn("failed to log audit event for upload",
			slog.String("bucket", bucketName),
			slog.String("key", key),
			slog.String("user_id", obj.UserID.String()),
			slog.Any("error", err))
	}

	platform.StorageOperations.WithLabelValues("upload", bucketName, "success").Inc()
	platform.StorageBytesTransferred.WithLabelValues("upload").Add(float64(size))

	return obj, nil
}

func (s *StorageService) Download(ctx context.Context, bucket, key string) (io.ReadCloser, *domain.Object, error) {
	tracer := otel.Tracer(tracerNameStorage)
	_, span := tracer.Start(ctx, "StorageService.Download",
		trace.WithAttributes(
			attribute.String("storage.bucket", bucket),
			attribute.String("storage.key", key),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageRead, bucket); err != nil {
		span.RecordError(err)
		return nil, nil, err
	}

	// 1. Get metadata
	obj, err := s.repo.GetMeta(ctx, bucket, key)
	if err != nil {
		return nil, nil, err
	}

	// 2. Open file
	storeKey := key
	if obj.VersionID != "" {
		storeKey = versionedStoreKey(key, obj.VersionID)
	}

	reader, err := s.store.Read(ctx, bucket, storeKey)
	if err != nil {
		return nil, nil, err
	}

	// Decryption
	b, err := s.repo.GetBucket(ctx, bucket)
	if err != nil {
		if closeErr := reader.Close(); closeErr != nil {
			return nil, nil, errors.Wrap(errors.Internal, "failed to get bucket; close error", fmt.Errorf("bucket error: %w; close error: %w", err, closeErr))
		}
		return nil, nil, errors.Wrap(errors.Internal, "failed to get bucket", err)
	}
	if b.EncryptionEnabled && s.encryptSvc != nil {
		decryptedReader, err := s.encryptSvc.Decrypt(ctx, bucket, reader)
		if err != nil {
			if closeErr := reader.Close(); closeErr != nil {
				return nil, nil, fmt.Errorf("decrypt error: %w; close error: %w", err, closeErr)
			}
			return nil, nil, err
		}
		// Wrap with readCloserWrapper to keep the original closer
		reader = &readCloserWrapper{Reader: decryptedReader, Closer: reader}
	}

	platform.StorageOperations.WithLabelValues("download", bucket, "success").Inc()
	if obj != nil {
		platform.StorageBytesTransferred.WithLabelValues("download").Add(float64(obj.SizeBytes))
	}

	return reader, obj, nil
}

type readCloserWrapper struct {
	io.Reader
	io.Closer
}

func (s *StorageService) ListObjects(ctx context.Context, bucket string) ([]*domain.Object, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageRead, bucket); err != nil {
		return nil, err
	}

	return s.repo.List(ctx, bucket)
}

func (s *StorageService) DownloadVersion(ctx context.Context, bucket, key, versionID string) (io.ReadCloser, *domain.Object, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageRead, bucket); err != nil {
		return nil, nil, err
	}

	// 1. Get metadata
	obj, err := s.repo.GetMetaByVersion(ctx, bucket, key, versionID)
	if err != nil {
		return nil, nil, err
	}

	// 2. Open file
	storeKey := key
	if obj.VersionID != "" {
		storeKey = versionedStoreKey(key, obj.VersionID)
	}

	reader, err := s.store.Read(ctx, bucket, storeKey)
	if err != nil {
		return nil, nil, err
	}

	return reader, obj, nil
}

func (s *StorageService) ListVersions(ctx context.Context, bucket, key string) ([]*domain.Object, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageRead, bucket); err != nil {
		return nil, err
	}

	return s.repo.ListVersions(ctx, bucket, key)
}

func (s *StorageService) DeleteVersion(ctx context.Context, bucket, key, versionID string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageDelete, bucket); err != nil {
		return err
	}

	// 1. Get meta to verify ownership/existence
	if _, err := s.repo.GetMetaByVersion(ctx, bucket, key, versionID); err != nil {
		return err
	}

	// 2. Delete from store
	storeKey := key
	if versionID != "" {
		storeKey = versionedStoreKey(key, versionID)
	}

	if err := s.store.Delete(ctx, bucket, storeKey); err != nil {
		return err
	}

	// 3. Delete meta (hard delete for specific version)
	return s.repo.DeleteVersion(ctx, bucket, key, versionID)
}

func (s *StorageService) DeleteObject(ctx context.Context, bucket, key string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageDelete, bucket); err != nil {
		return err
	}

	// 1. Soft delete in DB
	if err := s.repo.SoftDelete(ctx, bucket, key); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, userID, "storage.object_delete", "storage", bucket+"/"+key, map[string]interface{}{
		"bucket": bucket,
		"key":    key,
	}); err != nil {
		s.logger.Warn("failed to log audit event for object delete",
			slog.String("bucket", bucket),
			slog.String("key", key),
			slog.Any("error", err))
	}

	platform.StorageOperations.WithLabelValues("delete", bucket, "success").Inc()

	return nil
}

// CreateBucket creates a new storage bucket.
func (s *StorageService) CreateBucket(ctx context.Context, name string, isPublic bool) (*domain.Bucket, error) {
	tracer := otel.Tracer(tracerNameStorage)
	_, span := tracer.Start(ctx, "StorageService.CreateBucket",
		trace.WithAttributes(
			attribute.String("storage.bucket", name),
			attribute.Bool("storage.is_public", isPublic),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageWrite, "*"); err != nil {
		span.RecordError(err)
		return nil, err
	}

	// Validate bucket name (S3-like rules)
	if len(name) < 3 || len(name) > 63 {
		return nil, errors.New(errors.InvalidInput, "bucket name must be between 3 and 63 characters")
	}

	validName := validBucketNameRe
	if !validName.MatchString(name) {
		return nil, errors.New(errors.InvalidInput, "bucket name can only contain lowercase letters, numbers, dots, and hyphens")
	}

	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") {
		return nil, errors.New(errors.InvalidInput, "bucket name cannot start or end with a hyphen or dot")
	}
	if strings.Contains(name, "..") {
		return nil, errors.New(errors.InvalidInput, "bucket name cannot contain consecutive dots")
	}

	bucket := &domain.Bucket{
		ID:        uuid.New(),
		Name:      name,
		UserID:    userID,
		TenantID:  tenantID,
		IsPublic:  isPublic,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateBucket(ctx, bucket); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create bucket", err)
	}

	if err := s.auditSvc.Log(ctx, bucket.UserID, "storage.bucket_create", "bucket", name, map[string]interface{}{
		"is_public": isPublic,
	}); err != nil {
		s.logger.Warn("failed to log audit event for bucket creation",
			slog.String("bucket", name),
			slog.Any("error", err))
	}

	return bucket, nil
}

func (s *StorageService) GetBucket(ctx context.Context, name string) (*domain.Bucket, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageRead, name); err != nil {
		return nil, err
	}

	return s.repo.GetBucket(ctx, name)
}

func (s *StorageService) DeleteBucket(ctx context.Context, name string, force bool) error {
	tracer := otel.Tracer(tracerNameStorage)
	_, span := tracer.Start(ctx, "StorageService.DeleteBucket",
		trace.WithAttributes(
			attribute.String("storage.bucket", name),
			attribute.Bool("storage.force", force),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageDelete, name); err != nil {
		span.RecordError(err)
		return err
	}

	// 1. Check if empty
	objects, err := s.repo.List(ctx, name)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to list objects before bucket deletion", err)
	}

	if len(objects) > 0 && !force {
		return errors.New(errors.Conflict, "bucket is not empty")
	}

	// 2. Delete all objects if force
	if force {
		for _, obj := range objects {
			if err := s.DeleteObject(ctx, name, obj.Key); err != nil {
				return errors.Wrap(errors.Internal, fmt.Sprintf("failed to delete object %s during force bucket deletion", obj.Key), err)
			}
		}
	}

	// 3. Delete bucket record
	return s.repo.DeleteBucket(ctx, name)
}

func (s *StorageService) ListBuckets(ctx context.Context) ([]*domain.Bucket, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListBuckets(ctx, userID.String())
}

func (s *StorageService) SetBucketVersioning(ctx context.Context, name string, enabled bool) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageWrite, name); err != nil {
		return err
	}

	return s.repo.SetBucketVersioning(ctx, name, enabled)
}

func (s *StorageService) GetClusterStatus(ctx context.Context) (*domain.StorageCluster, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFullAccess, "*"); err != nil {
		return nil, err
	}

	return s.store.GetClusterStatus(ctx)
}

// CleanupDeleted removes soft-deleted objects from the storage backend.
func (s *StorageService) CleanupDeleted(ctx context.Context, limit int) (int, error) {
	deleted, err := s.repo.ListDeleted(ctx, limit)
	if err != nil {
		return 0, errors.Wrap(errors.Internal, "failed to list deleted objects", err)
	}

	deletedCount := 0
	for _, obj := range deleted {
		storeKey := obj.Key
		if obj.VersionID != "" {
			storeKey = versionedStoreKey(obj.Key, obj.VersionID)
		}

		if err := s.store.Delete(ctx, obj.Bucket, storeKey); err != nil {
			return deletedCount, errors.Wrap(errors.Internal, fmt.Sprintf("failed to delete physical file %s", storeKey), err)
		}

		if err := s.repo.HardDelete(ctx, obj.Bucket, obj.Key, obj.VersionID); err != nil {
			return deletedCount, errors.Wrap(errors.Internal, "failed to hard delete object", err)
		}
		deletedCount++
	}

	return deletedCount, nil
}

// CleanupPendingUploads removes orphaned files from failed uploads.
func (s *StorageService) CleanupPendingUploads(ctx context.Context, olderThan time.Duration, limit int) (int, error) {
	threshold := time.Now().Add(-olderThan)
	pending, err := s.repo.ListPending(ctx, threshold, limit)
	if err != nil {
		return 0, errors.Wrap(errors.Internal, "failed to list pending uploads", err)
	}

	cleanedCount := 0
	for _, obj := range pending {
		storeKey := obj.Key
		if obj.VersionID != "" {
			storeKey = versionedStoreKey(obj.Key, obj.VersionID)
		}

		if err := s.store.Delete(ctx, obj.Bucket, storeKey); err != nil {
			return cleanedCount, errors.Wrap(errors.Internal, fmt.Sprintf("failed to delete pending physical file %s", storeKey), err)
		}

		if err := s.repo.HardDelete(ctx, obj.Bucket, obj.Key, obj.VersionID); err != nil {
			return cleanedCount, errors.Wrap(errors.Internal, "failed to delete pending metadata", err)
		}
		cleanedCount++
	}

	return cleanedCount, nil
}

// CreateMultipartUpload initiates a new multipart upload session.
func (s *StorageService) CreateMultipartUpload(ctx context.Context, bucket, key string) (*domain.MultipartUpload, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageWrite, "*"); err != nil {
		return nil, err
	}

	if _, err := s.repo.GetBucket(ctx, bucket); err != nil {
		return nil, err
	}

	upload := &domain.MultipartUpload{
		ID:        uuid.New(),
		UserID:    userID,
		Bucket:    bucket,
		Key:       key,
		CreatedAt: time.Now(),
	}

	if err := s.repo.SaveMultipartUpload(ctx, upload); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to initiate multipart upload", err)
	}

	if err := s.auditSvc.Log(ctx, upload.UserID, "storage.multipart_init", "storage", upload.ID.String(), map[string]interface{}{
		"bucket": bucket,
		"key":    key,
	}); err != nil {
		s.logger.Warn("failed to log audit event for multipart init",
			slog.String("bucket", bucket),
			slog.String("key", key),
			slog.Any("error", err))
	}

	return upload, nil
}

// UploadPart uploads a single part of a multipart upload.
func (s *StorageService) UploadPart(ctx context.Context, uploadID uuid.UUID, partNumber int, r io.Reader, providedChecksum string) (*domain.Part, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	// 1. Get upload
	upload, err := s.repo.GetMultipartUpload(ctx, uploadID)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, errMultipartNotFound, err)
	}

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageWrite, "*"); err != nil {
		return nil, err
	}

	// 2. Calculate checksum while streaming to store
	hash := sha256.New()
	teeReader := io.TeeReader(r, hash)

	// 3. Write to store (use temporary location)
	partKey := fmt.Sprintf(partPathFormat, upload.ID.String(), partNumber)
	size, err := s.store.Write(ctx, upload.Bucket, partKey, io.LimitReader(teeReader, maxPartSize))
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to write part", err)
	}

	calculatedChecksum := hex.EncodeToString(hash.Sum(nil))

	// Verify integrity if checksum was provided
	if providedChecksum != "" && !strings.EqualFold(providedChecksum, calculatedChecksum) {
		_ = s.store.Delete(ctx, upload.Bucket, partKey)
		return nil, errors.New(errors.Conflict, fmt.Sprintf("part integrity check failed: checksum mismatch (provided: %s, calculated: %s)", providedChecksum, calculatedChecksum))
	}

	// 4. Save part metadata
	part := &domain.Part{
		UploadID:   uploadID,
		PartNumber: partNumber,
		SizeBytes:  size,
		ETag:       calculatedChecksum,
	}

	if err := s.repo.SavePart(ctx, part); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to save part metadata", err)
	}

	return part, nil
}

// CompleteMultipartUpload assembles all parts into a single object.
func (s *StorageService) CompleteMultipartUpload(ctx context.Context, uploadID uuid.UUID) (*domain.Object, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	// 1. Get upload and parts
	upload, err := s.repo.GetMultipartUpload(ctx, uploadID)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, errMultipartNotFound, err)
	}

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageWrite, "*"); err != nil {
		return nil, err
	}

	parts, err := s.repo.ListParts(ctx, uploadID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list parts", err)
	}

	if len(parts) == 0 {
		return nil, errors.New(errors.InvalidInput, "no parts uploaded")
	}

	// 2. Assembly in store
	partKeys := make([]string, len(parts))
	for i, p := range parts {
		partKeys[i] = fmt.Sprintf(partPathFormat, upload.ID.String(), p.PartNumber)
	}

	bucket, err := s.repo.GetBucket(ctx, upload.Bucket)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "bucket not found", err)
	}

	versionID := "null"
	if bucket.VersioningEnabled {
		versionID = generateVersionID()
	}

	storeKey := upload.Key
	if bucket.VersioningEnabled {
		storeKey = versionedStoreKey(upload.Key, versionID)
	}

	size, err := s.store.Assemble(ctx, upload.Bucket, storeKey, partKeys)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to assemble parts", err)
	}

	// 3. Create final object metadata
	obj := &domain.Object{
		ID:           uuid.New(),
		UserID:       upload.UserID,
		TenantID:     tenantID,
		Bucket:       upload.Bucket,
		Key:          upload.Key,
		VersionID:    versionID,
		IsLatest:     true,
		SizeBytes:    size,
		UploadStatus: domain.UploadStatusAvailable,
		CreatedAt:    time.Now(),
	}

	// Sniff content type and calculate checksum from the assembled file
	reader, err := s.store.Read(ctx, upload.Bucket, storeKey)
	if err != nil {
		obj.ContentType = "application/octet-stream"
		s.logger.Error("failed to open assembled file for metadata extraction",
			slog.String("bucket", upload.Bucket),
			slog.String("key", upload.Key),
			slog.Any("error", err))
		return nil, errors.Wrap(errors.Internal, "failed to read assembled file", err)
	}
	defer func() { _ = reader.Close() }()

	hash := sha256.New()
	sniffBuf := make([]byte, sniffLen)
	n, err := io.ReadFull(reader, sniffBuf)
	if err != nil && !go_errors.Is(err, io.EOF) && !go_errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, errors.Wrap(errors.Internal, "failed to read assembled file for MIME sniffing", err)
	}
	obj.ContentType = http.DetectContentType(sniffBuf[:n])

	// Continue reading for checksum
	fullReader := io.MultiReader(bytes.NewReader(sniffBuf[:n]), reader)
	if _, err := io.Copy(hash, fullReader); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to compute checksum for assembled file", err)
	}
	obj.Checksum = hex.EncodeToString(hash.Sum(nil))

	// Generate ARN
	obj.ARN = fmt.Sprintf("arn:thecloud:storage:local:default:object/%s/%s", upload.Bucket, upload.Key)
	if bucket.VersioningEnabled {
		obj.ARN += fmt.Sprintf("?versionId=%s", versionID)
	}

	// 4. Save metadata
	if err := s.repo.SaveMeta(ctx, obj); err != nil {
		return nil, err
	}

	// 5. Cleanup multipart records
	if err := s.repo.DeleteMultipartUpload(ctx, uploadID); err != nil {
		s.logger.Warn("failed to cleanup multipart upload metadata after completion",
			slog.String("upload_id", uploadID.String()),
			slog.Any("error", err))
	}

	if err := s.auditSvc.Log(ctx, obj.UserID, "storage.multipart_complete", "storage", obj.ARN, map[string]interface{}{
		"bucket": upload.Bucket,
		"key":    upload.Key,
		"size":   size,
	}); err != nil {
		s.logger.Warn("failed to log audit event for multipart completion",
			slog.String("bucket", upload.Bucket),
			slog.String("key", upload.Key),
			slog.Any("error", err))
	}

	return obj, nil
}

// AbortMultipartUpload cancels a multipart upload and cleans up parts.
func (s *StorageService) AbortMultipartUpload(ctx context.Context, uploadID uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	// 1. Get upload to resolve bucket name for auth
	upload, err := s.repo.GetMultipartUpload(ctx, uploadID)
	if err != nil {
		return errors.Wrap(errors.NotFound, errMultipartNotFound, err)
	}

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageDelete, upload.Bucket); err != nil {
		return err
	}

	// 2. List parts
	parts, err := s.repo.ListParts(ctx, uploadID)
	if err == nil {
		// 3. Delete each part from store
		for _, p := range parts {
			partKey := fmt.Sprintf(partPathFormat, upload.ID.String(), p.PartNumber)
			_ = s.store.Delete(ctx, upload.Bucket, partKey)
		}
	}

	// 4. Delete from repo
	if err := s.repo.DeleteMultipartUpload(ctx, uploadID); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete multipart upload metadata", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "storage.multipart_abort", "storage", uploadID.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "action", "storage.multipart_abort", "upload_id", uploadID, "error", err)
	}

	return nil
}

// GeneratePresignedURL generates a temporary signed URL for an object.
func (s *StorageService) GeneratePresignedURL(ctx context.Context, bucket, key, method string, expiry time.Duration) (*domain.PresignedURL, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	// Permission check depends on method
	var perm domain.Permission
	if method == http.MethodPut {
		perm = domain.PermissionStorageWrite
	} else {
		perm = domain.PermissionStorageRead
	}
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, perm, bucket); err != nil {
		return nil, err
	}

	// 1. Verify bucket exists
	if _, err := s.repo.GetBucket(ctx, bucket); err != nil {
		return nil, errors.Wrap(errors.NotFound, "bucket not found", err)
	}

	if expiry == 0 {
		expiry = 15 * time.Minute
	}

	expiresAt := time.Now().Add(expiry)

	// Use dependency injected config secret if available
	secret := s.cfg.StorageSecret
	if secret == "" {
		return nil, errors.New(errors.Internal, "storage secret not configured")
	}

	// Dynamic base URL derived from config or default
	baseURL := "http://localhost:8080"
	if s.cfg != nil && s.cfg.Port != "" {
		baseURL = fmt.Sprintf("http://localhost:%s", s.cfg.Port)
	}

	signedURL, err := crypto.SignURL(secret, baseURL, method, bucket, key, expiresAt)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to sign URL", err)
	}

	return &domain.PresignedURL{
		URL:       signedURL,
		Method:    method,
		ExpiresAt: expiresAt,
	}, nil
}
