// Package services implements core business workflows.
package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	stdlib_errors "errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"golang.org/x/crypto/bcrypt"
)

const (
	tokenDuration = 1 * time.Hour
)

// PasswordResetService handles password reset token lifecycle and updates.
type PasswordResetService struct {
	repo     ports.PasswordResetRepository
	userRepo ports.UserRepository
	logger   *slog.Logger
	// In a real app, we'd inject an EmailService here
}

// NewPasswordResetService constructs a PasswordResetService with its dependencies.
func NewPasswordResetService(repo ports.PasswordResetRepository, userRepo ports.UserRepository, logger *slog.Logger) *PasswordResetService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PasswordResetService{
		repo:     repo,
		userRepo: userRepo,
		logger:   logger,
	}
}
func (s *PasswordResetService) RequestReset(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Mask user existence for security, but return other errors
		if errors.Is(err, errors.NotFound) {
			return nil
		}
		return err
	}

	// Generate secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// Hash token for storage
	hash := sha256.Sum256([]byte(token))
	hashStr := hex.EncodeToString(hash[:])

	resetToken := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hashStr,
		ExpiresAt: time.Now().Add(tokenDuration),
		Used:      false,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, resetToken); err != nil {
		return err
	}

	// TODO: deliver `token` via an injected EmailService once available.
	// Never log or persist the plaintext token — its only safe destination is the user.
	s.logger.Info("password reset token issued", "user_id", user.ID, "token_id", resetToken.ID)

	return nil
}

func (s *PasswordResetService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Hash the incoming token to look it up
	hash := sha256.Sum256([]byte(token))
	hashStr := hex.EncodeToString(hash[:])

	resetToken, err := s.repo.GetByTokenHash(ctx, hashStr)
	if err != nil {
		return stdlib_errors.New("invalid or expired token")
	}

	if resetToken.Used {
		return stdlib_errors.New("token already used")
	}

	if time.Now().After(resetToken.ExpiresAt) {
		return stdlib_errors.New("token expired")
	}

	// Update User Password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user, err := s.userRepo.GetByID(ctx, resetToken.UserID)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Mark token as used
	return s.repo.MarkAsUsed(ctx, resetToken.ID.String())
}
