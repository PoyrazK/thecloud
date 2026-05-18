// Package ports defines service and repository interfaces.
package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// IdentityRepository manages the persistent storage of programmatic credentials (API keys).
type IdentityRepository interface {
	// CreateAPIKey saves a new API key record.
	CreateAPIKey(ctx context.Context, apiKey *domain.APIKey) error
	// GetAPIKeyByHash retrieves a key's metadata based on its hash.
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*domain.APIKey, error)
	// GetAPIKeyByID retrieves key metadata by its unique UUID.
	GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error)
	// ListAPIKeysByUserID returns all active and revoked keys for a specific user.
	ListAPIKeysByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error)
	// DeleteAPIKey removes an API key record from storage.
	DeleteAPIKey(ctx context.Context, id uuid.UUID) error
}

// ServiceAccountRepository manages service account persistence.
type ServiceAccountRepository interface {
	Create(ctx context.Context, sa *domain.ServiceAccount) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.ServiceAccount, error)
	GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*domain.ServiceAccount, error)
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]*domain.ServiceAccount, error)
	Update(ctx context.Context, sa *domain.ServiceAccount) error
	Delete(ctx context.Context, id uuid.UUID) error
	CreateSecret(ctx context.Context, secret *domain.ServiceAccountSecret) error
	GetSecretByHash(ctx context.Context, secretHash string) (*domain.ServiceAccountSecret, error)
	ListSecretsByServiceAccount(ctx context.Context, saID uuid.UUID) ([]*domain.ServiceAccountSecret, error)
	UpdateSecretLastUsed(ctx context.Context, secretID uuid.UUID) error
	DeleteSecret(ctx context.Context, secretID uuid.UUID) error
	DeleteAllSecrets(ctx context.Context, saID uuid.UUID) error
}

// IdentityService provides business logic for managing programmatic access and multi-user authentication.
type IdentityService interface {
	// CreateKey generates a new unique API key for a user.
	CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error)
	// ValidateAPIKey authenticates a request using a raw API key string.
	ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error)
	// GetAPIKeyByID retrieves a key's metadata by its unique UUID.
	GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error)
	// ListKeys returns all keys associated with an authorized user.
	ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error)
	// RevokeKey immediately invalidates an API key.
	RevokeKey(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	// RotateKey invalidates an existing key and generates a replacement.
	RotateKey(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.APIKey, error)

	// Service Account methods
	// CreateServiceAccount creates a new service account with initial credentials.
	CreateServiceAccount(ctx context.Context, tenantID uuid.UUID, name, role string) (*domain.ServiceAccountWithSecret, error)
	// GetServiceAccount retrieves a service account by ID.
	GetServiceAccount(ctx context.Context, id uuid.UUID) (*domain.ServiceAccount, error)
	// ListServiceAccounts returns all service accounts for a tenant.
	ListServiceAccounts(ctx context.Context, tenantID uuid.UUID) ([]*domain.ServiceAccount, error)
	// UpdateServiceAccount updates a service account.
	UpdateServiceAccount(ctx context.Context, sa *domain.ServiceAccount) error
	// DeleteServiceAccount removes a service account and all its secrets.
	DeleteServiceAccount(ctx context.Context, id uuid.UUID) error

	// ValidateClientCredentials exchanges client credentials for a JWT access token.
	ValidateClientCredentials(ctx context.Context, clientID, clientSecret string) (string, error)
	// ValidateAccessToken validates a Bearer JWT and returns claims.
	ValidateAccessToken(ctx context.Context, token string) (*domain.ServiceAccountClaims, error)

	// RotateServiceAccountSecret rotates the secret, returns new plaintext.
	RotateServiceAccountSecret(ctx context.Context, saID uuid.UUID) (string, error)
	// RevokeServiceAccountSecret invalidates a secret.
	RevokeServiceAccountSecret(ctx context.Context, saID uuid.UUID, secretID uuid.UUID) error
	// ListServiceAccountSecrets returns all secrets for a service account.
	ListServiceAccountSecrets(ctx context.Context, saID uuid.UUID) ([]*domain.ServiceAccountSecret, error)
	// TokenTTL returns the configured service account token TTL.
	TokenTTL() time.Duration
}
