package httphandlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

// IAMHandler handles HTTP requests for identity and access management.
type IAMHandler struct {
	svc         ports.IAMService
	identitySvc ports.IdentityService
}

// NewIAMHandler creates a new IAM handler.
func NewIAMHandler(svc ports.IAMService, identitySvc ports.IdentityService) *IAMHandler {
	return &IAMHandler{svc: svc, identitySvc: identitySvc}
}

// CreatePolicy creates a new IAM policy.
// @Summary Create IAM Policy
// @Description Create a new granular IAM policy with specified statements.
// @Tags iam
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param policy body domain.Policy true "Policy configuration"
// @Success 201 {object} domain.Policy
// @Failure 400 {object} httputil.Response
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/policies [post]
func (h *IAMHandler) CreatePolicy(c *gin.Context) {
	var policy domain.Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.validatePolicy(&policy); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	if err := h.svc.CreatePolicy(c.Request.Context(), &policy); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, policy)
}

// GetPolicyByID returns a specific IAM policy.
// @Summary Get IAM Policy
// @Description Get details of a specific IAM policy by its ID.
// @Tags iam
// @Security APIKeyAuth
// @Produce json
// @Param id path string true "Policy ID"
// @Success 200 {object} domain.Policy
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /iam/policies/{id} [get]
func (h *IAMHandler) GetPolicyByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	policy, err := h.svc.GetPolicyByID(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, policy)
}

// ListPolicies returns all IAM policies.
// @Summary List IAM Policies
// @Description List all IAM policies for the tenant.
// @Tags iam
// @Security APIKeyAuth
// @Produce json
// @Success 200 {array} domain.Policy
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/policies [get]
func (h *IAMHandler) ListPolicies(c *gin.Context) {
	policies, err := h.svc.ListPolicies(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, policies)
}

// UpdatePolicy updates an existing IAM policy.
// @Summary Update IAM Policy
// @Description Update the configuration of an existing IAM policy.
// @Tags iam
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Policy ID"
// @Param policy body domain.Policy true "Updated policy configuration"
// @Success 200 {object} domain.Policy
// @Failure 400 {object} httputil.Response
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /iam/policies/{id} [put]
func (h *IAMHandler) UpdatePolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	var policy domain.Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		httputil.Error(c, err)
		return
	}
	policy.ID = id

	if err := h.validatePolicy(&policy); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	if err := h.svc.UpdatePolicy(c.Request.Context(), &policy); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, policy)
}

// DeletePolicy removes an IAM policy.
// @Summary Delete IAM Policy
// @Description Delete an existing IAM policy by its ID.
// @Tags iam
// @Security APIKeyAuth
// @Produce json
// @Param id path string true "Policy ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /iam/policies/{id} [delete]
func (h *IAMHandler) DeletePolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.svc.DeletePolicy(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"status": "deleted"})
}

// AttachPolicyToUser attaches a policy to a user.
// @Summary Attach IAM Policy to User
// @Description Attach a specific IAM policy to a user.
// @Tags iam
// @Security APIKeyAuth
// @Produce json
// @Param userId path string true "User ID"
// @Param policyId path string true "Policy ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /iam/users/{userId}/policies/{policyId} [post]
func (h *IAMHandler) AttachPolicyToUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}
	policyID, err := uuid.Parse(c.Param("policyId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.svc.AttachPolicyToUser(c.Request.Context(), userID, policyID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"status": "attached"})
}

// DetachPolicyFromUser removes a policy from a user.
// @Summary Detach IAM Policy from User
// @Description Remove a specific IAM policy assignment from a user.
// @Tags iam
// @Security APIKeyAuth
// @Produce json
// @Param userId path string true "User ID"
// @Param policyId path string true "Policy ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /iam/users/{userId}/policies/{policyId} [delete]
func (h *IAMHandler) DetachPolicyFromUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}
	policyID, err := uuid.Parse(c.Param("policyId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.svc.DetachPolicyFromUser(c.Request.Context(), userID, policyID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"status": "detached"})
}

// GetUserPolicies lists all policies attached to a specific user.
// @Summary List User IAM Policies
// @Description List all IAM policies currently attached to a specific user.
// @Tags iam
// @Security APIKeyAuth
// @Produce json
// @Param userId path string true "User ID"
// @Success 200 {array} domain.Policy
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/users/{userId}/policies [get]
func (h *IAMHandler) GetUserPolicies(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	policies, err := h.svc.GetPoliciesForUser(c.Request.Context(), userID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, policies)
}

func (h *IAMHandler) validatePolicy(policy *domain.Policy) error {
	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if len(policy.Statements) == 0 {
		return fmt.Errorf("at least one policy statement is required")
	}
	for i, stmt := range policy.Statements {
		if stmt.Effect != domain.EffectAllow && stmt.Effect != domain.EffectDeny {
			return fmt.Errorf("statement %d has invalid effect: must be 'Allow' or 'Deny'", i)
		}
		if len(stmt.Action) == 0 {
			return fmt.Errorf("statement %d must specify at least one action", i)
		}
		if len(stmt.Resource) == 0 {
			return fmt.Errorf("statement %d must specify at least one resource", i)
		}
	}
	return nil
}

// CreateServiceAccount creates a new service account.
// @Summary Create Service Account
// @Description Create a new service account with credentials for M2M authentication.
// @Tags iam
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param request body CreateServiceAccountRequest true "Service account details"
// @Success 201 {object} domain.ServiceAccountWithSecret
// @Failure 400 {object} httputil.Response
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/service-accounts [post]
func (h *IAMHandler) CreateServiceAccount(c *gin.Context) {
	var req CreateServiceAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	tenantID := httputil.GetTenantID(c)
	sa, err := h.identitySvc.CreateServiceAccount(c.Request.Context(), tenantID, req.Name, req.Role)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, sa)
}

