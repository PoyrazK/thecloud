// Package domain defines core business entities.
package domain

import (
	"github.com/google/uuid"
)

// Permission represents a specific authorization grant string.
// Format: "resource:action" (e.g. "instance:launch")
type Permission string

// Permissions define coarse-grained access control for resources.
const (
	// Compute Permissions
	PermissionInstanceLaunch    Permission = "instance:launch"
	PermissionInstanceTerminate Permission = "instance:terminate"
	PermissionInstanceRead      Permission = "instance:read"
	PermissionInstanceUpdate    Permission = "instance:update"
	PermissionInstanceResize    Permission = "instance:resize"

	// SSH Key Permissions
	PermissionSSHKeyCreate Permission = "ssh_key:create"
	PermissionSSHKeyRead   Permission = "ssh_key:read"
	PermissionSSHKeyDelete Permission = "ssh_key:delete"

	// VPC Permissions
	PermissionVpcCreate Permission = "vpc:create"
	PermissionVpcDelete Permission = "vpc:delete"
	PermissionVpcRead   Permission = "vpc:read"
	PermissionVpcUpdate Permission = "vpc:update"

	// Security Group Permissions
	PermissionSgCreate Permission = "sg:create"
	PermissionSgDelete Permission = "sg:delete"
	PermissionSgRead   Permission = "sg:read"
	PermissionSgUpdate Permission = "sg:update"

	// VPC Peering Permissions
	PermissionVpcPeeringCreate Permission = "vpc_peering:create"
	PermissionVpcPeeringAccept Permission = "vpc_peering:accept"
	PermissionVpcPeeringDelete Permission = "vpc_peering:delete"
	PermissionVpcPeeringRead   Permission = "vpc_peering:read"

	// Elastic IP Permissions
	PermissionEipAllocate  Permission = "eip:allocate"
	PermissionEipRelease   Permission = "eip:release"
	PermissionEipRead      Permission = "eip:read"
	PermissionEipAssociate Permission = "eip:associate"

	// Storage Permissions
	PermissionVolumeCreate Permission = "volume:create"
	PermissionVolumeDelete Permission = "volume:delete"
	PermissionVolumeRead   Permission = "volume:read"
	PermissionVolumeUpdate Permission = "volume:update"

	PermissionStorageWrite  Permission = "storage:write"
	PermissionStorageRead   Permission = "storage:read"
	PermissionStorageDelete Permission = "storage:delete"

	// Snapshot Permissions
	PermissionSnapshotCreate  Permission = "snapshot:create"
	PermissionSnapshotDelete  Permission = "snapshot:delete"
	PermissionSnapshotRead    Permission = "snapshot:read"
	PermissionSnapshotRestore Permission = "snapshot:restore"

	// Load Balancer Permissions
	PermissionLbCreate Permission = "lb:create"
	PermissionLbDelete Permission = "lb:delete"
	PermissionLbRead   Permission = "lb:read"
	PermissionLbUpdate Permission = "lb:update"

	// Database Permissions
	PermissionDBCreate Permission = "db:create"
	PermissionDBDelete Permission = "db:delete"
	PermissionDBRead   Permission = "db:read"
	PermissionDBUpdate Permission = "db:update"

	// Secret Permissions
	PermissionSecretCreate Permission = "secret:create"
	PermissionSecretWrite  Permission = "secret:write"
	PermissionSecretDelete Permission = "secret:delete"
	PermissionSecretRead   Permission = "secret:read"

	// Function Permissions
	PermissionFunctionInvoke Permission = "function:invoke"
	PermissionFunctionCreate Permission = "function:create"
	PermissionFunctionDelete Permission = "function:delete"
	PermissionFunctionRead   Permission = "function:read"
	PermissionFunctionUpdate Permission = "function:update"

	// Function Schedule Permissions
	PermissionFunctionScheduleCreate Permission = "function_schedule:create"
	PermissionFunctionScheduleRead   Permission = "function_schedule:read"
	PermissionFunctionScheduleDelete Permission = "function_schedule:delete"
	PermissionFunctionScheduleUpdate Permission = "function_schedule:update"

	// Cache Permissions
	PermissionCacheCreate Permission = "cache:create"
	PermissionCacheDelete Permission = "cache:delete"
	PermissionCacheRead   Permission = "cache:read"
	PermissionCacheUpdate Permission = "cache:update"

	// Queue Permissions
	PermissionQueueCreate Permission = "queue:create"
	PermissionQueueDelete Permission = "queue:delete"
	PermissionQueueRead   Permission = "queue:read"
	PermissionQueueWrite  Permission = "queue:write"

	// Notify Permissions
	PermissionNotifyCreate Permission = "notify:create"
	PermissionNotifyDelete Permission = "notify:delete"
	PermissionNotifyRead   Permission = "notify:read"
	PermissionNotifyWrite  Permission = "notify:publish"

	// Cron Permissions
	PermissionCronCreate Permission = "cron:create"
	PermissionCronDelete Permission = "cron:delete"
	PermissionCronRead   Permission = "cron:read"
	PermissionCronUpdate Permission = "cron:update"

	// Gateway Permissions
	PermissionGatewayCreate Permission = "gateway:create"
	PermissionGatewayDelete Permission = "gateway:delete"
	PermissionGatewayRead   Permission = "gateway:read"

	// IaC Permissions
	PermissionStackCreate Permission = "stack:create"
	PermissionStackDelete Permission = "stack:delete"
	PermissionStackRead   Permission = "stack:read"
	// Auto-Scaling Permissions
	PermissionAsgCreate Permission = "asg:create"
	PermissionAsgDelete Permission = "asg:delete"

	PermissionAsgRead   Permission = "asg:read"
	PermissionAsgUpdate Permission = "asg:update"

	// Container Permissions
	PermissionContainerCreate Permission = "container:create"
	PermissionContainerDelete Permission = "container:delete"
	PermissionContainerRead   Permission = "container:read"
	PermissionContainerUpdate Permission = "container:scale"

	// Pipeline Permissions
	PermissionPipelineCreate  Permission = "pipeline:create"
	PermissionPipelineDelete  Permission = "pipeline:delete"
	PermissionPipelineRead    Permission = "pipeline:read"
	PermissionPipelineUpdate  Permission = "pipeline:update"
	PermissionPipelineRun     Permission = "pipeline:run"
	PermissionPipelineWebhook Permission = "pipeline:webhook"

	// Image Permissions
	PermissionImageCreate  Permission = "image:create"
	PermissionImageRead    Permission = "image:read"
	PermissionImageDelete  Permission = "image:delete"
	PermissionImageReadAll Permission = "image:read_all"

	// Cluster Permissions
	PermissionClusterCreate Permission = "cluster:create"
	PermissionClusterDelete Permission = "cluster:delete"
	PermissionClusterRead   Permission = "cluster:read"
	PermissionClusterUpdate Permission = "cluster:update"

	// DNS Permissions
	PermissionDNSCreate Permission = "dns:create"
	PermissionDNSDelete Permission = "dns:delete"
	PermissionDNSRead   Permission = "dns:read"
	PermissionDNSUpdate Permission = "dns:update"

	// Tenant Permissions
	PermissionTenantCreate Permission = "tenant:create"
	PermissionTenantRead   Permission = "tenant:read"
	PermissionTenantUpdate Permission = "tenant:update"
	PermissionTenantDelete Permission = "tenant:delete"

	// Identity Permissions
	PermissionIdentityCreate  Permission = "identity:create"
	PermissionIdentityRead    Permission = "identity:read"
	PermissionIdentityDelete  Permission = "identity:delete"
	PermissionIdentityReadAll Permission = "identity:read_all"

	// Service Account Permissions
	PermissionServiceAccountCreate Permission = "service_account:create"
	PermissionServiceAccountRead   Permission = "service_account:read"
	PermissionServiceAccountUpdate Permission = "service_account:update"
	PermissionServiceAccountDelete Permission = "service_account:delete"

	// Accounting Permissions
	PermissionAccountingRead Permission = "accounting:read"

	// Audit Permissions
	PermissionAuditRead Permission = "audit:read"

	// Dashboard Permissions
	PermissionDashboardRead Permission = "dashboard:read"

	// System Permissions
	PermissionFullAccess Permission = "*"
)

// Role represents a named collection of permissions.
type Role struct {
	ID          uuid.UUID    `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
}

// Default Roles
const (
	RoleAdmin     = "admin"
	RoleOwner     = "owner"
	RoleDeveloper = "developer"
	RoleViewer    = "viewer"
)
