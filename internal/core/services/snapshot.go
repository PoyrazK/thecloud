// Package services implements core business workflows.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

// SnapshotService manages volume snapshots and storage interactions.
type SnapshotService struct {
	repo       ports.SnapshotRepository
	rbacSvc    ports.RBACService
	volumeRepo ports.VolumeRepository
	storage    ports.StorageBackend
	eventSvc   ports.EventService
	auditSvc   ports.AuditService
	logger     *slog.Logger
	// asyncResults tracks in-progress snapshot goroutines for error propagation.
	asyncResults map[uuid.UUID]chan error
	mu           sync.Mutex
}

const snapshotNamePrefix = "thecloud-snap-"
const volumeNamePrefix = "thecloud-vol-"

// NewSnapshotService constructs a SnapshotService with its dependencies.
func NewSnapshotService(
	repo ports.SnapshotRepository,
	rbacSvc ports.RBACService,
	volumeRepo ports.VolumeRepository,
	storage ports.StorageBackend,
	eventSvc ports.EventService,
	auditSvc ports.AuditService,
	logger *slog.Logger,
) *SnapshotService {
	return &SnapshotService{
		repo:         repo,
		rbacSvc:      rbacSvc,
		volumeRepo:   volumeRepo,
		storage:      storage,
		eventSvc:     eventSvc,
		auditSvc:     auditSvc,
		logger:       logger,
		asyncResults: make(map[uuid.UUID]chan error),
	}
}

func (s *SnapshotService) CreateSnapshot(ctx context.Context, volumeID uuid.UUID, description string) (*domain.Snapshot, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSnapshotCreate, volumeID.String()); err != nil {
		return nil, err
	}

	// 1. Get volume
	vol, err := s.volumeRepo.GetByID(ctx, volumeID)
	if err != nil {
		return nil, err
	}

	// 2. Create domain entity
	snapshot := &domain.Snapshot{
		ID:          uuid.New(),
		UserID:      userID,
		TenantID:    &tenantID,
		VolumeID:    volumeID,
		VolumeName:  vol.Name,
		SizeGB:      vol.SizeGB,
		Status:      domain.SnapshotStatusCreating,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}

	// 3. Persist to DB initially
	if err := s.repo.Create(ctx, snapshot); err != nil {
		return nil, err
	}

	// 4. Perform async snapshot
	// Copy snapshot to avoid data race with returned pointer
	asyncSnap := *snapshot

	// Register error channel so caller can wait for async result
	errCh := make(chan error, 1)
	s.mu.Lock()
	s.asyncResults[snapshot.ID] = errCh
	s.mu.Unlock()

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		err := s.performSnapshot(bgCtx, vol, &asyncSnap)
		if err != nil {
			s.logger.Error("failed to perform snapshot", "snapshot_id", snapshot.ID, "error", err)
			asyncSnap.Status = domain.SnapshotStatusError
			errCh <- err
		} else {
			asyncSnap.Status = domain.SnapshotStatusAvailable
		}
		if updateErr := s.repo.Update(bgCtx, &asyncSnap); updateErr != nil {
			s.logger.Error("failed to update snapshot status", "snapshot_id", snapshot.ID, "error", updateErr)
			// Propagate Update failure so callers don't think snapshot succeeded
			// when it remains stuck in CREATING. Only send if performSnapshot
			// succeeded, since errCh already carries that error.
			if err == nil {
				errCh <- fmt.Errorf("failed to persist snapshot status: %w", updateErr)
			}
		}

		// Clean up tracking entry
		s.mu.Lock()
		delete(s.asyncResults, snapshot.ID)
		s.mu.Unlock()

		close(errCh)
	}()

	if err := s.eventSvc.RecordEvent(ctx, "SNAPSHOT_CREATE", snapshot.ID.String(), "SNAPSHOT", map[string]interface{}{
		"volume_id": volumeID.String(),
	}); err != nil {
		s.logger.Warn("failed to record event", "action", "SNAPSHOT_CREATE", "snapshot_id", snapshot.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, snapshot.UserID, "snapshot.create", "snapshot", snapshot.ID.String(), map[string]interface{}{
		"volume_id": volumeID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "snapshot.create", "snapshot_id", snapshot.ID, "error", err)
	}

	return snapshot, nil
}

func (s *SnapshotService) ListSnapshots(ctx context.Context) ([]*domain.Snapshot, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSnapshotRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListByUserID(ctx, userID)
}

