package services

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockFunctionRepo struct {
	mock.Mock
}

func (m *MockFunctionRepo) Create(ctx context.Context, f *domain.Function) error {
	args := m.Called(ctx, f)
	return args.Error(0)
}
func (m *MockFunctionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Function, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Function), args.Error(1)
}
func (m *MockFunctionRepo) GetByName(ctx context.Context, userID uuid.UUID, name string) (*domain.Function, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Function), args.Error(1)
}
func (m *MockFunctionRepo) List(ctx context.Context, userID uuid.UUID) ([]*domain.Function, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domain.Function), args.Error(1)
}
func (m *MockFunctionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockFunctionRepo) CreateInvocation(ctx context.Context, i *domain.Invocation) error {
	args := m.Called(ctx, i)
	return args.Error(0)
}
func (m *MockFunctionRepo) GetInvocations(ctx context.Context, functionID uuid.UUID, limit int) ([]*domain.Invocation, error) {
	args := m.Called(ctx, functionID, limit)
	return args.Get(0).([]*domain.Invocation), args.Error(1)
}

type MockFileStore struct {
	mock.Mock
}

func (m *MockFileStore) Write(ctx context.Context, bucket, key string, r io.Reader) (int64, error) {
	args := m.Called(ctx, bucket, key, r)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockFileStore) Read(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *MockFileStore) Delete(ctx context.Context, bucket, key string) error {
	args := m.Called(ctx, bucket, key)
	return args.Error(0)
}

func TestFunctionService_InvokeFunction(t *testing.T) {
	repo := new(MockFunctionRepo)
	docker := new(MockDocker)
	fileStore := new(MockFileStore)
	logger := slog.Default()

	svc := NewFunctionService(repo, docker, fileStore, logger)
	assert.NotNil(t, svc)

	fID := uuid.New()
	f := &domain.Function{
		ID:       fID,
		Name:     "test-fn",
		Runtime:  "nodejs20",
		Handler:  "index.handler",
		CodePath: "user1/fn1/code.zip",
		Timeout:  30,
		MemoryMB: 128,
	}

	repo.On("GetByID", mock.Anything, fID).Return(f, nil)
	// We'd need to mock prepareCode which is internal...
	// For unit testing, we might want to extract internal logic to helpers or interfaces.
	// But let's see if we can test create first.
}

func TestCreateFunction_Success(t *testing.T) {
	repo := new(MockFunctionRepo)
	docker := new(MockDocker)
	fileStore := new(MockFileStore)
	logger := slog.Default()

	svc := NewFunctionService(repo, docker, fileStore, logger)

	ctx := appcontext.WithUserID(context.Background(), uuid.New())

	fileStore.On("Write", mock.Anything, "functions", mock.Anything, mock.Anything).Return(int64(100), nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)

	fn, err := svc.CreateFunction(ctx, "my-fn", "nodejs20", "index.handler", []byte("code"))

	assert.NoError(t, err)
	assert.NotNil(t, fn)
	assert.Equal(t, "my-fn", fn.Name)
	repo.AssertCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateFunction_InvalidRuntime(t *testing.T) {
	repo := new(MockFunctionRepo)
	docker := new(MockDocker)
	fileStore := new(MockFileStore)
	logger := slog.Default()

	svc := NewFunctionService(repo, docker, fileStore, logger)

	ctx := appcontext.WithUserID(context.Background(), uuid.New())

	fn, err := svc.CreateFunction(ctx, "my-fn", "unsupported-runtime", "handler", []byte("code"))

	assert.Error(t, err)
	assert.Nil(t, fn)
	assert.Contains(t, err.Error(), "unsupported runtime")
}

func TestListFunctions(t *testing.T) {
	repo := new(MockFunctionRepo)
	docker := new(MockDocker)
	fileStore := new(MockFileStore)
	logger := slog.Default()

	svc := NewFunctionService(repo, docker, fileStore, logger)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)

	fns := []*domain.Function{{Name: "fn1"}, {Name: "fn2"}}
	repo.On("List", ctx, userID).Return(fns, nil)

	result, err := svc.ListFunctions(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	repo.AssertExpectations(t)
}

func TestDeleteFunction(t *testing.T) {
	repo := new(MockFunctionRepo)
	docker := new(MockDocker)
	fileStore := new(MockFileStore)
	logger := slog.Default()

	svc := NewFunctionService(repo, docker, fileStore, logger)

	ctx := context.Background()
	fnID := uuid.New()
	fn := &domain.Function{ID: fnID, Name: "to-delete", CodePath: "user/fn/code.zip"}

	repo.On("GetByID", ctx, fnID).Return(fn, nil)
	repo.On("Delete", ctx, fnID).Return(nil)
	// The function calls fileStore.Delete in a goroutine with context.Background()
	fileStore.On("Delete", mock.Anything, "functions", fn.CodePath).Return(nil).Maybe()

	err := svc.DeleteFunction(ctx, fnID) // Pass UUID directly

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestGetFunctionLogs(t *testing.T) {
	repo := new(MockFunctionRepo)
	docker := new(MockDocker)
	fileStore := new(MockFileStore)
	logger := slog.Default()

	svc := NewFunctionService(repo, docker, fileStore, logger)

	ctx := context.Background()
	fnID := uuid.New()

	invocations := []*domain.Invocation{
		{ID: uuid.New(), Logs: "Log line 1"},
		{ID: uuid.New(), Logs: "Log line 2"},
	}

	repo.On("GetInvocations", ctx, fnID, 50).Return(invocations, nil)

	logs, err := svc.GetFunctionLogs(ctx, fnID, 50)

	assert.NoError(t, err)
	assert.Len(t, logs, 2)
	assert.Equal(t, "Log line 1", logs[0].Logs)
	repo.AssertExpectations(t)
}
