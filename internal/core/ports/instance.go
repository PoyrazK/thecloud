// Package ports defines service and repository interfaces.
package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// InstanceRepository handles the persistence and retrieval of compute instance metadata.
type InstanceRepository interface {
	// Create saves a new compute instance record.
	Create(ctx context.Context, instance *domain.Instance) error
	// GetByID retrieves an instance by its unique UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error)
	// GetByName retrieves an instance by its friendly name (typically scoped to owner).
	GetByName(ctx context.Context, name string) (*domain.Instance, error)
	// List returns instances authorized for the current operational context.
	// tagFilter is an optional list of label constraints (e.g. ["env:prod", "team:backend"]);
	// instances must match all listed constraints to be returned.
	List(ctx context.Context, tagFilter []string) ([]*domain.Instance, error)
	// ListAll returns every instance in the system (for administrative or global tasks).
	ListAll(ctx context.Context) ([]*domain.Instance, error)
	// ListBySubnet returns all instances residing within a specific subnet.
	ListBySubnet(ctx context.Context, subnetID uuid.UUID) ([]*domain.Instance, error)
	// Update modifies an existing instance's metadata or status.
	Update(ctx context.Context, instance *domain.Instance) error
	// Delete removes an instance record from persistent storage.
	Delete(ctx context.Context, id uuid.UUID) error
}

// LaunchParams encapsulates parameters for LaunchInstance.
type LaunchParams struct {
	Name         string
	Image        string
	Ports        string
	InstanceType string
	VpcID        *uuid.UUID
	SubnetID     *uuid.UUID
	Volumes      []domain.VolumeAttachment
	VolumeBinds  []string
	Env          []string
	Cmd          []string
	CPULimit     int64
	MemoryLimit  int64
	DiskLimit    int64
	SSHKeyID     *uuid.UUID
	Metadata     map[string]string
	Labels       map[string]string
}

// InstanceService defines the business logic for managing the lifecycle of compute instances.
type InstanceService interface {
	// LaunchInstance provisions and starts a new compute resource with requested networking and storage.
	LaunchInstance(ctx context.Context, params LaunchParams) (*domain.Instance, error)
	// LaunchInstanceWithOptions provisions an instance using structured options (useful for Cloud-Init).
	LaunchInstanceWithOptions(ctx context.Context, opts CreateInstanceOptions) (*domain.Instance, error)
	// StartInstance boots up a stopped instance.
	StartInstance(ctx context.Context, idOrName string) error
	// StopInstance gracefully shuts down or halts a running compute resource.
	StopInstance(ctx context.Context, idOrName string) error
	// PauseInstance freezes a running instance (CPU halted, memory/network retained).
	PauseInstance(ctx context.Context, idOrName string) error
	// ResumeInstance resumes a paused instance back to running state.
	ResumeInstance(ctx context.Context, idOrName string) error
	// ListInstances returns a slice of all compute resources accessible to the caller.
	// tagFilter optionally restricts results to instances matching all given label constraints.
	ListInstances(ctx context.Context, tagFilter []string) ([]*domain.Instance, error)
	// GetInstance retrieves detailed information about a specific compute resource.
	GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error)
	// GetInstanceLogs retrieves recent console/system output from the instance.
	GetInstanceLogs(ctx context.Context, idOrName string) (string, error)
	// GetInstanceStats retrieves real-time performance metrics for the instance.
	GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error)
	// GetConsoleURL provides a secure endpoint for remote serial/VNC console access.
	GetConsoleURL(ctx context.Context, idOrName string) (string, error)
	// TerminateInstance permanently removes a compute resource and releases its allocated assets.
	TerminateInstance(ctx context.Context, idOrName string) error
	// Exec runs a command inside the instance and returns the output.
	Exec(ctx context.Context, idOrName string, cmd []string) (string, error)
	// UpdateInstanceMetadata updates the metadata and labels of an instance.
	UpdateInstanceMetadata(ctx context.Context, id uuid.UUID, metadata, labels map[string]string) error
	// ResizeInstance changes the instance type (CPU/memory) of an existing instance.
	ResizeInstance(ctx context.Context, idOrName, newInstanceType string) (*domain.Instance, error)
	// GetTags returns the label set for an instance.
	GetTags(ctx context.Context, id uuid.UUID) (map[string]string, error)
	// SetTags replaces an instance's labels with the provided map (add/update).
	SetTags(ctx context.Context, id uuid.UUID, labels map[string]string) error
	// RemoveTag deletes a single label key from an instance.
	RemoveTag(ctx context.Context, id uuid.UUID, key string) error
}
