package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type SecurityGroupRepository interface {
	Create(ctx context.Context, sg *domain.SecurityGroup) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.SecurityGroup, error)
	GetByName(ctx context.Context, vpcID uuid.UUID, name string) (*domain.SecurityGroup, error)
	ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error)
	AddRule(ctx context.Context, rule *domain.SecurityRule) error
	DeleteRule(ctx context.Context, ruleID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Instance association
	AddInstanceToGroup(ctx context.Context, instanceID, groupID uuid.UUID) error
	RemoveInstanceFromGroup(ctx context.Context, instanceID, groupID uuid.UUID) error
	ListInstanceGroups(ctx context.Context, instanceID uuid.UUID) ([]*domain.SecurityGroup, error)
}

type SecurityGroupService interface {
	CreateGroup(ctx context.Context, vpcID uuid.UUID, name, description string) (*domain.SecurityGroup, error)
	GetGroup(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.SecurityGroup, error)
	ListGroups(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error)
	DeleteGroup(ctx context.Context, id uuid.UUID) error

	AddRule(ctx context.Context, groupID uuid.UUID, rule domain.SecurityRule) (*domain.SecurityRule, error)
	RemoveRule(ctx context.Context, ruleID uuid.UUID) error

	AttachToInstance(ctx context.Context, instanceID, groupID uuid.UUID) error
	DetachFromInstance(ctx context.Context, instanceID, groupID uuid.UUID) error
}
