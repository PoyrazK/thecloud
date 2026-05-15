package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// IAMRepository manages the persistence of IAM policies and their assignments.
type IAMRepository interface {
	// Policy Management
	CreatePolicy(ctx context.Context, tenantID uuid.UUID, policy *domain.Policy) error
	GetPolicyByID(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*domain.Policy, error)
	ListPolicies(ctx context.Context, tenantID uuid.UUID) ([]*domain.Policy, error)
	UpdatePolicy(ctx context.Context, tenantID uuid.UUID, policy *domain.Policy) error
	DeletePolicy(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) error

	// User Policy Assignment
	AttachPolicyToUser(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, policyID uuid.UUID) error
	DetachPolicyFromUser(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, policyID uuid.UUID) error
	GetPoliciesForUser(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID) ([]*domain.Policy, error)

	// Role Policy Assignment
	AttachPolicyToRole(ctx context.Context, tenantID uuid.UUID, roleName string, policyID uuid.UUID) error
	DetachPolicyFromRole(ctx context.Context, tenantID uuid.UUID, roleName string, policyID uuid.UUID) error
	GetPoliciesForRole(ctx context.Context, tenantID uuid.UUID, roleName string) ([]*domain.Policy, error)

	// Service Account Policy Assignment
	AttachPolicyToServiceAccount(ctx context.Context, tenantID uuid.UUID, saID uuid.UUID, policyID uuid.UUID) error
	DetachPolicyFromServiceAccount(ctx context.Context, tenantID uuid.UUID, saID uuid.UUID, policyID uuid.UUID) error
	GetPoliciesForServiceAccount(ctx context.Context, tenantID uuid.UUID, saID uuid.UUID) ([]*domain.Policy, error)
}

// IAMService defines the business logic for IAM management.
type IAMService interface {
	CreatePolicy(ctx context.Context, policy *domain.Policy) error
	GetPolicyByID(ctx context.Context, id uuid.UUID) (*domain.Policy, error)
	ListPolicies(ctx context.Context) ([]*domain.Policy, error)
	UpdatePolicy(ctx context.Context, policy *domain.Policy) error
	DeletePolicy(ctx context.Context, id uuid.UUID) error

	AttachPolicyToUser(ctx context.Context, userID uuid.UUID, policyID uuid.UUID) error
	DetachPolicyFromUser(ctx context.Context, userID uuid.UUID, policyID uuid.UUID) error
	GetPoliciesForUser(ctx context.Context, userID uuid.UUID) ([]*domain.Policy, error)

	AttachPolicyToRole(ctx context.Context, roleName string, policyID uuid.UUID) error
	DetachPolicyFromRole(ctx context.Context, roleName string, policyID uuid.UUID) error
	GetPoliciesForRole(ctx context.Context, roleName string) ([]*domain.Policy, error)

	AttachPolicyToServiceAccount(ctx context.Context, saID uuid.UUID, policyID uuid.UUID) error
	DetachPolicyFromServiceAccount(ctx context.Context, saID uuid.UUID, policyID uuid.UUID) error
	GetPoliciesForServiceAccount(ctx context.Context, saID uuid.UUID) ([]*domain.Policy, error)

	// SimulatePolicy evaluates what-if actions/resources against the given principal's policies.
	// Returns the decision and which statement matched, for debugging.
	SimulatePolicy(ctx context.Context, principal Principal, actions []string, resources []string, evalCtx map[string]interface{}) (*SimulateResult, error)
}

// Principal identifies the actor whose policies will be evaluated in a simulation.
type Principal struct {
	UserID           *uuid.UUID
	ServiceAccountID *uuid.UUID
}

// SimulateResult is the outcome of a policy simulation.
type SimulateResult struct {
	Decision  domain.PolicyEffect
	Matched   *StatementMatch
	Evaluated int
}

// StatementMatch describes which statement allowed or denied the request.
type StatementMatch struct {
	Action      string
	Resource    string
	PolicyID    uuid.UUID
	PolicyName  string
	StatementSid string
	Effect      domain.PolicyEffect
	Reason      string
}

// PolicyEvaluator defines the logic for evaluating access based on a set of policies.
type PolicyEvaluator interface {
	// Evaluate checks if the given action on a resource is allowed by the provided policies.
	Evaluate(ctx context.Context, policies []*domain.Policy, action string, resource string, evalCtx map[string]interface{}) (*domain.EvalResult, error)
}
