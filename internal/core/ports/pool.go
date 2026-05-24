// Package ports defines service and repository interfaces.
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PoolConfig holds the sizing parameters for a function's warm container pool.
type PoolConfig struct {
	MinSize     int           `json:"min_size"`      // Minimum warm instances to maintain (default: 1)
	MaxSize     int           `json:"max_size"`      // Maximum warm instances / scale-out limit (default: 10)
	MaxIdleTime time.Duration `json:"max_idle_time"` // Reap instances idle longer than this (default: 5 min)
}

// PoolInstance represents a warm container/VM ready to execute a function.
type PoolInstance struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // "WARM", "BUSY", "STARTING"
	LastUsed  time.Time `json:"last_used"`
	BackendID string    `json:"backend_id"` // Docker container ID, Firecracker/Libvirt VM name
}

// PoolStats tracks the current state of a function's pool.
type PoolStats struct {
	TotalSize int `json:"total_size"` // warm + busy + starting
	WarmCount int `json:"warm_count"`
	BusyCount int `json:"busy_count"`
}

// PoolManager defines the interface for per-function container pools.
// Each function that has a PoolConfig can have its own FunctionPool.
type PoolManager interface {
	// Acquire gets a warm instance for the given function.
	// If no warm instance is available, it will try to scale up to MaxSize.
	// If already at MaxSize and all instances are busy, it waits with backpressure.
	// Returns the instance and a release function.
	// The release function takes an error: nil = return to pool, error = destroy instance.
	Acquire(ctx context.Context, functionID uuid.UUID) (*PoolInstance, func(error), error)

	// GetPoolStats returns current pool utilization for a function.
	GetPoolStats(ctx context.Context, functionID uuid.UUID) (PoolStats, error)

	// InvalidateFunction removes all warm instances for a function.
	// Called when function code or handler is updated, or function is deleted.
	InvalidateFunction(ctx context.Context, functionID uuid.UUID) error
}
