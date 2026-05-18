// Package services implements core business workflows.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/crypto"
)

const (
	errFailedDeriveKey = "failed to derive key"
	MinMasterKeyLength = 16
)

// SecretServiceParams defines the dependencies for SecretService.
type SecretServiceParams struct {
	Repo        ports.SecretRepository
	RBACSvc     ports.RBACService
	EventSvc    ports.EventService
	AuditSvc    ports.AuditService
	Logger      *slog.Logger
	MasterKey   string
	Environment string
}

// SecretService manages encrypted secret storage.
type SecretService struct {
	repo      ports.SecretRepository
	rbacSvc   ports.RBACService
	eventSvc  ports.EventService
	auditSvc  ports.AuditService
	logger    *slog.Logger
	masterKey []byte
}

// NewSecretService constructs a SecretService and validates the master key.
func NewSecretService(params SecretServiceParams) (*SecretService, error) {
	// 1. Critical Validation (Pre-logging)
	if params.Logger == nil {
		return nil, errors.New(errors.InvalidInput, "logger is required")
	}
	if params.Repo == nil {
		return nil, errors.New(errors.InvalidInput, "repository is required")
	}
	if params.RBACSvc == nil {
		return nil, errors.New(errors.InvalidInput, "rbac service is required")
	}
	if params.EventSvc == nil {
		return nil, errors.New(errors.InvalidInput, "event service is required")
	}
	if params.AuditSvc == nil {
		return nil, errors.New(errors.InvalidInput, "audit service is required")
	}

	// 2. Configuration Validation
	masterKey := params.MasterKey
	if masterKey == "" {
		if params.Environment == "production" {
			// In production, we MUST have a key
			params.Logger.Error("SECRETS_ENCRYPTION_KEY is required in production but was not set")
			return nil, errors.New(errors.InvalidInput, "SECRETS_ENCRYPTION_KEY is required in production but was not set")
		}
		masterKey = "default-thecloud-development-key-32chars"
		params.Logger.Warn("SECRETS_ENCRYPTION_KEY not set, using default key")

	}

	if len(masterKey) < MinMasterKeyLength {
		params.Logger.Warn(fmt.Sprintf("SECRETS_ENCRYPTION_KEY is too short, please use at least %d characters for better security", MinMasterKeyLength))
	}

	return &SecretService{
		repo:      params.Repo,
		rbacSvc:   params.RBACSvc,
		eventSvc:  params.EventSvc,
		auditSvc:  params.AuditSvc,
		logger:    params.Logger,
		masterKey: []byte(masterKey),
	}, nil
}

func (s *SecretService) getDerivedKey(userID uuid.UUID) ([]byte, error) {
	return crypto.DeriveKey(s.masterKey, userID[:])
}

func (s *SecretService) CreateSecret(ctx context.Context, name, value, description string) (*domain.Secret, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSecretWrite, "*"); err != nil {
		return nil, err
	}

	key, err := s.getDerivedKey(userID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, errFailedDeriveKey, err)
	}

	encrypted, err := crypto.Encrypt([]byte(value), key)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to encrypt secret", err)
	}

	secret := &domain.Secret{
		ID:             uuid.New(),
		UserID:         userID,
		TenantID:       tenantID,
		Name:           name,
		EncryptedValue: encrypted,
		Description:    description,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now(),
	}

	if err := s.repo.Create(ctx, secret); err != nil {
		return nil, err
	}

	if err := s.eventSvc.RecordEvent(ctx, "SECRET_CREATE", secret.ID.String(), "SECRET", map[string]interface{}{
		"name": name,
	}); err != nil {
		s.logger.Warn("failed to record event for secret creation", "secret_id", secret.ID.String(), "error", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "secret.create", "secret", secret.ID.String(), map[string]interface{}{
		"name": name,
	}); err != nil {
		s.logger.Warn("failed to log audit event for secret creation", "secret_id", secret.ID.String(), "error", err)
	}

	// Redact value for response if needed, but here domain object has it.
	return secret, nil
}

