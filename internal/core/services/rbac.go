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
)

// RBACServiceParams defines dependencies for rbacService.
type RBACServiceParams struct {
	UserRepo   ports.UserRepository
	RoleRepo   ports.RoleRepository
	TenantRepo ports.TenantRepository
	IAMRepo    ports.IAMRepository
	Evaluator  ports.PolicyEvaluator
	Logger     *slog.Logger
	// SA repo for service account authorization - used when checking IAM policies for SAs
	SARepo ports.ServiceAccountRepository
}

type rbacService struct {
	userRepo   ports.UserRepository
	roleRepo   ports.RoleRepository
	tenantRepo ports.TenantRepository
	iamRepo    ports.IAMRepository
	evaluator  ports.PolicyEvaluator
	logger     *slog.Logger
	saRepo     ports.ServiceAccountRepository
}

// NewRBACService constructs an RBAC service for role-based authorization.
func NewRBACService(params RBACServiceParams) *rbacService {
	return &rbacService{
		userRepo:   params.UserRepo,
		roleRepo:   params.RoleRepo,
		tenantRepo: params.TenantRepo,
		iamRepo:    params.IAMRepo,
		evaluator:  params.Evaluator,
		logger:     params.Logger,
		saRepo:     params.SARepo,
	}
}

func (s *rbacService) Authorize(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	if appcontext.IsInternalCall(ctx) {
		return nil
	}
	allowed, err := s.HasPermission(ctx, userID, tenantID, permission, resource)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New(errors.Forbidden, fmt.Sprintf("permission denied: %s on %s", permission, resource))
	}
	return nil
}

func (s *rbacService) HasPermission(ctx context.Context, userID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) (bool, error) {
	if appcontext.IsInternalCall(ctx) {
		return true, nil
	}
	if userID == uuid.Nil {
		return false, nil
	}
	// System user bypass - requires both system ID AND internal signal
	if systemID, err := appcontext.SystemUserID(); err == nil && userID == systemID {
		if appcontext.IsInternalCall(ctx) {
			return true, nil
		}
		s.logger.Warn("RBAC: system user ID used without internal signal", "user_id", userID)
	}

	var roleName string

	// 1. Resolve Role and check IAM Policies
	if tenantID == uuid.Nil {
		// Global context fallback (for system admins or global roles)
		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			if errors.Is(err, errors.NotFound) {
				s.logger.Warn("RBAC: user not found", "user_id", userID)
				return false, nil
			}
			s.logger.Error("RBAC: failed to get user", "user_id", userID, "error", err)
			return false, errors.Wrap(errors.Internal, "failed to get user", err)
		}
		roleName = user.Role
		s.logger.Debug("RBAC: checking global permission", "user_id", userID, "user_role", roleName, "permission", permission, "resource", resource)
	} else {
		// Tenant context
		member, err := s.tenantRepo.GetMembership(ctx, tenantID, userID)
		if err != nil {
			if errors.Is(err, errors.NotFound) {
				s.logger.Warn("RBAC: user is not a member of tenant", "user_id", userID, "tenant_id", tenantID)
				return false, nil
			}
			s.logger.Error("RBAC: failed to get membership", "user_id", userID, "tenant_id", tenantID, "error", err)
			return false, errors.Wrap(errors.Internal, "failed to get membership", err)
		}
		if member == nil {
			s.logger.Warn("RBAC: user is not a member of tenant", "user_id", userID, "tenant_id", tenantID)
			return false, nil
		}
		roleName = member.Role
		s.logger.Debug("RBAC: checking tenant permission", "user_id", userID, "tenant_id", tenantID, "tenant_role", roleName, "permission", permission, "resource", resource)
	}

	// 2. Check Attached IAM Policies (if IAMRepo and Evaluator are provided)
	if s.iamRepo != nil && s.evaluator != nil {
		evalCtx := s.buildEvalCtx(ctx, tenantID)

		// Check user-attached policies
		if allowed, stop := s.checkIAMPolicies(ctx, tenantID, userID, permission, resource, evalCtx); allowed || stop {
			return allowed, nil
		}

		// Check role-attached policies
		if roleName != "" {
			if allowed, stop := s.checkRoleIAMPolicies(ctx, tenantID, roleName, permission, resource, evalCtx); allowed || stop {
				return allowed, nil
			}
		}
	}
	// 3. Fallback to Role-based logic
	if roleName == domain.RoleAdmin {
		return true, nil
	}

	role, err := s.roleRepo.GetRoleByName(ctx, roleName)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			s.logger.Warn("RBAC: role not found in DB", "role", roleName)
			return false, nil
		}
		s.logger.Error("RBAC: failed to get role", "role", roleName, "error", err)
		return false, errors.Wrap(errors.Internal, "failed to get role", err)
	}

	s.logger.Debug("RBAC: found role in DB", "role", role.Name, "permissions_count", len(role.Permissions))

	// 4. Check permissions in role
	for _, p := range role.Permissions {
		if p == domain.PermissionFullAccess {
			return true, nil
		}
		if p == permission {
			return true, nil
		}
	}

	s.logger.Warn("RBAC: permission denied (role in DB but permission not listed)", "role", role.Name, "permission", permission, "resource", resource)
	return false, nil
}

