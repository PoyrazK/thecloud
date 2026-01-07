package services_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateFunction_Success(t *testing.T) {
	repo := new(MockFunctionRepository)
	compute := new(MockComputeBackend)
	fileStore := new(MockFileStore)
	auditSvc := new(MockAuditService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewFunctionService(repo, compute, fileStore, auditSvc, logger)

	ctx := context.Background()
	userID := uuid.New()
	// Simulate authenticated user
	ctx = appcontext.WithUserID(ctx, userID)

	name := "test-func"
	runtime := "nodejs20"
	handler := "index.handler"
	code := []byte("console.log('hello')")

	fileStore.On("Write", ctx, "functions", mock.MatchedBy(func(key string) bool {
		return true
	}), mock.Anything).Return(int64(len(code)), nil)

	repo.On("Create", ctx, mock.MatchedBy(func(f *domain.Function) bool {
		return f.Name == name && f.Runtime == runtime && f.UserID == userID
	})).Return(nil)

	auditSvc.On("Log", ctx, userID, "function.create", "function", mock.Anything, mock.Anything).Return(nil)

	f, err := svc.CreateFunction(ctx, name, runtime, handler, code)

	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, name, f.Name)
	assert.Equal(t, "ACTIVE", f.Status)

	repo.AssertExpectations(t)
	fileStore.AssertExpectations(t)
	auditSvc.AssertExpectations(t)
}

func TestCreateFunction_Unauthorized(t *testing.T) {
	repo := new(MockFunctionRepository)
	compute := new(MockComputeBackend)
	fileStore := new(MockFileStore)
	auditSvc := new(MockAuditService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewFunctionService(repo, compute, fileStore, auditSvc, logger)

	ctx := context.Background()
	// No user in context

	_, err := svc.CreateFunction(ctx, "test", "nodejs20", "handler", []byte("code"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not authenticated")
}

func TestCreateFunction_InvalidRuntime(t *testing.T) {
	repo := new(MockFunctionRepository)
	compute := new(MockComputeBackend)
	fileStore := new(MockFileStore)
	auditSvc := new(MockAuditService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewFunctionService(repo, compute, fileStore, auditSvc, logger)

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	// Invalid runtime
	_, err := svc.CreateFunction(ctx, "test", "invalid-runtime", "handler", []byte("code"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported runtime")
}

func TestInvokeFunction_Success(t *testing.T) {
	repo := new(MockFunctionRepository)
	compute := new(MockComputeBackend)
	fileStore := new(MockFileStore)
	auditSvc := new(MockAuditService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewFunctionService(repo, compute, fileStore, auditSvc, logger)

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	funcID := uuid.New()
	f := &domain.Function{
		ID:       funcID,
		UserID:   userID,
		Name:     "test-func",
		Runtime:  "nodejs20",
		Handler:  "index.handler",
		CodePath: "path/to/code.zip",
		MemoryMB: 128,
		Timeout:  30,
	}

	repo.On("GetByID", ctx, funcID).Return(f, nil)
	auditSvc.On("Log", ctx, userID, "function.invoke", "function", funcID.String(), mock.Anything).Return(nil)

	// Valid minimal empty zip for prepareCode
	zipBytes := []byte{0x50, 0x4b, 0x05, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	fileStore.On("Read", ctx, "functions", f.CodePath).Return(io.NopCloser(bytes.NewReader(zipBytes)), nil)

	// Mock container execution
	containerID := "container-123"
	compute.On("RunTask", ctx, mock.MatchedBy(func(opts ports.RunTaskOptions) bool {
		return opts.Image == "node:20-alpine" && opts.MemoryMB == 128
	})).Return(containerID, nil)

	// Compute backend expectations
	compute.On("WaitTask", mock.Anything, containerID).Return(int64(0), nil) // Success 0
	compute.On("GetInstanceLogs", mock.Anything, containerID).Return(io.NopCloser(bytes.NewReader([]byte("logs"))), nil)
	compute.On("DeleteInstance", mock.Anything, containerID).Return(nil)

	repo.On("CreateInvocation", mock.Anything, mock.MatchedBy(func(i *domain.Invocation) bool {
		return i.Status == "SUCCESS" && i.FunctionID == funcID
	})).Return(nil)

	inv, err := svc.InvokeFunction(ctx, funcID, []byte("payload"), false)

	assert.NoError(t, err)
	assert.NotNil(t, inv)
	assert.Equal(t, "SUCCESS", inv.Status)

	repo.AssertExpectations(t)
	compute.AssertExpectations(t)
}

func TestDeleteFunction_Success(t *testing.T) {
	repo := new(MockFunctionRepository)
	compute := new(MockComputeBackend)
	fileStore := new(MockFileStore)
	auditSvc := new(MockAuditService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewFunctionService(repo, compute, fileStore, auditSvc, logger)

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	funcID := uuid.New()
	f := &domain.Function{
		ID:       funcID,
		UserID:   userID,
		CodePath: "path/to/code.zip",
	}

	repo.On("GetByID", ctx, funcID).Return(f, nil)
	fileStore.On("Delete", mock.Anything, "functions", f.CodePath).Return(nil)
	repo.On("Delete", ctx, funcID).Return(nil)
	auditSvc.On("Log", ctx, userID, "function.delete", "function", funcID.String(), mock.Anything).Return(nil)

	err := svc.DeleteFunction(ctx, funcID)

	// Wait for async deletion
	time.Sleep(10 * time.Millisecond)

	assert.NoError(t, err)

	repo.AssertExpectations(t)
	fileStore.AssertExpectations(t)
}
