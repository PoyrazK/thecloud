// Package postgres provides PostgreSQL-backed repository implementations.
package postgres

import (
	"context"
	"fmt"
	"time"

	stdlib_errors "errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

// DatabaseRepository provides PostgreSQL-backed database persistence.
type DatabaseRepository struct {
	db DB
}

// NewDatabaseRepository creates a DatabaseRepository using the provided DB.
func NewDatabaseRepository(db DB) *DatabaseRepository {
	return &DatabaseRepository{db: db}
}

func (r *DatabaseRepository) Create(ctx context.Context, db *domain.Database) error {
	query := `
		INSERT INTO databases (id, user_id, tenant_id, name, engine, version, status, role, primary_id, vpc_id, container_id, port, username, password, created_at, updated_at, allocated_storage, parameters, metrics_enabled, metrics_port, exporter_container_id, pooling_enabled, pooling_port, pooler_container_id, credential_path, credential_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)
	`
	_, err := r.db.Exec(ctx, query,
		db.ID, db.UserID, db.TenantID, db.Name, db.Engine, db.Version, db.Status, db.Role, db.PrimaryID, db.VpcID, db.ContainerID, db.Port, db.Username, db.Password, db.CreatedAt, db.UpdatedAt, db.AllocatedStorage, db.Parameters, db.MetricsEnabled, db.MetricsPort, db.ExporterContainerID, db.PoolingEnabled, db.PoolingPort, db.PoolerContainerID, db.CredentialPath, db.CredentialVersion,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create database", err)
	}
	return nil
}

func (r *DatabaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, tenant_id, name, engine, version, status, role, primary_id, vpc_id, COALESCE(container_id, ''), port, username, password, created_at, updated_at, allocated_storage, parameters, metrics_enabled, COALESCE(metrics_port, 0), COALESCE(exporter_container_id, ''), pooling_enabled, COALESCE(pooling_port, 0), COALESCE(pooler_container_id, ''), COALESCE(credential_path, ''), COALESCE(credential_version, 1)
		FROM databases
		WHERE id = $1 AND tenant_id = $2
	`
	return r.scanDatabase(r.db.QueryRow(ctx, query, id, tenantID))
}

func (r *DatabaseRepository) List(ctx context.Context) ([]*domain.Database, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, tenant_id, name, engine, version, status, role, primary_id, vpc_id, COALESCE(container_id, ''), port, username, password, created_at, updated_at, allocated_storage, parameters, metrics_enabled, COALESCE(metrics_port, 0), COALESCE(exporter_container_id, ''), pooling_enabled, COALESCE(pooling_port, 0), COALESCE(pooler_container_id, ''), COALESCE(credential_path, ''), COALESCE(credential_version, 1)
		FROM databases
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list databases", err)
	}
	return r.scanDatabases(rows)
}

func (r *DatabaseRepository) ListReplicas(ctx context.Context, primaryID uuid.UUID) ([]*domain.Database, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT id, user_id, tenant_id, name, engine, version, status, role, primary_id, vpc_id, COALESCE(container_id, ''), port, username, password, created_at, updated_at, allocated_storage, parameters, metrics_enabled, COALESCE(metrics_port, 0), COALESCE(exporter_container_id, ''), pooling_enabled, COALESCE(pooling_port, 0), COALESCE(pooler_container_id, ''), COALESCE(credential_path, ''), COALESCE(credential_version, 1)
		FROM databases
		WHERE primary_id = $1 AND tenant_id = $2
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, primaryID, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list replicas", err)
	}
	return r.scanDatabases(rows)
}

func (r *DatabaseRepository) scanDatabase(row pgx.Row) (*domain.Database, error) {
	var db domain.Database
	var engine, status, role string
	err := row.Scan(
		&db.ID, &db.UserID, &db.TenantID, &db.Name, &engine, &db.Version, &status, &role, &db.PrimaryID, &db.VpcID, &db.ContainerID, &db.Port, &db.Username, &db.Password, &db.CreatedAt, &db.UpdatedAt, &db.AllocatedStorage, &db.Parameters, &db.MetricsEnabled, &db.MetricsPort, &db.ExporterContainerID, &db.PoolingEnabled, &db.PoolingPort, &db.PoolerContainerID, &db.CredentialPath, &db.CredentialVersion,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "database not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to scan database", err)
	}
	db.Engine = domain.DatabaseEngine(engine)
	db.Status = domain.DatabaseStatus(status)
	db.Role = domain.DatabaseRole(role)
	return &db, nil
}

func (r *DatabaseRepository) scanDatabases(rows pgx.Rows) ([]*domain.Database, error) {
	defer rows.Close()
	databases := make([]*domain.Database, 0)
	for rows.Next() {
		db, err := r.scanDatabase(rows)
		if err != nil {
			return nil, err
		}
		databases = append(databases, db)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return databases, nil
}

func (r *DatabaseRepository) Update(ctx context.Context, db *domain.Database) error {
	query := `
		UPDATE databases
		SET name = $1, status = $2, role = $3, primary_id = $4, container_id = $5, port = $6, updated_at = $7, parameters = $8, metrics_enabled = $9, metrics_port = $10, exporter_container_id = $11, pooling_enabled = $12, pooling_port = $13, pooler_container_id = $14, allocated_storage = $15, credential_path = $16, credential_version = $17
		WHERE id = $18 AND tenant_id = $19
	`
	now := time.Now()
	cmd, err := r.db.Exec(ctx, query, db.Name, db.Status, db.Role, db.PrimaryID, db.ContainerID, db.Port, now, db.Parameters, db.MetricsEnabled, db.MetricsPort, db.ExporterContainerID, db.PoolingEnabled, db.PoolingPort, db.PoolerContainerID, db.AllocatedStorage, db.CredentialPath, db.CredentialVersion, db.ID, db.TenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update database", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "database not found")
	}
	db.UpdatedAt = now
	return nil
}

func (r *DatabaseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `DELETE FROM databases WHERE id = $1 AND tenant_id = $2`
	cmd, err := r.db.Exec(ctx, query, id, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete database", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, fmt.Sprintf("database %s not found", id))
	}
	return nil
}