// checkIAMPolicies evaluates IAM policies attached directly to a user.
// Returns (allowed, stop) where stop=true means decision is final.
func (s *rbacService) checkIAMPolicies(ctx context.Context, tenantID, userID uuid.UUID, permission domain.Permission, resource string, evalCtx map[string]interface{}) (bool, bool) {
	policies, err := s.iamRepo.GetPoliciesForUser(ctx, tenantID, userID)
	if err != nil {
		s.logger.Error("RBAC: failed to get user IAM policies, falling through to role policies", "user_id", userID, "tenant_id", tenantID, "error", err)
		return false, false
	}
	if len(policies) == 0 {
		return false, false
	}
	return s.evaluatePolicies(ctx, policies, permission, resource, evalCtx)
}

// checkRoleIAMPolicies evaluates IAM policies attached to a user's role.
// Returns (allowed, stop) where stop=true means decision is final.
func (s *rbacService) checkRoleIAMPolicies(ctx context.Context, tenantID uuid.UUID, roleName string, permission domain.Permission, resource string, evalCtx map[string]interface{}) (bool, bool) {
	policies, err := s.iamRepo.GetPoliciesForRole(ctx, tenantID, roleName)
	if err != nil {
		s.logger.Error("RBAC: failed to get role IAM policies, falling through to RBAC fallback", "role", roleName, "tenant_id", tenantID, "error", err)
		return false, false
	}
	if len(policies) == 0 {
		return false, false
	}
	return s.evaluatePolicies(ctx, policies, permission, resource, evalCtx)
}

// evaluatePolicies evaluates a set of policies and returns (allowed, stop).
// stop=true means a final decision (Allow or Deny) was reached.
// If evaluation fails, returns an error via the logger and (false, false) to continue to next policy source.
func (s *rbacService) evaluatePolicies(ctx context.Context, policies []*domain.Policy, permission domain.Permission, resource string, evalCtx map[string]interface{}) (bool, bool) {
	effect, err := s.evaluator.Evaluate(ctx, policies, string(permission), resource, evalCtx)
	if err != nil {
		s.logger.Error("RBAC: IAM policy evaluation failed, falling through to next policy source", "error", err, "permission", permission, "resource", resource)
		return false, false
	}
	if effect == domain.EffectAllow {
		return true, true
	}
	if effect == domain.EffectDeny {
		return false, true
	}
	return false, false
}

func (s *rbacService) buildEvalCtx(ctx context.Context, tenantID uuid.UUID) map[string]interface{} {
	evalCtx := map[string]interface{}{
		string(domain.KeyTenantID):    tenantID.String(),
		string(domain.KeyCurrentTime): time.Now().UTC(),
	}

	if userID := appcontext.UserIDFromContext(ctx); userID != uuid.Nil {
		evalCtx[string(domain.KeyUserID)] = userID.String()
	}

	if sourceIP := appcontext.SourceIPFromContext(ctx); sourceIP != "" {
		evalCtx[string(domain.KeySourceIP)] = sourceIP
	}

	return evalCtx
}

func (s *rbacService) CreateRole(ctx context.Context, role *domain.Role) error {
	// Check if role name already exists to provide a better error than 500
	existing, _ := s.roleRepo.GetRoleByName(ctx, role.Name)
	if existing != nil {
		return errors.New(errors.Conflict, fmt.Sprintf("role with name '%s' already exists", role.Name))
	}

	if role.ID == uuid.Nil {
		role.ID = uuid.New()
	}
	return s.roleRepo.CreateRole(ctx, role)
}

func (s *rbacService) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	return s.roleRepo.GetRoleByID(ctx, id)
}

func (s *rbacService) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	return s.roleRepo.GetRoleByName(ctx, name)
}

func (s *rbacService) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	return s.roleRepo.ListRoles(ctx)
}

func (s *rbacService) UpdateRole(ctx context.Context, role *domain.Role) error {
	return s.roleRepo.UpdateRole(ctx, role)
}

func (s *rbacService) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return s.roleRepo.DeleteRole(ctx, id)
}

func (s *rbacService) AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return s.roleRepo.AddPermissionToRole(ctx, roleID, permission)
}

func (s *rbacService) RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return s.roleRepo.RemovePermissionFromRole(ctx, roleID, permission)
}

