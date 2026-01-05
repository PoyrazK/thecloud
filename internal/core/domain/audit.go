package domain

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID           uuid.UUID              `json:"id"`
	UserID       uuid.UUID              `json:"user_id"`
	Action       string                 `json:"action"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	Details      map[string]interface{} `json:"details"`
	IPAddress    string                 `json:"ip_address"`
	UserAgent    string                 `json:"user_agent"`
	CreatedAt    time.Time              `json:"created_at"`
}
