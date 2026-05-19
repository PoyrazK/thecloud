package services_test

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/mock"
)

// MockFunctionRepo
type MockFunctionRepo struct{ mock.Mock }

func (m *MockFunctionRepo) Create(ctx context.Context, f *domain.Function) error {
	return m.Called(ctx, f).Error(0)
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
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Function), args.Error(1)
}
func (m *MockFunctionRepo) Update(ctx context.Context, id uuid.UUID, u *domain.FunctionUpdate) error {
	return m.Called(ctx, id, u).Error(0)
}
func (m *MockFunctionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockFunctionRepo) CreateInvocation(ctx context.Context, i *domain.Invocation) error {
	return m.Called(ctx, i).Error(0)
}
func (m *MockFunctionRepo) GetInvocations(ctx context.Context, functionID uuid.UUID, limit int) ([]*domain.Invocation, error) {
	args := m.Called(ctx, functionID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Invocation), args.Error(1)
}
func (m *MockFunctionRepo) GetDLQInvocations(ctx context.Context, functionID uuid.UUID) ([]*domain.Invocation, error) {
	args := m.Called(ctx, functionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Invocation), args.Error(1)
}
func (m *MockFunctionRepo) GetInvocationByID(ctx context.Context, invocationID uuid.UUID) (*domain.Invocation, error) {
	args := m.Called(ctx, invocationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Invocation), args.Error(1)
}
func (m *MockFunctionRepo) UpdateInvocation(ctx context.Context, i *domain.Invocation) error {
	return m.Called(ctx, i).Error(0)
}

type MockFunctionRepository = MockFunctionRepo
