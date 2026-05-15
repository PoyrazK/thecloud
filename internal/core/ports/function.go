// Package ports defines service and repository interfaces.
package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// FunctionRepository manages the persistence of function metadata and invocation records.
type FunctionRepository interface {
	// Create saves a new function definition.
	Create(ctx context.Context, f *domain.Function) error
	// GetByID retrieves a function by its unique UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Function, error)
	// GetByName retrieves a function by name for a specific user.
	GetByName(ctx context.Context, userID uuid.UUID, name string) (*domain.Function, error)
	// List returns all functions owned by a user.
	List(ctx context.Context, userID uuid.UUID) ([]*domain.Function, error)
	// Update applies partial updates to a function via FunctionUpdate.
	Update(ctx context.Context, id uuid.UUID, u *domain.FunctionUpdate) error
	// Delete removes a function definition and its associated code metadata.
	Delete(ctx context.Context, id uuid.UUID) error
	// CreateInvocation records the start or completion of a function execution.
	CreateInvocation(ctx context.Context, i *domain.Invocation) error
	// GetInvocations retrieves a history of executions for a specific function.
	GetInvocations(ctx context.Context, functionID uuid.UUID, limit int) ([]*domain.Invocation, error)
	// GetDLQInvocations retrieves all DLQ (Dead Letter Queue) invocations for a specific function.
	GetDLQInvocations(ctx context.Context, functionID uuid.UUID) ([]*domain.Invocation, error)
	// UpdateInvocation updates an existing invocation record.
	UpdateInvocation(ctx context.Context, i *domain.Invocation) error
}

// FunctionService provides business logic for FaaS (Function-as-a-Service) management and execution.
type FunctionService interface {
	// CreateFunction registers a new serverless function and prepares its execution environment.
	CreateFunction(ctx context.Context, name, runtime, handler string, code []byte) (*domain.Function, error)
	// GetFunction retrieves details for a specific function by its UUID.
	GetFunction(ctx context.Context, id uuid.UUID) (*domain.Function, error)
	// ListFunctions returns all functions belonging to the current authorized context.
	ListFunctions(ctx context.Context) ([]*domain.Function, error)
	// UpdateFunction applies partial updates to a function.
	UpdateFunction(ctx context.Context, id uuid.UUID, req *domain.FunctionUpdate) (*domain.Function, error)
	// DeleteFunction decommission a serverless function.
	DeleteFunction(ctx context.Context, id uuid.UUID) error
	// InvokeFunction executes a function with the provided payload.
	InvokeFunction(ctx context.Context, id uuid.UUID, payload []byte, async bool) (*domain.Invocation, error)
	// GetFunctionLogs retrieves execution history and results for a function.
	GetFunctionLogs(ctx context.Context, id uuid.UUID, limit int) ([]*domain.Invocation, error)
	// GetDLQInvocations retrieves all DLQ (Dead Letter Queue) invocations for a specific function.
	GetDLQInvocations(ctx context.Context, id uuid.UUID) ([]*domain.Invocation, error)
	// RetryDLQInvocation retries a DLQ invocation by resetting its status to PENDING.
	RetryDLQInvocation(ctx context.Context, invocationID uuid.UUID) (*domain.Invocation, error)
}
