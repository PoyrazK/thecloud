package domain

import (
	"time"

	"github.com/google/uuid"
)

// PolicyEffect defines whether a statement allows or denies access.
type PolicyEffect string

const (
	EffectAllow PolicyEffect = "Allow"
	EffectDeny  PolicyEffect = "Deny"
)

// Condition represents a set of dynamic rules for policy evaluation.
// Example: {"IpAddress": {"thecloud:SourceIp": "192.168.1.0/24"}}
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

// PolicyVersion is a historical snapshot of a policy at a specific version.
type PolicyVersion struct {
	ID            uuid.UUID   `json:"id"`
	PolicyID      uuid.UUID   `json:"policy_id"`
	VersionNumber int         `json:"version_number"`
	Name         string      `json:"name"`
	Description  string      `json:"description,omitempty"`
	Statements   []Statement `json:"statements"`
	CreatedAt    time.Time   `json:"created_at"`
	CreatedBy   *uuid.UUID  `json:"created_by,omitempty"`
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

// EvalResult is the outcome of a policy evaluation with match metadata.
type EvalResult struct {
	Effect       PolicyEffect
	PolicyID     uuid.UUID
	PolicyName   string
	StatementSid string
	Reason       string
}
