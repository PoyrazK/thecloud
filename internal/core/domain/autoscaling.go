// Package domain contains the core domain models for the cloud platform.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// System-wide hard limits (Security requirement)
const (
	MaxInstancesHardLimit  = 20 // Maximum instances per scaling group to prevent resource exhaustion
	MaxScalingGroupsPerVPC = 5  // Maximum scaling groups per VPC
	MinCooldownSeconds     = 60 // Minimum cooldown to prevent rapid thrashing
)

type ScalingGroupStatus string

const (
	ScalingGroupStatusActive   ScalingGroupStatus = "ACTIVE"
	ScalingGroupStatusUpdating ScalingGroupStatus = "UPDATING"
	ScalingGroupStatusDeleting ScalingGroupStatus = "DELETING"
	ScalingGroupStatusDeleted  ScalingGroupStatus = "DELETED"
)

type ScalingGroup struct {
	ID             uuid.UUID          `json:"id"`
	UserID         uuid.UUID          `json:"user_id"`
	IdempotencyKey string             `json:"idempotency_key,omitempty"`
	Name           string             `json:"name"`
	VpcID          uuid.UUID          `json:"vpc_id"`
	LoadBalancerID *uuid.UUID         `json:"load_balancer_id,omitempty"`
	Image          string             `json:"image"`
	Ports          string             `json:"ports,omitempty"`
	MinInstances   int                `json:"min_instances"`
	MaxInstances   int                `json:"max_instances"`
	DesiredCount   int                `json:"desired_count"`
	CurrentCount   int                `json:"current_count"`
	Status         ScalingGroupStatus `json:"status"`
	FailureCount   int                `json:"failure_count"`
	LastFailureAt  *time.Time         `json:"last_failure_at,omitempty"`
	Version        int                `json:"version"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

type ScalingPolicy struct {
	ID             uuid.UUID  `json:"id"`
	ScalingGroupID uuid.UUID  `json:"scaling_group_id"`
	Name           string     `json:"name"`
	MetricType     string     `json:"metric_type"` // "cpu" | "memory"
	TargetValue    float64    `json:"target_value"`
	ScaleOutStep   int        `json:"scale_out_step"`
	ScaleInStep    int        `json:"scale_in_step"`
	CooldownSec    int        `json:"cooldown_sec"`
	LastScaledAt   *time.Time `json:"last_scaled_at,omitempty"`
}

type ScalingGroupInstance struct {
	ScalingGroupID uuid.UUID `json:"scaling_group_id"`
	InstanceID     uuid.UUID `json:"instance_id"`
	JoinedAt       time.Time `json:"joined_at"`
}
