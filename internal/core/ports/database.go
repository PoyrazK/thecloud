// Package ports defines service and repository interfaces.
package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// DatabaseRepository handles the persistence and retrieval of managed database metadata.
type DatabaseRepository interface {
	// Create saves a new database instance record.
	Create(ctx context.Context, db *domain.Database) error
	// GetByID retrieves a database instance by its unique UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Database, error)
	// List returns all database instances (typically filtered by current user context).
	List(ctx context.Context) ([]*domain.Database, error)
	// ListReplicas returns all replicas associated with a specific primary database.
	ListReplicas(ctx context.Context, primaryID uuid.UUID) ([]*domain.Database, error)
	// Update modifies an existing database's metadata or status.
	Update(ctx context.Context, db *domain.Database) error
	// Delete removes a database record from storage.
	Delete(ctx context.Context, id uuid.UUID) error
}

// CreateDatabaseRequest defines the parameters for provisioning a new database.
type CreateDatabaseRequest struct {
	Name             string            `json:"name"`
	Engine           string            `json:"engine"`
	Version          string            `json:"version"`
	VpcID            *uuid.UUID        `json:"vpc_id,omitempty"`
	AllocatedStorage int               `json:"allocated_storage"`
	Parameters       map[string]string `json:"parameters,omitempty"`
	MetricsEnabled   bool              `json:"metrics_enabled,omitempty"`
	PoolingEnabled   bool              `json:"pooling_enabled,omitempty"`
	KmsKeyID         string            `json:"kms_key_id,omitempty"`
}

// RestoreDatabaseRequest defines the parameters for restoring a database from a snapshot.
type RestoreDatabaseRequest struct {
	SnapshotID       uuid.UUID         `json:"snapshot_id"`
	NewName          string            `json:"new_name"`
	Engine           string            `json:"engine"`
	Version          string            `json:"version"`
	VpcID            *uuid.UUID        `json:"vpc_id,omitempty"`
	AllocatedStorage int               `json:"allocated_storage"`
	Parameters       map[string]string `json:"parameters,omitempty"`
	MetricsEnabled   bool              `json:"metrics_enabled,omitempty"`
	PoolingEnabled   bool              `json:"pooling_enabled,omitempty"`
}

// ModifyDatabaseRequest defines the parameters for updating an existing database.
type ModifyDatabaseRequest struct {
	ID               uuid.UUID
	Parameters       map[string]string
	MetricsEnabled   *bool
	PoolingEnabled   *bool
	AllocatedStorage *int
}

// DatabaseService provides business logic for managing relational database instances (DBaaS).
type DatabaseService interface {
	// CreateDatabase provisions a new managed database instance.
	CreateDatabase(ctx context.Context, req CreateDatabaseRequest) (*domain.Database, error)
	// CreateReplica provisions a new read-only replica of an existing database.
	CreateReplica(ctx context.Context, primaryID uuid.UUID, name string) (*domain.Database, error)
	// PromoteToPrimary promotes a replica to be a primary instance.
	PromoteToPrimary(ctx context.Context, id uuid.UUID) error
	// GetDatabase retrieves details for a specific database.
	GetDatabase(ctx context.Context, id uuid.UUID) (*domain.Database, error)
	// ListDatabases returns all databases for the authorized user.
	ListDatabases(ctx context.Context) ([]*domain.Database, error)
	// DeleteDatabase terminates and deletes a database instance.
	DeleteDatabase(ctx context.Context, id uuid.UUID) error
	// ModifyDatabase updates an existing database's configuration.
	ModifyDatabase(ctx context.Context, req ModifyDatabaseRequest) (*domain.Database, error)
	// GetConnectionString constructs and returns the authorized URI for connecting to the database.
	GetConnectionString(ctx context.Context, id uuid.UUID) (string, error)
	// CreateDatabaseSnapshot initiates a point-in-time copy of a database's underlying volume.
	CreateDatabaseSnapshot(ctx context.Context, databaseID uuid.UUID, description string) (*domain.Snapshot, error)
	// ListDatabaseSnapshots returns all snapshots belonging to a specific database.
	ListDatabaseSnapshots(ctx context.Context, databaseID uuid.UUID) ([]*domain.Snapshot, error)
	// RestoreDatabase creates a new database instance from an existing snapshot.
	RestoreDatabase(ctx context.Context, req RestoreDatabaseRequest) (*domain.Database, error)
	// RotateCredentials regenerates the database password and updates it in the secrets manager.
	RotateCredentials(ctx context.Context, id uuid.UUID, idempotencyKey string) error
	// StopDatabase stops a running database instance, retaining its data volume.
	StopDatabase(ctx context.Context, id uuid.UUID) error
	// StartDatabase starts a stopped database instance.
	StartDatabase(ctx context.Context, id uuid.UUID) error
	// ResizeDatabase resizes the allocated storage for a database instance.
	ResizeDatabase(ctx context.Context, id uuid.UUID, newSizeGB int) error
}
