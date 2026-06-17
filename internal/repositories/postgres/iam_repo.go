package postgres

import (
	"context"
	"encoding/json"
	stdlib_errors "errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

type iamRepository struct {
	db DB
}

// NewIAMRepository creates a new postgres-backed IAM repository.
func NewIAMRepository(db DB) *iamRepository {
	return &iamRepository{db: db}
}

func (r *iamRepository) CreatePolicy(ctx context.Context, tenantID uuid.UUID, policy *domain.Policy) error {
	statementsJSON, err := json.Marshal(policy.Statements)
	if err != nil {
		return fmt.Errorf("failed to marshal statements: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert current policy row
	_, err = tx.Exec(ctx, `
		INSERT INTO policies (id, tenant_id, name, description, statements)
		VALUES ($1, $2, $3, $4, $5)
	`, policy.ID, tenantID, policy.Name, policy.Description, statementsJSON)
	if err != nil {
		return err
	}

	// Insert version 1
	_, err = tx.Exec(ctx, `
		INSERT INTO policy_versions (id, policy_id, version_number, name, description, statements)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New(), policy.ID, 1, policy.Name, policy.Description, statementsJSON)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *iamRepository) GetPolicyByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*domain.Policy, error) {
	query := `SELECT id, tenant_id, name, description, statements FROM policies WHERE id = $1 AND tenant_id = $2`
	var p domain.Policy
	var statementsJSON []byte

	err := r.db.QueryRow(ctx, query, id, tenantID).Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &statementsJSON)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "policy not found")
		}
		return nil, err
	}

	if err := json.Unmarshal(statementsJSON, &p.Statements); err != nil {
		return nil, fmt.Errorf("failed to unmarshal statements: %w", err)
	}

	return &p, nil
}

func (r *iamRepository) ListPolicies(ctx context.Context, tenantID uuid.UUID) ([]*domain.Policy, error) {
	query := `SELECT id, tenant_id, name, description, statements FROM policies WHERE tenant_id = $1`
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*domain.Policy
	for rows.Next() {
		var p domain.Policy
		var statementsJSON []byte
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &statementsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(statementsJSON, &p.Statements); err != nil {
			return nil, err
		}
		policies = append(policies, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *iamRepository) UpdatePolicy(ctx context.Context, tenantID uuid.UUID, policy *domain.Policy) error {
	statementsJSON, err := json.Marshal(policy.Statements)
	if err != nil {
		return fmt.Errorf("failed to marshal statements: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Atomically get max version and insert new version row using CTE
	// This prevents race conditions where two concurrent updates both read the same max.
	_, err = tx.Exec(ctx, `
		WITH max_version AS (
			SELECT COALESCE(MAX(version_number), 0) + 1 AS new_version
			FROM policy_versions
			WHERE policy_id = $1
		)
		INSERT INTO policy_versions (id, policy_id, version_number, name, description, statements)
		SELECT $2, $1, new_version, $3, $4, $5 FROM max_version
	`, policy.ID, uuid.New(), policy.Name, policy.Description, statementsJSON)
	if err != nil {
		return err
	}

	// Update current policy row for fast lookups
	_, err = tx.Exec(ctx, `
		UPDATE policies
		SET name = $1, description = $2, statements = $3, updated_at = NOW()
		WHERE id = $4 AND tenant_id = $5
	`, policy.Name, policy.Description, statementsJSON, policy.ID, tenantID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// SyncPolicyCurrentState updates the policies table and inserts a new version row
// without incrementing the version number (used by rollback to sync to a specific version).
func (r *iamRepository) SyncPolicyCurrentState(ctx context.Context, tenantID uuid.UUID, pv *domain.PolicyVersion) error {
	statementsJSON, err := json.Marshal(pv.Statements)
	if err != nil {
		return fmt.Errorf("failed to marshal statements: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Insert version row
	_, err = tx.Exec(ctx, `
		INSERT INTO policy_versions (id, policy_id, version_number, name, description, statements)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, pv.ID, pv.PolicyID, pv.VersionNumber, pv.Name, pv.Description, statementsJSON)
	if err != nil {
		return err
	}

	// Update current policy row for fast lookups
	_, err = tx.Exec(ctx, `
		UPDATE policies
		SET name = $1, description = $2, statements = $3, updated_at = NOW()
		WHERE id = $4 AND tenant_id = $5
	`, pv.Name, pv.Description, statementsJSON, pv.PolicyID, tenantID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *iamRepository) DeletePolicy(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, "DELETE FROM policies WHERE id = $1 AND tenant_id = $2", id, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "policy not found")
	}
	return nil
}

func (r *iamRepository) ListPolicyVersions(ctx context.Context, tenantID uuid.UUID, policyID uuid.UUID) ([]*domain.PolicyVersion, error) {
	query := `
		SELECT pv.id, pv.policy_id, pv.version_number, pv.name, pv.description, pv.statements, pv.created_at, pv.created_by
		FROM policy_versions pv
		JOIN policies p ON pv.policy_id = p.id
		WHERE pv.policy_id = $1 AND p.tenant_id = $2
		ORDER BY pv.version_number DESC
	`
	rows, err := r.db.Query(ctx, query, policyID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*domain.PolicyVersion
	for rows.Next() {
		var pv domain.PolicyVersion
		var statementsJSON []byte
		if err := rows.Scan(&pv.ID, &pv.PolicyID, &pv.VersionNumber, &pv.Name, &pv.Description, &statementsJSON, &pv.CreatedAt, &pv.CreatedBy); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(statementsJSON, &pv.Statements); err != nil {
			return nil, err
		}
		versions = append(versions, &pv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

func (r *iamRepository) GetPolicyVersion(ctx context.Context, tenantID uuid.UUID, policyID uuid.UUID, versionNumber int) (*domain.PolicyVersion, error) {
	query := `
		SELECT pv.id, pv.policy_id, pv.version_number, pv.name, pv.description, pv.statements, pv.created_at, pv.created_by
		FROM policy_versions pv
		JOIN policies p ON pv.policy_id = p.id
		WHERE pv.policy_id = $1 AND p.tenant_id = $2 AND pv.version_number = $3
	`
	var pv domain.PolicyVersion
	var statementsJSON []byte
	err := r.db.QueryRow(ctx, query, policyID, tenantID, versionNumber).
		Scan(&pv.ID, &pv.PolicyID, &pv.VersionNumber, &pv.Name, &pv.Description, &statementsJSON, &pv.CreatedAt, &pv.CreatedBy)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "policy version not found")
		}
		return nil, err
	}
	if err := json.Unmarshal(statementsJSON, &pv.Statements); err != nil {
		return nil, fmt.Errorf("failed to unmarshal statements: %w", err)
	}
	return &pv, nil
}

func (r *iamRepository) InsertPolicyVersion(ctx context.Context, tenantID uuid.UUID, pv *domain.PolicyVersion) error {
	statementsJSON, err := json.Marshal(pv.Statements)
	if err != nil {
		return fmt.Errorf("failed to marshal statements: %w", err)
	}

	query := `
		INSERT INTO policy_versions (id, policy_id, version_number, name, description, statements, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = r.db.Exec(ctx, query, pv.ID, pv.PolicyID, pv.VersionNumber, pv.Name, pv.Description, statementsJSON, pv.CreatedBy)
	return err
}

func (r *iamRepository) AttachPolicyToUser(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, policyID uuid.UUID) error {
	// Verify policy belongs to tenant
	if _, err := r.GetPolicyByID(ctx, tenantID, policyID); err != nil {
		return err
	}

	query := `INSERT INTO user_policies (user_id, policy_id, tenant_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, query, userID, policyID, tenantID)
	return err
}

func (r *iamRepository) DetachPolicyFromUser(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, policyID uuid.UUID) error {
	query := `DELETE FROM user_policies WHERE user_id = $1 AND policy_id = $2 AND tenant_id = $3`
	result, err := r.db.Exec(ctx, query, userID, policyID, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "policy assignment not found")
	}
	return nil
}

func (r *iamRepository) GetPoliciesForUser(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID) ([]*domain.Policy, error) {
	query := `
		SELECT p.id, p.tenant_id, p.name, p.description, p.statements
		FROM policies p
		JOIN user_policies up ON p.id = up.policy_id
		WHERE up.user_id = $1 AND up.tenant_id = $2
	`
	rows, err := r.db.Query(ctx, query, userID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*domain.Policy
	for rows.Next() {
		var p domain.Policy
		var statementsJSON []byte
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &statementsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(statementsJSON, &p.Statements); err != nil {
			return nil, err
		}
		policies = append(policies, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *iamRepository) AttachPolicyToRole(ctx context.Context, tenantID uuid.UUID, roleName string, policyID uuid.UUID) error {
	// Verify policy belongs to tenant
	if _, err := r.GetPolicyByID(ctx, tenantID, policyID); err != nil {
		return err
	}

	query := `INSERT INTO role_policies (role_name, policy_id, tenant_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, query, roleName, policyID, tenantID)
	return err
}

func (r *iamRepository) DetachPolicyFromRole(ctx context.Context, tenantID uuid.UUID, roleName string, policyID uuid.UUID) error {
	query := `DELETE FROM role_policies WHERE role_name = $1 AND policy_id = $2 AND tenant_id = $3`
	result, err := r.db.Exec(ctx, query, roleName, policyID, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "policy assignment not found")
	}
	return nil
}

func (r *iamRepository) GetPoliciesForRole(ctx context.Context, tenantID uuid.UUID, roleName string) ([]*domain.Policy, error) {
	query := `
		SELECT p.id, p.tenant_id, p.name, p.description, p.statements
		FROM policies p
		JOIN role_policies rp ON p.id = rp.policy_id
		WHERE rp.role_name = $1 AND rp.tenant_id = $2
	`
	rows, err := r.db.Query(ctx, query, roleName, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*domain.Policy
	for rows.Next() {
		var p domain.Policy
		var statementsJSON []byte
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &statementsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(statementsJSON, &p.Statements); err != nil {
			return nil, err
		}
		policies = append(policies, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *iamRepository) AttachPolicyToServiceAccount(ctx context.Context, tenantID uuid.UUID, saID uuid.UUID, policyID uuid.UUID) error {
	// Verify policy belongs to tenant
	if _, err := r.GetPolicyByID(ctx, tenantID, policyID); err != nil {
		return err
	}

	query := `INSERT INTO service_account_policies (service_account_id, policy_id, tenant_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, query, saID, policyID, tenantID)
	return err
}

func (r *iamRepository) DetachPolicyFromServiceAccount(ctx context.Context, tenantID uuid.UUID, saID uuid.UUID, policyID uuid.UUID) error {
	query := `DELETE FROM service_account_policies WHERE service_account_id = $1 AND policy_id = $2 AND tenant_id = $3`
	result, err := r.db.Exec(ctx, query, saID, policyID, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "policy assignment not found")
	}
	return nil
}

func (r *iamRepository) GetPoliciesForServiceAccount(ctx context.Context, tenantID uuid.UUID, saID uuid.UUID) ([]*domain.Policy, error) {
	query := `
		SELECT p.id, p.tenant_id, p.name, p.description, p.statements
		FROM policies p
		JOIN service_account_policies sap ON p.id = sap.policy_id
		WHERE sap.service_account_id = $1 AND sap.tenant_id = $2
	`
	rows, err := r.db.Query(ctx, query, saID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*domain.Policy
	for rows.Next() {
		var p domain.Policy
		var statementsJSON []byte
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &statementsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(statementsJSON, &p.Statements); err != nil {
			return nil, err
		}
		policies = append(policies, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return policies, nil
}

// CreateGroup creates a new group.
func (r *iamRepository) CreateGroup(ctx context.Context, tenantID uuid.UUID, group *domain.Group) error {
	query := `INSERT INTO "groups" (id, tenant_id, name, description) VALUES ($1, $2, $3, $4)`
	_, err := r.db.Exec(ctx, query, group.ID, tenantID, group.Name, group.Description)
	return err
}

// GetGroupByID retrieves a group by ID.
func (r *iamRepository) GetGroupByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*domain.Group, error) {
	query := `SELECT id, tenant_id, name, description, created_at, updated_at FROM "groups" WHERE id = $1 AND tenant_id = $2`
	var g domain.Group
	err := r.db.QueryRow(ctx, query, id, tenantID).Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "group not found")
		}
		return nil, err
	}
	return &g, nil
}

// ListGroups lists all groups for a tenant.
func (r *iamRepository) ListGroups(ctx context.Context, tenantID uuid.UUID) ([]*domain.Group, error) {
	query := `SELECT id, tenant_id, name, description, created_at, updated_at FROM "groups" WHERE tenant_id = $1`
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*domain.Group
	for rows.Next() {
		var g domain.Group
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, &g)
	}
	return groups, rows.Err()
}

// UpdateGroup updates a group's name and description.
func (r *iamRepository) UpdateGroup(ctx context.Context, tenantID uuid.UUID, group *domain.Group) error {
	query := `UPDATE "groups" SET name = $1, description = $2, updated_at = NOW() WHERE id = $3 AND tenant_id = $4`
	result, err := r.db.Exec(ctx, query, group.Name, group.Description, group.ID, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "group not found")
	}
	return nil
}

// DeleteGroup deletes a group and cascades to user_groups and group_policies.
func (r *iamRepository) DeleteGroup(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `DELETE FROM "groups" WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "group not found")
	}
	return nil
}

// AddUserToGroup adds a user to a group.
func (r *iamRepository) AddUserToGroup(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, groupID uuid.UUID) error {
	// Verify group belongs to tenant
	if _, err := r.GetGroupByID(ctx, tenantID, groupID); err != nil {
		return err
	}
	query := `INSERT INTO user_groups (user_id, group_id, tenant_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, query, userID, groupID, tenantID)
	return err
}

// RemoveUserFromGroup removes a user from a group.
func (r *iamRepository) RemoveUserFromGroup(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, groupID uuid.UUID) error {
	query := `DELETE FROM user_groups WHERE user_id = $1 AND group_id = $2 AND tenant_id = $3`
	result, err := r.db.Exec(ctx, query, userID, groupID, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "group membership not found")
	}
	return nil
}

