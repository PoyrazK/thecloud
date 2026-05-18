// Package services implements core business workflows.
package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	stdlib_errors "errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
)

// serverSecret is used as HMAC key to prevent rainbow table attacks on API key hashes.
var serverSecret = getServerSecret()

func getServerSecret() string {
	secret := platform.GetSecretsEncryptionKey()
	if secret != "" {
		return secret
	}
	// For tests, allow a fallback to avoid os.Exit in package init
	if os.Getenv("TEST_SECRETS") != "" {
		return os.Getenv("TEST_SECRETS")
	}
	slog.Default().Error("SECRETS_ENCRYPTION_KEY environment variable is required")
	os.Exit(1)
	return "" // unreachable but satisfies compiler
}

// computeKeyHash creates a HMAC-SHA256 hash of the API key using the server secret.
func computeKeyHash(key string) string {
	//nolint:codeql // HMAC-SHA256 is used for key fingerprinting, not password hashing.
	h := hmac.New(sha256.New, []byte(serverSecret))
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

// ServiceAccountRepository interface for service account persistence.
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

// IdentityServiceParams defines the dependencies for IdentityService.
type IdentityServiceParams struct {
	Repo     ports.IdentityRepository
	SARepo   ServiceAccountRepository
	RbacSvc  ports.RBACService
	AuditSvc ports.AuditService
	Logger   *slog.Logger
	// TokenTTL is the lifetime of service account JWTs. Defaults to 1 hour.
	TokenTTL time.Duration
}

// IdentityService manages API key lifecycle and validation.
type IdentityService struct {
	repo     ports.IdentityRepository
	saRepo   ServiceAccountRepository
	rbacSvc  ports.RBACService
	auditSvc ports.AuditService
	logger   *slog.Logger
	tokenTTL time.Duration
}

// NewIdentityService constructs an IdentityService with its dependencies.
func NewIdentityService(params IdentityServiceParams) *IdentityService {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	tokenTTL := params.TokenTTL
	if tokenTTL == 0 {
		tokenTTL = time.Hour
	}
	return &IdentityService{
		repo:     params.Repo,
		saRepo:   params.SARepo,
		rbacSvc:  params.RbacSvc,
		auditSvc: params.AuditSvc,
		logger:   logger,
		tokenTTL: tokenTTL,
	}
}

func (s *IdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	uID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if uID != uuid.Nil && uID != userID {
		if err := s.rbacSvc.Authorize(ctx, uID, tenantID, domain.PermissionIdentityCreate, "*"); err != nil {
			return nil, err
		}
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to generate secure key", err)
	}
	keyStr := "thecloud_" + hex.EncodeToString(b)

	apiKey := &domain.APIKey{
		ID:              uuid.New(),
		UserID:          userID,
		Key:             keyStr,
		KeyHash:         computeKeyHash(keyStr),
		Name:            name,
		CreatedAt:       time.Now(),
		TenantID:        tenantID,
		DefaultTenantID: nil,
	}
	if tenantID != uuid.Nil {
		apiKey.DefaultTenantID = &tenantID
	}

	if err := s.repo.CreateAPIKey(ctx, apiKey); err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, userID, "api_key.create", "api_key", apiKey.ID.String(), map[string]interface{}{
		"name": name,
	}); err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to log audit event", "action", "api_key.create", "resource_id", apiKey.ID.String(), "error", err)
		}
	}

	return apiKey, nil
}

func (s *IdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	keyHash := computeKeyHash(key)
	apiKey, err := s.repo.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, errors.New(errors.Unauthorized, "invalid api key")
	}

	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, errors.New(errors.Unauthorized, "api key has expired")
	}

	platform.AuthAttemptsTotal.WithLabelValues("success_api_key").Inc()

	return apiKey, nil
}

func (s *IdentityService) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	return s.repo.GetAPIKeyByID(ctx, id)
}

func (s *IdentityService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	uID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, uID, tenantID, domain.PermissionIdentityRead, "*"); err != nil {
		return nil, err
	}

	if userID != uID {
		if err := s.rbacSvc.Authorize(ctx, uID, tenantID, domain.PermissionIdentityReadAll, "*"); err != nil {
			return nil, errors.New(errors.Forbidden, "cannot list keys for another user")
		}
	}

	return s.repo.ListAPIKeysByUserID(ctx, userID)
}

