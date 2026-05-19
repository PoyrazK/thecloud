//go:build integration
// +build integration

package services_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/repositories/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIAMServiceTest(t *testing.T) (ports.IAMService, *postgres.IAMRepository, context.Context) {
	t.Helper()
	db := setupDB(t)
	cleanDB(t, db)
	ctx := setupTestUser(t, db)

	repo := postgres.NewIAMRepository(db)
	auditSvc := new(MockAuditService)
	eventSvc := new(MockEventService)
	logger := slog.Default()

	svc := services.NewIAMService(repo, auditSvc, eventSvc, logger)

	return svc, repo, ctx
}

func TestIAMService_Integration_SimulatePolicy(t *testing.T) {
	svc, repo, ctx := setupIAMServiceTest(t)
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	t.Run("SimulatePolicy_AllowPolicyMatches", func(t *testing.T) {
		policy := &domain.Policy{
			ID:          uuid.New(),
			TenantID:    tenantID,
			Name:        "AllowInstanceLaunch",
			Description: "Allow launching instances",
			Statements: []domain.Statement{
				{
					Effect:   domain.EffectAllow,
					Action:   []string{"compute:instance:launch"},
					Resource: []string{"arn:thecloud:compute:us-east-1:*:instance/*"},
				},
			},
		}
		err := repo.CreatePolicy(ctx, tenantID, policy)
		require.NoError(t, err)

		err = repo.AttachPolicyToUser(ctx, tenantID, userID, policy.ID)
		require.NoError(t, err)

		result, err := svc.SimulatePolicy(ctx, ports.Principal{UserID: &userID}, []string{"compute:instance:launch"}, []string{"arn:thecloud:compute:us-east-1:*:instance/123"}, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, domain.EffectAllow, result.Decision)
		assert.NotNil(t, result.Matched)
		assert.Equal(t, "AllowInstanceLaunch", result.Matched.PolicyName)
		assert.Equal(t, 1, result.Evaluated)
	})

	t.Run("SimulatePolicy_DenyWinsOverAllow", func(t *testing.T) {
		// Create both an allow and a deny policy; deny should win
		allowPolicy := &domain.Policy{
			ID:       uuid.New(),
			TenantID: tenantID,
			Name:     "AllowInstanceLaunch",
			Statements: []domain.Statement{
				{
					Effect:   domain.EffectAllow,
					Action:   []string{"compute:instance:*"},
					Resource: []string{"*"},
				},
			},
		}
		denyPolicy := &domain.Policy{
			ID:       uuid.New(),
			TenantID: tenantID,
			Name:     "DenyInstanceDelete",
			Statements: []domain.Statement{
				{
					Effect:   domain.EffectDeny,
					Action:   []string{"compute:instance:delete"},
					Resource: []string{"*"},
				},
			},
		}
		err := repo.CreatePolicy(ctx, tenantID, allowPolicy)
		require.NoError(t, err)
		err = repo.CreatePolicy(ctx, tenantID, denyPolicy)
		require.NoError(t, err)

		err = repo.AttachPolicyToUser(ctx, tenantID, userID, allowPolicy.ID)
		require.NoError(t, err)
		err = repo.AttachPolicyToUser(ctx, tenantID, userID, denyPolicy.ID)
		require.NoError(t, err)

		result, err := svc.SimulatePolicy(ctx, ports.Principal{UserID: &userID}, []string{"compute:instance:delete"}, []string{"arn:thecloud:compute:us-east-1:*:instance/123"}, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, domain.EffectDeny, result.Decision)
		assert.NotNil(t, result.Matched)
		assert.Equal(t, "DenyInstanceDelete", result.Matched.PolicyName)
		assert.Equal(t, 1, result.Evaluated) // Deny short-circuits
	})

	t.Run("SimulatePolicy_PairCapEnforced", func(t *testing.T) {
		policy := &domain.Policy{
			ID:       uuid.New(),
			TenantID: tenantID,
			Name:     "AllowAll",
			Statements: []domain.Statement{
				{Effect: domain.EffectAllow, Action: []string{"*"}, Resource: []string{"*"}},
			},
		}
		err := repo.CreatePolicy(ctx, tenantID, policy)
		require.NoError(t, err)
		err = repo.AttachPolicyToUser(ctx, tenantID, userID, policy.ID)
		require.NoError(t, err)

		// 11 actions × 10 resources = 110 pairs > 100 cap
		actions := make([]string, 11)
		for i := range actions {
			actions[i] = "compute:instance:launch"
		}
		resources := make([]string, 10)
		for i := range resources {
			resources[i] = "arn:thecloud:compute:us-east-1:*:instance/*"
		}

		result, err := svc.SimulatePolicy(ctx, ports.Principal{UserID: &userID}, actions, resources, nil)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "too many action-resource pairs")
	})

	t.Run("SimulatePolicy_ContextOverridesApplied", func(t *testing.T) {
		// Create a policy that only allows during a specific time window
		// Using a past time so the condition is NOT met if evaluated at current time
		pastTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
		policy := &domain.Policy{
			ID:       uuid.New(),
			TenantID: tenantID,
			Name:     "TimeGatedPolicy",
			Statements: []domain.Statement{
				{
					Effect:   domain.EffectAllow,
					Action:   []string{"compute:instance:*"},
					Resource: []string{"*"},
					Condition: domain.Condition{
						"DateGreaterThan": {
							"thecloud:CurrentTime": pastTime,
						},
					},
				},
			},
		}
		err := repo.CreatePolicy(ctx, tenantID, policy)
		require.NoError(t, err)
		err = repo.AttachPolicyToUser(ctx, tenantID, userID, policy.ID)
		require.NoError(t, err)

		// When no context override, current time is used → condition likely met → allow
		result, err := svc.SimulatePolicy(ctx, ports.Principal{UserID: &userID}, []string{"compute:instance:launch"}, []string{"instance:123"}, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, domain.EffectAllow, result.Decision)

		// Override thecloud:CurrentTime to a time far in the future → condition NOT met
		futureTime := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
		evalCtx := map[string]interface{}{
			"thecloud:CurrentTime": futureTime,
		}
		result, err = svc.SimulatePolicy(ctx, ports.Principal{UserID: &userID}, []string{"compute:instance:launch"}, []string{"instance:123"}, evalCtx)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, domain.PolicyEffect(""), result.Decision) // No match since condition not met
	})
}
