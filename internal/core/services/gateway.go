// Package services implements core business workflows.
package services

import (
	"context"
	cryptoRand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/internal/routing"
	"golang.org/x/sync/singleflight"
)

// GatewayService manages API gateway routes and reverse proxies.
type GatewayService struct {
	repo               ports.GatewayRepository
	rbacSvc            ports.RBACService
	proxyMu            sync.RWMutex
	proxies            map[uuid.UUID]*httputil.ReverseProxy
	routes             []*domain.GatewayRoute
	matchers           map[uuid.UUID]*routing.PatternMatcher
	auditSvc           ports.AuditService
	logger             *slog.Logger
	jwksMu             sync.RWMutex
	jwksCache          map[string]*jwksCacheEntry
	httpClient         *http.Client
	jwksCircuitBreaker *platform.CircuitBreaker
	jwksInFlight       singleflight.Group
}

// NewGatewayService constructs a GatewayService and loads existing routes.
func NewGatewayService(repo ports.GatewayRepository, rbacSvc ports.RBACService, auditSvc ports.AuditService, logger *slog.Logger) *GatewayService {
	s := &GatewayService{
		repo:       repo,
		rbacSvc:    rbacSvc,
		proxies:    make(map[uuid.UUID]*httputil.ReverseProxy),
		routes:     make([]*domain.GatewayRoute, 0),
		matchers:   make(map[uuid.UUID]*routing.PatternMatcher),
		auditSvc:   auditSvc,
		logger:     logger,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	s.jwksCircuitBreaker = platform.NewCircuitBreakerWithOpts(platform.CircuitBreakerOpts{
		Name:         "jwks-fetch",
		Threshold:    3,
		ResetTimeout: 30 * time.Second,
		OnStateChange: func(name string, from, to platform.State) {
			platform.JWKSBreakerState.Set(float64(to))
			if s.logger != nil {
				s.logger.Warn("JWKS circuit breaker state change", "name", name, "from", from.String(), "to", to.String())
			}
		},
	})
	// Initial load
	if err := s.RefreshRoutes(context.Background()); err != nil {
		s.logger.Error("failed to refresh routes on startup", "error", err)
	}
	return s
}

func (s *GatewayService) CreateRoute(ctx context.Context, params ports.CreateRouteParams) (*domain.GatewayRoute, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionGatewayCreate, "*"); err != nil {
		return nil, err
	}

	// Detect if it's a pattern or prefix
	patternType := "prefix"
	var paramNames []string
	if strings.Contains(params.Pattern, "{") || strings.Contains(params.Pattern, "*") {
		patternType = "pattern"
		matcher, err := routing.CompilePattern(params.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %w", err)
		}
		paramNames = matcher.ParamNames
	}

	route := &domain.GatewayRoute{
		ID:                      uuid.New(),
		UserID:                  userID,
		TenantID:                tenantID,
		Name:                    params.Name,
		PathPrefix:              params.Pattern,
		PathPattern:             params.Pattern,
		PatternType:             patternType,
		ParamNames:              paramNames,
		TargetURL:               params.Target,
		Methods:                 params.Methods,
		StripPrefix:             params.StripPrefix,
		RateLimit:               params.RateLimit,
		DialTimeout:             params.DialTimeout,
		ResponseHeaderTimeout:   params.ResponseHeaderTimeout,
		IdleConnTimeout:         params.IdleConnTimeout,
		TLSSkipVerify:           params.TLSSkipVerify,
		RequireTLS:              params.RequireTLS,
		AllowedCIDRs:            params.AllowedCIDRs,
		BlockedCIDRs:            params.BlockedCIDRs,
		MaxBodySize:             params.MaxBodySize,
		CircuitBreakerThreshold: params.CircuitBreakerThreshold,
		CircuitBreakerTimeout:   params.CircuitBreakerTimeout,
		MaxRetries:              params.MaxRetries,
		RetryTimeout:            params.RetryTimeout,
		Priority:                params.Priority,
		AllowedOrigins:          params.AllowedOrigins,
		AllowedMethods:          params.AllowedMethods,
		AllowedHeaders:          params.AllowedHeaders,
		ExposeHeaders:           params.ExposeHeaders,
		MaxAge:                  params.MaxAge,
		StripResponseHeaders:    params.StripResponseHeaders,
		Compression:             params.Compression,
		JWTIssuer:               params.JWTIssuer,
		JWTJwksURL:              params.JWTJwksURL,
		JWTAudience:             params.JWTAudience,
		ClientCert:              params.ClientCert,
		ClientKey:               params.ClientKey,
		CACert:                  params.CACert,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}

	// Apply default values for resilience parameters
	if route.CircuitBreakerThreshold == 0 {
		route.CircuitBreakerThreshold = 5
	}
	if route.CircuitBreakerTimeout == 0 {
		route.CircuitBreakerTimeout = 30000 // ms
	}
	if route.MaxRetries == 0 {
		route.MaxRetries = 2
	}
	if route.RetryTimeout == 0 {
		route.RetryTimeout = 5000 // ms
	}

	// Validate CIDRs before saving
	for _, cidr := range route.AllowedCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return nil, errors.New(errors.InvalidInput, fmt.Sprintf("invalid allowed CIDR %q: %v", cidr, err))
		}
	}
	for _, cidr := range route.BlockedCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return nil, errors.New(errors.InvalidInput, fmt.Sprintf("invalid blocked CIDR %q: %v", cidr, err))
		}
	}

	// Pre-parse CIDRs into []*net.IPNet for fast per-request matching
	for _, cidr := range route.AllowedCIDRs {
		_, ipNet, _ := net.ParseCIDR(cidr) // err already nil per validation above
		route.AllowedIPNets = append(route.AllowedIPNets, ipNet)
	}
	for _, cidr := range route.BlockedCIDRs {
		_, ipNet, _ := net.ParseCIDR(cidr)
		route.BlockedIPNets = append(route.BlockedIPNets, ipNet)
	}

	if err := s.repo.CreateRoute(ctx, route); err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, route.UserID, "gateway.route_create", "gateway", route.ID.String(), map[string]interface{}{
		"name":    route.Name,
		"pattern": route.PathPattern,
		"methods": route.Methods,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "gateway.route_create", "route_id", route.ID, "error", err)
	}

	if err := s.RefreshRoutes(ctx); err != nil {
		s.logger.Warn("failed to refresh routes after create", "route_id", route.ID, "error", err)
	}
	return route, nil
}

