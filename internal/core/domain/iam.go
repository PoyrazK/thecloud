package domain

import (
	"github.com/google/uuid"
)

// PolicyEffect defines whether a statement allows or denies access.
type PolicyEffect string

const (
	EffectAllow PolicyEffect = "Allow"
	EffectDeny  PolicyEffect = "Deny"
)

// Condition represents a set of dynamic rules for policy evaluation.
// Example: {"IpAddress": {"aws:SourceIp": "192.168.1.0/24"}}
type Condition map[string]map[string]interface{}

// Statement is a single rule within a policy.
type Statement struct {
	Sid       string       `json:"sid,omitempty"`
	Effect    PolicyEffect `json:"effect"`
	Action    []string     `json:"action"`
	Resource  []string     `json:"resource"`
	Condition Condition    `json:"condition,omitempty"`
}

// Policy represents a JSON-based identity policy.
type Policy struct {
	ID          uuid.UUID   `json:"id"`
	TenantID    uuid.UUID   `json:"tenant_id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Statements  []Statement `json:"statements"`
}

// UserPolicy maps a policy to a user.
type UserPolicy struct {
	UserID   uuid.UUID `json:"user_id"`
	PolicyID uuid.UUID `json:"policy_id"`
}

// RolePolicy maps a policy to an RBAC role.
type RolePolicy struct {
	RoleName string    `json:"role_name"`
	PolicyID uuid.UUID `json:"policy_id"`
}
