package domain

import (
	"time"

	"github.com/google/uuid"
)

type InstanceStatus string

const (
	StatusStarting InstanceStatus = "STARTING"
	StatusRunning  InstanceStatus = "RUNNING"
	StatusStopped  InstanceStatus = "STOPPED"
	StatusError    InstanceStatus = "ERROR"
	StatusDeleted  InstanceStatus = "DELETED"
)

const (
	MinPort             = 1
	MaxPort             = 65535
	MaxPortsPerInstance = 10
)

type Instance struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Image       string         `json:"image"`
	ContainerID string         `json:"container_id,omitempty"`
	Status      InstanceStatus `json:"status"`
	Ports       string         `json:"ports,omitempty"`
	VpcID       *uuid.UUID     `json:"vpc_id,omitempty"`
	Version     int            `json:"version"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type InstanceStats struct {
	CPUPercentage    float64 `json:"cpu_percentage"`
	MemoryUsageBytes float64 `json:"memory_usage_bytes"`
	MemoryLimitBytes float64 `json:"memory_limit_bytes"`
	MemoryPercentage float64 `json:"memory_percentage"`
}
