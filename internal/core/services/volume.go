// Package services implements core business workflows.
package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const (
	tracerName          = "volume-service"
	BackendVolumePrefix = "thecloud-vol-"
	BackendIDPrefixLen  = 8
)

// FormatBackendVolumeName returns the formatted name for the storage backend.
func FormatBackendVolumeName(id uuid.UUID) string {
	return BackendVolumePrefix + id.String()[:BackendIDPrefixLen]
}

// VolumeServiceParams defines dependencies for VolumeService.
type VolumeServiceParams struct {
	Repo         ports.VolumeRepository
	RBACSvc      ports.RBACService
	Storage      ports.StorageBackend
	Compute      ports.ComputeBackend
	EventSvc     ports.EventService
	AuditSvc     ports.AuditService
	Logger       *slog.Logger
	InstanceRepo ports.InstanceRepository
}

// VolumeService manages block volume lifecycle and attachments.
type VolumeService struct {
	repo         ports.VolumeRepository
	rbacSvc      ports.RBACService
	storage      ports.StorageBackend
	compute      ports.ComputeBackend
	eventSvc     ports.EventService
	auditSvc     ports.AuditService
	logger       *slog.Logger
	instanceRepo ports.InstanceRepository
}

// NewVolumeService constructs a VolumeService with its dependencies.
func NewVolumeService(params VolumeServiceParams) *VolumeService {
	return &VolumeService{
		repo:         params.Repo,
		rbacSvc:      params.RBACSvc,
		storage:      params.Storage,
		compute:      params.Compute,
		eventSvc:     params.EventSvc,
		auditSvc:     params.AuditSvc,
		logger:       params.Logger,
		instanceRepo: params.InstanceRepo,
	}
}

func (s *VolumeService) CreateVolume(ctx context.Context, name string, sizeGB int) (*domain.Volume, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CreateVolume")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVolumeCreate, "*"); err != nil {
		return nil, err
	}

	span.SetAttributes(
		attribute.String("volume.name", name),
		attribute.Int("volume.size_gb", sizeGB),
	)
	// 1. Create domain entity
	vol := &domain.Volume{
		ID:        uuid.New(),
		UserID:    userID,
		TenantID:  tenantID,
		Name:      name,
		SizeGB:    sizeGB,
		Status:    domain.VolumeStatusAvailable,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now(),
	}

	// 2. Create Block Volume
	backendName := FormatBackendVolumeName(vol.ID)
	path, err := s.storage.CreateVolume(ctx, backendName, sizeGB)
	if err != nil {
		s.logger.Error("failed to create storage volume", "name", backendName, "error", err)
		return nil, errors.Wrap(errors.Internal, "failed to create volume", err)
	}
	vol.BackendPath = path // We need to add this field to domain.Volume

	// 3. Persist to DB
	if err := s.repo.Create(ctx, vol); err != nil {
		// Rollback Storage Volume
		if delErr := s.storage.DeleteVolume(ctx, backendName); delErr != nil {
			s.logger.Error("failed to rollback storage volume", "name", backendName, "error", delErr)
		}
		return nil, err
	}

	if err := s.eventSvc.RecordEvent(ctx, "VOLUME_CREATE", vol.ID.String(), "VOLUME", map[string]interface{}{
		"name":    vol.Name,
		"size_gb": vol.SizeGB,
		"path":    path,
	}); err != nil {
		s.logger.Warn("failed to record event", "action", "VOLUME_CREATE", "volume_id", vol.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, vol.UserID, "volume.create", "volume", vol.ID.String(), map[string]interface{}{
		"name":    vol.Name,
		"size_gb": vol.SizeGB,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "volume.create", "volume_id", vol.ID, "error", err)
	}

	s.logger.Info("volume created", "volume_id", vol.ID, "name", vol.Name, "path", path)
	// platform metrics ...
	return vol, nil
}

func (s *VolumeService) ListVolumes(ctx context.Context) ([]*domain.Volume, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVolumeRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.List(ctx)
}

func (s *VolumeService) GetVolume(ctx context.Context, idOrName string) (*domain.Volume, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "GetVolume")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVolumeRead, idOrName); err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.String("volume.id_or_name", idOrName))
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.repo.GetByID(ctx, id)
	}
	return s.repo.GetByName(ctx, idOrName)
}