func (s *IdentityService) RevokeKey(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	uID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	isSelf := uID == userID
	authErr := s.rbacSvc.Authorize(ctx, uID, tenantID, domain.PermissionIdentityDelete, id.String())
	if authErr != nil && !isSelf {
		return authErr
	}

	key, err := s.repo.GetAPIKeyByID(ctx, id)
	if err != nil {
		return err
	}

	if authErr != nil && isSelf && key.UserID != uID {
		return errors.New(errors.Forbidden, "unauthorized access to api key")
	}

	if err := s.repo.DeleteAPIKey(ctx, id); err != nil {
		return err
	}

	if auditErr := s.auditSvc.Log(ctx, uID, "api_key.revoke", "api_key", id.String(), map[string]interface{}{
		"name": key.Name,
	}); auditErr != nil {
		s.logger.Error("failed to log audit event for api key revocation", "user_id", uID, "key_id", id, "error", auditErr)
	}

	return nil
}

func (s *IdentityService) RotateKey(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.APIKey, error) {
	uID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	isSelf := uID == userID
	authErr := s.rbacSvc.Authorize(ctx, uID, tenantID, domain.PermissionIdentityDelete, id.String())
	if authErr != nil && !isSelf {
		return nil, authErr
	}

	key, err := s.repo.GetAPIKeyByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if authErr != nil && isSelf && key.UserID != uID {
		return nil, errors.New(errors.Forbidden, "unauthorized access to api key")
	}

	newKey, err := s.CreateKey(ctx, userID, key.Name)
	if err != nil {
		return nil, err
	}

	if err := s.repo.DeleteAPIKey(ctx, id); err != nil {
		s.logger.Error("failed to delete old api key during rotation, rolling back new key", "id", id, "error", err)
		if rbErr := s.repo.DeleteAPIKey(ctx, newKey.ID); rbErr != nil {
			s.logger.Error("failed to rollback new api key after rotation failure", "new_key_id", newKey.ID, "error", rbErr)
			return nil, errors.Wrap(errors.Internal, "rotation failed and rollback failed", err)
		}
		return nil, errors.Wrap(errors.Internal, "failed to delete old key, rotation rolled back", err)
	}

	if auditErr := s.auditSvc.Log(ctx, uID, "api_key.rotate", "api_key", id.String(), map[string]interface{}{
		"name":   key.Name,
		"new_id": newKey.ID.String(),
	}); auditErr != nil {
		s.logger.Error("failed to log audit event for api key rotation", "user_id", uID, "key_id", id, "error", auditErr)
	}

	return newKey, nil
}

// generateSAToken generates a JWT for a service account.
func (s *IdentityService) generateSAToken(saID, tenantID uuid.UUID, role string) (string, error) {
	claims := domain.ServiceAccountClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "thecloud",
			Subject:   saID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
		ServiceAccountID: saID,
		TenantID:         tenantID,
		Role:             role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(serverSecret))
}

// validateSAToken validates a service account JWT and returns claims.
func validateSAToken(tokenString string) (*domain.ServiceAccountClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &domain.ServiceAccountClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(serverSecret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*domain.ServiceAccountClaims)
	if !ok || !token.Valid {
		return nil, stdlib_errors.New("invalid token")
	}

	return claims, nil
}

