// Package services implements core business workflows.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/redis/go-redis/v9"
)

type cachedIdentityService struct {
	base   ports.IdentityService
	redis  *redis.Client
	logger *slog.Logger
	ttl    time.Duration
}

// NewCachedIdentityService wraps an IdentityService with a redis-backed cache.
func NewCachedIdentityService(base ports.IdentityService, redis *redis.Client, logger *slog.Logger) ports.IdentityService {
	if redis == nil {
		return base
	}
	return &cachedIdentityService{
		base:   base,
		redis:  redis,
		logger: logger,
		ttl:    5 * time.Minute,
	}
}
func (s *cachedIdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	return s.base.CreateKey(ctx, userID, name)
}

func (s *cachedIdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	keyHash := computeKeyHash(key)
	cacheKey := fmt.Sprintf("apikey:hash:%s", keyHash)

	val, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var apiKey domain.APIKey
		if err := json.Unmarshal([]byte(val), &apiKey); err == nil {
			return &apiKey, nil
		}
	}

	apiKey, err := s.base.ValidateAPIKey(ctx, key)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(apiKey); err == nil {
		s.redis.Set(ctx, cacheKey, data, s.ttl)
	}

	return apiKey, nil
}

func (s *cachedIdentityService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	return s.base.ListKeys(ctx, userID)
}

func (s *cachedIdentityService) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	return s.base.GetAPIKeyByID(ctx, id)
}

func (s *cachedIdentityService) RevokeKey(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	key, err := s.base.GetAPIKeyByID(ctx, id)
	if err != nil {
		return err
	}
	keyHash := computeKeyHash(key.Key)

	if err := s.base.RevokeKey(ctx, userID, id); err != nil {
		return err
	}

	if err := s.redis.Del(ctx, fmt.Sprintf("apikey:hash:%s", keyHash)).Err(); err != nil {
		s.logger.Warn("failed to delete API key from cache",
			"keyHash", keyHash, "userID", userID, "id", id, "error", err)
	}
	return nil
}

func (s *cachedIdentityService) RotateKey(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.APIKey, error) {
	oldKey, err := s.base.GetAPIKeyByID(ctx, id)
	if err != nil {
		return nil, err
	}
	oldHash := computeKeyHash(oldKey.Key)

	newKey, err := s.base.RotateKey(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	if err := s.redis.Del(ctx, fmt.Sprintf("apikey:hash:%s", oldHash)).Err(); err != nil {
		s.logger.Warn("failed to delete old API key hash from cache",
			"keyHash", oldHash, "userID", userID, "id", id, "error", err)
	}
	return newKey, nil
}

func (s *cachedIdentityService) CreateServiceAccount(ctx context.Context, tenantID uuid.UUID, name, role string) (*domain.ServiceAccountWithSecret, error) {
	return s.base.CreateServiceAccount(ctx, tenantID, name, role)
}

func (s *cachedIdentityService) GetServiceAccount(ctx context.Context, id uuid.UUID) (*domain.ServiceAccount, error) {
	return s.base.GetServiceAccount(ctx, id)
}

func (s *cachedIdentityService) ListServiceAccounts(ctx context.Context, tenantID uuid.UUID) ([]*domain.ServiceAccount, error) {
	return s.base.ListServiceAccounts(ctx, tenantID)
}

func (s *cachedIdentityService) UpdateServiceAccount(ctx context.Context, sa *domain.ServiceAccount) error {
	return s.base.UpdateServiceAccount(ctx, sa)
}

func (s *cachedIdentityService) DeleteServiceAccount(ctx context.Context, id uuid.UUID) error {
	return s.base.DeleteServiceAccount(ctx, id)
}

func (s *cachedIdentityService) ValidateClientCredentials(ctx context.Context, clientID, clientSecret string) (string, error) {
	return s.base.ValidateClientCredentials(ctx, clientID, clientSecret)
}

func (s *cachedIdentityService) ValidateAccessToken(ctx context.Context, token string) (*domain.ServiceAccountClaims, error) {
	return s.base.ValidateAccessToken(ctx, token)
}

func (s *cachedIdentityService) RotateServiceAccountSecret(ctx context.Context, saID uuid.UUID) (string, error) {
	secret, err := s.base.RotateServiceAccountSecret(ctx, saID)
	if err == nil {
		s.redis.Del(ctx, fmt.Sprintf("sa:auth:%s", saID))
	}
	return secret, err
}

func (s *cachedIdentityService) RevokeServiceAccountSecret(ctx context.Context, saID uuid.UUID, secretID uuid.UUID) error {
	err := s.base.RevokeServiceAccountSecret(ctx, saID, secretID)
	if err == nil {
		s.redis.Del(ctx, fmt.Sprintf("sa:auth:%s", saID))
	}
	return err
}

func (s *cachedIdentityService) ListServiceAccountSecrets(ctx context.Context, saID uuid.UUID) ([]*domain.ServiceAccountSecret, error) {
	return s.base.ListServiceAccountSecrets(ctx, saID)
}

func (s *cachedIdentityService) TokenTTL() time.Duration {
	return s.base.TokenTTL()
}
