package domain

import (
	"time"

	"github.com/google/uuid"
)

type SnapshotStatus string

const (
	SnapshotStatusCreating  SnapshotStatus = "CREATING"
	SnapshotStatusAvailable SnapshotStatus = "AVAILABLE"
	SnapshotStatusDeleting  SnapshotStatus = "DELETING"
	SnapshotStatusError     SnapshotStatus = "ERROR"
)

type Snapshot struct {
	ID          uuid.UUID      `json:"id"`
	UserID      uuid.UUID      `json:"user_id"`
	VolumeID    uuid.UUID      `json:"volume_id"`
	VolumeName  string         `json:"volume_name"`
	SizeGB      int            `json:"size_gb"`
	Status      SnapshotStatus `json:"status"`
	Description string         `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}