// CreateServiceAccount creates a new service account with credentials.
func (s *IdentityService) CreateServiceAccount(ctx context.Context, tenantID uuid.UUID, name, role string) (*domain.ServiceAccountWithSecret, error) {
	uID := appcontext.UserIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, uID, tenantID, domain.PermissionServiceAccountCreate, "*"); err != nil {
		return nil, err
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to generate secret", err)
	}
	secretStr := "sa_" + hex.EncodeToString(secretBytes)

	sa := &domain.ServiceAccount{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      name,
		Role:      role,
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.saRepo.Create(ctx, sa); err != nil {
		return nil, err
	}

	saSecret := &domain.ServiceAccountSecret{
		ID:               uuid.New(),
		ServiceAccountID: sa.ID,
		SecretHash:       computeKeyHash(secretStr),
		Name:             "default",
		CreatedAt:        time.Now(),
	}

	if err := s.saRepo.CreateSecret(ctx, saSecret); err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, uID, "service_account.create", "service_account", sa.ID.String(), map[string]interface{}{
		"name": name,
		"role": role,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "service_account.create", "error", err)
	}

	return &domain.ServiceAccountWithSecret{
		ServiceAccount: *sa,
		PlainSecret:    secretStr,
	}, nil
}

// GetServiceAccount retrieves a service account by ID.
func (s *IdentityService) GetServiceAccount(ctx context.Context, id uuid.UUID) (*domain.ServiceAccount, error) {
	return s.saRepo.GetByID(ctx, id)
}

// ListServiceAccounts returns all service accounts for a tenant.
func (s *IdentityService) ListServiceAccounts(ctx context.Context, tenantID uuid.UUID) ([]*domain.ServiceAccount, error) {
	return s.saRepo.ListByTenant(ctx, tenantID)
}

// UpdateServiceAccount updates a service account.
func (s *IdentityService) UpdateServiceAccount(ctx context.Context, sa *domain.ServiceAccount) error {
	uID := appcontext.UserIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, uID, sa.TenantID, domain.PermissionServiceAccountUpdate, "*"); err != nil {
		return err
	}
	return s.saRepo.Update(ctx, sa)
}

// DeleteServiceAccount removes a service account and all its secrets.
func (s *IdentityService) DeleteServiceAccount(ctx context.Context, id uuid.UUID) error {
	uID := appcontext.UserIDFromContext(ctx)

	sa, err := s.saRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.rbacSvc.Authorize(ctx, uID, sa.TenantID, domain.PermissionServiceAccountDelete, "*"); err != nil {
		return err
	}

	if err := s.saRepo.DeleteAllSecrets(ctx, id); err != nil {
		return err
	}

	return s.saRepo.Delete(ctx, id)
}

// ValidateClientCredentials exchanges client credentials for a JWT.
func (s *IdentityService) ValidateClientCredentials(ctx context.Context, clientID, clientSecret string) (string, error) {
	saID, err := uuid.Parse(clientID)
	if err != nil {
		return "", errors.New(errors.Unauthorized, "invalid client credentials")
	}

	secretHash := computeKeyHash(clientSecret)
	secret, err := s.saRepo.GetSecretByHash(ctx, secretHash)
	if err != nil {
		platform.AuthAttemptsTotal.WithLabelValues("fail_service_account").Inc()
		return "", errors.New(errors.Unauthorized, "invalid client credentials")
	}

	if secret.ServiceAccountID != saID {
		platform.AuthAttemptsTotal.WithLabelValues("fail_service_account").Inc()
		return "", errors.New(errors.Unauthorized, "invalid client credentials")
	}

	sa, err := s.saRepo.GetByID(ctx, saID)
	if err != nil {
		return "", errors.New(errors.Unauthorized, "invalid client credentials")
	}
	if !sa.Enabled {
		return "", errors.New(errors.Unauthorized, "service account is disabled")
	}

	if secret.ExpiresAt != nil && time.Now().After(*secret.ExpiresAt) {
		return "", errors.New(errors.Unauthorized, "client secret has expired")
	}

	if err := s.saRepo.UpdateSecretLastUsed(ctx, secret.ID); err != nil {
		s.logger.Warn("failed to update secret last used", "error", err)
	}

	platform.AuthAttemptsTotal.WithLabelValues("success_service_account").Inc()

	token, err := s.generateSAToken(saID, sa.TenantID, sa.Role)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to generate token", err)
	}

	return token, nil
}

