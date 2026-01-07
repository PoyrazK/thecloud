package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
)

type IdentityService struct {
	repo     ports.IdentityRepository
	auditSvc ports.AuditService
}

func NewIdentityService(repo ports.IdentityRepository, auditSvc ports.AuditService) *IdentityService {
	return &IdentityService{repo: repo, auditSvc: auditSvc}
}

func (s *IdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	// Generate a secure random key
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to generate random key", err)
	}
	keyStr := "thecloud_" + hex.EncodeToString(b)

	key := &domain.APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		Key:       keyStr,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateAPIKey(ctx, key); err != nil {
		return nil, err
	}

	platform.APIKeysActive.Inc()

	// Log audit event
	_ = s.auditSvc.Log(ctx, userID, "api_key.create", "api_key", key.ID.String(), map[string]interface{}{
		"name": name,
	})

	return key, nil
}

func (s *IdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	apiKey, err := s.repo.GetAPIKeyByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	return apiKey, nil
}

func (s *IdentityService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	return s.repo.ListAPIKeysByUserID(ctx, userID)
}

func (s *IdentityService) RevokeKey(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	key, err := s.repo.GetAPIKeyByID(ctx, id)
	if err != nil {
		return err
	}

	if key.UserID != userID {
		return errors.New(errors.Forbidden, "cannot revoke key owned by another user")
	}

	if err := s.repo.DeleteAPIKey(ctx, id); err != nil {
		return err
	}

	platform.APIKeysActive.Dec()

	// Log audit event
	_ = s.auditSvc.Log(ctx, userID, "api_key.revoke", "api_key", id.String(), map[string]interface{}{
		"name": key.Name,
	})

	return nil
}

func (s *IdentityService) RotateKey(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*domain.APIKey, error) {
	key, err := s.repo.GetAPIKeyByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if key.UserID != userID {
		return nil, errors.New(errors.Forbidden, "cannot rotate key owned by another user")
	}

	// Create new key
	newKey, err := s.CreateKey(ctx, userID, key.Name+" (rotated)")
	if err != nil {
		return nil, err
	}

	// Delete old key
	if err := s.repo.DeleteAPIKey(ctx, id); err != nil {
		// Log error but we already have a new key
		return newKey, nil
	}

	// Log audit event
	_ = s.auditSvc.Log(ctx, userID, "api_key.rotate", "api_key", id.String(), map[string]interface{}{
		"name":   key.Name,
		"new_id": newKey.ID.String(),
	})

	return newKey, nil
}
