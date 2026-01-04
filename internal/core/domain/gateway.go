package domain

import (
	"time"

	"github.com/google/uuid"
)

type GatewayRoute struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Name        string    `json:"name"`
	PathPrefix  string    `json:"path_prefix"` // e.g., /api/v1
	TargetURL   string    `json:"target_url"`  // e.g., http://my-instance:8080
	StripPrefix bool      `json:"strip_prefix"`
	RateLimit   int       `json:"rate_limit"` // req/sec
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
