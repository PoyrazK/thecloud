package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	apperrors "github.com/poyrazk/thecloud/internal/errors"
)

type IdentityProviderService struct {
	idpRepo      ports.IdentityProviderRepository
	fedIdentRepo ports.FederatedIdentityRepository
	userRepo     ports.UserRepository
	tenantSvc    ports.TenantService
	auditSvc     ports.AuditService
	apiKeySvc    ports.IdentityService
	httpClient   *http.Client
	logger       *slog.Logger
}

type IdentityProviderServiceParams struct {
	IdPRepo      ports.IdentityProviderRepository
	FedIdentRepo ports.FederatedIdentityRepository
	UserRepo     ports.UserRepository
	TenantSvc    ports.TenantService
	AuditSvc     ports.AuditService
	APIKeySvc    ports.IdentityService
	Logger       *slog.Logger
}

func NewIdentityProviderService(params IdentityProviderServiceParams) *IdentityProviderService {
	return &IdentityProviderService{
		idpRepo:      params.IdPRepo,
		fedIdentRepo: params.FedIdentRepo,
		userRepo:     params.UserRepo,
		tenantSvc:    params.TenantSvc,
		auditSvc:     params.AuditSvc,
		apiKeySvc:    params.APIKeySvc,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		logger:       params.Logger,
	}
}

func (s *IdentityProviderService) CreateIdP(ctx context.Context, idp *domain.IdentityProvider) (*domain.IdentityProvider, error) {
	if idp.ID == uuid.Nil {
		idp.ID = uuid.New()
	}
	idp.CreatedAt = time.Now()
	idp.UpdatedAt = time.Now()

	if err := s.idpRepo.Create(ctx, idp); err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, uuid.Nil, "idp.create", "identity_provider", idp.ID.String(), map[string]interface{}{
		"name": idp.Name,
		"type": idp.Type,
		"scope": idp.Scope,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	return idp, nil
}

func (s *IdentityProviderService) GetIdP(ctx context.Context, id uuid.UUID) (*domain.IdentityProvider, error) {
	return s.idpRepo.GetByID(ctx, id)
}

func (s *IdentityProviderService) ListIdPs(ctx context.Context, scope domain.IdentityProviderScope, tenantID *uuid.UUID) ([]*domain.IdentityProvider, error) {
	return s.idpRepo.List(ctx, scope, tenantID)
}

func (s *IdentityProviderService) UpdateIdP(ctx context.Context, idp *domain.IdentityProvider) error {
	idp.UpdatedAt = time.Now()
	if err := s.idpRepo.Update(ctx, idp); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, uuid.Nil, "idp.update", "identity_provider", idp.ID.String(), map[string]interface{}{
		"name": idp.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	return nil
}

func (s *IdentityProviderService) DeleteIdP(ctx context.Context, id uuid.UUID) error {
	if err := s.idpRepo.Delete(ctx, id); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, uuid.Nil, "idp.delete", "identity_provider", id.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	return nil
}

func (s *IdentityProviderService) HandleOIDCCallback(ctx context.Context, code, state string, idpID uuid.UUID) (*domain.User, string, error) {
	idp, err := s.idpRepo.GetByID(ctx, idpID)
	if err != nil {
		return nil, "", apperrors.Wrap(apperrors.NotFound, "identity provider not found", err)
	}
	if !idp.Enabled {
		return nil, "", apperrors.New(apperrors.Unauthorized, "identity provider is disabled")
	}
	if idp.Type != domain.IdPTypeOIDC {
		return nil, "", apperrors.New(apperrors.InvalidInput, "identity provider is not OIDC type")
	}

	discovery, err := s.DiscoverOIDCConfig(ctx, idp.DiscoveryURL)
	if err != nil {
		return nil, "", apperrors.Wrap(apperrors.Internal, "failed to fetch OIDC discovery", err)
	}

	tokenResp, err := s.exchangeCode(ctx, discovery.TokenEndpoint, code, idp.ClientID, idp.ClientSecret, idp.RedirectURIs[0])
	if err != nil {
		return nil, "", apperrors.Wrap(apperrors.Unauthorized, "token exchange failed", err)
	}

	userInfo, err := s.ValidateOIDCToken(ctx, tokenResp.IDToken, idp)
	if err != nil {
		return nil, "", apperrors.Wrap(apperrors.Unauthorized, "ID token validation failed", err)
	}

	return s.jitProvisionOrLink(ctx, idp, userInfo.Subject, userInfo.Email, userInfo.EmailVerified, userInfo.Groups, userInfo.Name)
}

func (s *IdentityProviderService) HandleSAMLAssertion(ctx context.Context, assertionXML []byte, idpID uuid.UUID) (*domain.User, string, error) {
	idp, err := s.idpRepo.GetByID(ctx, idpID)
	if err != nil {
		return nil, "", apperrors.Wrap(apperrors.NotFound, "identity provider not found", err)
	}
	if !idp.Enabled {
		return nil, "", apperrors.New(apperrors.Unauthorized, "identity provider is disabled")
	}
	if idp.Type != domain.IdPTypeSAML {
		return nil, "", apperrors.New(apperrors.InvalidInput, "identity provider is not SAML type")
	}

	assertion, err := s.parseSAMLAssertion(ctx, assertionXML, idp)
	if err != nil {
		return nil, "", apperrors.Wrap(apperrors.Unauthorized, "SAML assertion parsing failed", err)
	}

	return s.jitProvisionOrLink(ctx, idp, assertion.Subject, assertion.Email, assertion.EmailVerified, assertion.Groups, assertion.Name)
}

func (s *IdentityProviderService) DiscoverOIDCConfig(ctx context.Context, discoveryURL string) (*ports.OIDCDiscovery, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery returned status %d", resp.StatusCode)
	}

	var discovery ports.OIDCDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return nil, err
	}

	return &discovery, nil
}