func (s *GatewayService) ListRoutes(ctx context.Context) ([]*domain.GatewayRoute, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionGatewayRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListRoutes(ctx, userID)
}

func (s *GatewayService) DeleteRoute(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionGatewayDelete, id.String()); err != nil {
		return err
	}

	route, err := s.repo.GetRouteByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteRoute(ctx, id); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, route.UserID, "gateway.route_delete", "gateway", route.ID.String(), map[string]interface{}{
		"name": route.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "gateway.route_delete", "route_id", id, "error", err)
	}

	return s.RefreshRoutes(ctx)
}

func (s *GatewayService) RefreshRoutes(ctx context.Context) error {
	routes, err := s.repo.GetAllActiveRoutes(ctx)
	if err != nil {
		return err
	}

	newProxies := make(map[uuid.UUID]*httputil.ReverseProxy)
	newMatchers := make(map[uuid.UUID]*routing.PatternMatcher)

	for _, r := range routes {
		proxy, err := s.createReverseProxy(r)
		if err != nil {
			s.logger.Error("failed to create reverse proxy for route", "route_id", r.ID, "route_name", r.Name, "target_url", r.TargetURL, "error", err)
			continue
		}

		// Pre-parse CIDRs for fast per-request matching
		for _, cidr := range r.AllowedCIDRs {
			_, ipNet, _ := net.ParseCIDR(cidr)
			r.AllowedIPNets = append(r.AllowedIPNets, ipNet)
		}
		for _, cidr := range r.BlockedCIDRs {
			_, ipNet, _ := net.ParseCIDR(cidr)
			r.BlockedIPNets = append(r.BlockedIPNets, ipNet)
		}

		newProxies[r.ID] = proxy
		if r.PatternType == "pattern" {
			matcher, err := routing.CompilePattern(r.PathPattern)
			if err == nil {
				newMatchers[r.ID] = matcher
			}
		}
	}

	s.sortRoutes(routes)

	s.proxyMu.Lock()
	s.proxies = newProxies
	s.routes = routes
	s.matchers = newMatchers
	s.proxyMu.Unlock()

	return nil
}