func (s *SecretService) GetSecret(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSecretRead, id.String()); err != nil {
		return nil, err
	}

	secret, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if secret.TenantID != tenantID && secret.TenantID != uuid.Nil {
		return nil, errors.New(errors.NotFound, "secret not found")
	}

	key, err := s.getDerivedKey(secret.UserID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, errFailedDeriveKey, err)
	}

	decrypted, err := crypto.Decrypt(secret.EncryptedValue, key)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to decrypt secret", err)
	}

	// Update last accessed
	now := time.Now()
	secret.LastAccessedAt = &now
	_ = s.repo.Update(ctx, secret)

	if err := s.eventSvc.RecordEvent(ctx, "SECRET_ACCESS", secret.ID.String(), "SECRET", map[string]interface{}{
		"name": secret.Name,
	}); err != nil {
		s.logger.Warn("failed to record event for secret access", "secret_id", secret.ID.String(), "error", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "secret.access", "secret", secret.ID.String(), map[string]interface{}{
		"name": secret.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event for secret access", "secret_id", secret.ID.String(), "error", err)
	}

	secret.EncryptedValue = string(decrypted) // Re-use field for plaintext in response
	return secret, nil
}

func (s *SecretService) GetSecretByName(ctx context.Context, name string) (*domain.Secret, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSecretRead, name); err != nil {
		return nil, err
	}

	secret, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	if secret.TenantID != tenantID && secret.TenantID != uuid.Nil {
		return nil, errors.New(errors.NotFound, "secret not found")
	}

	key, err := s.getDerivedKey(secret.UserID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, errFailedDeriveKey, err)
	}

	decrypted, err := crypto.Decrypt(secret.EncryptedValue, key)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to decrypt secret", err)
	}

	now := time.Now()
	secret.LastAccessedAt = &now
	_ = s.repo.Update(ctx, secret)

	if err := s.eventSvc.RecordEvent(ctx, "SECRET_ACCESS", secret.ID.String(), "SECRET", map[string]interface{}{
		"name": secret.Name,
	}); err != nil {
		s.logger.Warn("failed to record event for secret access", "secret_id", secret.ID.String(), "error", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "secret.access", "secret", secret.ID.String(), map[string]interface{}{
		"name": secret.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event for secret access", "secret_id", secret.ID.String(), "error", err)
	}

	secret.EncryptedValue = string(decrypted)
	return secret, nil
}

func (s *SecretService) ListSecrets(ctx context.Context) ([]*domain.Secret, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSecretRead, "*"); err != nil {
		return nil, err
	}

	secrets, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	// Filter by tenant to enforce isolation
	var tenantSecrets []*domain.Secret
	for _, sec := range secrets {
		if sec.TenantID == tenantID || sec.TenantID == uuid.Nil {
			sec.EncryptedValue = "[REDACTED]"
			tenantSecrets = append(tenantSecrets, sec)
		}
	}

	return tenantSecrets, nil
}

func (s *SecretService) DeleteSecret(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSecretDelete, id.String()); err != nil {
		return err
	}

	secret, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if secret.TenantID != tenantID && secret.TenantID != uuid.Nil {
		return errors.New(errors.Forbidden, "cannot delete secret in another tenant")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	if err := s.eventSvc.RecordEvent(ctx, "SECRET_DELETE", id.String(), "SECRET", map[string]interface{}{
		"name": secret.Name,
	}); err != nil {
		s.logger.Warn("failed to record event for secret deletion", "secret_id", id.String(), "error", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "secret.delete", "secret", id.String(), map[string]interface{}{
		"name": secret.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event for secret deletion", "secret_id", id.String(), "error", err)
	}

	return nil
}

func (s *SecretService) Encrypt(ctx context.Context, userID uuid.UUID, plainText string) (string, error) {
	// Privileged operation, typically called by other services
	key, err := s.getDerivedKey(userID)
	if err != nil {
		return "", errors.Wrap(errors.Internal, errFailedDeriveKey, err)
	}

	encrypted, err := crypto.Encrypt([]byte(plainText), key)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to encrypt", err)
	}

	return encrypted, nil
}

func (s *SecretService) Decrypt(ctx context.Context, userID uuid.UUID, cipherText string) (string, error) {
	// Privileged operation, typically called by other services
	key, err := s.getDerivedKey(userID)
	if err != nil {
		return "", errors.Wrap(errors.Internal, errFailedDeriveKey, err)
	}

	decrypted, err := crypto.Decrypt(cipherText, key)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to decrypt", err)
	}

	return string(decrypted), nil
}
