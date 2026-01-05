package ports

import (
	"context"

	"github.com/poyrazk/thecloud/internal/core/domain"
)

type PasswordResetRepository interface {
	Create(ctx context.Context, token *domain.PasswordResetToken) error
	GetByTokenHash(ctx context.Context, hash string) (*domain.PasswordResetToken, error)
	MarkAsUsed(ctx context.Context, tokenID string) error
	DeleteExpired(ctx context.Context) error
}

type PasswordResetService interface {
	// RequestReset generates a token for the user and (mock) sends an email
	RequestReset(ctx context.Context, email string) error

	// ResetPassword verifies the token and updates the user's password
	ResetPassword(ctx context.Context, token, newPassword string) error
}
