// Package postgres provides Postgres-backed repository implementations.
package postgres

import (
	"context"

	stdlib_errors "errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poyrazk/thecloud/internal/core"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

// TenantRepo persists tenant data in Postgres.
type TenantRepo struct {
	db DB
}

// NewTenantRepo constructs a TenantRepo.
func NewTenantRepo(db DB) *TenantRepo {
	return &TenantRepo{db: db}
}

// getDB returns the underlying DB, or a transaction from context if present.
func (r *TenantRepo) getDB(ctx context.Context) txCapableDB {
	if tx := core.TransactionFromContext(ctx); tx != nil {
		return tx.(pgx.Tx)
	}
	return r.db
}

func (r *TenantRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
	query := `
		INSERT INTO tenants (id, name, slug, owner_id, plan, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		tenant.ID, tenant.Name, tenant.Slug, tenant.OwnerID,
		tenant.Plan, tenant.Status, tenant.CreatedAt, tenant.UpdatedAt,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create tenant", err)
	}
	return nil
}

func (r *TenantRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	query := `SELECT id, name, slug, owner_id, plan, status, created_at, updated_at FROM tenants WHERE id = $1`
	var t domain.Tenant
	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(
		&t.ID, &t.Name, &t.Slug, &t.OwnerID, &t.Plan, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "tenant not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to get tenant", err)
	}
	return &t, nil
}

func (r *TenantRepo) GetBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	query := `SELECT id, name, slug, owner_id, plan, status, created_at, updated_at FROM tenants WHERE slug = $1`
	var t domain.Tenant
	err := r.getDB(ctx).QueryRow(ctx, query, slug).Scan(
		&t.ID, &t.Name, &t.Slug, &t.OwnerID, &t.Plan, &t.Status, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "tenant not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to get tenant by slug", err)
	}
	return &t, nil
}

func (r *TenantRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
	query := `
		UPDATE tenants
		SET name = $2, slug = $3, owner_id = $4, plan = $5, status = $6, updated_at = $7
		WHERE id = $1
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		tenant.ID, tenant.Name, tenant.Slug, tenant.OwnerID,
		tenant.Plan, tenant.Status, tenant.UpdatedAt,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update tenant", err)
	}
	return nil
}

func (r *TenantRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.getDB(ctx).Exec(ctx, "DELETE FROM tenants WHERE id = $1", id)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete tenant", err)
	}
	return nil
}

func (r *TenantRepo) AddMember(ctx context.Context, tenantID, userID uuid.UUID, role string) error {
	query := `INSERT INTO tenant_members (tenant_id, user_id, role, joined_at) VALUES ($1, $2, $3, NOW())`
	_, err := r.getDB(ctx).Exec(ctx, query, tenantID, userID, role)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to add tenant member", err)
	}
	return nil
}

func (r *TenantRepo) RemoveMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	_, err := r.getDB(ctx).Exec(ctx, "DELETE FROM tenant_members WHERE tenant_id = $1 AND user_id = $2", tenantID, userID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to remove tenant member", err)
	}
	return nil
}

func (r *TenantRepo) ListMembers(ctx context.Context, tenantID uuid.UUID) ([]domain.TenantMember, error) {
	query := `SELECT tenant_id, user_id, role, joined_at FROM tenant_members WHERE tenant_id = $1`
	rows, err := r.getDB(ctx).Query(ctx, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list tenant members", err)
	}
	defer rows.Close()

	var members []domain.TenantMember
	for rows.Next() {
		var m domain.TenantMember
		if err := rows.Scan(&m.TenantID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan tenant member", err)
		}
		members = append(members, m)
	}
	return members, nil
}

func (r *TenantRepo) GetMembership(ctx context.Context, tenantID, userID uuid.UUID) (*domain.TenantMember, error) {
	query := `SELECT tenant_id, user_id, role, joined_at FROM tenant_members WHERE tenant_id = $1 AND user_id = $2`
	var m domain.TenantMember
	err := r.getDB(ctx).QueryRow(ctx, query, tenantID, userID).Scan(&m.TenantID, &m.UserID, &m.Role, &m.JoinedAt)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No membership is not an error here
		}
		return nil, errors.Wrap(errors.Internal, "failed to get membership", err)
	}
	return &m, nil
}

