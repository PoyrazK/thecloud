package domain

import (
	"time"

	"github.com/google/uuid"
)

type VPC struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	CIDRBlock string    `json:"cidr_block"`
	NetworkID string    `json:"network_id"` // OVS bridge name
	VXLANID   int       `json:"vxlan_id"`
	Status    string    `json:"status"`
	ARN       string    `json:"arn"`
	CreatedAt time.Time `json:"created_at"`
}
