package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type IdentityProviderHandler struct {
	idpSvc    ports.IdentityProviderService
	tenantSvc ports.TenantService
}

func NewIdentityProviderHandler(idpSvc ports.IdentityProviderService, tenantSvc ports.TenantService) *IdentityProviderHandler {
	return &IdentityProviderHandler{idpSvc: idpSvc, tenantSvc: tenantSvc}
}

// OIDCCallback handles the OIDC callback after IdP authorization
// GET /auth/sso/oidc/:idp_id/callback?code=xxx&state=xxx
func (h *IdentityProviderHandler) OIDCCallback(c *gin.Context) {
	idpID, err := uuid.Parse(c.Param("idp_id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	code := c.Query("code")
	state := c.Query("state")
	if code == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "authorization code required"))
		return
	}

	user, apiKey, err := h.idpSvc.HandleOIDCCallback(c.Request.Context(), code, state, idpID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if apiKey != "" {
		httputil.Success(c, http.StatusOK, gin.H{"user": user, "api_key": apiKey})
	} else {
		httputil.Success(c, http.StatusOK, gin.H{"user": user})
	}
}

// SAMLACS handles SAML Assertion Consumer Service
// POST /auth/sso/saml/:idp_id/acs
func (h *IdentityProviderHandler) SAMLACS(c *gin.Context) {
	idpID, err := uuid.Parse(c.Param("idp_id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	var req struct {
		SAMLResponse string `json:"saml_response" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	user, apiKey, err := h.idpSvc.HandleSAMLAssertion(c.Request.Context(), []byte(req.SAMLResponse), idpID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if apiKey != "" {
		httputil.Success(c, http.StatusOK, gin.H{"user": user, "api_key": apiKey})
	} else {
		httputil.Success(c, http.StatusOK, gin.H{"user": user})
	}
}

// InitiateOIDCLogin redirects to IdP authorization endpoint
// GET /auth/sso/oidc/:idp_id
func (h *IdentityProviderHandler) InitiateOIDCLogin(c *gin.Context) {
	idpID, err := uuid.Parse(c.Param("idp_id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	idp, err := h.idpSvc.GetIdP(c.Request.Context(), idpID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if idp.Type != domain.IdPTypeOIDC {
		httputil.Error(c, errors.New(errors.InvalidInput, "identity provider is not OIDC type"))
		return
	}

	discovery, err := h.idpSvc.DiscoverOIDCConfig(c.Request.Context(), idp.DiscoveryURL)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	state := uuid.New().String()
	c.SetCookie("oidc_state", state, 600, "/", "", false, true)

	redirectURL := discovery.AuthorizationEndpoint +
		"?client_id=" + idp.ClientID +
		"&response_type=code" +
		"&scope=openid+profile+email" +
		"&redirect_uri=" + idp.RedirectURIs[0] +
		"&state=" + state

	c.Redirect(http.StatusFound, redirectURL)
}

// InitiateSAMLLogin redirects to IdP SSO URL
// GET /auth/sso/saml/:idp_id
func (h *IdentityProviderHandler) InitiateSAMLLogin(c *gin.Context) {
	idpID, err := uuid.Parse(c.Param("idp_id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	idp, err := h.idpSvc.GetIdP(c.Request.Context(), idpID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if idp.Type != domain.IdPTypeSAML {
		httputil.Error(c, errors.New(errors.InvalidInput, "identity provider is not SAML type"))
		return
	}

	c.Redirect(http.StatusFound, idp.SSOURL)
}

// Create creates a new IdP
// POST /admin/identity-providers
func (h *IdentityProviderHandler) Create(c *gin.Context) {
	var req struct {
		Name         string                    `json:"name" binding:"required"`
		Type         domain.IdentityProviderType `json:"type" binding:"required,oneof=oidc saml"`
		Scope        domain.IdentityProviderScope `json:"scope" binding:"required,oneof=global tenant"`
		TenantID     *uuid.UUID                `json:"tenant_id,omitempty"`
		ClientID     string                    `json:"client_id"`
		ClientSecret string                    `json:"client_secret"`
		IssuerURL    string                    `json:"issuer_url"`
		DiscoveryURL string                    `json:"discovery_url"`
		EntityID     string                    `json:"entity_id"`
		SSOURL       string                    `json:"sso_url"`
		Certificate  string                    `json:"certificate"`
		Scopes       []string                  `json:"scopes"`
		RedirectURIs []string                  `json:"redirect_uris" binding:"required"`
		Enabled      bool                      `json:"enabled"`
		DefaultRole  string                    `json:"default_role"`
		GroupMapping []domain.GroupMapping     `json:"group_mapping"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	idp := &domain.IdentityProvider{
		ID:           uuid.New(),
		Name:         req.Name,
		Type:         req.Type,
		Scope:        req.Scope,
		TenantID:     req.TenantID,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		IssuerURL:    req.IssuerURL,
		DiscoveryURL: req.DiscoveryURL,
		EntityID:     req.EntityID,
		SSOURL:       req.SSOURL,
		Certificate:  req.Certificate,
		Scopes:       req.Scopes,
		RedirectURIs: req.RedirectURIs,
		Enabled:      req.Enabled,
		DefaultRole:  req.DefaultRole,
		GroupMapping: req.GroupMapping,
	}

	if idp.Scopes == nil {
		idp.Scopes = []string{"openid", "profile", "email"}
	}

	created, err := h.idpSvc.CreateIdP(c.Request.Context(), idp)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, created)
}

// List lists all IdPs
// GET /admin/identity-providers?scope=global|tenant
func (h *IdentityProviderHandler) List(c *gin.Context) {
	scope := domain.IdentityProviderScope(c.Query("scope"))
	var tenantID *uuid.UUID
	if tidStr := c.Query("tenant_id"); tidStr != "" {
		tid, _ := uuid.Parse(tidStr)
		tenantID = &tid
	}

	idps, err := h.idpSvc.ListIdPs(c.Request.Context(), scope, tenantID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, idps)
}

// Get returns a specific IdP
// GET /admin/identity-providers/:id
func (h *IdentityProviderHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	idp, err := h.idpSvc.GetIdP(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, idp)
}

// Update updates an IdP
// PUT /admin/identity-providers/:id
func (h *IdentityProviderHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	var req domain.IdentityProvider
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}
	req.ID = id

	if err := h.idpSvc.UpdateIdP(c.Request.Context(), &req); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, req)
}

// Delete deletes an IdP
// DELETE /admin/identity-providers/:id
func (h *IdentityProviderHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.idpSvc.DeleteIdP(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"status": "deleted"})
}