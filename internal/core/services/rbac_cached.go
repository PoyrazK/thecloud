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
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/redis/go-redis/v9"
)

type cachedRBACService struct {
	rbac   ports.RBACService
	saRepo ServiceAccountRepository
	cache  *redis.Client
	logger *slog.Logger
	ttl    time.Duration
}

// NewCachedRBACService wraps an RBACService with a redis-backed cache.
func NewCachedRBACService(rbac ports.RBACService, saRepo ServiceAccountRepository, cache *redis.Client, logger *slog.Logger) ports.RBACService {
	return &cachedRBACService{
		rbac:   rbac,
		saRepo: saRepo,
		cache:  cache,
		logger: logger,
		ttl:    5 * time.Minute,
	}
}

func (s *cachedRBACService) Authorize(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	return s.rbac.Authorize(ctx, userID, tenantID, permission, resource)
}

func (s *cachedRBACService) HasPermission(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) (bool, error) {
	key := fmt.Sprintf("rbac:perm:%s:%s:%s:%s", tenantID, userID, permission, resource)

	// Try cache
	val, err := s.cache.Get(ctx, key).Result()
	if err == nil {
		return val == "1", nil
	}

	// Cache miss
	allowed, err := s.rbac.HasPermission(ctx, userID, tenantID, permission, resource)
	if err != nil {
		return false, err
	}

	// Store in cache
	cacheVal := "0"
	if allowed {
		cacheVal = "1"
	}
	s.cache.Set(ctx, key, cacheVal, s.ttl)

	return allowed, nil
}

func (s *cachedRBACService) CreateRole(ctx context.Context, role *domain.Role) error {
	return s.rbac.CreateRole(ctx, role)
}

func (s *cachedRBACService) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	key := fmt.Sprintf("rbac:role:id:%s", id)

	val, err := s.cache.Get(ctx, key).Result()
	if err == nil {
		var role domain.Role
		if err := json.Unmarshal([]byte(val), &role); err == nil {
			return &role, nil
		}
	}

	role, err := s.rbac.GetRoleByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if role != nil {
		data, _ := json.Marshal(role)
		s.cache.Set(ctx, key, string(data), s.ttl)
	}

	return role, nil
}

func (s *cachedRBACService) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	key := fmt.Sprintf("rbac:role:name:%s", name)

	val, err := s.cache.Get(ctx, key).Result()
	if err == nil {
		var role domain.Role
		if err := json.Unmarshal([]byte(val), &role); err == nil {
			return &role, nil
		}
	}

	role, err := s.rbac.GetRoleByName(ctx, name)
	if err != nil {
		return nil, err
	}

	if role != nil {
		data, _ := json.Marshal(role)
		s.cache.Set(ctx, key, string(data), s.ttl)
	}

	return role, nil
}

func (s *cachedRBACService) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	return s.rbac.ListRoles(ctx)
}

func (s *cachedRBACService) UpdateRole(ctx context.Context, role *domain.Role) error {
	err := s.rbac.UpdateRole(ctx, role)
	if err == nil {
		s.invalidateRoleCache(ctx, role.ID, role.Name)
	}
	return err
}

func (s *cachedRBACService) DeleteRole(ctx context.Context, id uuid.UUID) error {
	role, _ := s.rbac.GetRoleByID(ctx, id)
	err := s.rbac.DeleteRole(ctx, id)
	if err == nil && role != nil {
		s.invalidateRoleCache(ctx, id, role.Name)
	}
	return err
}

func (s *cachedRBACService) AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	err := s.rbac.AddPermissionToRole(ctx, roleID, permission)
	if err == nil {
		role, _ := s.rbac.GetRoleByID(ctx, roleID)
		if role != nil {
			s.invalidateRoleCache(ctx, roleID, role.Name)
		}
	}
	return err
}

func (s *cachedRBACService) RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	err := s.rbac.RemovePermissionFromRole(ctx, roleID, permission)
	if err == nil {
		role, _ := s.rbac.GetRoleByID(ctx, roleID)
		if role != nil {
			s.invalidateRoleCache(ctx, roleID, role.Name)
		}
	}
	return err
}

func (s *cachedRBACService) BindRole(ctx context.Context, userIdentifier, roleName string) error {
	err := s.rbac.BindRole(ctx, userIdentifier, roleName)
	if err == nil {
		// Invalidate user permission cache
		// Since we don't know the exact userID if email was used, we'd need to fetch it first or use a pattern delete
		// For simplicity, let's just clear the rbac perms pattern
		s.cache.Del(ctx, "rbac:perm:*") // In a real system, use more targeted invalidation
	}
	return err
}

func (s *cachedRBACService) ListRoleBindings(ctx context.Context) ([]*domain.User, error) {
	return s.rbac.ListRoleBindings(ctx)
}

func (s *cachedRBACService) EvaluatePolicy(ctx context.Context, userID uuid.UUID, action string, resource string, evalCtx map[string]interface{}) (bool, error) {
	return s.rbac.EvaluatePolicy(ctx, userID, action, resource, evalCtx)
}

func (s *cachedRBACService) AuthorizeServiceAccount(ctx context.Context, saID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	cacheKey := fmt.Sprintf("sa:auth:%s", saID)

	// Try cache hit first — cache stores full SA object as JSON
	val, err := s.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		var sa domain.ServiceAccount
		if json.Unmarshal([]byte(val), &sa) == nil {
			if !sa.Enabled {
				return errors.New(errors.Forbidden, "service account is disabled")
			}
			// SA is enabled — delegate to base for IAM policy evaluation
			return s.rbac.AuthorizeServiceAccount(ctx, saID, tenantID, permission, resource)
		}
	}

	// Cache miss — look up SA
	sa, dbErr := s.saRepo.GetByID(ctx, saID)
	if dbErr != nil {
		if errors.Is(dbErr, errors.NotFound) {
			return errors.New(errors.Forbidden, "service account not found")
		}
		return dbErr
	}

	if !sa.Enabled {
		return errors.New(errors.Forbidden, "service account is disabled")
	}

	// Cache full SA object (including Enabled state) with short TTL
	if data, err := json.Marshal(sa); err == nil {
		s.cache.Set(ctx, cacheKey, data, 30*time.Second)
	}

	return s.rbac.AuthorizeServiceAccount(ctx, saID, tenantID, permission, resource)
}

func (s *cachedRBACService) invalidateRoleCache(ctx context.Context, id uuid.UUID, name string) {
	s.cache.Del(ctx, fmt.Sprintf("rbac:role:id:%s", id))
	s.cache.Del(ctx, fmt.Sprintf("rbac:role:name:%s", name))
	s.cache.Del(ctx, "rbac:perm:*") // Invalidate all cached permissions since a role changed
}
