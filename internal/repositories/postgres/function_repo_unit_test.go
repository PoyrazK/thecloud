package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testFuncName    = "test-func"
	testFuncRuntime = "python3.9"
	testFuncHandler = "main.handler"
	testFuncPath    = "/tmp/code"
)

func TestFunctionRepositoryCreate(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewFunctionRepository(mock)
	tenantID := uuid.New()
	f := &domain.Function{
		ID:                       uuid.New(),
		UserID:                   uuid.New(),
		TenantID:                 tenantID,
		Name:                     testFuncName,
		Runtime:                  testFuncRuntime,
		Handler:                  testFuncHandler,
		CodePath:                 testFuncPath,
		Timeout:                  30,
		MemoryMB:                 128,
		CPUs:                     0.5,
		Status:                   "ready",
		MaxConcurrentInvocations: 0,
		MaxQueueDepth:            0,
		MaxRetries:               0,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}

	mock.ExpectExec("INSERT INTO functions").
		WithArgs(f.ID, f.UserID, f.TenantID, f.Name, f.Runtime, f.Handler, f.CodePath, f.Timeout, f.MemoryMB, f.CPUs, f.Status, f.MaxConcurrentInvocations, f.MaxQueueDepth, f.MaxRetries, f.CreatedAt, f.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Create(context.Background(), f)
	require.NoError(t, err)
}

func TestFunctionRepositoryGetByID(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewFunctionRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, tenant_id, name, runtime, handler, code_path, timeout_seconds, memory_mb, cpus, status, max_concurrent_invocations, max_queue_depth, max_retries, env_vars, created_at, updated_at FROM functions WHERE id = \\$1 AND tenant_id = \\$2").
		WithArgs(id, tenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "runtime", "handler", "code_path", "timeout_seconds", "memory_mb", "cpus", "status", "max_concurrent_invocations", "max_queue_depth", "max_retries", "env_vars", "created_at", "updated_at"}).
			AddRow(id, uuid.New(), tenantID, testFuncName, testFuncRuntime, testFuncHandler, testFuncPath, 30, 128, 0.5, "ready", 0, 0, 0, []byte("{}"), now, now))

	f, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, id, f.ID)
	assert.Equal(t, tenantID, f.TenantID)
}

func TestFunctionRepositoryList(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewFunctionRepository(mock)
	userID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)
	now := time.Now()

	mock.ExpectQuery("SELECT id, user_id, tenant_id, name, runtime, handler, code_path, timeout_seconds, memory_mb, cpus, status, max_concurrent_invocations, max_queue_depth, max_retries, env_vars, created_at, updated_at FROM functions WHERE tenant_id = \\$1").
		WithArgs(tenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "runtime", "handler", "code_path", "timeout_seconds", "memory_mb", "cpus", "status", "max_concurrent_invocations", "max_queue_depth", "max_retries", "env_vars", "created_at", "updated_at"}).
			AddRow(uuid.New(), userID, tenantID, testFuncName, testFuncRuntime, testFuncHandler, testFuncPath, 30, 128, 0.5, "ready", 0, 0, 0, []byte("{}"), now, now))

	functions, err := repo.List(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, functions, 1)
}

func TestFunctionRepositoryDelete(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewFunctionRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("DELETE FROM functions WHERE id = \\$1 AND tenant_id = \\$2").
		WithArgs(id, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(ctx, id)
	require.NoError(t, err)
}

func TestFunctionRepositoryUpdate(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewFunctionRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	timeout := 300
	mock.ExpectExec("UPDATE functions SET").
		WithArgs(id, tenantID, timeout).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.Update(ctx, id, &domain.FunctionUpdate{Timeout: &timeout})
	require.NoError(t, err)
}

func TestFunctionRepositoryGetDLQInvocations(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewFunctionRepository(mock)
	functionID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)
	now := time.Now()

	mock.ExpectQuery("SELECT id, function_id, status, started_at, ended_at, duration_ms, status_code, logs, retry_count, max_retries FROM invocations WHERE function_id = \\$1 AND status = 'DLQ'").
		WithArgs(functionID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "function_id", "status", "started_at", "ended_at", "duration_ms", "status_code", "logs", "retry_count", "max_retries"}).
			AddRow(uuid.New(), functionID, "DLQ", now, nil, 500, 1, "failed after 3 retries", 3, 3))

	invocations, err := repo.GetDLQInvocations(ctx, functionID)
	require.NoError(t, err)
	require.Len(t, invocations, 1)
	assert.Equal(t, "DLQ", invocations[0].Status)
	assert.Equal(t, 3, invocations[0].RetryCount)
}

func TestFunctionRepositoryUpdateInvocation(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewFunctionRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)
	now := time.Now()
	endedAt := now

	inv := &domain.Invocation{
		ID:         id,
		FunctionID: uuid.New(),
		Status:     "PENDING",
		RetryCount: 0,
		EndedAt:    &endedAt,
	}

	mock.ExpectExec("UPDATE invocations SET").
		WithArgs("PENDING", &endedAt, 0, 0, "", 0, id).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.UpdateInvocation(ctx, inv)
	require.NoError(t, err)
}

func TestFunctionRepositoryGetInvocationByID(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewFunctionRepository(mock)
	id := uuid.New()
	functionID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)
	now := time.Now()

	mock.ExpectQuery("SELECT id, function_id, status, started_at, ended_at, duration_ms, status_code, logs, retry_count, max_retries FROM invocations WHERE id = \\$1").
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{"id", "function_id", "status", "started_at", "ended_at", "duration_ms", "status_code", "logs", "retry_count", "max_retries"}).
			AddRow(id, functionID, "DLQ", now, nil, 500, 1, "failed after 3 retries", 3, 3))

	inv, err := repo.GetInvocationByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.Equal(t, id, inv.ID)
	assert.Equal(t, functionID, inv.FunctionID)
	assert.Equal(t, "DLQ", inv.Status)
	assert.Equal(t, 3, inv.RetryCount)
}
