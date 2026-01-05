package services

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/crypto"
)

type SecretService struct {
	repo      ports.SecretRepository
	eventSvc  ports.EventService
	auditSvc  ports.AuditService
	logger    *slog.Logger
	masterKey []byte
}

func NewSecretService(repo ports.SecretRepository, eventSvc ports.EventService, auditSvc ports.AuditService, logger *slog.Logger, masterKey string, environment string) *SecretService {
	if masterKey == "" {
		if environment == "production" {
			// In production, we MUST have a key
			logger.Error("SECRETS_ENCRYPTION_KEY is required in production but was not set")
			os.Exit(1)
		}
		// FALLBACK for development
		masterKey = "default-thecloud-development-key-32chars"
		logger.Warn("SECRETS_ENCRYPTION_KEY not set, using default key")
	}

	if len(masterKey) < 16 {
		logger.Warn("SECRETS_ENCRYPTION_KEY is too short, please use at least 16 characters for better security")
	}

	return &SecretService{
		repo:      repo,
		eventSvc:  eventSvc,
		auditSvc:  auditSvc,
		logger:    logger,
		masterKey: []byte(masterKey),
	}
}

func (s *SecretService) getDerivedKey(userID uuid.UUID) ([]byte, error) {
	return crypto.DeriveKey(s.masterKey, userID[:])
}

func (s *SecretService) CreateSecret(ctx context.Context, name, value, description string) (*domain.Secret, error) {
	userID := appcontext.UserIDFromContext(ctx)

	key, err := s.getDerivedKey(userID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to derive key", err)
	}

	encrypted, err := crypto.Encrypt([]byte(value), key)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to encrypt secret", err)
	}

	secret := &domain.Secret{
		ID:             uuid.New(),
		UserID:         userID,
		Name:           name,
		EncryptedValue: encrypted,
		Description:    description,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.repo.Create(ctx, secret); err != nil {
		return nil, err
	}

	_ = s.eventSvc.RecordEvent(ctx, "SECRET_CREATE", secret.ID.String(), "SECRET", map[string]interface{}{
		"name": name,
	})

	_ = s.auditSvc.Log(ctx, userID, "secret.create", "secret", secret.ID.String(), map[string]interface{}{
		"name": name,
	})

	// Redact value for response if needed, but here domain object has it.
	return secret, nil
}

func (s *SecretService) GetSecret(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	secret, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	key, err := s.getDerivedKey(secret.UserID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to derive key", err)
	}

	decrypted, err := crypto.Decrypt(secret.EncryptedValue, key)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to decrypt secret", err)
	}

	// Update last accessed
	now := time.Now()
	secret.LastAccessedAt = &now
	_ = s.repo.Update(ctx, secret)

	_ = s.eventSvc.RecordEvent(ctx, "SECRET_ACCESS", secret.ID.String(), "SECRET", map[string]interface{}{
		"name": secret.Name,
	})

	_ = s.auditSvc.Log(ctx, secret.UserID, "secret.access", "secret", secret.ID.String(), map[string]interface{}{
		"name": secret.Name,
	})

	secret.EncryptedValue = string(decrypted) // Re-use field for plaintext in response
	return secret, nil
}

func (s *SecretService) GetSecretByName(ctx context.Context, name string) (*domain.Secret, error) {
	secret, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	key, err := s.getDerivedKey(secret.UserID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to derive key", err)
	}

	decrypted, err := crypto.Decrypt(secret.EncryptedValue, key)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to decrypt secret", err)
	}

	now := time.Now()
	secret.LastAccessedAt = &now
	_ = s.repo.Update(ctx, secret)

	_ = s.eventSvc.RecordEvent(ctx, "SECRET_ACCESS", secret.ID.String(), "SECRET", map[string]interface{}{
		"name": secret.Name,
	})

	_ = s.auditSvc.Log(ctx, secret.UserID, "secret.access", "secret", secret.ID.String(), map[string]interface{}{
		"name": secret.Name,
	})

	secret.EncryptedValue = string(decrypted)
	return secret, nil
}

func (s *SecretService) ListSecrets(ctx context.Context) ([]*domain.Secret, error) {
	secrets, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	// For listing, we REDACT the encrypted values for security
	for _, sec := range secrets {
		sec.EncryptedValue = "[REDACTED]"
	}

	return secrets, nil
}

func (s *SecretService) DeleteSecret(ctx context.Context, id uuid.UUID) error {
	secret, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "SECRET_DELETE", id.String(), "SECRET", map[string]interface{}{
		"name": secret.Name,
	})

	_ = s.auditSvc.Log(ctx, secret.UserID, "secret.delete", "secret", id.String(), map[string]interface{}{
		"name": secret.Name,
	})

	return nil
}
