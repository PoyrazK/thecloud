package k8s

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/mock"
)

type MockSecurityGroupService struct{ mock.Mock }

func (m *MockSecurityGroupService) CreateGroup(ctx context.Context, vpcID uuid.UUID, name, description string) (*domain.SecurityGroup, error) {
	args := m.Called(ctx, vpcID, name, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.SecurityGroup)
	return r0, args.Error(1)
}

func (m *MockSecurityGroupService) GetGroup(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.SecurityGroup, error) {
	args := m.Called(ctx, idOrName, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.SecurityGroup)
	return r0, args.Error(1)
}

func (m *MockSecurityGroupService) ListGroups(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error) {
	args := m.Called(ctx, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.SecurityGroup)
	return r0, args.Error(1)
}

func (m *MockSecurityGroupService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockSecurityGroupService) AddRule(ctx context.Context, groupID uuid.UUID, rule domain.SecurityRule) (*domain.SecurityRule, error) {
	args := m.Called(ctx, groupID, rule)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.SecurityRule)
	return r0, args.Error(1)
}

func (m *MockSecurityGroupService) RemoveRule(ctx context.Context, ruleID uuid.UUID) error {
	return m.Called(ctx, ruleID).Error(0)
}

func (m *MockSecurityGroupService) AttachToInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	return m.Called(ctx, instanceID, groupID).Error(0)
}

func (m *MockSecurityGroupService) DetachFromInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	return m.Called(ctx, instanceID, groupID).Error(0)
}
