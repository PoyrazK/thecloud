package noop

import (
	"context"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type NoopInstanceRepository struct{}

func (r *NoopInstanceRepository) Create(ctx context.Context, i *domain.Instance) error { return nil }
func (r *NoopInstanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	return &domain.Instance{ID: id}, nil
}
func (r *NoopInstanceRepository) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	return &domain.Instance{ID: uuid.New(), Name: name}, nil
}
func (r *NoopInstanceRepository) List(ctx context.Context) ([]*domain.Instance, error) {
	return []*domain.Instance{}, nil
}
func (r *NoopInstanceRepository) Update(ctx context.Context, i *domain.Instance) error { return nil }
func (r *NoopInstanceRepository) Delete(ctx context.Context, id uuid.UUID) error       { return nil }
func (r *NoopInstanceRepository) ListBySubnet(ctx context.Context, subnetID uuid.UUID) ([]*domain.Instance, error) {
	return []*domain.Instance{}, nil
}
func (r *NoopInstanceRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Instance, error) {
	return []*domain.Instance{}, nil
}

type NoopVpcRepository struct{}

func (r *NoopVpcRepository) Create(ctx context.Context, v *domain.VPC) error { return nil }
func (r *NoopVpcRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.VPC, error) {
	return &domain.VPC{ID: id}, nil
}
func (r *NoopVpcRepository) GetByName(ctx context.Context, name string) (*domain.VPC, error) {
	return &domain.VPC{ID: uuid.New(), Name: name}, nil
}
func (r *NoopVpcRepository) List(ctx context.Context) ([]*domain.VPC, error) {
	return []*domain.VPC{}, nil
}
func (r *NoopVpcRepository) Delete(ctx context.Context, id uuid.UUID) error { return nil }
func (r *NoopVpcRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.VPC, error) {
	return []*domain.VPC{}, nil
}

type NoopSubnetRepository struct{}

func (r *NoopSubnetRepository) Create(ctx context.Context, s *domain.Subnet) error { return nil }
func (r *NoopSubnetRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Subnet, error) {
	return &domain.Subnet{ID: id}, nil
}
func (r *NoopSubnetRepository) GetByName(ctx context.Context, vpcID uuid.UUID, name string) (*domain.Subnet, error) {
	return &domain.Subnet{ID: uuid.New(), Name: name, VPCID: vpcID}, nil
}
func (r *NoopSubnetRepository) ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.Subnet, error) {
	return []*domain.Subnet{}, nil
}
func (r *NoopSubnetRepository) Delete(ctx context.Context, id uuid.UUID) error { return nil }

type NoopVolumeRepository struct{}

func (r *NoopVolumeRepository) Create(ctx context.Context, v *domain.Volume) error { return nil }
func (r *NoopVolumeRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Volume, error) {
	return &domain.Volume{ID: id}, nil
}
func (r *NoopVolumeRepository) GetByName(ctx context.Context, name string) (*domain.Volume, error) {
	return &domain.Volume{ID: uuid.New(), Name: name}, nil
}
func (r *NoopVolumeRepository) List(ctx context.Context) ([]*domain.Volume, error) {
	return []*domain.Volume{}, nil
}
func (r *NoopVolumeRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Volume, error) {
	return []*domain.Volume{}, nil
}
func (r *NoopVolumeRepository) ListByInstanceID(ctx context.Context, instanceID uuid.UUID) ([]*domain.Volume, error) {
	return []*domain.Volume{}, nil
}
func (r *NoopVolumeRepository) Update(ctx context.Context, v *domain.Volume) error { return nil }
func (r *NoopVolumeRepository) Delete(ctx context.Context, id uuid.UUID) error     { return nil }

type NoopComputeBackend struct{}

func (c *NoopComputeBackend) CreateInstance(ctx context.Context, name, image string, ports []string, networkID string, volumeBinds []string, env []string, cmd []string) (string, error) {
	return "noop-id", nil
}
func (c *NoopComputeBackend) StopInstance(ctx context.Context, id string) error   { return nil }
func (c *NoopComputeBackend) DeleteInstance(ctx context.Context, id string) error { return nil }
func (c *NoopComputeBackend) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (c *NoopComputeBackend) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("{}")), nil
}
func (c *NoopComputeBackend) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	return 0, nil
}
func (c *NoopComputeBackend) GetInstanceIP(ctx context.Context, id string) (string, error) {
	return "127.0.0.1", nil
}
func (c *NoopComputeBackend) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", nil
}
func (c *NoopComputeBackend) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, error) {
	return "task-id", nil
}
func (c *NoopComputeBackend) WaitTask(ctx context.Context, id string) (int64, error) { return 0, nil }
func (c *NoopComputeBackend) CreateNetwork(ctx context.Context, name string) (string, error) {
	return "net-id", nil
}
func (c *NoopComputeBackend) DeleteNetwork(ctx context.Context, id string) error  { return nil }
func (c *NoopComputeBackend) CreateVolume(ctx context.Context, name string) error { return nil }
func (c *NoopComputeBackend) DeleteVolume(ctx context.Context, name string) error { return nil }
func (c *NoopComputeBackend) CreateVolumeSnapshot(ctx context.Context, volumeID string, destinationPath string) error {
	return nil
}
func (c *NoopComputeBackend) RestoreVolumeSnapshot(ctx context.Context, volumeID string, sourcePath string) error {
	return nil
}
func (c *NoopComputeBackend) Ping(ctx context.Context) error { return nil }
func (c *NoopComputeBackend) Type() string                   { return "noop" }

type NoopEventService struct{}

func (e *NoopEventService) RecordEvent(ctx context.Context, eventType, resourceID, resourceType string, data map[string]interface{}) error {
	return nil
}
func (e *NoopEventService) ListEvents(ctx context.Context, limit int) ([]*domain.Event, error) {
	return []*domain.Event{}, nil
}

type NoopAuditService struct{}

func (a *NoopAuditService) Log(ctx context.Context, userID uuid.UUID, action, resourceType, resourceID string, metadata map[string]interface{}) error {
	return nil
}
func (a *NoopAuditService) ListLogs(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	return []*domain.AuditLog{}, nil
}

type NoopDatabaseRepository struct{}

func (r *NoopDatabaseRepository) Create(ctx context.Context, db *domain.Database) error { return nil }
func (r *NoopDatabaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	return &domain.Database{ID: id}, nil
}
func (r *NoopDatabaseRepository) List(ctx context.Context) ([]*domain.Database, error) {
	return []*domain.Database{}, nil
}
func (r *NoopDatabaseRepository) Update(ctx context.Context, db *domain.Database) error { return nil }
func (r *NoopDatabaseRepository) Delete(ctx context.Context, id uuid.UUID) error        { return nil }

type NoopCacheRepository struct{}

func (r *NoopCacheRepository) Create(ctx context.Context, cache *domain.Cache) error { return nil }
func (r *NoopCacheRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Cache, error) {
	return &domain.Cache{ID: id}, nil
}
func (r *NoopCacheRepository) GetByName(ctx context.Context, userID uuid.UUID, name string) (*domain.Cache, error) {
	return &domain.Cache{ID: uuid.New(), Name: name, UserID: userID}, nil
}
func (r *NoopCacheRepository) List(ctx context.Context, userID uuid.UUID) ([]*domain.Cache, error) {
	return []*domain.Cache{}, nil
}
func (r *NoopCacheRepository) Update(ctx context.Context, cache *domain.Cache) error { return nil }
func (r *NoopCacheRepository) Delete(ctx context.Context, id uuid.UUID) error        { return nil }