// CreateServiceAccountRequest is the payload for service account creation.
type CreateServiceAccountRequest struct {
	Name        string `json:"name" binding:"required"`
	Role        string `json:"role" binding:"required"`
	Description string `json:"description"`
}

// ListServiceAccounts returns all service accounts for the tenant.
// @Summary List Service Accounts
// @Description List all service accounts for the tenant.
// @Tags iam
// @Security APIKeyAuth
// @Produce json
// @Success 200 {array} domain.ServiceAccount
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/service-accounts [get]
func (h *IAMHandler) ListServiceAccounts(c *gin.Context) {
	tenantID := httputil.GetTenantID(c)
	accounts, err := h.identitySvc.ListServiceAccounts(c.Request.Context(), tenantID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, accounts)
}

// GetServiceAccount returns a specific service account.
// @Summary Get Service Account
// @Description Get details of a specific service account.
// @Tags iam
// @Security APIKeyAuth
// @Produce json
// @Param id path string true "Service Account ID"
// @Success 200 {object} domain.ServiceAccount
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /iam/service-accounts/{id} [get]
func (h *IAMHandler) GetServiceAccount(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	sa, err := h.identitySvc.GetServiceAccount(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, sa)
}

// DeleteServiceAccount removes a service account.
// @Summary Delete Service Account
// @Description Delete an existing service account and all its secrets.
// @Tags iam
// @Security APIKeyAuth
// @Produce json
// @Param id path string true "Service Account ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /iam/service-accounts/{id} [delete]
func (h *IAMHandler) DeleteServiceAccount(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.identitySvc.DeleteServiceAccount(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"status": "deleted"})
}

// RevokeServiceAccountSecret invalidates a secret.
// @Summary Revoke Service Account Secret
// @Description Revoke a specific secret from a service account.
// @Tags iam
// @Security APIKeyAuth
// @Param id path string true "Service Account ID"
// @Param secretId path string true "Secret ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/service-accounts/{id}/secrets/{secretId} [post]
func (h *IAMHandler) RevokeServiceAccountSecret(c *gin.Context) {
	saID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}
	secretID, err := uuid.Parse(c.Param("secretId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.identitySvc.RevokeServiceAccountSecret(c.Request.Context(), saID, secretID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"status": "revoked"})
}

// ListServiceAccountSecrets returns all secrets for a service account.
// @Summary List Service Account Secrets
// @Description List all secrets for a service account.
// @Tags iam
// @Security APIKeyAuth
// @Param id path string true "Service Account ID"
// @Success 200 {array} domain.ServiceAccountSecret
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/service-accounts/{id}/secrets [get]
func (h *IAMHandler) ListServiceAccountSecrets(c *gin.Context) {
	saID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	secrets, err := h.identitySvc.ListServiceAccountSecrets(c.Request.Context(), saID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, secrets)
}

// RotateServiceAccountSecret rotates the secret and returns new plaintext.
// @Summary Rotate Service Account Secret
// @Description Rotate the secret for a service account, returns new plaintext.
// @Tags iam
// @Security APIKeyAuth
// @Param id path string true "Service Account ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/service-accounts/{id}/rotate-secret [post]
func (h *IAMHandler) RotateServiceAccountSecret(c *gin.Context) {
	saID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	secret, err := h.identitySvc.RotateServiceAccountSecret(c.Request.Context(), saID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"secret": secret})
}

