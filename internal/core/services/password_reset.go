package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"golang.org/x/crypto/bcrypt"
)

const (
	tokenDuration = 1 * time.Hour
)

type PasswordResetService struct {
	repo     ports.PasswordResetRepository
	userRepo ports.UserRepository
	// In a real app, we'd inject an EmailService here
}

func NewPasswordResetService(repo ports.PasswordResetRepository, userRepo ports.UserRepository) *PasswordResetService {
	return &PasswordResetService{
		repo:     repo,
		userRepo: userRepo,
	}
}

func (s *PasswordResetService) RequestReset(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Verify user existence but don't leak it to the caller (security best practice)
		// Return nil effectively masking the error, or a generic "if email exists..." message
		return nil
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
		CreatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, resetToken); err != nil {
		return err
	}

	// TODO: Integrate EmailService to send 'token' to 'email'
	// For MVP/Demo: Log the token so we can test it manually
	fmt.Printf(" [DEBUG] Password Reset Token for %s: %s\n", email, token)

	return nil
}

func (s *PasswordResetService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Hash the incoming token to look it up
	hash := sha256.Sum256([]byte(token))
	hashStr := hex.EncodeToString(hash[:])

	resetToken, err := s.repo.GetByTokenHash(ctx, hashStr)
	if err != nil {
		return errors.New("invalid or expired token")
	}

	if resetToken.Used {
		return errors.New("token already used")
	}

	if time.Now().After(resetToken.ExpiresAt) {
		return errors.New("token expired")
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
