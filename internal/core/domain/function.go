// Package domain defines core business entities.
package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/errors"
)

// EnvVar represents a key-value environment variable for a function.
// Value and SecretRef are mutually exclusive — at least one must be set when an EnvVar is provided.
type EnvVar struct {
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	SecretRef string `json:"secret_ref,omitempty"` // reference to a Secret, e.g. "@my-api-key"
}

// FunctionUpdate describes fields that can be updated on a function.
// All fields are pointers so nil means "do not update".
// Note: Status is a string (not pointer) because empty string is used as sentinel
// to distinguish "not provided" from "set to empty string" — this is intentional
// and differs from the pointer pattern used by other fields.
type FunctionUpdate struct {
	Handler  *string   `json:"handler,omitempty"`
	Timeout  *int      `json:"timeout,omitempty"`
	MemoryMB *int      `json:"memory_mb,omitempty"`
	CPUs     *float64  `json:"cpus,omitempty"`
	Status   string    `json:"status,omitempty"`
	EnvVars  []*EnvVar `json:"env_vars,omitempty"`
}

// Validate checks that timeout, memory, and CPU values are within acceptable bounds.
func (u *FunctionUpdate) Validate() error {
	if u.Timeout != nil && (*u.Timeout < 1 || *u.Timeout > 900) {
		return errors.New(errors.InvalidInput, "timeout must be between 1 and 900 seconds")
	}
	if u.MemoryMB != nil && (*u.MemoryMB < 64 || *u.MemoryMB > 10240) {
		return errors.New(errors.InvalidInput, "memory must be between 64 and 10240 MB")
	}
	if u.CPUs != nil && (*u.CPUs < 0.1 || *u.CPUs > 8.0) {
		return errors.New(errors.InvalidInput, "cpu must be between 0.1 and 8.0 cores")
	}
	for _, e := range u.EnvVars {
		if e.Value != "" && e.SecretRef != "" {
			return errors.New(errors.InvalidInput, "env var cannot have both value and secret_ref")
		}
		if e.Value == "" && e.SecretRef == "" {
			return errors.New(errors.InvalidInput, "env var must have either value or secret_ref")
		}
	}
	return nil
}

// SetColumns returns the names of non-zero/nil fields for dynamic SQL UPDATE.
func (u *FunctionUpdate) SetColumns() []string {
	var cols []string
	if u.Handler != nil {
		cols = append(cols, "handler")
	}
	if u.Timeout != nil {
		cols = append(cols, "timeout_seconds")
	}
	if u.MemoryMB != nil {
		cols = append(cols, "memory_mb")
	}
	if u.CPUs != nil {
		cols = append(cols, "cpus")
	}
	if u.Status != "" {
		cols = append(cols, "status")
	}
	if u.EnvVars != nil {
		cols = append(cols, "env_vars")
	}
	return cols
}

// Function represents a serverless function.
// Functions are event-driven units of execution (FaaS).
type Function struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	Runtime   string    `json:"runtime"`   // e.g. "python3.9", "go1.21"
	Handler   string    `json:"handler"`   // Entry point (e.g. "main.Handle")
	CodePath  string    `json:"code_path"` // Path to code artifact
	Timeout   int       `json:"timeout"`   // Execution timeout in seconds
	MemoryMB  int       `json:"memory_mb"` // Memory allocation in MB
	CPUs      float64   `json:"cpus"`      // CPU cores (e.g., 0.5, 1.0, 2.0)
	Status    string    `json:"status"`    // e.g. "DEPLOYING", "READY"
	EnvVars   []*EnvVar `json:"env_vars,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Invocation represents a single execution of a Function.
type Invocation struct {
	ID         uuid.UUID  `json:"id"`
	FunctionID uuid.UUID  `json:"function_id"`
	Status     string     `json:"status"` // "PENDING", "RUNNING", "SUCCESS", "FAILED"
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	DurationMs int        `json:"duration_ms"` // Execution time in milliseconds
	StatusCode int        `json:"status_code"` // Exit code or HTTP status
	Logs       string     `json:"logs"`        // Captured stdout/stderr
}
