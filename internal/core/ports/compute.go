// Package ports defines service and repository interfaces.
package ports

import (
	"context"
	"io"
)

// ComputeBackend abstracts the underlying infrastructure provider (Docker, Libvirt, Cloud Hypervisor, etc.)
// It provides a unified set of operations to manage compute instances and their environment.
type ComputeBackend interface {
	// Instance Lifecycle

	// LaunchInstanceWithOptions provisions a new compute entity based on the provided options.
	// Returns the unique identifier of the instance and any allocated ports.
	LaunchInstanceWithOptions(ctx context.Context, opts CreateInstanceOptions) (string, []string, error)
	// StartInstance boots up a stopped instance.
	StartInstance(ctx context.Context, id string) error
	// StopInstance gracefully shuts down or forcibly terminates a running instance.
	StopInstance(ctx context.Context, id string) error
	// PauseInstance freezes a running instance (CPU halted, memory/network retained).
	PauseInstance(ctx context.Context, id string) error
	// ResumeInstance resumes a paused instance back to running state.
	ResumeInstance(ctx context.Context, id string) error
	// DeleteInstance removes an instance and its ephemeral resources.
	DeleteInstance(ctx context.Context, id string) error
	// GetInstanceLogs returns a stream of stdout/stderr from the instance.
	GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error)
	// GetInstanceStats retrieves real-time resource utilization (CPU, RAM, Network).
	GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error)
	// GetInstancePort retrieves the mapped host port for a specific internal port.
	GetInstancePort(ctx context.Context, id string, internalPort string) (int, error)
	// GetInstanceIP retrieves the primary internal IP address of the instance.
	GetInstanceIP(ctx context.Context, id string) (string, error)
	// GetConsoleURL returns a secure URL for interacting with the instance's serial/VNC console.
	GetConsoleURL(ctx context.Context, id string) (string, error)

	// Execution

	// Exec runs a command within the context of a running instance and returns its output.
	Exec(ctx context.Context, id string, cmd []string) (string, error)
	// RunTask launches a short-lived execution task.
	RunTask(ctx context.Context, opts RunTaskOptions) (string, []string, error)
	// WaitTask blocks until a specific task completes and returns its exit code.
	WaitTask(ctx context.Context, id string) (int64, error)

	// Network Management

	// CreateNetwork establishes a new L2/L3 isolation boundary.
	CreateNetwork(ctx context.Context, name string) (string, error)
	// DeleteNetwork removes a network boundary.
	DeleteNetwork(ctx context.Context, id string) error

	// Volume/Disk Attachment (Physical/Block)

	// AttachVolume connects a storage resource to the instance.
	// Returns the guest-assigned device path (e.g., "/dev/vdb") and the new container ID
	// (container ID may change after stop->recreate->start for Docker bind mounts).
	AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error)
	// DetachVolume disconnects a storage resource from the instance.
	// Returns the new container ID (container ID may change after stop->recreate->start for Docker bind mounts).
	DetachVolume(ctx context.Context, id string, volumePath string) (string, error)

	// Health

	// Ping checks the connectivity and health of the backend provider.
	Ping(ctx context.Context) error
	// Type returns a string identifier of the backend (e.g., "docker", "kvm").
	Type() string
	// ResizeInstance updates the CPU and memory limits of a running or stopped instance.
	ResizeInstance(ctx context.Context, id string, cpu, memory int64) error

	// Snapshots

	// CreateSnapshot creates a point-in-time snapshot of the instance's root disk.
	// The snapshot is stored internally and can be used for rollback via RestoreSnapshot.
	CreateSnapshot(ctx context.Context, id, name string) error
	// RestoreSnapshot restores the instance's root disk from a previously created snapshot.
	RestoreSnapshot(ctx context.Context, id, name string) error
	// DeleteSnapshot deletes a previously created snapshot.
	DeleteSnapshot(ctx context.Context, id, name string) error

	// Test support

	// ResetCircuitBreaker resets the circuit breaker state. Used by E2E tests
	// to ensure clean state between test runs. No-op for backends that don't
	// implement circuit breaker.
	ResetCircuitBreaker()
}