// GetGroupsForUser returns all groups a user belongs to.
func (r *iamRepository) GetGroupsForUser(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID) ([]*domain.Group, error) {
	query := `
		SELECT g.id, g.tenant_id, g.name, g.description, g.created_at, g.updated_at
		FROM "groups" g
		JOIN user_groups ug ON g.id = ug.group_id
		WHERE ug.user_id = $1 AND ug.tenant_id = $2`
	rows, err := r.db.Query(ctx, query, userID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*domain.Group
	for rows.Next() {
		var g domain.Group
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, &g)
	}
	return groups, rows.Err()
}

// GetUsersInGroup returns all user IDs in a group.
func (r *iamRepository) GetUsersInGroup(ctx context.Context, tenantID uuid.UUID, groupID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT user_id FROM user_groups WHERE group_id = $1 AND tenant_id = $2`
	rows, err := r.db.Query(ctx, query, groupID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, id)
	}
	return userIDs, rows.Err()
}

// AttachPolicyToGroup attaches a policy to a group.
func (r *iamRepository) AttachPolicyToGroup(ctx context.Context, tenantID uuid.UUID, groupID uuid.UUID, policyID uuid.UUID) error {
	// Verify group belongs to tenant
	if _, err := r.GetGroupByID(ctx, tenantID, groupID); err != nil {
		return err
	}
	// Verify policy belongs to tenant
	if _, err := r.GetPolicyByID(ctx, tenantID, policyID); err != nil {
		return err
	}
	query := `INSERT INTO group_policies (group_id, policy_id, tenant_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := r.db.Exec(ctx, query, groupID, policyID, tenantID)
	return err
}

// DetachPolicyFromGroup removes a policy from a group.
func (r *iamRepository) DetachPolicyFromGroup(ctx context.Context, tenantID uuid.UUID, groupID uuid.UUID, policyID uuid.UUID) error {
	query := `DELETE FROM group_policies WHERE group_id = $1 AND policy_id = $2 AND tenant_id = $3`
	result, err := r.db.Exec(ctx, query, groupID, policyID, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "policy assignment not found")
	}
	return nil
}

