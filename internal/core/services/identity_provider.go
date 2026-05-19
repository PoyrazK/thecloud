package services

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/beevik/etree"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	apperrors "github.com/poyrazk/thecloud/internal/errors"
	"github.com/russellhaering/goxmldsig"
)

// jwkKey represents a single JWK key.
type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwksResponse represents the JWKS endpoint response.
type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

// jwksCache caches JWKS per IdP with TTL.
type jwksCache struct {
	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey // kid -> key
	expires time.Time
	idpID   uuid.UUID
}

func newJwksCache(idpID uuid.UUID) *jwksCache {
	return &jwksCache{keys: make(map[string]*rsa.PublicKey), idpID: idpID}
}

func (c *jwksCache) isExpired() bool { return time.Now().After(c.expires) }

type IdentityProviderService struct {
	idpRepo      ports.IdentityProviderRepository
	fedIdentRepo ports.FederatedIdentityRepository
	userRepo     ports.UserRepository
	tenantSvc    ports.TenantService
	tenantRepo   ports.TenantRepository
	auditSvc     ports.AuditService
	apiKeySvc    ports.IdentityService
	httpClient   *http.Client
	logger       *slog.Logger
	jwksCache    map[uuid.UUID]*jwksCache
	jwksMu       sync.Mutex
}

