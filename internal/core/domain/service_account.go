package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ServiceAccount represents a machine-to-machine identity.
type ServiceAccount struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Role        string    `json:"role"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ServiceAccountSecret stores hashed credentials for a service account.
type ServiceAccountSecret struct {
	ID               uuid.UUID  `json:"id"`
	ServiceAccountID uuid.UUID  `json:"service_account_id"`
	SecretHash       string     `json:"-"` // Never expose hash in JSON
	Name             string     `json:"name"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
}

// ServiceAccountWithSecret is returned on create/rotate - contains plaintext secret.
type ServiceAccountWithSecret struct {
	ServiceAccount
	PlainSecret string `json:"secret,omitempty"` // Only set on create/rotate
}

// ServiceAccountClaims are the JWT claims for a service account access token.
type ServiceAccountClaims struct {
	jwt.RegisteredClaims
	ServiceAccountID uuid.UUID `json:"sa_id"`
	TenantID         uuid.UUID `json:"tenant_id"`
	Role             string    `json:"role"`
	Scopes           []string  `json:"scopes,omitempty"`
}
