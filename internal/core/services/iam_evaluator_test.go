package services

import (
	"context"
	"testing"
	"time"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIAMEvaluator_Evaluate_Conditions(t *testing.T) {
	evaluator := NewIAMEvaluator()
	ctx := context.Background()

	tests := []struct {
		name     string
		policies []*domain.Policy
		action   string
		resource string
		evalCtx  map[string]interface{}
		want     domain.PolicyEffect
	}{
		{
			name: "IpAddress condition - matching IP",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"IpAddress": {
									"aws:SourceIp": []interface{}{"192.168.1.0/24"},
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:SourceIp": "192.168.1.50",
			},
			want: domain.EffectAllow,
		},
		{
			name: "IpAddress condition - non-matching IP",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"IpAddress": {
									"aws:SourceIp": []interface{}{"192.168.1.0/24"},
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:SourceIp": "10.0.0.1",
			},
			want: "",
		},
		{
			name: "NotIpAddress condition - IP not in CIDR",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"NotIpAddress": {
									"aws:SourceIp": []interface{}{"192.168.1.0/24"},
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:SourceIp": "10.0.0.1",
			},
			want: domain.EffectAllow,
		},
		{
			name: "StringEquals condition - matching tenant",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringEquals": {
									"thecloud:TenantId": "tenant-123",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"thecloud:TenantId": "tenant-123",
			},
			want: domain.EffectAllow,
		},
		{
			name: "StringEquals condition - non-matching tenant",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringEquals": {
									"thecloud:TenantId": "tenant-123",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"thecloud:TenantId": "tenant-456",
			},
			want: "",
		},
		{
			name: "StringLike condition - wildcard matching",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringLike": {
									"aws:UserId": "user-*",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:UserId": "user-abc-123",
			},
			want: domain.EffectAllow,
		},
		{
			name: "StringNotEquals condition - values do not match",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringNotEquals": {
									"thecloud:TenantId": "tenant-123",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"thecloud:TenantId": "tenant-456",
			},
			want: domain.EffectAllow,
		},
		{
			name: "StringNotEquals condition - values match returns no effect",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringNotEquals": {
									"thecloud:TenantId": "tenant-123",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"thecloud:TenantId": "tenant-123",
			},
			want: "",
		},
		{
			name: "StringNotLike condition - pattern does not match",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringNotLike": {
									"aws:UserId": "admin-*",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:UserId": "user-abc-123",
			},
			want: domain.EffectAllow,
		},
		{
			name: "StringNotLike condition - pattern matches returns no effect",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringNotLike": {
									"aws:UserId": "admin-*",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:UserId": "admin-123",
			},
			want: "",
		},
		{
			name: "DateEquals condition - times are equal",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"DateEquals": {
									"aws:CurrentTime": "2025-06-15T10:00:00Z",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:CurrentTime": "2025-06-15T10:00:00Z",
			},
			want: domain.EffectAllow,
		},
		{
			name: "DateGreaterThan condition - current time after threshold",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"DateGreaterThan": {
									"aws:CurrentTime": "2020-01-01T00:00:00Z",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:CurrentTime": "2030-06-15T10:00:00Z",
			},
			want: domain.EffectAllow,
		},
		{
			name: "DateLessThan condition - current time before threshold",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"DateLessThan": {
									"aws:CurrentTime": "2099-01-01T00:00:00Z",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:CurrentTime": "2025-01-01T00:00:00Z",
			},
			want: domain.EffectAllow,
		},
		{
			name: "Bool condition - matching true",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"Bool": {
									"thecloud:IsAdmin": true,
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"thecloud:IsAdmin": true,
			},
			want: domain.EffectAllow,
		},
		{
			name: "Null condition - key does not exist",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"Null": {
									"thecloud:SomeKey": "true",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"otherKey": "value",
			},
			want: domain.EffectAllow,
		},
		{
			name: "Deny with condition - condition met",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectDeny,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"IpAddress": {
									"aws:SourceIp": []interface{}{"192.168.1.0/24"},
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:SourceIp": "192.168.1.50",
			},
			want: domain.EffectDeny,
		},
		{
			name: "Multiple conditions - all must be met",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"IpAddress": {
									"aws:SourceIp": []interface{}{"192.168.1.0/24"},
								},
								"StringEquals": {
									"thecloud:TenantId": "tenant-123",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:SourceIp":      "192.168.1.50",
				"thecloud:TenantId": "tenant-123",
			},
			want: domain.EffectAllow,
		},
		{
			name: "Multiple conditions - one not met",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"IpAddress": {
									"aws:SourceIp": []interface{}{"192.168.1.0/24"},
								},
								"StringEquals": {
									"thecloud:TenantId": "tenant-123",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:SourceIp":      "192.168.1.50",
				"thecloud:TenantId": "tenant-456",
			},
			want: "",
		},
		{
			name: "No condition - works without evalCtx",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx:  nil,
			want:     domain.EffectAllow,
		},
		{
			name: "Condition with nil evalCtx - fails",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"IpAddress": {
									"aws:SourceIp": []interface{}{"192.168.1.0/24"},
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx:  nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluator.Evaluate(ctx, tt.policies, tt.action, tt.resource, tt.evalCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIAMEvaluator_MatchPattern(t *testing.T) {
	evaluator := NewIAMEvaluator()

	tests := []struct {
		pattern string
		target  string
		want    bool
	}{
		{"*", "anything", true},
		{"instance:*", "instance:launch", true},
		{"instance:*", "instance:stop", true},
		{"instance:*", "other:launch", false},
		{"instance:launch", "instance:launch", true},
		{"instance:launch", "instance:stop", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.target, func(t *testing.T) {
			got := evaluator.matchPattern(tt.pattern, tt.target)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIAMEvaluator_EdgeCases(t *testing.T) {
	evaluator := NewIAMEvaluator()
	ctx := context.Background()

	tests := []struct {
		name     string
		policies []*domain.Policy
		action   string
		resource string
		evalCtx  map[string]interface{}
		want     domain.PolicyEffect
	}{
		// Null condition - expected "false" cases
		{
			name: "Null condition - expected false, key exists returns true",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"Null": {
									"thecloud:SomeKey": "false",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"thecloud:SomeKey": "value",
			},
			want: domain.EffectAllow,
		},
		{
			name: "Null condition - expected false, key does not exist returns false",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"Null": {
									"thecloud:SomeKey": "false",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"otherKey": "value",
			},
			want: "",
		},
		// StringEquals type mismatch cases
		{
			name: "StringEquals condition - expected is bool, actual is string",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringEquals": {
									"thecloud:TenantId": true, // bool expected
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"thecloud:TenantId": "tenant-123", // string actual
			},
			want: "",
		},
		// StringLike type mismatch cases
		{
			name: "StringLike condition - actual is int not string",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringLike": {
									"aws:UserId": "user-*",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:UserId": 12345, // int instead of string
			},
			want: "",
		},
		// Date condition - invalid format cases
		{
			name: "DateEquals condition - invalid date format string",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"DateEquals": {
									"aws:CurrentTime": "2020-01-01", // missing time component
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:CurrentTime": "not-a-valid-date",
			},
			want: "",
		},
		// IP condition - invalid format cases
		{
			name: "IpAddress condition - invalid CIDR format",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"IpAddress": {
									"aws:SourceIp": []interface{}{"999.999.999.999/24"},
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:SourceIp": "10.0.0.1",
			},
			want: "",
		},
		// Bool condition - type mismatch
		{
			name: "Bool condition - expected is string, actual is bool",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"Bool": {
									"thecloud:IsAdmin": "yes", // string instead of bool
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"thecloud:IsAdmin": true,
			},
			want: "",
		},
		// DateEquals with time.Time actual type
		{
			name: "DateEquals condition - actual is time.Time object",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"DateEquals": {
									"aws:CurrentTime": "2025-06-15T10:00:00Z",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:CurrentTime": time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC),
			},
			want: domain.EffectAllow,
		},
		// DateGreaterThan at exact boundary - should be false
		{
			name: "DateGreaterThan condition - at exact boundary returns false",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"DateGreaterThan": {
									"aws:CurrentTime": "2025-06-15T10:00:00Z",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:CurrentTime": "2025-06-15T10:00:00Z", // exactly equal - not greater
			},
			want: "",
		},
		// DateLessThan at exact boundary - should be false
		{
			name: "DateLessThan condition - at exact boundary returns false",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"DateLessThan": {
									"aws:CurrentTime": "2025-06-15T10:00:00Z",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:CurrentTime": "2025-06-15T10:00:00Z", // exactly equal - not less
			},
			want: "",
		},
		// Null condition - expected not string
		{
			name: "Null condition - expected is int not string",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"Null": {
									"thecloud:SomeKey": 123, // int instead of string
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"thecloud:SomeKey": "value",
			},
			want: "",
		},
		// StringLike with float actual type
		{
			name: "StringLike condition - expected string, actual is float",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringLike": {
									"aws:UserId": "user-*",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:UserId": 123.456, // float instead of string
			},
			want: "",
		},
		// DateEquals with invalid expected format - parse error
		{
			name: "DateEquals condition - expected is invalid RFC3339 format",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"DateEquals": {
									"aws:CurrentTime": "invalid-date-format", // invalid expected
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"aws:CurrentTime": "2025-06-15T10:00:00Z", // valid actual
			},
			want: "", // parse fails → condition not met
		},
		// StringEquals with nil actual value
		{
			name: "StringEquals condition - actual is nil",
			policies: []*domain.Policy{
				{
					Statements: []domain.Statement{
						{
							Effect:   domain.EffectAllow,
							Action:   []string{"instance:*"},
							Resource: []string{"*"},
							Condition: domain.Condition{
								"StringEquals": {
									"thecloud:TenantId": "tenant-123",
								},
							},
						},
					},
				},
			},
			action:   "instance:launch",
			resource: "instance:123",
			evalCtx: map[string]interface{}{
				"thecloud:TenantId": nil, // nil value
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluator.Evaluate(ctx, tt.policies, tt.action, tt.resource, tt.evalCtx)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
