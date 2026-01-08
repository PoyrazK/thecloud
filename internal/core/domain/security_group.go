package domain

import (
	"time"

	"github.com/google/uuid"
)

type SecurityGroup struct {
	ID          uuid.UUID      `json:"id"`
	UserID      uuid.UUID      `json:"user_id"`
	VPCID       uuid.UUID      `json:"vpc_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ARN         string         `json:"arn"`
	Rules       []SecurityRule `json:"rules,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type SecurityRule struct {
	ID        uuid.UUID     `json:"id"`
	GroupID   uuid.UUID     `json:"group_id"`
	Direction RuleDirection `json:"direction"`
	Protocol  string        `json:"protocol"` // "tcp", "udp", "icmp", "all"
	PortMin   int           `json:"port_min,omitempty"`
	PortMax   int           `json:"port_max,omitempty"`
	CIDR      string        `json:"cidr"`
	Priority  int           `json:"priority"`
	CreatedAt time.Time     `json:"created_at"`
}

type RuleDirection string

const (
	RuleIngress RuleDirection = "ingress"
	RuleEgress  RuleDirection = "egress"
)