func (r *TenantRepo) ListUserTenants(ctx context.Context, userID uuid.UUID) ([]domain.Tenant, error) {
	query := `
		SELECT t.id, t.name, t.slug, t.owner_id, t.plan, t.status, t.created_at, t.updated_at
		FROM tenants t
		JOIN tenant_members tm ON t.id = tm.tenant_id
		WHERE tm.user_id = $1
	`
	rows, err := r.getDB(ctx).Query(ctx, query, userID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list user tenants", err)
	}
	defer rows.Close()

	var tenants []domain.Tenant
	for rows.Next() {
		var t domain.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.Slug, &t.OwnerID, &t.Plan, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan tenant", err)
		}
		tenants = append(tenants, t)
	}
	return tenants, nil
}

func (r *TenantRepo) GetQuota(ctx context.Context, tenantID uuid.UUID) (*domain.TenantQuota, error) {
	query := `
		SELECT 
			tq.tenant_id, 
			tq.max_instances, 
			tq.max_vpcs, 
			tq.max_storage_gb, 
			tq.max_memory_gb, 
			tq.max_vcpus,
			(SELECT COUNT(*) FROM instances WHERE tenant_id = tq.tenant_id AND status != 'DELETED') as used_instances,
			(SELECT COUNT(*) FROM vpcs WHERE tenant_id = tq.tenant_id) as used_vpcs,
			(SELECT COALESCE(SUM(size_gb), 0) FROM volumes WHERE tenant_id = tq.tenant_id AND status != 'deleted') as used_storage_gb,
			tq.used_memory_gb,
			tq.used_vcpus
		FROM tenant_quotas tq 
		WHERE tq.tenant_id = $1
	`
	var q domain.TenantQuota
	err := r.getDB(ctx).QueryRow(ctx, query, tenantID).Scan(
		&q.TenantID,
		&q.MaxInstances,
		&q.MaxVPCs,
		&q.MaxStorageGB,
		&q.MaxMemoryGB,
		&q.MaxVCPUs,
		&q.UsedInstances,
		&q.UsedVPCs,
		&q.UsedStorageGB,
		&q.UsedMemoryGB,
		&q.UsedVCPUs,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "quota not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to get quota", err)
	}
	return &q, nil
}

func (r *TenantRepo) UpdateQuota(ctx context.Context, quota *domain.TenantQuota) error {
	query := `
		INSERT INTO tenant_quotas (tenant_id, max_instances, max_vpcs, max_storage_gb, max_memory_gb, max_vcpus)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id) DO UPDATE
		SET max_instances = $2, max_vpcs = $3, max_storage_gb = $4, max_memory_gb = $5, max_vcpus = $6
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		quota.TenantID, quota.MaxInstances, quota.MaxVPCs, quota.MaxStorageGB, quota.MaxMemoryGB, quota.MaxVCPUs,
	)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update quota", err)
	}
	return nil
}

func (r *TenantRepo) IncrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int) error {
	var query string
	switch resource {
	case "vcpus":
		query = `UPDATE tenant_quotas SET used_vcpus = used_vcpus + $1 WHERE tenant_id = $2`
	case "memory":
		query = `UPDATE tenant_quotas SET used_memory_gb = used_memory_gb + $1 WHERE tenant_id = $2`
	// For instances, vpcs, storage we rely on live counts for now, but if we wanted to switch:
	// case "instances":
	// 	query = `UPDATE tenant_quotas SET used_instances = used_instances + $1 WHERE tenant_id = $2`
	default:
		// No-op for resources tracked via live count
		return nil
	}

	_, err := r.getDB(ctx).Exec(ctx, query, amount, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to increment quota usage", err)
	}
	return nil
}

func (r *TenantRepo) DecrementUsage(ctx context.Context, tenantID uuid.UUID, resource string, amount int) error {
	var query string
	switch resource {
	case "vcpus":
		query = `UPDATE tenant_quotas SET used_vcpus = GREATEST(0, used_vcpus - $1) WHERE tenant_id = $2`
	case "memory":
		query = `UPDATE tenant_quotas SET used_memory_gb = GREATEST(0, used_memory_gb - $1) WHERE tenant_id = $2`
	default:
		return nil
	}

	_, err := r.getDB(ctx).Exec(ctx, query, amount, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to decrement quota usage", err)
	}
	return nil
}
