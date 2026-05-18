// Package services implements core business logic.
package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

// LifecycleService implements bucket lifecycle rule operations.
type LifecycleService struct {
	repo        ports.LifecycleRepository
	rbacSvc     ports.RBACService
	storageRepo ports.StorageRepository
}

// NewLifecycleService constructs a LifecycleService.
func NewLifecycleService(repo ports.LifecycleRepository, rbacSvc ports.RBACService, storageRepo ports.StorageRepository) *LifecycleService {
	return &LifecycleService{
		repo:        repo,
		rbacSvc:     rbacSvc,
		storageRepo: storageRepo,
	}
}

func (s *LifecycleService) CreateRule(ctx context.Context, bucket string, prefix string, expirationDays int, enabled bool) (*domain.LifecycleRule, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageWrite, bucket); err != nil {
		return nil, err
	}

	// 1. Verify bucket exists
	b, err := s.storageRepo.GetBucket(ctx, bucket)
	if err != nil {
		return nil, err
	}

	// 2. Verify ownership (Implicitly handled by Authorize if we want to support shared buckets,
	// but currently scoped to user in GetBucket or simple comparison)
	if userID != b.UserID {
		return nil, errors.New(errors.Forbidden, "you don't own this bucket")
	}

	if expirationDays < 1 {
		return nil, errors.New(errors.InvalidInput, "expiration days must be at least 1")
	}

	rule := &domain.LifecycleRule{
		ID:             uuid.New(),
		UserID:         userID,
		TenantID:       tenantID,
		BucketName:     bucket,
		Prefix:         prefix,
		ExpirationDays: expirationDays,
		Enabled:        enabled,
		CreatedAt:      time.Now().UTC().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, err
	}

	return rule, nil
}

func (s *LifecycleService) ListRules(ctx context.Context, bucket string) ([]*domain.LifecycleRule, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageRead, bucket); err != nil {
		return nil, err
	}

	// Verify bucket existence/ownership first
	b, err := s.storageRepo.GetBucket(ctx, bucket)
	if err != nil {
		return nil, err
	}

	if userID != b.UserID {
		return nil, errors.New(errors.Forbidden, "you don't own this bucket")
	}

	return s.repo.List(ctx, bucket)
}

func (s *LifecycleService) DeleteRule(ctx context.Context, bucket string, ruleID string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStorageDelete, ruleID); err != nil {
		return err
	}

	id, err := uuid.Parse(ruleID)
	if err != nil {
		return errors.New(errors.InvalidInput, "invalid rule id")
	}

	// Verify bucket existence/ownership
	b, err := s.storageRepo.GetBucket(ctx, bucket)
	if err != nil {
		return err
	}
	if userID != b.UserID {
		return errors.New(errors.Forbidden, "you don't own this bucket")
	}

	// Verify rule belongs to bucket
	rule, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if rule.BucketName != bucket {
		return errors.New(errors.InvalidInput, "rule does not belong to the specified bucket")
	}

	return s.repo.Delete(ctx, id)
}
