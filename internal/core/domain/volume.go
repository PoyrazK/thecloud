package domain

import (
	"time"

	"github.com/google/uuid"
)

type VolumeStatus string

const (
	VolumeStatusAvailable VolumeStatus = "AVAILABLE"
	VolumeStatusInUse     VolumeStatus = "IN-USE"
	VolumeStatusDeleting  VolumeStatus = "DELETING"
)

type Volume struct {
	ID         uuid.UUID    `json:"id"`
	Name       string       `json:"name"`
	SizeGB     int          `json:"size_gb"`
	Status     VolumeStatus `json:"status"`
	InstanceID *uuid.UUID   `json:"instance_id,omitempty"`
	MountPath  string       `json:"mount_path,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}
