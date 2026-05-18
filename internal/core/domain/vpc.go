// Package domain defines core business entities.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// VPC represents a Virtual Private Cloud (isolated network).
// It acts as a container for subnets and other network resources.
type VPC struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	Name           string    `json:"name"`
	CIDRBlock      string    `json:"cidr_block"` // IPv4 range (e.g. "10.0.0.0/16")
	NetworkID      string    `json:"network_id"` // OVS bridge name or backend ID
	VXLANID        int       `json:"vxlan_id"`   // Tunnel ID for isolation
	Status         string    `json:"status"`     // e.g. "ACTIVE"
	ARN            string    `json:"arn"`        // Amazon Resource Name compatible ID
	CreatedAt      time.Time `json:"created_at"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
}
