// Package postgres provides PostgreSQL-backed repository implementations.
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

// FunctionRepository provides PostgreSQL-backed function persistence.
type FunctionRepository struct {
	db DB
}

// NewFunctionRepository creates a FunctionRepository using the provided DB.
func NewFunctionRepository(db DB) *FunctionRepository {
	return &FunctionRepository{db: db}
}

func (r *FunctionRepository) Create(ctx context.Context, f *domain.Function) error {
	query := `
		INSERT INTO functions (id, user_id, tenant_id, name, runtime, handler, code_path, timeout_seconds, memory_mb, cpus, status, max_concurrent_invocations, max_queue_depth, max_retries, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err := r.db.Exec(ctx, query,
		f.ID, f.UserID, f.TenantID, f.Name, f.Runtime, f.Handler, f.CodePath, f.Timeout, f.MemoryMB, f.CPUs, f.Status, f.MaxConcurrentInvocations, f.MaxQueueDepth, f.MaxRetries, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create function", err)
	}
	return nil
}

func (r *FunctionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Function, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `SELECT id, user_id, tenant_id, name, runtime, handler, code_path, timeout_seconds, memory_mb, cpus, status, max_concurrent_invocations, max_queue_depth, max_retries, env_vars, created_at, updated_at FROM functions WHERE id = $1 AND tenant_id = $2`
	return r.scanFunction(r.db.QueryRow(ctx, query, id, tenantID))
}

func (r *FunctionRepository) GetByName(ctx context.Context, userID uuid.UUID, name string) (*domain.Function, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `SELECT id, user_id, tenant_id, name, runtime, handler, code_path, timeout_seconds, memory_mb, cpus, status, max_concurrent_invocations, max_queue_depth, max_retries, env_vars, created_at, updated_at FROM functions WHERE name = $1 AND tenant_id = $2`
	return r.scanFunction(r.db.QueryRow(ctx, query, name, tenantID))
}

func (r *FunctionRepository) List(ctx context.Context, userID uuid.UUID) ([]*domain.Function, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `SELECT id, user_id, tenant_id, name, runtime, handler, code_path, timeout_seconds, memory_mb, cpus, status, max_concurrent_invocations, max_queue_depth, max_retries, env_vars, created_at, updated_at FROM functions WHERE tenant_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list functions", err)
	}
	return r.scanFunctions(rows)
}

func (r *FunctionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `DELETE FROM functions WHERE id = $1 AND tenant_id = $2`
	_, err := r.db.Exec(ctx, query, id, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete function", err)
	}
	return nil
}

func (r *FunctionRepository) Update(ctx context.Context, id uuid.UUID, u *domain.FunctionUpdate) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	cols := u.SetColumns()
	if len(cols) == 0 {
		return nil
	}

	setClause := ""
	args := []interface{}{id, tenantID}
	argIdx := 1
	for i, col := range cols {
		if i > 0 {
			setClause += ", "
		}
		setClause += col + " = $" + fmt.Sprintf("%d", i+2)
	}

	query := fmt.Sprintf(`UPDATE functions SET %s, updated_at = NOW() WHERE id = $1 AND tenant_id = $2`, setClause)

	for _, col := range cols {
		switch col {
		case "handler":
			args = append(args, *u.Handler)
		case "timeout_seconds":
			args = append(args, *u.Timeout)
		case "memory_mb":
			args = append(args, *u.MemoryMB)
		case "cpus":
			args = append(args, *u.CPUs)
		case "status":
			args = append(args, u.Status)
		case "max_concurrent_invocations":
			args = append(args, *u.MaxConcurrentInvocations)
		case "max_queue_depth":
			args = append(args, *u.MaxQueueDepth)
		case "max_retries":
			args = append(args, *u.MaxRetries)
		case "env_vars":
			envMap := make(map[string]string)
			for _, e := range u.EnvVars {
				envMap[e.Key] = e.Value
			}
			data, err := json.Marshal(envMap)
			if err != nil {
				return errors.Wrap(errors.Internal, "failed to marshal env vars", err)
			}
			args = append(args, data)
		}
		argIdx++
	}

	_, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update function", err)
	}
	return nil
}

func (r *FunctionRepository) CreateInvocation(ctx context.Context, i *domain.Invocation) error {
	query := `
		INSERT INTO invocations (id, function_id, status, started_at, ended_at, duration_ms, status_code, logs, retry_count, max_retries)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		i.ID, i.FunctionID, i.Status, i.StartedAt, i.EndedAt, i.DurationMs, i.StatusCode, i.Logs, i.RetryCount, i.MaxRetries,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create invocation", err)
	}
	return nil
}

func (r *FunctionRepository) GetInvocations(ctx context.Context, functionID uuid.UUID, limit int) ([]*domain.Invocation, error) {
	query := `SELECT id, function_id, status, started_at, ended_at, duration_ms, status_code, logs, retry_count, max_retries FROM invocations WHERE function_id = $1 ORDER BY started_at DESC LIMIT $2`
	rows, err := r.db.Query(ctx, query, functionID, limit)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to get invocations", err)
	}
	return r.scanInvocations(rows)
}

func (r *FunctionRepository) scanFunction(row pgx.Row) (*domain.Function, error) {
	f := &domain.Function{}
	var envVarsJSON []byte
	err := row.Scan(&f.ID, &f.UserID, &f.TenantID, &f.Name, &f.Runtime, &f.Handler, &f.CodePath, &f.Timeout, &f.MemoryMB, &f.CPUs, &f.Status, &f.MaxConcurrentInvocations, &f.MaxQueueDepth, &f.MaxRetries, &envVarsJSON, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "function not found", err)
	}
	if len(envVarsJSON) > 0 {
		envMap := make(map[string]string)
		if err := json.Unmarshal(envVarsJSON, &envMap); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to unmarshal env vars", err)
		}
		for k, v := range envMap {
			f.EnvVars = append(f.EnvVars, &domain.EnvVar{Key: k, Value: v})
		}
	}
	return f, nil
}

func (r *FunctionRepository) scanFunctions(rows pgx.Rows) ([]*domain.Function, error) {
	defer rows.Close()
	var functions []*domain.Function
	for rows.Next() {
		f, err := r.scanFunction(rows)
		if err != nil {
			return nil, err
		}
		functions = append(functions, f)
	}
	return functions, nil
}

func (r *FunctionRepository) scanInvocation(row pgx.Row) (*domain.Invocation, error) {
	i := &domain.Invocation{}
	err := row.Scan(&i.ID, &i.FunctionID, &i.Status, &i.StartedAt, &i.EndedAt, &i.DurationMs, &i.StatusCode, &i.Logs, &i.RetryCount, &i.MaxRetries)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (r *FunctionRepository) scanInvocations(rows pgx.Rows) ([]*domain.Invocation, error) {
	defer rows.Close()
	var invocations []*domain.Invocation
	for rows.Next() {
		i, err := r.scanInvocation(rows)
		if err != nil {
			return nil, err
		}
		invocations = append(invocations, i)
	}
	return invocations, nil
}
