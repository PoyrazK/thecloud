package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// IdentityProviderRepository defines persistence for IdP configurations.
type IdentityProviderRepository interface {
	Create(ctx context.Context, idp *domain.IdentityProvider) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.IdentityProvider, error)
	List(ctx context.Context, scope domain.IdentityProviderScope, tenantID *uuid.UUID) ([]*domain.IdentityProvider, error)
	Update(ctx context.Context, idp *domain.IdentityProvider) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// FederatedIdentityRepository defines persistence for federated identity links.
type FederatedIdentityRepository interface {
	Create(ctx context.Context, fi *domain.FederatedIdentity) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.FederatedIdentity, error)
	GetByIdPAndSubject(ctx context.Context, idpID uuid.UUID, subject string) (*domain.FederatedIdentity, error)
	Update(ctx context.Context, fi *domain.FederatedIdentity) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
}

// OIDCDiscovery represents the OIDC discovery document.
type OIDCDiscovery struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint"`
	JwksURI               string   `json:"jwks_uri"`
	ScopesSupported       []string `json:"scopes_supported"`
}

// OIDCUserInfo represents claims from OIDC UserInfo endpoint.
type OIDCUserInfo struct {
	Subject       string   `json:"sub"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	GivenName     string   `json:"given_name"`
	FamilyName    string   `json:"family_name"`
	Groups        []string `json:"groups"`
}

// IdentityProviderService defines business logic for IdP management.
type IdentityProviderService interface {
	// CRUD
	CreateIdP(ctx context.Context, idp *domain.IdentityProvider) (*domain.IdentityProvider, error)
	GetIdP(ctx context.Context, id uuid.UUID) (*domain.IdentityProvider, error)
	ListIdPs(ctx context.Context, scope domain.IdentityProviderScope, tenantID *uuid.UUID) ([]*domain.IdentityProvider, error)
	UpdateIdP(ctx context.Context, idp *domain.IdentityProvider) error
	DeleteIdP(ctx context.Context, id uuid.UUID) error

	// Federation flows
	HandleOIDCCallback(ctx context.Context, code, pkceVerifier string, idpID uuid.UUID) (*domain.User, string, error)
	HandleSAMLAssertion(ctx context.Context, assertionXML []byte, idpID uuid.UUID) (*domain.User, string, error)

	// Discovery
	DiscoverOIDCConfig(ctx context.Context, discoveryURL string) (*OIDCDiscovery, error)

	// Token / assertion validation
	ValidateOIDCToken(ctx context.Context, rawIDToken string, idp *domain.IdentityProvider, jwksURI string) (*OIDCUserInfo, error)
}