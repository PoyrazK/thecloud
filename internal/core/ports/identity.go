package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type IdentityRepository interface {
	CreateAPIKey(ctx context.Context, apiKey *domain.APIKey) error
	GetAPIKeyByKey(ctx context.Context, key string) (*domain.APIKey, error)
	// list, delete etc can be added later
}

type IdentityService interface {
	CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error)
	ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error)
}
