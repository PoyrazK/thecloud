package domain

import (
	"time"

	"github.com/google/uuid"
)

// IdentityProviderType distinguishes between OIDC and SAML protocols.
type IdentityProviderType string

const (
	IdPTypeOIDC IdentityProviderType = "oidc"
	IdPTypeSAML IdentityProviderType = "saml"
)

// IdentityProviderScope defines whether an IdP is global or per-tenant.
type IdentityProviderScope string

const (
	IdPScopeGlobal IdentityProviderScope = "global"
	IdPScopeTenant IdentityProviderScope = "tenant"
)

// GroupMapping maps IdP groups/roles to TheCloud roles.
type GroupMapping struct {
	IdPGroup  string `json:"idp_group"`
	CloudRole string `json:"cloud_role"`
}

// IdentityProvider represents a configured external identity provider.
type IdentityProvider struct {
	ID           uuid.UUID             `json:"id"`
	Name         string                `json:"name"`
	Type         IdentityProviderType `json:"type"`
	Scope        IdentityProviderScope `json:"scope"`
	TenantID     *uuid.UUID            `json:"tenant_id,omitempty"`
	ClientID     string                `json:"client_id,omitempty"`
	ClientSecret string                `json:"-"` // OIDC: encrypted, never serialized
	IssuerURL    string                `json:"issuer_url,omitempty"`
	DiscoveryURL string                `json:"discovery_url,omitempty"`
	EntityID     string                `json:"entity_id,omitempty"`
	SSOURL       string                `json:"sso_url,omitempty"`
	Certificate  string                `json:"certificate,omitempty"`
	Scopes       []string              `json:"scopes"`
	RedirectURIs []string              `json:"redirect_uris"`
	Enabled      bool                  `json:"enabled"`
	DefaultRole  string                `json:"default_role"`
	GroupMapping []GroupMapping        `json:"group_mapping"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

// FederatedIdentity maps an external IdP subject to a local user account.
type FederatedIdentity struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	IdPID         uuid.UUID `json:"idp_id"`
	Subject       string    `json:"subject"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	Groups        []string  `json:"groups"`
	LastLoginAt   time.Time `json:"last_login_at"`
	CreatedAt     time.Time `json:"created_at"`
}