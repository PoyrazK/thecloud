package domain

import (
	"time"

	"github.com/google/uuid"
)

// Group represents a collection of users for IAM policy assignment.
type Group struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UserGroup represents membership of a user in a group.
type UserGroup struct {
	UserID   uuid.UUID `json:"user_id"`
	GroupID  uuid.UUID `json:"group_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	AddedAt  time.Time `json:"added_at"`
}

// GroupPolicy maps a policy to a group (mirrors UserPolicy, RolePolicy pattern).
type GroupPolicy struct {
	GroupID  uuid.UUID `json:"group_id"`
	PolicyID uuid.UUID `json:"policy_id"`
	TenantID uuid.UUID `json:"tenant_id"`
}