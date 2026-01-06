package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrStackNameAlreadyExists = errors.New("stack with this name already exists")
)

type StackStatus string

const (
	StackStatusCreateInProgress   StackStatus = "CREATE_IN_PROGRESS"
	StackStatusCreateComplete     StackStatus = "CREATE_COMPLETE"
	StackStatusCreateFailed       StackStatus = "CREATE_FAILED"
	StackStatusDeleteInProgress   StackStatus = "DELETE_IN_PROGRESS"
	StackStatusDeleteComplete     StackStatus = "DELETE_COMPLETE"
	StackStatusDeleteFailed       StackStatus = "DELETE_FAILED"
	StackStatusRollbackInProgress StackStatus = "ROLLBACK_IN_PROGRESS"
	StackStatusRollbackComplete   StackStatus = "ROLLBACK_COMPLETE"
	StackStatusRollbackFailed     StackStatus = "ROLLBACK_FAILED"
)

type Stack struct {
	ID           uuid.UUID       `json:"id"`
	UserID       uuid.UUID       `json:"user_id"`
	Name         string          `json:"name"`
	Template     string          `json:"template"` // Raw YAML or JSON
	Parameters   json.RawMessage `json:"parameters" swaggertype:"string"`
	Status       StackStatus     `json:"status"`
	StatusReason string          `json:"status_reason,omitempty"`
	Resources    []StackResource `json:"resources,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type StackResource struct {
	ID           uuid.UUID `json:"id"`
	StackID      uuid.UUID `json:"stack_id"`
	LogicalID    string    `json:"logical_id"`    // ID in template
	PhysicalID   string    `json:"physical_id"`   // ID in The Cloud (UUID)
	ResourceType string    `json:"resource_type"` // e.g. "Instance", "VPC"
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type TemplateValidateResponse struct {
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors,omitempty"`
	Parameters []string `json:"parameters,omitempty"`
}
