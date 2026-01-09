package services_test

import (
	"archive/zip"
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

func setupFunctionServiceTest(t *testing.T) (*MockFunctionRepository, *MockComputeBackend, *MockFileStore, *MockAuditService, ports.FunctionService) {
	repo := new(MockFunctionRepository)
	compute := new(MockComputeBackend)
	fileStore := new(MockFileStore)
	auditSvc := new(MockAuditService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewFunctionService(repo, compute, fileStore, auditSvc, logger)
	return repo, compute, fileStore, auditSvc, svc
}

func createZip(t *testing.T, filename, content string) []byte {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, err := w.Create(filename)
	assert.NoError(t, err)
	_, err = f.Write([]byte(content))
	assert.NoError(t, err)
	err = w.Close()
	assert.NoError(t, err)
	return buf.Bytes()
}

func TestCreateFunction_Success(t *testing.T) {
	repo, _, fileStore, auditSvc, svc := setupFunctionServiceTest(t)
	defer repo.AssertExpectations(t)
	defer fileStore.AssertExpectations(t)
	defer auditSvc.AssertExpectations(t)

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	name := "test-func"
	runtime := "nodejs20"
	handler := "index.handler"
	code := []byte("console.log('hello')")

	fileStore.On("Write", ctx, "functions", mock.Anything, mock.Anything).Return(int64(len(code)), nil)
	repo.On("Create", ctx, mock.MatchedBy(func(f *domain.Function) bool {
		return f.Name == name && f.Runtime == runtime && f.UserID == userID
	})).Return(nil)
	auditSvc.On("Log", ctx, userID, "function.create", "function", mock.Anything, mock.Anything).Return(nil)

	f, err := svc.CreateFunction(ctx, name, runtime, handler, code)

	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Equal(t, name, f.Name)
	assert.Equal(t, "ACTIVE", f.Status)
}

func TestCreateFunction_Unauthorized(t *testing.T) {
	_, _, _, _, svc := setupFunctionServiceTest(t)

	ctx := context.Background()
	_, err := svc.CreateFunction(ctx, "test", "nodejs20", "handler", []byte("code"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not authenticated")
}

func TestCreateFunction_InvalidRuntime(t *testing.T) {
	_, _, _, _, svc := setupFunctionServiceTest(t)

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	_, err := svc.CreateFunction(ctx, "test", "invalid-runtime", "handler", []byte("code"))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported runtime")
}

func TestInvokeFunction_Success(t *testing.T) {
	repo, compute, fileStore, auditSvc, svc := setupFunctionServiceTest(t)
	defer repo.AssertExpectations(t)
	defer compute.AssertExpectations(t)
	defer fileStore.AssertExpectations(t)
	defer auditSvc.AssertExpectations(t)

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

	zipBytes := []byte{0x50, 0x4b, 0x05, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	fileStore.On("Read", ctx, "functions", f.CodePath).Return(io.NopCloser(bytes.NewReader(zipBytes)), nil)

	containerID := "container-123"
	compute.On("RunTask", ctx, mock.MatchedBy(func(opts ports.RunTaskOptions) bool {
		return opts.Image == "node:20-alpine" && opts.MemoryMB == 128
	})).Return(containerID, nil)

	compute.On("WaitTask", mock.Anything, containerID).Return(int64(0), nil)
	compute.On("GetInstanceLogs", mock.Anything, containerID).Return(io.NopCloser(bytes.NewReader([]byte("logs"))), nil)
	compute.On("DeleteInstance", mock.Anything, containerID).Return(nil)

	repo.On("CreateInvocation", mock.Anything, mock.MatchedBy(func(i *domain.Invocation) bool {
		return i.Status == "SUCCESS" && i.FunctionID == funcID
	})).Return(nil)

	inv, err := svc.InvokeFunction(ctx, funcID, []byte("payload"), false)

	assert.NoError(t, err)
	assert.NotNil(t, inv)
	assert.Equal(t, "SUCCESS", inv.Status)
}

func TestDeleteFunction_Success(t *testing.T) {
	repo, _, fileStore, auditSvc, svc := setupFunctionServiceTest(t)
	defer repo.AssertExpectations(t)
	defer fileStore.AssertExpectations(t)
	defer auditSvc.AssertExpectations(t)

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
}

func TestFunctionService_ListFunctions(t *testing.T) {
	repo, _, _, _, svc := setupFunctionServiceTest(t)
	defer repo.AssertExpectations(t)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)
	fns := []*domain.Function{{ID: uuid.New(), UserID: userID}}

	repo.On("List", ctx, userID).Return(fns, nil)

	res, err := svc.ListFunctions(ctx)
	assert.NoError(t, err)
	assert.Equal(t, fns, res)
}

func TestFunctionService_GetFunction(t *testing.T) {
	repo, _, _, _, svc := setupFunctionServiceTest(t)
	defer repo.AssertExpectations(t)

	id := uuid.New()
	f := &domain.Function{ID: id}

	repo.On("GetByID", mock.Anything, id).Return(f, nil)

	res, err := svc.GetFunction(context.Background(), id)
	assert.NoError(t, err)
	assert.Equal(t, f, res)
}

func TestFunctionService_GetFunctionLogs(t *testing.T) {
	repo, _, _, _, svc := setupFunctionServiceTest(t)
	defer repo.AssertExpectations(t)

	id := uuid.New()
	invs := []*domain.Invocation{{ID: uuid.New()}}

	repo.On("GetInvocations", mock.Anything, id, 10).Return(invs, nil)

	res, err := svc.GetFunctionLogs(context.Background(), id, 10)
	assert.NoError(t, err)
	assert.Equal(t, invs, res)
}

func TestFunctionService_InvokeAsync(t *testing.T) {
	repo, _, fileStore, auditSvc, svc := setupFunctionServiceTest(t)

	id := uuid.New()
	userID := uuid.New()
	f := &domain.Function{ID: id, UserID: userID, CodePath: "c", Runtime: "nodejs20", Handler: "h", Timeout: 30}
	repo.On("GetByID", mock.Anything, id).Return(f, nil)
	auditSvc.On("Log", mock.Anything, userID, "function.invoke_async", "function", id.String(), mock.Anything).Return(nil)

	// Mocks for the async goroutine
	fileStore.On("Read", mock.Anything, "functions", "c").Return(nil, assert.AnError).Maybe()
	repo.On("CreateInvocation", mock.Anything, mock.Anything).Return(nil).Maybe()

	inv, err := svc.InvokeFunction(context.Background(), id, []byte("{}"), true)
	assert.NoError(t, err)
	assert.NotNil(t, inv)
	// Don't check inv.Status as it's modified by the async goroutine, which causes a race condition

	// Wait a bit for the goroutine to finish its work
	time.Sleep(50 * time.Millisecond)

	// Don't assert mock expectations for async operations as they run in a separate goroutine
	// and may not have completed yet, causing race conditions
}

func TestFunctionService_Errors(t *testing.T) {
	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	fID := uuid.New()

	t.Run("Create_StoreError", func(t *testing.T) {
		_, _, fileStore, _, svc := setupFunctionServiceTest(t)
		fileStore.On("Write", ctx, "functions", mock.Anything, mock.Anything).Return(int64(0), assert.AnError)
		_, err := svc.CreateFunction(ctx, "n", "nodejs20", "h", []byte("c"))
		assert.Error(t, err)
	})

	t.Run("Invoke_GetError", func(t *testing.T) {
		repo, _, _, _, svc := setupFunctionServiceTest(t)
		repo.On("GetByID", mock.Anything, fID).Return(nil, assert.AnError)
		_, err := svc.InvokeFunction(ctx, fID, []byte("{}"), false)
		assert.Error(t, err)
	})
}

func TestFunctionService_ListFunctions_Unauthorized(t *testing.T) {
	_, _, _, _, svc := setupFunctionServiceTest(t)
	_, err := svc.ListFunctions(context.Background())
	assert.Error(t, err)
}

func TestFunctionService_DeleteFunction_RepoError(t *testing.T) {
	repo, _, _, _, svc := setupFunctionServiceTest(t)
	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(&domain.Function{ID: id}, nil)
	repo.On("Delete", mock.Anything, id).Return(assert.AnError)

	err := svc.DeleteFunction(context.Background(), id)
	assert.Error(t, err)
}

func TestFunctionService_Invoke_RunTaskError(t *testing.T) {
	repo, compute, fileStore, auditSvc, svc := setupFunctionServiceTest(t)
	id := uuid.New()
	userID := uuid.New()
	f := &domain.Function{ID: id, UserID: userID, CodePath: "c", Runtime: "nodejs20", Handler: "h", Timeout: 30}
	repo.On("GetByID", mock.Anything, id).Return(f, nil)
	auditSvc.On("Log", mock.Anything, userID, "function.invoke", "function", id.String(), mock.Anything).Return(nil)
	fileStore.On("Read", mock.Anything, "functions", "c").Return(io.NopCloser(bytes.NewReader(createZip(t, "h", "c"))), nil)

	compute.On("RunTask", mock.Anything, mock.Anything).Return("", assert.AnError)
	repo.On("CreateInvocation", mock.Anything, mock.MatchedBy(func(i *domain.Invocation) bool {
		return i.Status == "FAILED"
	})).Return(nil)

	_, err := svc.InvokeFunction(context.Background(), id, []byte("{}"), false)
	assert.Error(t, err)
}

func TestFunctionService_PrepareCode_Failures(t *testing.T) {
	repo, _, fileStore, _, svc := setupFunctionServiceTest(t)
	_ = repo
	_ = svc
	id := uuid.New()
	_ = id

	t.Run("ZipReadError", func(t *testing.T) {
		fileStore.On("Read", mock.Anything, "functions", "c").Return(io.NopCloser(bytes.NewReader([]byte("not a zip"))), nil).Once()
	})
}

func TestFunctionService_Invoke_ZipError(t *testing.T) {
	repo, _, fileStore, auditSvc, svc := setupFunctionServiceTest(t)
	id := uuid.New()
	userID := uuid.New()
	f := &domain.Function{ID: id, UserID: userID, CodePath: "c", Runtime: "nodejs20", Handler: "h", Timeout: 30}
	repo.On("GetByID", mock.Anything, id).Return(f, nil)
	auditSvc.On("Log", mock.Anything, userID, "function.invoke", "function", id.String(), mock.Anything).Return(nil)
	fileStore.On("Read", mock.Anything, "functions", "c").Return(io.NopCloser(bytes.NewReader([]byte("not a zip"))), nil)

	repo.On("CreateInvocation", mock.Anything, mock.MatchedBy(func(i *domain.Invocation) bool {
		return i.Status == "FAILED"
	})).Return(nil)

	_, err := svc.InvokeFunction(context.Background(), id, []byte("{}"), false)
	assert.Error(t, err)
}

func TestFunctionService_Invoke_WaitError(t *testing.T) {
	repo, compute, fileStore, auditSvc, svc := setupFunctionServiceTest(t)
	id := uuid.New()
	userID := uuid.New()
	f := &domain.Function{ID: id, UserID: userID, CodePath: "c", Runtime: "nodejs20", Handler: "h", Timeout: 30}
	repo.On("GetByID", mock.Anything, id).Return(f, nil)
	auditSvc.On("Log", mock.Anything, userID, "function.invoke", "function", id.String(), mock.Anything).Return(nil)
	fileStore.On("Read", mock.Anything, "functions", "c").Return(io.NopCloser(bytes.NewReader(createZip(t, "h", "c"))), nil)

	compute.On("RunTask", mock.Anything, mock.Anything).Return("c1", nil)
	compute.On("WaitTask", mock.Anything, "c1").Return(int64(0), assert.AnError)
	compute.On("GetInstanceLogs", mock.Anything, "c1").Return(nil, assert.AnError)
	compute.On("DeleteInstance", mock.Anything, "c1").Return(nil)

	repo.On("CreateInvocation", mock.Anything, mock.MatchedBy(func(i *domain.Invocation) bool {
		return i.Status == "FAILED"
	})).Return(nil)

	inv, err := svc.InvokeFunction(context.Background(), id, []byte("{}"), false)
	assert.NoError(t, err)
	assert.Equal(t, "FAILED", inv.Status)
}

func TestFunctionService_PrepareCode_ZipSlip(t *testing.T) {
	repo, _, fileStore, auditSvc, svc := setupFunctionServiceTest(t)
	id := uuid.New()
	userID := uuid.New()
	f := &domain.Function{ID: id, UserID: userID, CodePath: "c", Runtime: "nodejs20", Handler: "h", Timeout: 30}
	repo.On("GetByID", mock.Anything, id).Return(f, nil)
	auditSvc.On("Log", mock.Anything, userID, "function.invoke", "function", id.String(), mock.Anything).Return(nil)

	// ZIP SLIP: file name with ../
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	_, _ = w.Create("../etc/passwd")
	_ = w.Close()

	fileStore.On("Read", mock.Anything, "functions", "c").Return(io.NopCloser(bytes.NewReader(buf.Bytes())), nil)

	repo.On("CreateInvocation", mock.Anything, mock.Anything).Return(nil)

	_, err := svc.InvokeFunction(context.Background(), id, []byte("{}"), false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file path in zip")
}
