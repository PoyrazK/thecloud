package services_test

import (
	"context"
	"testing"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIAMEvaluator_Unit(t *testing.T) {
	t.Run("Evaluate_Allow", testIAMEvaluatorEvaluateAllow)
	t.Run("Evaluate_Deny", testIAMEvaluatorEvaluateDeny)
	t.Run("Evaluate_NoMatch", testIAMEvaluatorEvaluateNoMatch)
	t.Run("Evaluate_ExplicitDenyWins", testIAMEvaluatorEvaluateExplicitDenyWins)
	t.Run("Evaluate_WildcardAction", testIAMEvaluatorEvaluateWildcardAction)
	t.Run("Evaluate_WildcardResource", testIAMEvaluatorEvaluateWildcardResource)
}

func testIAMEvaluatorEvaluateAllow(t *testing.T) {
	evaluator := services.NewIAMEvaluator()
	ctx := context.Background()

	policies := []*domain.Policy{
		{
			Statements: []domain.Statement{
				{
					Effect:   domain.EffectAllow,
					Action:   []string{"instance:launch", "instance:stop"},
					Resource: []string{"*"},
				},
			},
		},
	}

	result, err := evaluator.Evaluate(ctx, policies, "instance:launch", "resource-1", nil)
	require.NoError(t, err)
	assert.Equal(t, domain.EffectAllow, result.Effect)
}

func testIAMEvaluatorEvaluateDeny(t *testing.T) {
	evaluator := services.NewIAMEvaluator()
	ctx := context.Background()

	policies := []*domain.Policy{
		{
			Statements: []domain.Statement{
				{
					Effect:   domain.EffectDeny,
					Action:   []string{"instance:launch"},
					Resource: []string{"*"},
				},
			},
		},
	}

	result, err := evaluator.Evaluate(ctx, policies, "instance:launch", "resource-1", nil)
	require.NoError(t, err)
	assert.Equal(t, domain.EffectDeny, result.Effect)
}

func testIAMEvaluatorEvaluateNoMatch(t *testing.T) {
	evaluator := services.NewIAMEvaluator()
	ctx := context.Background()

	policies := []*domain.Policy{
		{
			Statements: []domain.Statement{
				{
					Effect:   domain.EffectAllow,
					Action:   []string{"volume:create"},
					Resource: []string{"*"},
				},
			},
		},
	}

	result, err := evaluator.Evaluate(ctx, policies, "instance:launch", "resource-1", nil)
	require.NoError(t, err)
	assert.Equal(t, domain.PolicyEffect(""), result.Effect)
}

func testIAMEvaluatorEvaluateExplicitDenyWins(t *testing.T) {
	evaluator := services.NewIAMEvaluator()
	ctx := context.Background()

	policies := []*domain.Policy{
		{
			Statements: []domain.Statement{
				{
					Effect:   domain.EffectAllow,
					Action:   []string{"instance:launch"},
					Resource: []string{"*"},
				},
				{
					Effect:   domain.EffectDeny,
					Action:   []string{"instance:launch"},
					Resource: []string{"*"},
				},
			},
		},
	}

	result, err := evaluator.Evaluate(ctx, policies, "instance:launch", "resource-1", nil)
	require.NoError(t, err)
	assert.Equal(t, domain.EffectDeny, result.Effect)
}

func testIAMEvaluatorEvaluateWildcardAction(t *testing.T) {
	evaluator := services.NewIAMEvaluator()
	ctx := context.Background()

	policies := []*domain.Policy{
		{
			Statements: []domain.Statement{
				{
					Effect:   domain.EffectAllow,
					Action:   []string{"instance:*"},
					Resource: []string{"*"},
				},
			},
		},
	}

	tests := []struct {
		action   string
		expected domain.PolicyEffect
	}{
		{"instance:launch", domain.EffectAllow},
		{"instance:stop", domain.EffectAllow},
		{"instance:terminate", domain.EffectAllow},
	}

	for _, tt := range tests {
		result, err := evaluator.Evaluate(ctx, policies, tt.action, "resource-1", nil)
		require.NoError(t, err)
		assert.Equal(t, tt.expected, result.Effect, "action %s should match", tt.action)
	}
}

func testIAMEvaluatorEvaluateWildcardResource(t *testing.T) {
	evaluator := services.NewIAMEvaluator()
	ctx := context.Background()

	policies := []*domain.Policy{
		{
			Statements: []domain.Statement{
				{
					Effect:   domain.EffectAllow,
					Action:   []string{"instance:launch"},
					Resource: []string{"*"},
				},
			},
		},
	}

	result, err := evaluator.Evaluate(ctx, policies, "instance:launch", "any-resource", nil)
	require.NoError(t, err)
	assert.Equal(t, domain.EffectAllow, result.Effect)
}