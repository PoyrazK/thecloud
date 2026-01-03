package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo  ports.UserRepository
	apiKeySvc ports.IdentityService
}

func NewAuthService(userRepo ports.UserRepository, apiKeySvc ports.IdentityService) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		apiKeySvc: apiKeySvc,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, name string) (*domain.User, error) {
	// Check if user already exists
	existing, _ := s.userRepo.GetByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New(errors.InvalidInput, "user with this email already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hashedPassword),
		Name:         name,
		Role:         domain.RoleUser,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", errors.New(errors.Unauthorized, "invalid email or password")
	}

	// Compare password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, "", errors.New(errors.Unauthorized, "invalid email or password")
	}

	// For MVP, we'll generate an initial API key upon login if they don't have one,
	// or just return a fresh one. In a real platform, login gives you a JWT and
	// you manage API keys separately.
	// For now, let's create a default key for them.
	key, err := s.apiKeySvc.CreateKey(ctx, user.ID, "Default Key")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create initial API key: %w", err)
	}

	return user, key.Key, nil
}

func (s *AuthService) ValidateUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

func (s *AuthService) UpdateUserRole(ctx context.Context, userID uuid.UUID, role string) (*domain.User, error) {
	if !domain.IsValidRole(role) {
		return nil, errors.New(errors.InvalidInput, "invalid role")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.Role = role
	user.UpdatedAt = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}
