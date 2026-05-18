// Package ports defines service and repository interfaces.
package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// CacheStats provides operational metrics for a running cache instance.
type CacheStats struct {
	UsedMemoryBytes  int64 // Current memory consumption
	MaxMemoryBytes   int64 // Total memory capacity allocated
	ConnectedClients int   // Number of active client connections
	TotalKeys        int64 // Approximate count of keys stored
}

// CacheService orchestrates the lifecycle and management of managed cache instances (e.g., Redis).
type CacheService interface {
	// CreateCache provisions a new managed cache.
	CreateCache(ctx context.Context, name, version string, memoryMB int, vpcID *uuid.UUID) (*domain.Cache, error)
	// GetCache retrieves a cache instance by its UUID or unique name.
	GetCache(ctx context.Context, idOrName string) (*domain.Cache, error)
	// ListCaches lists all cache instances for the current user.
	ListCaches(ctx context.Context) ([]*domain.Cache, error)
	// DeleteCache decommission an existing cache.
	DeleteCache(ctx context.Context, idOrName string) error
	// GetConnectionString returns the access URI for the cache instance.
	GetConnectionString(ctx context.Context, idOrName string) (string, error)
	// FlushCache purges all data within the cache instance.
	FlushCache(ctx context.Context, idOrName string) error
	// GetCacheStats retrieves real-time performance metrics from the engine.
	GetCacheStats(ctx context.Context, idOrName string) (*CacheStats, error)
	// ResizeCache changes the memory allocation of a running cache instance.
	ResizeCache(ctx context.Context, idOrName string, newMemoryMB int) error
}

// CacheRepository handles the persistence of cache metadata.
type CacheRepository interface {
	// Create saves a new cache record.
	Create(ctx context.Context, cache *domain.Cache) error
	// GetByID fetches a cache by its unique UUID.
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Cache, error)
	// GetByName fetches a cache by its friendly name.
	GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*domain.Cache, error)
	// List returns all caches for a tenant.
	List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Cache, error)
	// Update modifies an existing cache's metadata or status.
	Update(ctx context.Context, cache *domain.Cache) error
	// Delete removes a cache record from storage.
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
}
