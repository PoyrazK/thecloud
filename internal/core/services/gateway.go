package services

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type GatewayService struct {
	repo    ports.GatewayRepository
	proxyMu sync.RWMutex
	proxies map[string]*httputil.ReverseProxy
	routes  map[string]*domain.GatewayRoute
}

func NewGatewayService(repo ports.GatewayRepository) *GatewayService {
	s := &GatewayService{
		repo:    repo,
		proxies: make(map[string]*httputil.ReverseProxy),
		routes:  make(map[string]*domain.GatewayRoute),
	}
	// Initial load
	_ = s.RefreshRoutes(context.Background())
	return s
}

func (s *GatewayService) CreateRoute(ctx context.Context, name, prefix, target string, strip bool, rateLimit int) (*domain.GatewayRoute, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, fmt.Errorf("unauthorized")
	}

	route := &domain.GatewayRoute{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        name,
		PathPrefix:  prefix,
		TargetURL:   target,
		StripPrefix: strip,
		RateLimit:   rateLimit,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateRoute(ctx, route); err != nil {
		return nil, err
	}

	_ = s.RefreshRoutes(ctx)
	return route, nil
}

func (s *GatewayService) ListRoutes(ctx context.Context) ([]*domain.GatewayRoute, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, fmt.Errorf("unauthorized")
	}
	return s.repo.ListRoutes(ctx, userID)
}

func (s *GatewayService) DeleteRoute(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	_, err := s.repo.GetRouteByID(ctx, id, userID)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteRoute(ctx, id); err != nil {
		return err
	}
	return s.RefreshRoutes(ctx)
}

func (s *GatewayService) RefreshRoutes(ctx context.Context) error {
	routes, err := s.repo.GetAllActiveRoutes(ctx)
	if err != nil {
		return err
	}

	newProxies := make(map[string]*httputil.ReverseProxy)
	newRoutes := make(map[string]*domain.GatewayRoute)

	for _, r := range routes {
		target, err := url.Parse(r.TargetURL)
		if err != nil {
			continue
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		// Custom director to handle prefix stripping if needed
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)
			if r.StripPrefix {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, "/gw"+r.PathPrefix)
				if !strings.HasPrefix(req.URL.Path, "/") {
					req.URL.Path = "/" + req.URL.Path
				}
			}
			req.Host = target.Host
		}

		newProxies[r.PathPrefix] = proxy
		newRoutes[r.PathPrefix] = r
	}

	s.proxyMu.Lock()
	s.proxies = newProxies
	s.routes = newRoutes
	s.proxyMu.Unlock()

	return nil
}

// ProxyHandler is handled in the API layer for now

func (s *GatewayService) GetProxy(path string) (*httputil.ReverseProxy, bool) {
	s.proxyMu.RLock()
	defer s.proxyMu.RUnlock()

	for prefix, proxy := range s.proxies {
		if strings.HasPrefix(path, prefix) {
			return proxy, true
		}
	}
	return nil, false
}