func (s *VolumeService) DeleteVolume(ctx context.Context, idOrName string) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "DeleteVolume")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVolumeDelete, idOrName); err != nil {
		return err
	}

	span.SetAttributes(attribute.String("volume.id_or_name", idOrName))
	vol, err := s.GetVolume(ctx, idOrName)
	if err != nil {
		return err
	}

	if vol.Status == domain.VolumeStatusInUse {
		return errors.New(errors.InvalidInput, "cannot delete volume that is in use")
	}

	// 1. Delete Storage Volume
	backendName := FormatBackendVolumeName(vol.ID)
	if err := s.storage.DeleteVolume(ctx, backendName); err != nil {
		s.logger.Warn("failed to delete storage volume", "name", backendName, "error", err)
	}

	// 2. Delete from DB
	if err := s.repo.Delete(ctx, vol.ID); err != nil {
		return err
	}

	platform.VolumesTotal.WithLabelValues(string(vol.Status)).Dec()
	platform.VolumeSizeBytes.Sub(float64(vol.SizeGB * 1024 * 1024 * 1024))
	platform.StorageOperationsTotal.WithLabelValues("volume_delete").Inc()

	if err := s.eventSvc.RecordEvent(ctx, "VOLUME_DELETE", vol.ID.String(), "VOLUME", map[string]interface{}{}); err != nil {
		s.logger.Warn("failed to record event", "action", "VOLUME_DELETE", "volume_id", vol.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, vol.UserID, "volume.delete", "volume", vol.ID.String(), map[string]interface{}{
		"name": vol.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "volume.delete", "volume_id", vol.ID, "error", err)
	}

	s.logger.Info("volume deleted", "volume_id", vol.ID)
	return nil
}

func (s *VolumeService) ResizeVolume(ctx context.Context, idOrName string, newSizeGB int) error {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "ResizeVolume")
	defer span.End()
	span.SetAttributes(
		attribute.String("volume.id_or_name", idOrName),
		attribute.Int("volume.new_size_gb", newSizeGB),
	)

	vol, err := s.GetVolume(ctx, idOrName)
	if err != nil {
		return err
	}

	if newSizeGB <= vol.SizeGB {
		return errors.New(errors.InvalidInput, "new size must be larger than current size")
	}

	// 1. Resize in Backend
	backendName := FormatBackendVolumeName(vol.ID)
	if err := s.storage.ResizeVolume(ctx, backendName, newSizeGB); err != nil {
		return errors.Wrap(errors.Internal, "failed to resize volume in backend", err)
	}

	// 2. Update DB
	oldSizeGB := vol.SizeGB
	vol.SizeGB = newSizeGB
	vol.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, vol); err != nil {
		return err
	}

	platform.VolumeSizeBytes.Add(float64((newSizeGB - oldSizeGB) * 1024 * 1024 * 1024))
	if err := s.eventSvc.RecordEvent(ctx, "VOLUME_RESIZE", vol.ID.String(), "VOLUME", map[string]interface{}{
		"old_size_gb": oldSizeGB,
		"new_size_gb": newSizeGB,
	}); err != nil {
		s.logger.Warn("failed to record event", "action", "VOLUME_RESIZE", "volume_id", vol.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, vol.UserID, "volume.resize", "volume", vol.ID.String(), map[string]interface{}{
		"name":        vol.Name,
		"old_size_gb": oldSizeGB,
		"new_size_gb": newSizeGB,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "volume.resize", "volume_id", vol.ID, "error", err)
	}

	s.logger.Info("volume resized", "volume_id", vol.ID, "old_size", oldSizeGB, "new_size", newSizeGB)
	return nil
}

func (s *VolumeService) AttachVolume(ctx context.Context, volumeID string, instanceID string, mountPath string) (string, error) {
	vol, err := s.GetVolume(ctx, volumeID)
	if err != nil {
		return "", err
	}

	if vol.Status == domain.VolumeStatusInUse {
		return "", errors.New(errors.Conflict, "volume is already attached to an instance")
	}

	instUUID, err := uuid.Parse(instanceID)
	if err != nil {
		return "", errors.New(errors.InvalidInput, "invalid instance ID")
	}

	// 1. Attach via Storage Backend (LVM/physical) - returns device path
	backendName := FormatBackendVolumeName(vol.ID)
	devicePath, err := s.storage.AttachVolume(ctx, backendName, instanceID)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to attach volume in backend", err)
	}

	// 2. If Compute backend exists and instance has ContainerID, update container with new volume bind
	var newContainerID string
	if s.compute != nil {
		inst, err := s.instanceRepo.GetByID(ctx, instUUID)
		if err != nil {
			// Rollback storage attach
			_ = s.storage.DetachVolume(ctx, backendName, instanceID)
			return "", errors.Wrap(errors.Internal, "failed to get instance", err)
		}

		if inst.ContainerID != "" {
			bindSpec := devicePath + ":" + mountPath + ":rw"
			_, newContainerID, err = s.compute.AttachVolume(ctx, inst.ContainerID, bindSpec)
			if err != nil {
				// Rollback storage attach
				_ = s.storage.DetachVolume(ctx, backendName, instanceID)
				return "", errors.Wrap(errors.Internal, "failed to attach volume to container", err)
			}
			// Update instance.ContainerID only when backend returns non-empty (Docker recreates container)
			if newContainerID != "" {
				inst.ContainerID = newContainerID
				if err := s.instanceRepo.Update(ctx, inst); err != nil {
					s.logger.Warn("failed to update instance ContainerID after volume attach",
						"instance_id", instUUID, "new_container_id", newContainerID, "error", err)
				}
			}
		}
	}

	// 4. Update DB
	vol.Status = domain.VolumeStatusInUse
	vol.InstanceID = &instUUID
	vol.MountPath = mountPath
	vol.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, vol); err != nil {
		// Rollback compute attach if it happened
		if newContainerID != "" && s.compute != nil {
			_, _ = s.compute.DetachVolume(ctx, newContainerID, mountPath)
		}
		_ = s.storage.DetachVolume(ctx, backendName, instanceID)
		return "", err
	}

	if err := s.auditSvc.Log(ctx, vol.UserID, "volume.attach", "volume", vol.ID.String(), map[string]interface{}{
		"instance_id": instanceID,
		"mount_path":  mountPath,
		"device_path": devicePath,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "volume.attach", "volume_id", vol.ID, "error", err)
	}

	return devicePath, nil
}

