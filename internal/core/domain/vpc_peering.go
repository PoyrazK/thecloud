// Package domain defines core business entities.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// VPC Peering status constants.
const (
	PeeringStatusPendingAcceptance = "pending-acceptance"
	PeeringStatusActive            = "active"
	PeeringStatusRejected          = "rejected"
	PeeringStatusDeleted           = "deleted"
	PeeringStatusFailed            = "failed"
)

// VPCPeering represents a network peering connection between two VPCs.
// It enables private IP communication between instances in different VPCs
// by programming cross-bridge OVS flow rules.
type VPCPeering struct {
	ID               uuid.UUID `json:"id"`
	RequesterVPCID   uuid.UUID `json:"requester_vpc_id"`
	AccepterVPCID    uuid.UUID `json:"accepter_vpc_id"`
	RequesterTenantID uuid.UUID `json:"requester_tenant_id"`
	AccepterTenantID  uuid.UUID `json:"accepter_tenant_id"`
	Status           string    `json:"status"`
	ARN              string    `json:"arn"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
