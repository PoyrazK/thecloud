// Package domain defines core business entities.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// DatabaseEngine represents the type of relational database management system.
type DatabaseEngine string

const (
	// EnginePostgres represents PostgreSQL.
	EnginePostgres DatabaseEngine = "postgres"
	// EngineMySQL represents MySQL.
	EngineMySQL DatabaseEngine = "mysql"
)

// DatabaseStatus represents the lifecycle state of a managed database instance.
type DatabaseStatus string

const (
	// DatabaseStatusCreating indicates the database is being provisioned.
	DatabaseStatusCreating DatabaseStatus = "CREATING"
	// DatabaseStatusRunning indicates the database is online and accepting connections.
	DatabaseStatusRunning DatabaseStatus = "RUNNING"
	// DatabaseStatusStopped indicates the database instance is halted.
	DatabaseStatusStopped DatabaseStatus = "STOPPED"
	// DatabaseStatusDeleting indicates the database is being removed.
	DatabaseStatusDeleting DatabaseStatus = "DELETING"
	// DatabaseStatusFailed indicates the database encountered a critical error.
	DatabaseStatusFailed DatabaseStatus = "FAILED"
)

// DatabaseRole represents the replication role of a database instance.
type DatabaseRole string

const (
	// RolePrimary indicates this is the primary database instance (read-write).
	RolePrimary DatabaseRole = "PRIMARY"
	// RoleReplica indicates this is a read-only replica of a primary instance.
	RoleReplica DatabaseRole = "REPLICA"
)

// Database represents a managed relational database instance.
type Database struct {
	ID                  uuid.UUID         `json:"id"`
	UserID              uuid.UUID         `json:"user_id"`
	TenantID            uuid.UUID         `json:"tenant_id"`
	Name                string            `json:"name"`
	Engine              DatabaseEngine    `json:"engine"`
	Version             string            `json:"version"` // Engine version (e.g. "15", "8.0")
	Status              DatabaseStatus    `json:"status"`
	Role                DatabaseRole      `json:"role"`
	PrimaryID           *uuid.UUID        `json:"primary_id,omitempty"` // ID of the primary if this is a replica
	VpcID               *uuid.UUID        `json:"vpc_id,omitempty"`     // Optional private networking
	ContainerID         string            `json:"container_id,omitempty"`
	Port                int               `json:"port"`
	Username            string            `json:"username"`
	Password            string            `json:"-"` // Never serialize password to JSON
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	AllocatedStorage    int               `json:"allocated_storage"`
	Parameters          map[string]string `json:"parameters,omitempty"`
	MetricsEnabled      bool              `json:"metrics_enabled"`
	MetricsPort         int               `json:"metrics_port,omitempty"`
	ExporterContainerID string            `json:"-"`
	PoolingEnabled      bool              `json:"pooling_enabled"`
	PoolingPort         int               `json:"pooling_port,omitempty"`
	PoolerContainerID   string            `json:"-"`
	CredentialPath      string            `json:"-"`
	KmsKeyID            string            `json:"kms_key_id,omitempty"`
	EncryptedVolume     bool              `json:"encrypted_volume"`
	VolumeKeyRef        string            `json:"volume_key_ref,omitempty"`
	CredentialVersion   int               `json:"-"`
}
