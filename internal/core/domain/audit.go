// Package domain contains the core domain models for the cloud platform.
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AuditAction defines the type of action being logged
type AuditAction string

const (
	AuditLoginSuccess       AuditAction = "LOGIN_SUCCESS"
	AuditLoginFailed        AuditAction = "LOGIN_FAILED"
	AuditRegister           AuditAction = "REGISTER_USER"
	AuditResourceCreate     AuditAction = "RESOURCE_CREATE"
	AuditResourceUpdate     AuditAction = "RESOURCE_UPDATE"
	AuditResourceDelete     AuditAction = "RESOURCE_DELETE"
	AuditUnauthorizedAccess AuditAction = "UNAUTHORIZED_ACCESS"
)

// AuditLog represents a security audit record
type AuditLog struct {
	ID        uuid.UUID       `json:"id"`
	UserID    *uuid.UUID      `json:"user_id,omitempty"` // Nullable if unauthenticated
	Action    AuditAction     `json:"action"`
	Resource  string          `json:"resource"` // Resource type or path
	IPAddress string          `json:"ip_address"`
	UserAgent string          `json:"user_agent"`
	Details   json.RawMessage `json:"details,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}