func (s *rbacService) BindRole(ctx context.Context, userIdentifier string, roleName string) error {
	// 1. Verify role exists
	_, err := s.roleRepo.GetRoleByName(ctx, roleName)
	if err != nil {
		return fmt.Errorf("role %s does not exist: %w", roleName, err)
	}

	// 2. Get user (by ID or Email)
	var user *domain.User
	userID, err := uuid.Parse(userIdentifier)
	if err == nil {
		user, err = s.userRepo.GetByID(ctx, userID)
	} else {
		user, err = s.userRepo.GetByEmail(ctx, userIdentifier)
	}

	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// 3. Update role
	user.Role = roleName
	return s.userRepo.Update(ctx, user)
}

func (s *rbacService) ListRoleBindings(ctx context.Context) ([]*domain.User, error) {
	// In this implementation, bindings are just users with their roles
	return s.userRepo.List(ctx)
}

func (s *rbacService) EvaluatePolicy(ctx context.Context, userID uuid.UUID, action string, resource string, evalCtx map[string]interface{}) (bool, error) {
	if s.iamRepo == nil || s.evaluator == nil {
		return false, fmt.Errorf("IAM support not initialized")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}

	tenantID := uuid.Nil
	if user.TenantID != uuid.Nil {
		tenantID = user.TenantID
	}

	policies, err := s.iamRepo.GetPoliciesForUser(ctx, tenantID, userID)
	if err != nil {
		return false, err
	}
	if len(policies) == 0 {
		return false, nil
	}
	effect, err := s.evaluator.Evaluate(ctx, policies, action, resource, evalCtx)
	if err != nil {
		return false, err
	}
	return effect == domain.EffectAllow, nil
}

func (s *rbacService) AuthorizeServiceAccount(ctx context.Context, saID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	if appcontext.IsInternalCall(ctx) {
		return nil
	}

	allowed, err := s.hasPermissionForSA(ctx, saID, tenantID, permission, resource)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New(errors.Forbidden, fmt.Sprintf("permission denied: %s on %s for service account", permission, resource))
	}
	return nil
}

func (s *rbacService) hasPermissionForSA(ctx context.Context, saID uuid.UUID, tenantID uuid.UUID, permission domain.Permission, resource string) (bool, error) {
	if saID == uuid.Nil {
		return false, nil
	}

	sa, err := s.saRepo.GetByID(ctx, saID)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			s.logger.Warn("RBAC: service account not found", "sa_id", saID)
			return false, nil
		}
		s.logger.Error("RBAC: failed to get service account", "sa_id", saID, "error", err)
		return false, errors.Wrap(errors.Internal, "failed to get service account", err)
	}

	roleName := sa.Role
	s.logger.Debug("RBAC: checking SA permission", "sa_id", saID, "tenant_id", tenantID, "role", roleName, "permission", permission, "resource", resource)

	evalCtx := s.buildEvalCtx(ctx, tenantID)

	// 1. Check IAM policies attached to SA
	if s.iamRepo != nil && s.evaluator != nil {
		if allowed, stop := s.checkSAIAMPolicies(ctx, tenantID, saID, permission, resource, evalCtx); allowed || stop {
			return allowed, nil
		}

		// 2. Check IAM policies attached to SA's role
		if roleName != "" {
			if allowed, stop := s.checkRoleIAMPolicies(ctx, tenantID, roleName, permission, resource, evalCtx); allowed || stop {
				return allowed, nil
			}
		}
	}

	// 3. Fallback to Role-based logic
	if roleName == domain.RoleAdmin {
		return true, nil
	}

	role, err := s.roleRepo.GetRoleByName(ctx, roleName)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			s.logger.Warn("RBAC: role not found in DB", "role", roleName)
			return false, nil
		}
		s.logger.Error("RBAC: failed to get role", "role", roleName, "error", err)
		return false, errors.Wrap(errors.Internal, "failed to get role", err)
	}

	for _, p := range role.Permissions {
		if p == domain.PermissionFullAccess {
			return true, nil
		}
		if p == permission {
			return true, nil
		}
	}

	s.logger.Warn("RBAC: permission denied for SA (role in DB but permission not listed)", "role", role.Name, "sa_id", saID, "permission", permission, "resource", resource)
	return false, nil
}

// checkSAIAMPolicies evaluates IAM policies attached directly to a service account.
func (s *rbacService) checkSAIAMPolicies(ctx context.Context, tenantID, saID uuid.UUID, permission domain.Permission, resource string, evalCtx map[string]interface{}) (bool, bool) {
	policies, err := s.iamRepo.GetPoliciesForServiceAccount(ctx, tenantID, saID)
	if err != nil {
		s.logger.Error("RBAC: failed to get SA IAM policies, denying access", "sa_id", saID, "tenant_id", tenantID, "error", err)
		return false, true
	}
	if len(policies) == 0 {
		return false, false
	}
	return s.evaluatePolicies(ctx, policies, permission, resource, evalCtx)
}
