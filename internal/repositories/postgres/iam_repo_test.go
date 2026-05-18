package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIAMRepository_CreatePolicy(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIAMRepository(mock)
	ctx := context.Background()

	policyID := uuid.New()
	tenantID := uuid.New()
	policy := &domain.Policy{
		ID:       policyID,
		TenantID: tenantID,
		Name:     "TestPolicy",
		Statements: []domain.Statement{
			{Effect: domain.EffectAllow, Action: []string{"*"}},
		},
	}

	t.Run("creates policy and version 1", func(t *testing.T) {
		statementsJSON, _ := json.Marshal(policy.Statements)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO policies").
			WithArgs(policy.ID, tenantID, policy.Name, policy.Description, statementsJSON).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("INSERT INTO policy_versions").
			WithArgs(pgxmock.AnyArg(), policy.ID, 1, policy.Name, policy.Description, statementsJSON).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()

		err := repo.CreatePolicy(ctx, tenantID, policy)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestIAMRepository_GetPolicyByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIAMRepository(mock)
	ctx := context.Background()

	policyID := uuid.New()
	tenantID := uuid.New()
	policy := &domain.Policy{
		ID:       policyID,
		TenantID: tenantID,
		Name:     "TestPolicy",
		Statements: []domain.Statement{
			{Effect: domain.EffectAllow, Action: []string{"*"}},
		},
	}

	t.Run("fetches policy by id", func(t *testing.T) {
		statementsJSON, err := json.Marshal(policy.Statements)
		require.NoError(t, err)
		rows := pgxmock.NewRows([]string{"id", "tenant_id", "name", "description", "statements"}).
			AddRow(policy.ID, policy.TenantID, policy.Name, policy.Description, statementsJSON)

		mock.ExpectQuery("SELECT id, tenant_id, name, description, statements FROM policies").
			WithArgs(policy.ID, policy.TenantID).
			WillReturnRows(rows)

		fetched, err := repo.GetPolicyByID(ctx, policy.TenantID, policy.ID)
		require.NoError(t, err)
		assert.Equal(t, policy.ID, fetched.ID)
		assert.Equal(t, policy.Name, fetched.Name)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestIAMRepository_AttachPolicyToUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIAMRepository(mock)
	ctx := context.Background()

	policyID := uuid.New()
	tenantID := uuid.New()
	userID := uuid.New()
	policy := &domain.Policy{
		ID:       policyID,
		TenantID: tenantID,
		Name:     "TestPolicy",
		Statements: []domain.Statement{
			{Effect: domain.EffectAllow, Action: []string{"*"}},
		},
	}

	t.Run("attaches policy to user", func(t *testing.T) {
		statementsJSON, _ := json.Marshal(policy.Statements)
		policyRows := pgxmock.NewRows([]string{"id", "tenant_id", "name", "description", "statements"}).
			AddRow(policy.ID, policy.TenantID, policy.Name, policy.Description, statementsJSON)
		mock.ExpectQuery("SELECT id, tenant_id, name, description, statements FROM policies").
			WithArgs(policy.ID, tenantID).
			WillReturnRows(policyRows)

		mock.ExpectExec("INSERT INTO user_policies").
			WithArgs(userID, policy.ID, tenantID).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := repo.AttachPolicyToUser(ctx, tenantID, userID, policy.ID)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestIAMRepository_UpdatePolicy_CreatesVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIAMRepository(mock)
	ctx := context.Background()

	policyID := uuid.New()
	tenantID := uuid.New()
	updatedPolicy := &domain.Policy{
		ID:          policyID,
		TenantID:    tenantID,
		Name:        "UpdatedPolicy",
		Description: "Updated description",
		Statements: []domain.Statement{
			{Effect: domain.EffectDeny, Action: []string{"compute:*"}},
		},
	}

	t.Run("creates new version on update", func(t *testing.T) {
		updatedStatementsJSON, _ := json.Marshal(updatedPolicy.Statements)
		mock.ExpectQuery("SELECT COALESCE").
			WithArgs(policyID).
			WillReturnRows(pgxmock.NewRows([]string{"max"}).AddRow(2))

		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO policy_versions").
			WithArgs(pgxmock.AnyArg(), policyID, 3, updatedPolicy.Name, updatedPolicy.Description, updatedStatementsJSON).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("UPDATE policies").
			WithArgs(updatedPolicy.Name, updatedPolicy.Description, updatedStatementsJSON, policyID, tenantID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectCommit()

		err := repo.UpdatePolicy(ctx, tenantID, updatedPolicy)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestIAMRepository_ListPolicyVersions(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIAMRepository(mock)
	ctx := context.Background()

	policyID := uuid.New()
	tenantID := uuid.New()
	policy := &domain.Policy{
		ID:       policyID,
		TenantID: tenantID,
		Name:     "TestPolicy",
		Statements: []domain.Statement{
			{Effect: domain.EffectAllow, Action: []string{"*"}},
		},
	}

	t.Run("lists versions in descending order", func(t *testing.T) {
		statementsJSON, _ := json.Marshal(policy.Statements)
		rows := pgxmock.NewRows([]string{"id", "policy_id", "version_number", "name", "description", "statements", "created_at", "created_by"}).
			AddRow(uuid.New(), policyID, 2, policy.Name, policy.Description, statementsJSON, nil, nil).
			AddRow(uuid.New(), policyID, 1, policy.Name, policy.Description, statementsJSON, nil, nil)

		mock.ExpectQuery("SELECT pv.id, pv.policy_id, pv.version_number, pv.name, pv.description, pv.statements, pv.created_at, pv.created_by").
			WithArgs(policyID, tenantID).
			WillReturnRows(rows)

		versions, err := repo.ListPolicyVersions(ctx, tenantID, policyID)
		require.NoError(t, err)
		require.Len(t, versions, 2)
		assert.Equal(t, 2, versions[0].VersionNumber)
		assert.Equal(t, 1, versions[1].VersionNumber)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestIAMRepository_GetPolicyVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIAMRepository(mock)
	ctx := context.Background()

	policyID := uuid.New()
	tenantID := uuid.New()
	policy := &domain.Policy{
		ID:       policyID,
		TenantID: tenantID,
		Name:     "TestPolicy",
		Statements: []domain.Statement{
			{Effect: domain.EffectAllow, Action: []string{"*"}},
		},
	}

	t.Run("returns specific version", func(t *testing.T) {
		statementsJSON, _ := json.Marshal(policy.Statements)
		rows := pgxmock.NewRows([]string{"id", "policy_id", "version_number", "name", "description", "statements", "created_at", "created_by"}).
			AddRow(uuid.New(), policyID, 1, policy.Name, policy.Description, statementsJSON, nil, nil)

		mock.ExpectQuery("SELECT pv.id, pv.policy_id, pv.version_number, pv.name, pv.description, pv.statements, pv.created_at, pv.created_by").
			WithArgs(policyID, tenantID, 1).
			WillReturnRows(rows)

		version, err := repo.GetPolicyVersion(ctx, tenantID, policyID, 1)
		require.NoError(t, err)
		assert.Equal(t, 1, version.VersionNumber)
		assert.Equal(t, policyID, version.PolicyID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns not found for nonexistent version", func(t *testing.T) {
		mock.ExpectQuery("SELECT pv.id, pv.policy_id, pv.version_number, pv.name, pv.description, pv.statements, pv.created_at, pv.created_by").
			WithArgs(policyID, tenantID, 99).
			WillReturnError(pgx.ErrNoRows)

		_, err := repo.GetPolicyVersion(ctx, tenantID, policyID, 99)
		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestIAMRepository_InsertPolicyVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIAMRepository(mock)
	ctx := context.Background()

	policyID := uuid.New()
	tenantID := uuid.New()
	policy := &domain.Policy{
		ID:       policyID,
		TenantID: tenantID,
		Name:     "TestPolicy",
		Statements: []domain.Statement{
			{Effect: domain.EffectAllow, Action: []string{"*"}},
		},
	}

	t.Run("inserts new version", func(t *testing.T) {
		pv := &domain.PolicyVersion{
			ID:            uuid.New(),
			PolicyID:      policyID,
			VersionNumber: 3,
			Name:          policy.Name,
			Description:   policy.Description,
			Statements:    policy.Statements,
		}
		statementsJSON, _ := json.Marshal(pv.Statements)

		mock.ExpectExec("INSERT INTO policy_versions").
			WithArgs(pv.ID, pv.PolicyID, pv.VersionNumber, pv.Name, pv.Description, statementsJSON, pv.CreatedBy).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := repo.InsertPolicyVersion(ctx, tenantID, pv)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}