func (s *GatewayService) createReverseProxy(route *domain.GatewayRoute) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(route.TargetURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Configure custom transport with timeouts and TLS
	tlsConfig, err := s.buildTLSConfig(route)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}

	dialTimeout := time.Duration(route.DialTimeout) * time.Millisecond
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	responseHeaderTimeout := time.Duration(route.ResponseHeaderTimeout) * time.Millisecond
	if responseHeaderTimeout <= 0 {
		responseHeaderTimeout = 30 * time.Second
	}
	idleConnTimeout := time.Duration(route.IdleConnTimeout) * time.Millisecond
	if idleConnTimeout <= 0 {
		idleConnTimeout = 90 * time.Second
	}

	baseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: responseHeaderTimeout,
		IdleConnTimeout:       idleConnTimeout,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   10 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
	}

	proxy.Transport = newRetryTransport(baseTransport, route, s.logger)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		if route.StripPrefix {
			prefix := route.PathPrefix
			if route.PatternType == "pattern" {
				prefix = routing.GetLiteralPrefix(route.PathPattern)
			}
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/gw"+prefix)
			if !strings.HasPrefix(req.URL.Path, "/") {
				req.URL.Path = "/" + req.URL.Path
			}
		}
		originalDirector(req)
		req.Host = target.Host
	}

	return proxy, nil
}