// AttachPolicyToServiceAccount attaches a policy to a service account.
// @Summary Attach IAM Policy to Service Account
// @Description Attach a specific IAM policy to a service account.
// @Tags iam
// @Security APIKeyAuth
// @Param saId path string true "Service Account ID"
// @Param policyId path string true "Policy ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/service-accounts/{saId}/policies/{policyId} [post]
func (h *IAMHandler) AttachPolicyToServiceAccount(c *gin.Context) {
	saID, err := uuid.Parse(c.Param("saId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}
	policyID, err := uuid.Parse(c.Param("policyId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.svc.AttachPolicyToServiceAccount(c.Request.Context(), saID, policyID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"status": "attached"})
}

// DetachPolicyFromServiceAccount removes a policy from a service account.
// @Summary Detach IAM Policy from Service Account
// @Description Remove a specific IAM policy assignment from a service account.
// @Tags iam
// @Security APIKeyAuth
// @Param saId path string true "Service Account ID"
// @Param policyId path string true "Policy ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/service-accounts/{saId}/policies/{policyId} [delete]
func (h *IAMHandler) DetachPolicyFromServiceAccount(c *gin.Context) {
	saID, err := uuid.Parse(c.Param("saId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}
	policyID, err := uuid.Parse(c.Param("policyId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.svc.DetachPolicyFromServiceAccount(c.Request.Context(), saID, policyID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"status": "detached"})
}

// GetServiceAccountPolicies lists all policies attached to a specific service account.
// @Summary List Service Account IAM Policies
// @Description List all IAM policies currently attached to a specific service account.
// @Tags iam
// @Security APIKeyAuth
// @Param saId path string true "Service Account ID"
// @Success 200 {array} domain.Policy
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/service-accounts/{saId}/policies [get]
func (h *IAMHandler) GetServiceAccountPolicies(c *gin.Context) {
	saID, err := uuid.Parse(c.Param("saId"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	policies, err := h.svc.GetPoliciesForServiceAccount(c.Request.Context(), saID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, policies)
}

// SimulateRequest is the payload for policy simulation.
type SimulateRequest struct {
	// Principal identifies whose policies to simulate.
	// Exactly one of UserID or ServiceAccountID must be set.
	UserID           *uuid.UUID `json:"user_id,omitempty"`
	ServiceAccountID *uuid.UUID `json:"service_account_id,omitempty"`
	// Actions to simulate (e.g., ["compute:instance:launch"]).
	Actions []string `json:"actions" binding:"required,min=1"`
	// Resources to test (e.g., ["arn:thecloud:compute:us-east-1:*:instance/*"]).
	Resources []string `json:"resources" binding:"required,min=1"`
	// Context overrides for condition evaluation.
	// Keys: aws:SourceIp, aws:CurrentTime, thecloud:TenantId, etc.
	Context map[string]interface{} `json:"context,omitempty"`
}

// SimulateResponse is the result of a policy simulation.
type SimulateResponse struct {
	Decision  string            `json:"decision"` // "allow" or "deny"
	Matched   *StatementMatch  `json:"matched,omitempty"`
	Evaluated int              `json:"evaluated"`
}

// StatementMatch describes which statement allowed or denied the request.
type StatementMatch struct {
	Action      string    `json:"action,omitempty"`
	Resource    string    `json:"resource,omitempty"`
	PolicyID    uuid.UUID `json:"policy_id"`
	PolicyName  string    `json:"policy_name"`
	StatementSid string    `json:"statement_sid,omitempty"`
	Effect      string    `json:"effect"`
	Reason      string    `json:"reason"`
}

// Simulate godoc
// @Summary Simulate IAM policy decision
// @Description Evaluates what actions and resources would be allowed or denied by attached policies
// @Tags iam
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param request body SimulateRequest true "Simulation parameters"
// @Success 200 {object} httputil.Response{data=SimulateResponse}
// @Failure 400 {object} httputil.Response
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Router /iam/simulate [post]
func (h *IAMHandler) Simulate(c *gin.Context) {
	var req SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	if req.UserID == nil && req.ServiceAccountID == nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "user_id or service_account_id is required"))
		return
	}

	principal := ports.Principal{
		UserID:           req.UserID,
		ServiceAccountID: req.ServiceAccountID,
	}

	evalCtx := buildSimulateCtx(c, req.Context)

	result, err := h.svc.SimulatePolicy(c.Request.Context(), principal, req.Actions, req.Resources, evalCtx)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	response := SimulateResponse{
		Decision:  strings.ToLower(string(result.Decision)),
		Evaluated: result.Evaluated,
	}
	if result.Matched != nil {
		response.Matched = &StatementMatch{
			Action:      result.Matched.Action,
			Resource:    result.Matched.Resource,
			PolicyID:    result.Matched.PolicyID,
			PolicyName:  result.Matched.PolicyName,
			StatementSid: result.Matched.StatementSid,
			Effect:      string(result.Matched.Effect),
			Reason:      result.Matched.Reason,
		}
	}

	httputil.Success(c, http.StatusOK, response)
}

func buildSimulateCtx(c *gin.Context, overrides map[string]interface{}) map[string]interface{} {
	ctx := map[string]interface{}{
		"thecloud:TenantId": httputil.GetTenantID(c),
		"aws:CurrentTime":   time.Now().Format(time.RFC3339),
	}
	if ip := c.ClientIP(); ip != "" {
		ctx["aws:SourceIp"] = ip
	}
	if ua := c.GetHeader("User-Agent"); ua != "" {
		ctx["aws:UserAgent"] = ua
	}
	for k, v := range overrides {
		ctx[k] = v
	}
	return ctx
}