func (s *IdentityProviderService) ValidateOIDCToken(ctx context.Context, rawIDToken string, idp *domain.IdentityProvider) (*ports.OIDCUserInfo, error) {
	parts := strings.Split(rawIDToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format")
	}

	payload, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, err
	}

	var claims struct {
		Subject       string   `json:"sub"`
		Email         string   `json:"email"`
		EmailVerified bool     `json:"email_verified"`
		Name          string   `json:"name"`
		GivenName     string   `json:"given_name"`
		FamilyName    string   `json:"family_name"`
		Groups        []string `json:"groups"`
		Issuer        string   `json:"iss"`
		Audience      any      `json:"aud"`
	}

	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	if claims.Issuer != idp.IssuerURL {
		return nil, errors.New("token issuer mismatch")
	}

	return &ports.OIDCUserInfo{
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Groups:        claims.Groups,
	}, nil
}

func (s *IdentityProviderService) jitProvisionOrLink(ctx context.Context, idp *domain.IdentityProvider, subject, email string, emailVerified bool, groups []string, name string) (*domain.User, string, error) {
	existing, err := s.fedIdentRepo.GetByIdPAndSubject(ctx, idp.ID, subject)
	if err != nil && !errors.Is(err, apperrors.NotFound) {
		return nil, "", apperrors.Wrap(apperrors.Internal, "failed to check existing federated identity", err)
	}

	if existing != nil {
		existing.LastLoginAt = time.Now()
		existing.Groups = groups
		existing.Email = email
		existing.EmailVerified = emailVerified
		if updateErr := s.fedIdentRepo.Update(ctx, existing); updateErr != nil {
			s.logger.Warn("failed to update federated identity", "error", updateErr)
		}

		user, err := s.userRepo.GetByID(ctx, existing.UserID)
		if err != nil {
			return nil, "", apperrors.Wrap(apperrors.Internal, "failed to fetch user for federated identity", err)
		}
		return user, "", nil
	}

	localUser, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, apperrors.NotFound) {
		return nil, "", apperrors.Wrap(apperrors.Internal, "failed to check existing user by email", err)
	}

	var user *domain.User
	if localUser != nil {
		user = localUser
	} else {
		user, err = s.createJITUser(ctx, idp, email, name, groups)
		if err != nil {
			return nil, "", err
		}
	}

	fedIdent := &domain.FederatedIdentity{
		ID:            uuid.New(),
		UserID:        user.ID,
		IdPID:         idp.ID,
		Subject:       subject,
		Email:         email,
		EmailVerified: emailVerified,
		Groups:        groups,
		LastLoginAt:   time.Now(),
		CreatedAt:     time.Now(),
	}
	if err := s.fedIdentRepo.Create(ctx, fedIdent); err != nil {
		s.logger.Warn("failed to create federated identity link", "error", err)
	}

	if err := s.auditSvc.Log(ctx, user.ID, "user.federated_login", "user", user.ID.String(), map[string]interface{}{
		"idp_id": idp.ID.String(),
		"idp_type": idp.Type,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	key, err := s.apiKeySvc.CreateKey(ctx, user.ID, fmt.Sprintf("SSO Key (%s)", idp.Name))
	if err != nil {
		s.logger.Warn("failed to create SSO API key", "error", err)
		return user, "", nil
	}

	return user, key.Key, nil
}

func (s *IdentityProviderService) createJITUser(ctx context.Context, idp *domain.IdentityProvider, email, name string, groups []string) (*domain.User, error) {
	role := idp.DefaultRole
	if role == "" {
		role = domain.RoleDeveloper
	}
	for _, mapping := range idp.GroupMapping {
		for _, g := range groups {
			if g == mapping.IdPGroup {
				role = mapping.CloudRole
				break
			}
		}
	}

	var tenantID uuid.UUID
	if idp.Scope == domain.IdPScopeTenant && idp.TenantID != nil {
		tenantID = *idp.TenantID
	} else {
		tenant, err := s.tenantSvc.GetDefaultTenant(ctx)
		if err != nil {
			tenant, err = s.tenantSvc.CreateTenant(ctx, "Default Tenant", "default-"+uuid.New().String()[:8], uuid.Nil)
		}
		if err != nil {
			return nil, apperrors.Wrap(apperrors.Internal, "failed to get/create default tenant", err)
		}
		tenantID = tenant.ID
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		Name:         name,
		Role:         role,
		TenantID:     tenantID,
		DefaultTenantID: &tenantID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, apperrors.Wrap(apperrors.Internal, "failed to create JIT user", err)
	}

	if idp.Scope == domain.IdPScopeTenant && idp.TenantID != nil {
		if err := s.tenantSvc.AddMember(ctx, tenantID, user.ID, role); err != nil {
			s.logger.Warn("failed to add JIT user to tenant", "error", err)
		}
	}

	return user, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

func (s *IdentityProviderService) exchangeCode(ctx context.Context, tokenEndpoint, code, clientID, clientSecret, redirectURI string) (*tokenResponse, error) {
	data := map[string]string{
		"grant_type":   "authorization_code",
		"code":         code,
		"client_id":    clientID,
		"client_secret": clientSecret,
		"redirect_uri": redirectURI,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, encodeFormData(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange returned status %d", resp.StatusCode)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

type samlAssertion struct {
	Subject       string
	Email         string
	EmailVerified bool
	Groups        []string
	Name          string
}

func (s *IdentityProviderService) parseSAMLAssertion(ctx context.Context, assertionXML []byte, idp *domain.IdentityProvider) (*samlAssertion, error) {
	return nil, errors.New("SAML parsing not implemented - requires crewjam/saml dependency")
}

func encodeFormData(data map[string]string) *strings.Reader {
	form := make([]string, 0, len(data))
	for k, v := range data {
		form = append(form, k+"="+v)
	}
	return strings.NewReader(strings.Join(form, "&"))
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(s)
}