// ValidateAccessToken validates a Bearer JWT and returns claims.
//
// SECURITY NOTE: This validates that the SA has at least one active (non-expired) secret
// at the time of this call. However, this does not revoke JWTs that were issued when the SA
// had valid secrets. A JWT remains valid until its natural expiry (tokenTTL).
// If a secret is revoked, the next ValidateAccessToken call will fail because the SA will
// have no active secrets — but previously-issued JWTs remain valid until exp.
//
// This is a fundamental trade-off of stateless JWT design. For immediate revocation,
// a revocation list or shorter tokenTTL is required.
func (s *IdentityService) ValidateAccessToken(ctx context.Context, tokenString string) (*domain.ServiceAccountClaims, error) {
	claims, err := validateSAToken(tokenString)
	if err != nil {
		return nil, errors.New(errors.Unauthorized, "invalid access token")
	}

	sa, err := s.saRepo.GetByID(ctx, claims.ServiceAccountID)
	if err != nil {
		return nil, errors.New(errors.Unauthorized, "service account not found")
	}
	if !sa.Enabled {
		return nil, errors.New(errors.Unauthorized, "service account is disabled")
	}

	secrets, err := s.saRepo.ListSecretsByServiceAccount(ctx, claims.ServiceAccountID)
	if err != nil {
		return nil, errors.New(errors.Unauthorized, "service account not found")
	}
	hasValidSecret := false
	for _, sec := range secrets {
		if sec.ExpiresAt == nil || time.Now().Before(*sec.ExpiresAt) {
			hasValidSecret = true
			break
		}
	}
	if !hasValidSecret {
		return nil, errors.New(errors.Unauthorized, "service account has no active secrets")
	}

	return claims, nil
}

// RotateServiceAccountSecret rotates the secret, returns new plaintext.
func (s *IdentityService) RotateServiceAccountSecret(ctx context.Context, saID uuid.UUID) (string, error) {
	uID := appcontext.UserIDFromContext(ctx)

	sa, err := s.saRepo.GetByID(ctx, saID)
	if err != nil {
		return "", err
	}

	if err := s.rbacSvc.Authorize(ctx, uID, sa.TenantID, domain.PermissionServiceAccountUpdate, "*"); err != nil {
		return "", err
	}

	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", errors.Wrap(errors.Internal, "failed to generate secret", err)
	}
	secretStr := "sa_" + hex.EncodeToString(secretBytes)

	if err := s.saRepo.DeleteAllSecrets(ctx, saID); err != nil {
		return "", err
	}

	saSecret := &domain.ServiceAccountSecret{
		ID:               uuid.New(),
		ServiceAccountID: saID,
		SecretHash:       computeKeyHash(secretStr),
		Name:             "default",
		CreatedAt:        time.Now(),
	}

	if err := s.saRepo.CreateSecret(ctx, saSecret); err != nil {
		return "", err
	}

	if err := s.auditSvc.Log(ctx, uID, "service_account.rotate_secret", "service_account", saID.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	return secretStr, nil
}

// RevokeServiceAccountSecret invalidates a secret.
func (s *IdentityService) RevokeServiceAccountSecret(ctx context.Context, saID uuid.UUID, secretID uuid.UUID) error {
	uID := appcontext.UserIDFromContext(ctx)

	sa, err := s.saRepo.GetByID(ctx, saID)
	if err != nil {
		return err
	}

	if err := s.rbacSvc.Authorize(ctx, uID, sa.TenantID, domain.PermissionServiceAccountUpdate, "*"); err != nil {
		return err
	}

	return s.saRepo.DeleteSecret(ctx, secretID)
}

// ListServiceAccountSecrets returns all secrets for a service account.
func (s *IdentityService) ListServiceAccountSecrets(ctx context.Context, saID uuid.UUID) ([]*domain.ServiceAccountSecret, error) {
	uID := appcontext.UserIDFromContext(ctx)

	sa, err := s.saRepo.GetByID(ctx, saID)
	if err != nil {
		return nil, err
	}

	if err := s.rbacSvc.Authorize(ctx, uID, sa.TenantID, domain.PermissionServiceAccountRead, "*"); err != nil {
		return nil, err
	}

	return s.saRepo.ListSecretsByServiceAccount(ctx, saID)
}

// TokenTTL returns the configured service account token TTL.
func (s *IdentityService) TokenTTL() time.Duration {
	return s.tokenTTL
}
