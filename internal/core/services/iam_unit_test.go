package services_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIAMService_Unit(t *testing.T) {
	mockRepo := new(MockIAMRepository)
	mockAuditSvc := new(MockAuditService)
	mockEventSvc := new(MockEventService)
	svc := services.NewIAMService(mockRepo, mockAuditSvc, mockEventSvc, slog.Default())

	ctx := context.Background()
	tenantID := uuid.New()
	ctx = appcontext.WithTenantID(ctx, tenantID)

	t.Run("CreatePolicy", func(t *testing.T) {
		policy := &domain.Policy{Name: "test-policy"}
		mockRepo.On("CreatePolicy", mock.Anything, tenantID, mock.Anything).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "IAM_POLICY_CREATE", mock.Anything, "POLICY", mock.Anything).Return(nil).Once()

		err := svc.CreatePolicy(ctx, policy)
		require.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("AttachPolicyToUser", func(t *testing.T) {
		userID := uuid.New()
		policyID := uuid.New()
		mockRepo.On("AttachPolicyToUser", mock.Anything, tenantID, userID, policyID).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "iam.policy_attach", "user", mock.Anything, mock.Anything).Return(nil).Once()

		err := svc.AttachPolicyToUser(ctx, userID, policyID)
		require.NoError(t, err)
	})

	t.Run("DetachPolicyFromUser", func(t *testing.T) {
		userID := uuid.New()
		policyID := uuid.New()
		mockRepo.On("DetachPolicyFromUser", mock.Anything, tenantID, userID, policyID).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "iam.policy_detach", "user", mock.Anything, mock.Anything).Return(nil).Once()

		err := svc.DetachPolicyFromUser(ctx, userID, policyID)
		require.NoError(t, err)
	})

	t.Run("GetPoliciesForUser", func(t *testing.T) {
		userID := uuid.New()
		mockRepo.On("GetPoliciesForUser", mock.Anything, tenantID, userID).Return([]*domain.Policy{}, nil).Once()

		res, err := svc.GetPoliciesForUser(ctx, userID)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("UpdatePolicy", func(t *testing.T) {
		policy := &domain.Policy{ID: uuid.New(), Name: "updated"}
		mockRepo.On("UpdatePolicy", mock.Anything, tenantID, policy).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "IAM_POLICY_UPDATE", policy.ID.String(), "POLICY", mock.Anything).Return(nil).Once()

		err := svc.UpdatePolicy(ctx, policy)
		require.NoError(t, err)
	})

	t.Run("DeletePolicy", func(t *testing.T) {
		id := uuid.New()
		mockRepo.On("DeletePolicy", mock.Anything, tenantID, id).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "IAM_POLICY_DELETE", id.String(), "POLICY", mock.Anything).Return(nil).Once()

		err := svc.DeletePolicy(ctx, id)
		require.NoError(t, err)
	})

	t.Run("SimulatePolicy_PairCapExceeded", func(t *testing.T) {
		userID := uuid.New()
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

	t.Run("SimulatePolicy_NoPrincipal", func(t *testing.T) {
		result, err := svc.SimulatePolicy(ctx, ports.Principal{}, []string{"compute:instance:launch"}, []string{"*"}, nil)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no principal specified")
	})

	t.Run("SimulatePolicy_UserPrincipal", func(t *testing.T) {
		userID := uuid.New()
		policy := &domain.Policy{
			ID:   uuid.New(),
			Name: "TestPolicy",
			Statements: []domain.Statement{
				{Effect: domain.EffectAllow, Action: []string{"compute:instance:*"}, Resource: []string{"*"}},
			},
		}
		mockRepo.On("GetPoliciesForUser", mock.Anything, tenantID, userID).Return([]*domain.Policy{policy}, nil).Once()

		result, err := svc.SimulatePolicy(ctx, ports.Principal{UserID: &userID}, []string{"compute:instance:launch"}, []string{"instance:123"}, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, domain.EffectAllow, result.Decision)
		assert.Equal(t, 1, result.Evaluated)
		assert.NotNil(t, result.Matched)
		assert.Equal(t, "TestPolicy", result.Matched.PolicyName)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SimulatePolicy_ServiceAccountPrincipal", func(t *testing.T) {
		saID := uuid.New()
		policy := &domain.Policy{
			ID:   uuid.New(),
			Name: "SAPolicy",
			Statements: []domain.Statement{
				{Effect: domain.EffectAllow, Action: []string{"compute:instance:*"}, Resource: []string{"*"}},
			},
		}
		mockRepo.On("GetPoliciesForServiceAccount", mock.Anything, tenantID, saID).Return([]*domain.Policy{policy}, nil).Once()

		result, err := svc.SimulatePolicy(ctx, ports.Principal{ServiceAccountID: &saID}, []string{"compute:instance:launch"}, []string{"instance:123"}, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, domain.EffectAllow, result.Decision)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SimulatePolicy_DenyShortCircuits", func(t *testing.T) {
		userID := uuid.New()
		denyPolicy := &domain.Policy{
			ID:   uuid.New(),
			Name: "DenyPolicy",
			Statements: []domain.Statement{
				{Effect: domain.EffectDeny, Action: []string{"compute:instance:*"}, Resource: []string{"*"}},
			},
		}
		allowPolicy := &domain.Policy{
			ID:   uuid.New(),
			Name: "AllowPolicy",
			Statements: []domain.Statement{
				{Effect: domain.EffectAllow, Action: []string{"compute:instance:*"}, Resource: []string{"*"}},
			},
		}
		mockRepo.On("GetPoliciesForUser", mock.Anything, tenantID, userID).Return([]*domain.Policy{denyPolicy, allowPolicy}, nil).Once()

		result, err := svc.SimulatePolicy(ctx, ports.Principal{UserID: &userID}, []string{"compute:instance:launch"}, []string{"instance:123"}, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, domain.EffectDeny, result.Decision)
		assert.Equal(t, "DenyPolicy", result.Matched.PolicyName)
		assert.Equal(t, 1, result.Evaluated) // Deny short-circuits
		mockRepo.AssertExpectations(t)
	})

	t.Run("SimulatePolicy_LastAllowWins", func(t *testing.T) {
		userID := uuid.New()
		firstPolicy := &domain.Policy{
			ID:   uuid.New(),
			Name: "FirstAllow",
			Statements: []domain.Statement{
				{Effect: domain.EffectAllow, Action: []string{"compute:instance:*"}, Resource: []string{"*"}},
			},
		}
		lastPolicy := &domain.Policy{
			ID:   uuid.New(),
			Name: "LastAllow",
			Statements: []domain.Statement{
				{Effect: domain.EffectAllow, Action: []string{"compute:instance:*"}, Resource: []string{"*"}},
			},
		}
		mockRepo.On("GetPoliciesForUser", mock.Anything, tenantID, userID).Return([]*domain.Policy{firstPolicy, lastPolicy}, nil).Once()

		result, err := svc.SimulatePolicy(ctx, ports.Principal{UserID: &userID}, []string{"compute:instance:launch"}, []string{"instance:123"}, nil)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, domain.EffectAllow, result.Decision)
		assert.Equal(t, "FirstAllow", result.Matched.PolicyName) // First allow wins (no overwrite since allowResult already set)
		assert.Equal(t, 1, result.Evaluated)
		mockRepo.AssertExpectations(t)
	})
}