// GetPoliciesForGroup returns all policies attached to a group.
func (r *iamRepository) GetPoliciesForGroup(ctx context.Context, tenantID uuid.UUID, groupID uuid.UUID) ([]*domain.Policy, error) {
	query := `
		SELECT p.id, p.tenant_id, p.name, p.description, p.statements
		FROM policies p
		JOIN group_policies gp ON p.id = gp.policy_id
		WHERE gp.group_id = $1 AND gp.tenant_id = $2`
	rows, err := r.db.Query(ctx, query, groupID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*domain.Policy
	for rows.Next() {
		var p domain.Policy
		var statementsJSON []byte
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Description, &statementsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(statementsJSON, &p.Statements); err != nil {
			return nil, err
		}
		policies = append(policies, &p)
	}
	return policies, rows.Err()
}

// GetGroupsForPolicy returns all groups a policy is attached to.
func (r *iamRepository) GetGroupsForPolicy(ctx context.Context, tenantID uuid.UUID, policyID uuid.UUID) ([]*domain.Group, error) {
	query := `
		SELECT g.id, g.tenant_id, g.name, g.description, g.created_at, g.updated_at
		FROM "groups" g
		JOIN group_policies gp ON g.id = gp.group_id
		WHERE gp.policy_id = $1 AND gp.tenant_id = $2`
	rows, err := r.db.Query(ctx, query, policyID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*domain.Group
	for rows.Next() {
		var g domain.Group
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, &g)
	}
	return groups, rows.Err()
}
