// Package ports defines service and repository interfaces.
package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// SecurityGroupRepository manages the persistent state of virtual firewalls (security groups).
type SecurityGroupRepository interface {
	// Create saves a new security group definition.
	Create(ctx context.Context, sg *domain.SecurityGroup) error
	// GetByID retrieves a security group by its unique ID.
	GetByID(ctx context.Context, id uuid.UUID) (*domain.SecurityGroup, error)
	// GetByName retrieves a security group by name within a specific VPC.
	GetByName(ctx context.Context, vpcID uuid.UUID, name string) (*domain.SecurityGroup, error)
	// GetByNameAcrossVPCs retrieves a security group by name across all VPCs in the tenant.
	GetByNameAcrossVPCs(ctx context.Context, name string) (*domain.SecurityGroup, error)
	// ListByVPC returns all security groups belonging to a VPC.
	ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error)
	// AddRule appends a new traffic filtering rule to an existing group.
	AddRule(ctx context.Context, rule *domain.SecurityRule) error
	// GetRuleByID retrieves a specific security rule.
	GetRuleByID(ctx context.Context, ruleID uuid.UUID) (*domain.SecurityRule, error)
	// DeleteRule removes a specific traffic filtering rule.
	DeleteRule(ctx context.Context, ruleID uuid.UUID) error
	// Delete removes a security group and all its rules.
	Delete(ctx context.Context, id uuid.UUID) error

	// Instance association

	// AddInstanceToGroup links a compute instance to a security group.
	AddInstanceToGroup(ctx context.Context, instanceID, groupID uuid.UUID) error
	// RemoveInstanceFromGroup unlinks a compute instance from a security group.
	RemoveInstanceFromGroup(ctx context.Context, instanceID, groupID uuid.UUID) error
	// ListInstanceGroups retrieves all security groups currently protecting a specific instance.
	ListInstanceGroups(ctx context.Context, instanceID uuid.UUID) ([]*domain.SecurityGroup, error)
}

// SecurityGroupService provides business logic for managing virtual networking firewalls.
type SecurityGroupService interface {
	// CreateGroup establishes a new security group within a VPC.
	CreateGroup(ctx context.Context, vpcID uuid.UUID, name, description string) (*domain.SecurityGroup, error)
	// GetGroup retrieves details for a specific security group.
	GetGroup(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.SecurityGroup, error)
	// ListGroups returns all security groups for an authorized VPC.
	ListGroups(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error)
	// DeleteGroup decommissioning a security group.
	DeleteGroup(ctx context.Context, id uuid.UUID) error

	// AddRule adds an ingress or egress firewall rule to a group.
	AddRule(ctx context.Context, idOrName string, rule domain.SecurityRule) (*domain.SecurityRule, error)
	// RemoveRule deletes an existing firewall rule.
	RemoveRule(ctx context.Context, ruleID uuid.UUID) error

	// AttachToInstance applies a security group's rules to a compute instance.
	AttachToInstance(ctx context.Context, instanceID, groupID uuid.UUID) error
	// DetachFromInstance removes a security group's rules from a compute instance.
	DetachFromInstance(ctx context.Context, instanceID, groupID uuid.UUID) error
}
