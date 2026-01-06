package domain

import (
	"github.com/google/uuid"
)

type Permission string

const (
	// Compute Permissions
	PermissionInstanceLaunch    Permission = "instance:launch"
	PermissionInstanceTerminate Permission = "instance:terminate"
	PermissionInstanceRead      Permission = "instance:read"
	PermissionInstanceUpdate    Permission = "instance:update"

	// VPC Permissions
	PermissionVpcCreate Permission = "vpc:create"
	PermissionVpcDelete Permission = "vpc:delete"
	PermissionVpcRead   Permission = "vpc:read"

	// Storage Permissions
	PermissionVolumeCreate Permission = "volume:create"
	PermissionVolumeDelete Permission = "volume:delete"
	PermissionVolumeRead   Permission = "volume:read"

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
	PermissionDbCreate Permission = "db:create"
	PermissionDbDelete Permission = "db:delete"
	PermissionDbRead   Permission = "db:read"

	// Secret Permissions
	PermissionSecretCreate Permission = "secret:create"
	PermissionSecretDelete Permission = "secret:delete"
	PermissionSecretRead   Permission = "secret:read"

	// Function Permissions
	PermissionFunctionInvoke Permission = "function:invoke"
	PermissionFunctionCreate Permission = "function:create"
	PermissionFunctionDelete Permission = "function:delete"
	PermissionFunctionRead   Permission = "function:read"

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

	// Auto-Scaling Permissions
	PermissionAsCreate Permission = "as:create"
	PermissionAsDelete Permission = "as:delete"
	PermissionAsRead   Permission = "as:read"
	PermissionAsUpdate Permission = "as:update"

	// Container Permissions
	PermissionContainerCreate Permission = "container:create"
	PermissionContainerDelete Permission = "container:delete"
	PermissionContainerRead   Permission = "container:read"
	PermissionContainerUpdate Permission = "container:scale"

	// System Permissions
	PermissionFullAccess Permission = "*"
)

type Role struct {
	ID          uuid.UUID    `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
}

// Default Roles
const (
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
	RoleViewer    = "viewer"
)
