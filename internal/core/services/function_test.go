package services_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
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
		return true // validating key format in logic is enough, usually userID/funcID/code.zip
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
