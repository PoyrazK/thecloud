package services

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/platform"
)

type StorageService struct {
	repo     ports.StorageRepository
	store    ports.FileStore
	auditSvc ports.AuditService
}

func NewStorageService(repo ports.StorageRepository, store ports.FileStore, auditSvc ports.AuditService) *StorageService {
	return &StorageService{
		repo:     repo,
		store:    store,
		auditSvc: auditSvc,
	}
}

func (s *StorageService) Upload(ctx context.Context, bucket, key string, r io.Reader) (*domain.Object, error) {
	// 1. Write file to store
	size, err := s.store.Write(ctx, bucket, key, r)
	if err != nil {
		return nil, err
	}

	// 2. Prepare metadata
	obj := &domain.Object{
		ID:        uuid.New(),
		UserID:    appcontext.UserIDFromContext(ctx),
		Bucket:    bucket,
		Key:       key,
		SizeBytes: size,
		// In a real system we'd detect Content-Type
		ContentType: "application/octet-stream",
		CreatedAt:   time.Now(),
	}

	// Generate ARN
	// arn:thecloud:storage:local:default:object/<bucket>/<key>
	obj.ARN = fmt.Sprintf("arn:thecloud:storage:local:default:object/%s/%s", bucket, key)

	// 3. Save metadata
	if err := s.repo.SaveMeta(ctx, obj); err != nil {
		// Cleanup file if DB save fails
		_ = s.store.Delete(ctx, bucket, key)
		return nil, err
	}

	_ = s.auditSvc.Log(ctx, obj.UserID, "storage.object_upload", "storage", obj.ID.String(), map[string]interface{}{
		"bucket": obj.Bucket,
		"key":    obj.Key,
	})

	platform.StorageOperationsTotal.WithLabelValues("object_upload").Inc()

	return obj, nil
}

func (s *StorageService) Download(ctx context.Context, bucket, key string) (io.ReadCloser, *domain.Object, error) {
	// 1. Get metadata
	obj, err := s.repo.GetMeta(ctx, bucket, key)
	if err != nil {
		return nil, nil, err
	}

	// 2. Open file
	reader, err := s.store.Read(ctx, bucket, key)
	if err != nil {
		return nil, nil, err
	}

	return reader, obj, nil
}

func (s *StorageService) ListObjects(ctx context.Context, bucket string) ([]*domain.Object, error) {
	return s.repo.List(ctx, bucket)
}

func (s *StorageService) DeleteObject(ctx context.Context, bucket, key string) error {
	// 1. Soft delete in DB
	if err := s.repo.SoftDelete(ctx, bucket, key); err != nil {
		return err
	}

	// Note: We don't delete from FileStore yet because it's a "soft delete".
	// A background job could clean up Filesystem objects with deleted_at set.

	_ = s.auditSvc.Log(ctx, appcontext.UserIDFromContext(ctx), "storage.object_delete", "storage", bucket+"/"+key, map[string]interface{}{
		"bucket": bucket,
		"key":    key,
	})

	platform.StorageOperationsTotal.WithLabelValues("object_delete").Inc()

	return nil
}