func (s *SnapshotService) GetSnapshot(ctx context.Context, id uuid.UUID) (*domain.Snapshot, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSnapshotRead, id.String()); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}

// WaitForSnapshot blocks until the snapshot async operation completes.
// Returns the final snapshot state and any error that occurred during async creation.
func (s *SnapshotService) WaitForSnapshot(ctx context.Context, id uuid.UUID) (*domain.Snapshot, error) {
	s.mu.Lock()
	errCh, ok := s.asyncResults[id]
	s.mu.Unlock()

	if !ok {
		// Not in progress — return current state from repo
		return s.GetSnapshot(ctx, id)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		snap, snapErr := s.GetSnapshot(ctx, id)
		if snapErr != nil {
			return nil, snapErr
		}
		if err != nil {
			return snap, fmt.Errorf("async snapshot failed: %w", err)
		}
		return snap, nil
	}
}

func (s *SnapshotService) performSnapshot(ctx context.Context, vol *domain.Volume, snapshot *domain.Snapshot) error {
	backendVolName := volumeNamePrefix + vol.ID.String()[:8]
	backendSnapName := snapshotNamePrefix + snapshot.ID.String()[:8]

	if err := s.storage.CreateSnapshot(ctx, backendVolName, backendSnapName); err != nil {
		return fmt.Errorf("failed to create block volume snapshot: %w", err)
	}

	return nil
}

func (s *SnapshotService) DeleteSnapshot(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSnapshotDelete, id.String()); err != nil {
		return err
	}

	snapshot, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 1. Delete from Backend
	backendSnapName := snapshotNamePrefix + snapshot.ID.String()[:8]
	if err := s.storage.DeleteSnapshot(ctx, backendSnapName); err != nil {
		s.logger.Warn("failed to delete backend snapshot", "name", backendSnapName, "error", err)
	}

	// 2. Delete from DB
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	if err := s.eventSvc.RecordEvent(ctx, "SNAPSHOT_DELETE", id.String(), "SNAPSHOT", map[string]interface{}{}); err != nil {
		s.logger.Warn("failed to record event", "action", "SNAPSHOT_DELETE", "snapshot_id", id, "error", err)
	}
	if err := s.auditSvc.Log(ctx, snapshot.UserID, "snapshot.delete", "snapshot", id.String(), map[string]interface{}{}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "snapshot.delete", "snapshot_id", id, "error", err)
	}

	return nil
}

func (s *SnapshotService) RestoreSnapshot(ctx context.Context, snapshotID uuid.UUID, newVolumeName string) (*domain.Volume, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSnapshotRestore, snapshotID.String()); err != nil {
		return nil, err
	}

	snapshot, err := s.repo.GetByID(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	if snapshot.Status != domain.SnapshotStatusAvailable {
		return nil, errors.New(errors.InvalidInput, "cannot restore from snapshot that is not available")
	}

	// 1. Create new volume domain entity
	vol := &domain.Volume{
		ID:        uuid.New(),
		UserID:    userID,
		TenantID:  tenantID,
		Name:      newVolumeName,
		SizeGB:    snapshot.SizeGB,
		Status:    domain.VolumeStatusAvailable,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now(),
	}

	// 2. Create Block Volume
	backendVolName := volumeNamePrefix + vol.ID.String()[:8]
	path, err := s.storage.CreateVolume(ctx, backendVolName, vol.SizeGB)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create volume for restore", err)
	}
	vol.BackendPath = path

	// 3. Restore snapshot into new volume
	backendSnapName := snapshotNamePrefix + snapshot.ID.String()[:8]
	if err := s.storage.RestoreSnapshot(ctx, backendVolName, backendSnapName); err != nil {
		_ = s.storage.DeleteVolume(ctx, backendVolName)
		return nil, fmt.Errorf("failed to restore volume snapshot: %w", err)
	}

	// 4. Save to DB
	if err := s.volumeRepo.Create(ctx, vol); err != nil {
		_ = s.storage.DeleteVolume(ctx, backendVolName)
		return nil, err
	}

	if err := s.eventSvc.RecordEvent(ctx, "VOLUME_RESTORE", vol.ID.String(), "VOLUME", map[string]interface{}{
		"snapshot_id": snapshotID.String(),
	}); err != nil {
		s.logger.Warn("failed to record event", "action", "VOLUME_RESTORE", "volume_id", vol.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, vol.UserID, "volume.restore", "volume", vol.ID.String(), map[string]interface{}{
		"snapshot_id": snapshotID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "volume.restore", "volume_id", vol.ID, "error", err)
	}

	return vol, nil
}