func (s *GatewayService) buildTLSConfig(route *domain.GatewayRoute) (*tls.Config, error) {
	cfg := &tls.Config{
		InsecureSkipVerify: route.TLSSkipVerify, //nolint:gosec // User-controlled option for development/testing
	}
	// Always set baseline TLS 1.2, raise to 1.3 if RequireTLS
	cfg.MinVersion = tls.VersionTLS12
	if route.RequireTLS {
		cfg.MinVersion = tls.VersionTLS13
	}
	// mTLS: load client certificate if provided
	if route.ClientCert != "" && route.ClientKey != "" {
		cert, err := tls.X509KeyPair([]byte(route.ClientCert), []byte(route.ClientKey))
		if err != nil {
			return nil, fmt.Errorf("invalid client cert/key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	// CA cert for backend verification
	if route.CACert != "" {
		cfg.RootCAs = x509.NewCertPool()
		if !cfg.RootCAs.AppendCertsFromPEM([]byte(route.CACert)) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
	}
	return cfg, nil
}

func (s *GatewayService) sortRoutes(routes []*domain.GatewayRoute) {
	// Sort routes by specificity (longer literal prefixes and higher priority first)
	sort.Slice(routes, func(i, j int) bool {
		scoreI := calculateMatchScore(routes[i], "")
		scoreJ := calculateMatchScore(routes[j], "")
		return scoreI > scoreJ // Descending order
	})
}

type jwksCacheEntry struct {
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func (s *GatewayService) getJWKS(url string) (map[string]*rsa.PublicKey, error) {
	s.jwksMu.Lock()
	if entry, ok := s.jwksCache[url]; ok && time.Since(entry.fetchedAt) < 5*time.Minute {
		s.jwksMu.Unlock()
		return entry.keys, nil
	}
	s.jwksMu.Unlock()

	// Use singleflight to dedupe concurrent requests for the same URL
	val, err, _ := s.jwksInFlight.Do(url, func() (any, error) {
		s.jwksMu.Lock()
		// Double-check after acquiring lock
		if entry, ok := s.jwksCache[url]; ok && time.Since(entry.fetchedAt) < 5*time.Minute {
			s.jwksMu.Unlock()
			return entry.keys, nil
		}
		s.jwksMu.Unlock()

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		var resp *http.Response
		if cbErr := s.jwksCircuitBreaker.Execute(func() error {
			resp, err = s.httpClient.Do(req) //nolint:bodyclose
			return err
		}); cbErr != nil {
			platform.JWKSFetchTotal.WithLabelValues("circuit_open").Inc()
			return nil, cbErr
		}
		defer func() {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
		}()

		// Only cache successful JWKS responses
		if resp.StatusCode != http.StatusOK {
			platform.JWKSFetchTotal.WithLabelValues("error").Inc()
			return nil, fmt.Errorf("JWKS fetch returned status %d", resp.StatusCode)
		}

		var jwks struct {
			Keys []map[string]any `json:"keys"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
			platform.JWKSFetchTotal.WithLabelValues("error").Inc()
			return nil, err
		}

		keys := make(map[string]*rsa.PublicKey)
		for _, k := range jwks.Keys {
			if kid, ok := k["kid"].(string); ok {
				if kty, _ := k["kty"].(string); kty == "RSA" {
					if pubKey, err := parseRSAPublicKeyFromJWK(k); err == nil {
						keys[kid] = pubKey
					}
				}
			}
		}
		// If JWKS had keys but none parsed successfully, don't cache an empty result
		if len(keys) == 0 && len(jwks.Keys) > 0 {
			platform.JWKSFetchTotal.WithLabelValues("error").Inc()
			return nil, fmt.Errorf("JWKS returned %d keys but none were valid RSA keys", len(jwks.Keys))
		}

		platform.JWKSFetchTotal.WithLabelValues("success").Inc()
		s.jwksMu.Lock()
		if s.jwksCache == nil {
			s.jwksCache = make(map[string]*jwksCacheEntry)
		}
		s.jwksCache[url] = &jwksCacheEntry{keys: keys, fetchedAt: time.Now()}
		s.jwksMu.Unlock()
		return keys, nil
	})
	if err != nil {
		return nil, err
	}
	return val.(map[string]*rsa.PublicKey), nil
}

// parseRSAPublicKeyFromJWK parses an RSA public key from a JWK map.
func parseRSAPublicKeyFromJWK(k map[string]any) (*rsa.PublicKey, error) {
	nVal, ok := k["n"].(string)
	if !ok {
		return nil, fmt.Errorf("JWK missing 'n'")
	}
	eVal, ok := k["e"].(string)
	if !ok {
		return nil, fmt.Errorf("JWK missing 'e'")
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(nVal)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eVal)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	// e is typically 65537 (0x10001) which encodes as "AQAB" in base64url
	e := 0
	for _, b := range eBytes {
		if e >= 1<<30 { // exponents > 2^30 are invalid for RSA
			return nil, fmt.Errorf("exponent too large")
		}
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

func (s *GatewayService) ValidateJWT(ctx context.Context, route *domain.GatewayRoute, tokenString string) (map[string]string, error) {
	if route.JWTJwksURL == "" || tokenString == "" {
		return nil, nil
	}

	keys, err := s.getJWKS(route.JWTJwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}

	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		// Only allow RSA signature algorithms to prevent algorithm confusion attacks
		switch t.Method.Alg() {
		case "RS256", "RS384", "RS512":
			// OK
		default:
			return nil, fmt.Errorf("unexpected signing method: %v", t.Method.Alg())
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("kid not found in token header")
		}
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("key %s not found in JWKS", kid)
		}
		return key, nil
	})
	if err != nil {
		// Return generic error to avoid leaking timing info about signature validation
		return nil, fmt.Errorf("invalid token")
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	claims, _ := token.Claims.(jwt.MapClaims)
	if route.JWTIssuer != "" {
		if iss, _ := claims["iss"].(string); iss != route.JWTIssuer {
			return nil, fmt.Errorf("invalid issuer")
		}
	}
	if route.JWTAudience != "" {
		if !verifyAudience(claims, route.JWTAudience) {
			return nil, fmt.Errorf("invalid audience")
		}
	}

	result := make(map[string]string)
	for k, v := range claims {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result, nil
}

// verifyAudience checks if the given audience claim matches the expected audience.
// Handles both string and []interface{} audience claims per RFC 7519.
func verifyAudience(claims jwt.MapClaims, expected string) bool {
	aud, ok := claims["aud"]
	if !ok {
		return false
	}
	switch v := aud.(type) {
	case string:
		return v == expected
	case []interface{}:
		for _, a := range v {
			if s, ok := a.(string); ok && s == expected {
				return true
			}
		}
	}
	return false
}

// ProxyHandler is handled in the API layer for now

func (s *GatewayService) GetProxy(method, path string) (*httputil.ReverseProxy, *domain.GatewayRoute, map[string]string, bool) {
	s.proxyMu.RLock()
	defer s.proxyMu.RUnlock()

	var bestMatch *domain.RouteMatch

	for _, route := range s.routes {
		match := s.checkRouteMatch(route, method, path)
		if match != nil {
			if bestMatch == nil || match.MatchScore > bestMatch.MatchScore {
				bestMatch = match
			}
		}
	}

	if bestMatch != nil {
		proxy := s.proxies[bestMatch.Route.ID]
		if proxy == nil {
			s.logger.Error("proxy not found for matched route, possible proxy creation failure",
				"route_id", bestMatch.Route.ID.String(),
				"route_name", bestMatch.Route.Name,
				"path", path)
			return nil, nil, nil, false
		}
		return proxy, bestMatch.Route, bestMatch.Params, true
	}

	return nil, nil, nil, false
}

func (s *GatewayService) checkRouteMatch(route *domain.GatewayRoute, method, path string) *domain.RouteMatch {
	// 1. Method filter
	if !s.isMethodAllowed(route, method) {
		return nil
	}

	// 2. Path matching
	if route.PatternType == "pattern" {
		matcher, ok := s.matchers[route.ID]
		if ok {
			if params, ok := matcher.Match(path); ok {
				return &domain.RouteMatch{
					Route:      route,
					Params:     params,
					MatchScore: calculateMatchScore(route, path),
				}
			}
		}
	} else if strings.HasPrefix(path, route.PathPrefix) {
		return &domain.RouteMatch{
			Route:      route,
			Params:     nil,
			MatchScore: calculateMatchScore(route, path),
		}
	}

	return nil
}

func (s *GatewayService) isMethodAllowed(route *domain.GatewayRoute, method string) bool {
	if len(route.Methods) == 0 {
		return true
	}
	for _, m := range route.Methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

func calculateMatchScore(route *domain.GatewayRoute, _ string) int {
	// 1. Literal prefix length is a good indicator of specificity
	score := len(routing.GetLiteralPrefix(route.PathPattern))

	// 2. Bonus for exact matches (no parameters or wildcards)
	if !strings.ContainsAny(route.PathPattern, "{*") {
		score += 100
	}

	// 3. Priority is the strongest signal if provided
	if route.Priority > 0 {
		score += route.Priority * 1000
	}

	return score
}

// retryTransport wraps an http.Transport with circuit breaker and retry logic.
type retryTransport struct {
	base         http.RoundTripper
	cb           *platform.CircuitBreaker // nil if circuit breaker is disabled
	maxRetries   int
	retryTimeout time.Duration
	logger       *slog.Logger
	routeID      string
	// fastFailThreshold prevents retry storms when upstream is unreachable.
	// When >0, consecutive connection errors exceeding this count trips the
	// circuit breaker immediately (bypassing normal failure counting).
	fastFailThreshold      int
	consecutiveConnErrors atomic.Int32
}

// retryableStatusError wraps a response returned when retries are exhausted
// on a retryable status code. It allows the circuit breaker to count the
// failure while still returning the response to the caller.
type retryableStatusError struct {
	resp *http.Response
}

func (e *retryableStatusError) Error() string {
	if e.resp == nil {
		return "retryable status exhausted"
	}
	return fmt.Sprintf("retryable status exhausted: %d", e.resp.StatusCode)
}

// newRetryTransport wraps a base http.Transport with per-route retry and circuit breaker behavior.
func newRetryTransport(base http.RoundTripper, route *domain.GatewayRoute, logger *slog.Logger) *retryTransport {
	rt := &retryTransport{
		base:              base,
		maxRetries:        route.MaxRetries,
		retryTimeout:      time.Duration(route.RetryTimeout) * time.Millisecond,
		logger:            logger,
		routeID:           route.ID.String(),
		fastFailThreshold: route.CircuitBreakerThreshold,
	}
	if route.CircuitBreakerThreshold > 0 {
		rt.cb = platform.NewCircuitBreakerWithOpts(platform.CircuitBreakerOpts{
			Name:         route.ID.String(),
			Threshold:    route.CircuitBreakerThreshold,
			ResetTimeout: time.Duration(route.CircuitBreakerTimeout) * time.Millisecond,
			OnStateChange: func(name string, from, to platform.State) {
				platform.GatewayCircuitBreakerState.WithLabelValues(name).Set(float64(to))
				if logger != nil {
					logger.Warn("circuit breaker state change",
						"route_id", name,
						"from", from.String(),
						"to", to.String())
				}
			},
		})
		platform.GatewayCircuitBreakerState.WithLabelValues(route.ID.String()).Set(float64(platform.StateClosed))
	}
	return rt
}

// RoundTrip implements http.RoundTripper.
func (rt *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.cb == nil {
		resp, err := rt.doRoundTrip(req)
		var se *retryableStatusError
		if stderrors.As(err, &se) && se.resp != nil {
			return se.resp, nil //nolint:bodyclose
		}
		return resp, err
	}

	type result struct {
		resp *http.Response
		err  error
	}
	var r result
	cbErr := rt.cb.Execute(func() error {
		r.resp, r.err = rt.doRoundTrip(req) //nolint:bodyclose
		return r.err
	})
	if cbErr != nil {
		return nil, cbErr
	}
	if r.err != nil {
		var se *retryableStatusError
		if stderrors.As(r.err, &se) && se.resp != nil {
			return se.resp, nil //nolint:bodyclose
		}
		return nil, r.err
	}
	return r.resp, nil //nolint:bodyclose
}

func (rt *retryTransport) doRoundTrip(req *http.Request) (*http.Response, error) {
	if rt.maxRetries <= 0 || !rt.isIdempotent(req.Method) {
		return rt.base.RoundTrip(req)
	}

	var lastResp *http.Response
	var lastErr error
	maxAttempts := rt.maxRetries + 1 // first attempt + retries
	start := time.Now()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Check overall retry window
		if rt.retryTimeout > 0 && time.Since(start) >= rt.retryTimeout {
			break
		}
		if attempt > 0 {
			platform.GatewayRetryTotal.WithLabelValues(rt.routeID, "retry").Inc()
			delay := rt.backoffWithJitter(attempt)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay):
			}
		}

		resp, err := rt.base.RoundTrip(req)
		if err == nil {
			// Reset consecutive error counter on success
			rt.consecutiveConnErrors.Store(0)
			if !rt.isRetryableStatus(resp.StatusCode) {
				// Drain body before returning so connection can be reused
				if resp.Body != nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
				return resp, nil //nolint:bodyclose
			}
			// drain and close body so connection can be reused, then retry
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastResp = resp
			continue
		}

		if !rt.isRetryableError(err) {
			// Drain body before returning so connection can be reused
			if resp != nil && resp.Body != nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
			// Reset consecutive errors on non-retryable error (upstream responded)
			rt.consecutiveConnErrors.Store(0)
			return nil, err
		}

		// Fast-fail: if we hit too many consecutive connection errors, trip the
		// circuit breaker immediately to prevent retry storms.
		if rt.cb != nil && rt.fastFailThreshold > 0 {
			if rt.consecutiveConnErrors.Add(1) >= int32(rt.fastFailThreshold) {
				platform.GatewayRetryTotal.WithLabelValues(rt.routeID, "fast_fail").Inc()
				// Trip the circuit breaker open immediately
				rt.cb.RecordFailure()
				return nil, fmt.Errorf("fast-fail: too many consecutive connection errors: %w", err)
			}
		}

		lastErr = err
		lastResp = resp

		// For idempotent methods with a replayable body, clone the request before retry.
		// This ensures subsequent attempts get a fresh body.
		if attempt < maxAttempts-1 && req.GetBody != nil {
			body, err := req.GetBody()
			if err == nil {
				req = req.Clone(req.Context())
				req.Body = body
			}
		}
	}

	// If we exhausted retries on a retryable status, return a wrapped error
	// so the circuit breaker can count this as a failure. The response is
	// unwrapped and returned to the caller in RoundTrip.
	if lastResp != nil && rt.isRetryableStatus(lastResp.StatusCode) {
		platform.GatewayRetryTotal.WithLabelValues(rt.routeID, "exhausted").Inc()
		return nil, &retryableStatusError{resp: lastResp}
	}
	// lastResp may have an undrained body from a network error — drain before returning
	if lastResp != nil && lastResp.Body != nil {
		_, _ = io.Copy(io.Discard, lastResp.Body)
		lastResp.Body.Close()
	}
	// If we have a last error, return it; otherwise return the last response
	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil //nolint:bodyclose
}

func (rt *retryTransport) isRetryableStatus(code int) bool {
	return code == 502 || code == 503 || code == 504 || code == 429
}

func (rt *retryTransport) isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	// Use net.Error interface for robust detection of transient errors
	var netErr net.Error
	if stderrors.As(err, &netErr) {
		// Use both Temporary() and Timeout() - connection refused has Temporary()=true
		return netErr.Temporary() || netErr.Timeout()
	}
	// Fallback to string matching for errors not wrapped as net.Error
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "reset by peer") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset")
}

func (rt *retryTransport) isIdempotent(method string) bool {
	return method == "GET" || method == "HEAD" || method == "PUT" ||
		method == "DELETE" || method == "OPTIONS"
}

func (rt *retryTransport) backoffWithJitter(attempt int) time.Duration {
	base := 100 * time.Millisecond
	cap := rt.retryTimeout
	if cap <= 0 {
		cap = 5 * time.Second
	}
	multiplier := 2.0
	delay := float64(base) * math.Pow(multiplier, float64(attempt-1))
	if delay > float64(cap) {
		delay = float64(cap)
	}
	return rt.jitter(time.Duration(delay))
}

// jitter returns a random duration in [0, max) using crypto/rand.
// crypto/rand is safe for concurrent use and provides cryptographic randomness.
func (rt *retryTransport) jitter(max time.Duration) time.Duration {
	b := make([]byte, 8)
	_, _ = cryptoRand.Read(b)
	val := float64(uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 | uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7]))
	return time.Duration(float64(max) * (val / float64(1<<64)))
}