type IdentityProviderServiceParams struct {
	IdPRepo      ports.IdentityProviderRepository
	FedIdentRepo ports.FederatedIdentityRepository
	UserRepo     ports.UserRepository
	TenantSvc    ports.TenantService
	TenantRepo   ports.TenantRepository
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
		tenantRepo:   params.TenantRepo,
		auditSvc:     params.AuditSvc,
		apiKeySvc:    params.APIKeySvc,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		logger:       params.Logger,
		jwksCache:   make(map[uuid.UUID]*jwksCache),
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

func (s *IdentityProviderService) HandleOIDCCallback(ctx context.Context, code, pkceVerifier string, idpID uuid.UUID) (*domain.User, string, error) {
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

	tokenResp, err := s.exchangeCode(ctx, discovery.TokenEndpoint, code, idp.ClientID, idp.ClientSecret, idp.RedirectURIs[0], pkceVerifier)
	if err != nil {
		return nil, "", apperrors.Wrap(apperrors.Unauthorized, "token exchange failed", err)
	}

	userInfo, err := s.ValidateOIDCToken(ctx, tokenResp.IDToken, idp, discovery.JwksURI)
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

// fetchJWKS retrieves and caches JWKS from the given URI.
func (s *IdentityProviderService) fetchJWKS(ctx context.Context, jwksURI string, idpID uuid.UUID) error {
	s.jwksMu.Lock()
	cache, exists := s.jwksCache[idpID]
	if !exists {
		cache = newJwksCache(idpID)
		s.jwksCache[idpID] = cache
	}
	if !cache.isExpired() {
		s.jwksMu.Unlock()
		return nil
	}
	s.jwksMu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS fetch returned status %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()
	for _, key := range jwks.Keys {
		if key.Kty != "RSA" {
			continue
		}
		pubKey, err := parseRSAPublicKey(key.N, key.E)
		if err != nil {
			s.logger.Warn("failed to parse JWK", "kid", key.Kid, "error", err)
			continue
		}
		cache.keys[key.Kid] = pubKey
	}
	cache.expires = time.Now().Add(1 * time.Hour)
	return nil
}

// parseRSAPublicKey decodes n and e base64url big-endian integers into *rsa.PublicKey.
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.URLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode N: %w", err)
	}
	eBytes, err := base64.URLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode E: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

// ValidateOIDCToken validates an OIDC ID token signature using JWKS.
func (s *IdentityProviderService) ValidateOIDCToken(ctx context.Context, rawIDToken string, idp *domain.IdentityProvider, jwksURI string) (*ports.OIDCUserInfo, error) {
	parts := strings.Split(rawIDToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format")
	}

	// Parse header to get kid
	headerBytes, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT header: %w", err)
	}
	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to parse JWT header: %w", err)
	}

	// Fetch JWKS and get the key
	if err := s.fetchJWKS(ctx, jwksURI, idp.ID); err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	s.jwksMu.Lock()
	cache := s.jwksCache[idp.ID]
	s.jwksMu.Unlock()

	cache.mu.RLock()
	pubKey, ok := cache.keys[header.Kid]
	cache.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("key with kid %s not found in JWKS", header.Kid)
	}

	// Verify signature: header.payload using RS256
	signedData := parts[0] + "." + parts[1]
	sigBytes, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	h := sha256.New()
	h.Write([]byte(signedData))
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, h.Sum(nil), sigBytes); err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	// Parse claims
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
		Exp           int64    `json:"exp"`
		Iat           int64    `json:"iat"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}

	if claims.Issuer != idp.IssuerURL {
		return nil, errors.New("token issuer mismatch")
	}

	// Validate audience
	if claims.Audience != nil {
		audValid := false
		switch aud := claims.Audience.(type) {
		case string:
			audValid = aud == idp.ClientID
		case []any:
			for _, a := range aud {
				if s, ok := a.(string); ok && s == idp.ClientID {
					audValid = true
					break
				}
			}
		}
		if !audValid {
			return nil, errors.New("token audience mismatch")
		}
	}

	// Validate expiration
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, errors.New("token expired")
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
	if err != nil && !apperrors.Is(err, apperrors.NotFound) {
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
	if err != nil && !apperrors.Is(err, apperrors.NotFound) {
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
		return user, "", apperrors.Wrap(apperrors.Internal, "failed to link federated identity", err)
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
		// Look for an existing default tenant via the tenant service
		tenants, err := s.tenantSvc.ListUserTenants(ctx, uuid.Nil)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.Internal, "failed to list tenants", err)
		}
		var tenant *domain.Tenant
		for _, t := range tenants {
			if t.Slug == "default" {
				tenant = &t
				break
			}
		}
		if tenant == nil {
			// Create a default tenant
			tenant, err = s.tenantSvc.CreateTenant(ctx, "Default Tenant", "default-"+uuid.New().String()[:8], uuid.Nil)
			if err != nil {
				return nil, apperrors.Wrap(apperrors.Internal, "failed to create default tenant", err)
			}
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
		if err := s.tenantRepo.AddMember(ctx, tenantID, user.ID, role); err != nil {
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

func (s *IdentityProviderService) exchangeCode(ctx context.Context, tokenEndpoint, code, clientID, clientSecret, redirectURI, pkceVerifier string) (*tokenResponse, error) {
	data := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"client_id":     clientID,
		"client_secret": clientSecret,
		"redirect_uri":  redirectURI,
	}
	if pkceVerifier != "" {
		data["code_verifier"] = pkceVerifier
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
	if idp.Certificate == "" {
		return nil, errors.New("SAML IdP certificate is required for assertion validation")
	}

	// Parse the IdP's certificate
	certBlock, _ := pem.Decode([]byte(idp.Certificate))
	if certBlock == nil {
		return nil, errors.New("failed to parse IdP certificate PEM")
	}
	idpCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IdP certificate: %w", err)
	}

	// Create validation context with IdP certificate
	certStore := dsig.MemoryX509CertificateStore{
		Roots: []*x509.Certificate{idpCert},
	}
	validationContext := dsig.NewDefaultValidationContext(&certStore)

	// Parse XML into etree for signature validation
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(assertionXML); err != nil {
		return nil, fmt.Errorf("failed to parse assertion XML: %w", err)
	}
	root := doc.Root()

	// Validate the signature using goxmldsig
	_, err = validationContext.Validate(root)
	if err != nil {
		return nil, fmt.Errorf("SAML signature validation failed: %w", err)
	}

	// Extract assertion from XML
	assertion, err := extractSAMLAttributes(string(assertionXML))
	if err != nil {
		return nil, fmt.Errorf("failed to extract SAML attributes: %w", err)
	}

	if assertion.Email == "" && assertion.Subject != "" {
		assertion.Email = assertion.Subject
	}

	return &assertion, nil
}

// extractSAMLAttributes extracts user attributes from a SAML assertion XML string.
func extractSAMLAttributes(xmlStr string) (samlAssertion, error) {
	var assertion samlAssertion

	decoder := strings.NewReader(xmlStr)
	dec := xml.NewDecoder(decoder)

	for {
		token, err := dec.Token()
		if err != nil {
			break
		}

		switch se := token.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "NameID":
				if data, err := dec.Token(); err == nil {
					if cd, ok := data.(xml.CharData); ok {
						assertion.Subject = strings.TrimSpace(string(cd))
					}
				}
			case "Attribute":
				var attrName string
				for _, attr := range se.Attr {
					if attr.Name.Local == "Name" {
						attrName = attr.Value
						break
					}
				}
				// Read the AttributeValue content
				for {
					tok, err := dec.Token()
					if err != nil {
						break
					}
					if end, ok := tok.(xml.EndElement); ok && end.Name.Local == "Attribute" {
						break
					}
					if cd, ok := tok.(xml.CharData); ok {
						val := strings.TrimSpace(string(cd))
						if val == "" {
							continue
						}
						switch strings.ToLower(attrName) {
						case "email", "emailaddress", "mail":
							assertion.Email = val
						case "name", "displayname", "cn", "givenname":
							assertion.Name = val
						case "groups", "memberof", "role", "member":
							assertion.Groups = strings.Split(val, ";")
						}
					}
				}
			}
		}
	}

	return assertion, nil
}

func encodeFormData(data map[string]string) *strings.Reader {
	form := make([]string, 0, len(data))
	for k, v := range data {
		form = append(form, k+"="+v)
	}
	return strings.NewReader(strings.Join(form, "&"))
}

func base64URLDecode(s string) ([]byte, error) {
	// Add padding if necessary
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

// GeneratePKCEPair generates a code verifier and code challenge for PKCE.
func GeneratePKCEPair() (verifier string, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.URLEncoding.EncodeToString(b)[:32]
	h := sha256.New()
	h.Write([]byte(verifier))
	challenge = base64.URLEncoding.EncodeToString(h.Sum(nil))
	return verifier, challenge, nil
}

// GenerateState generates a random state parameter for CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}