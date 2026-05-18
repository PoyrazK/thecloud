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

	query := `
		INSERT INTO policies (id, tenant_id, name, description, statements)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = r.db.Exec(ctx, query, policy.ID, tenantID, policy.Name, policy.Description, statementsJSON)
	return err
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

	query := `
		UPDATE policies
		SET name = $1, description = $2, statements = $3, updated_at = NOW()
		WHERE id = $4 AND tenant_id = $5
	`
	result, err := r.db.Exec(ctx, query, policy.Name, policy.Description, statementsJSON, policy.ID, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "policy not found")
	}
	return nil
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
