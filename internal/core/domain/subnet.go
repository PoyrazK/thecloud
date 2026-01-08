package domain

import (
	"time"

	"github.com/google/uuid"
)

type Subnet struct {
	ID               uuid.UUID `json:"id"`
	UserID           uuid.UUID `json:"user_id"`
	VPCID            uuid.UUID `json:"vpc_id"`
	Name             string    `json:"name"`
	CIDRBlock        string    `json:"cidr_block"`
	AvailabilityZone string    `json:"availability_zone"`
	GatewayIP        string    `json:"gateway_ip"`
	ARN              string    `json:"arn"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}
