package domain

import (
	"time"

	"github.com/google/uuid"
)

type VPC struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	NetworkID string    `json:"network_id"`
	CreatedAt time.Time `json:"created_at"`
}
