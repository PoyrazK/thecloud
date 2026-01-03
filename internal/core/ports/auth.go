package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type AuthService interface {
	Register(ctx context.Context, email, password, name string) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*domain.User, string, error) // Returns User and an initial API key
	ValidateUser(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	UpdateUserRole(ctx context.Context, userID uuid.UUID, role string) (*domain.User, error)
}

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}
