// Package services implements the core business logic.
package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	passwordvalidator "github.com/wagslane/go-password-validator"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxFailedAttempts = 5
	lockoutDuration   = 15 * time.Minute
)

type AuthService struct {
	userRepo       ports.UserRepository
	apiKeySvc      ports.IdentityService
	auditSvc       ports.AuditService
	failedAttempts map[string]int
	lockouts       map[string]time.Time
	mu             sync.Mutex
}

func NewAuthService(userRepo ports.UserRepository, apiKeySvc ports.IdentityService, auditSvc ports.AuditService) *AuthService {
	return &AuthService{
		userRepo:       userRepo,
		apiKeySvc:      apiKeySvc,
		auditSvc:       auditSvc,
		failedAttempts: make(map[string]int),
		lockouts:       make(map[string]time.Time),
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, name string) (*domain.User, error) {
	// Validate password strength
	if err := passwordvalidator.Validate(password, 50); err != nil {
		return nil, errors.New(errors.InvalidInput, "password is too weak: "+err.Error())
	}

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
		Role:         "user",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	_ = s.auditSvc.Log(ctx, user.ID, "user.register", "user", user.ID.String(), map[string]interface{}{
		"email": email,
	})

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	s.mu.Lock()
	if lockoutTime, ok := s.lockouts[email]; ok {
		if time.Now().Before(lockoutTime) {
			s.mu.Unlock()
			return nil, "", errors.New(errors.Unauthorized, "account is locked due to too many failed attempts")
		}
		delete(s.lockouts, email)
		delete(s.failedAttempts, email)
	}
	s.mu.Unlock()

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil || user == nil {
		s.incrementFailure(email)
		platform.AuthAttemptsTotal.WithLabelValues("failure_not_found").Inc()
		return nil, "", errors.New(errors.Unauthorized, "invalid email or password")
	}

	// Compare password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		s.incrementFailure(email)
		return nil, "", errors.New(errors.Unauthorized, "invalid email or password")
	}

	// Clear failures on success
	s.mu.Lock()
	delete(s.failedAttempts, email)
	s.mu.Unlock()

	// For MVP, we'll generate an initial API key upon login if they don't have one,
	// or just return a fresh one. In a real platform, login gives you a JWT and
	// you manage API keys separately.
	// For now, let's create a default key for them.
	key, err := s.apiKeySvc.CreateKey(ctx, user.ID, "Default Key")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create initial API key: %w", err)
	}

	_ = s.auditSvc.Log(ctx, user.ID, "user.login", "user", user.ID.String(), map[string]interface{}{})

	platform.AuthAttemptsTotal.WithLabelValues("success").Inc()

	return user, key.Key, nil
}

func (s *AuthService) incrementFailure(email string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedAttempts[email]++
	platform.AuthAttemptsTotal.WithLabelValues("failure_incorrect_password").Inc()
	if s.failedAttempts[email] >= maxFailedAttempts {
		s.lockouts[email] = time.Now().Add(lockoutDuration)
		platform.AuthAttemptsTotal.WithLabelValues("failure_lockout").Inc()
	}
}

func (s *AuthService) ValidateUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}