func (s *VolumeService) DetachVolume(ctx context.Context, volumeID string) error {
	vol, err := s.GetVolume(ctx, volumeID)
	if err != nil {
		return err
	}

	if vol.Status != domain.VolumeStatusInUse || vol.InstanceID == nil {
		return errors.New(errors.InvalidInput, "volume is not attached")
	}

	instanceID := vol.InstanceID.String()
	backendName := FormatBackendVolumeName(vol.ID)

	// 1. If Compute backend exists, detach from container first
	var newContainerID string
	if s.compute != nil && vol.MountPath != "" {
		inst, err := s.instanceRepo.GetByID(ctx, *vol.InstanceID)
		if err != nil {
			return errors.Wrap(errors.Internal, "failed to get instance", err)
		}

		if inst.ContainerID != "" {
			newContainerID, err = s.compute.DetachVolume(ctx, inst.ContainerID, vol.MountPath)
			if err != nil {
				return errors.Wrap(errors.Internal, "failed to detach volume from container", err)
			}
			// Update instance.ContainerID only when backend returns non-empty (Docker recreates container)
			if newContainerID != "" {
				inst.ContainerID = newContainerID
				if err := s.instanceRepo.Update(ctx, inst); err != nil {
					s.logger.Warn("failed to update instance ContainerID after volume detach",
						"instance_id", vol.InstanceID, "error", err)
				}
			}
		}
	}

	// 3. Detach via Storage Backend
	if err := s.storage.DetachVolume(ctx, backendName, instanceID); err != nil {
		return errors.Wrap(errors.Internal, "failed to detach volume in backend", err)
	}

	// 4. Update DB
	vol.Status = domain.VolumeStatusAvailable
	vol.InstanceID = nil
	vol.MountPath = ""
	vol.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, vol); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, vol.UserID, "volume.detach", "volume", vol.ID.String(), map[string]interface{}{
		"instance_id": instanceID,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "volume.detach", "volume_id", vol.ID, "error", err)
	}

	return nil
}

// ReleaseVolumesForInstance detaches all volumes attached to an instance and marks them as available.
// This should be called when an instance is terminated to free up its volumes.
func (s *VolumeService) ReleaseVolumesForInstance(ctx context.Context, instanceID uuid.UUID) error {
	// Internal method, typically called by InstanceService which has its own RBAC
	volumes, err := s.repo.ListByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("failed to list volumes for instance", "instance_id", instanceID, "error", err)
		return err
	}

	for _, vol := range volumes {
		backendName := FormatBackendVolumeName(vol.ID)
		if err := s.storage.DetachVolume(ctx, backendName, instanceID.String()); err != nil {
			s.logger.Error("failed to detach volume during release", "volume_id", vol.ID, "instance_id", instanceID, "error", err)
			continue
		}

		vol.Status = domain.VolumeStatusAvailable
		vol.InstanceID = nil
		vol.MountPath = ""
		vol.UpdatedAt = time.Now()

		if err := s.repo.Update(ctx, vol); err != nil {
			s.logger.Warn("failed to release volume record in DB", "volume_id", vol.ID, "error", err)
			continue // Continue releasing other volumes even if one fails
		}

		s.logger.Info("volume released", "volume_id", vol.ID, "instance_id", instanceID)
	}

	return nil
}
