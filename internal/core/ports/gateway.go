// Package ports defines service and repository interfaces.
package ports

import (
	"context"
	"net/http/httputil"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// GatewayRepository manages the persistence of reverse proxy routes and ingress rules.
type GatewayRepository interface {
	// CreateRoute saves a new gateway route configuration.
	CreateRoute(ctx context.Context, route *domain.GatewayRoute) error
	// GetRouteByID retrieves a specific route for an authorized user.
	GetRouteByID(ctx context.Context, id, userID uuid.UUID) (*domain.GatewayRoute, error)
	// ListRoutes returns all routes defined by a user.
	ListRoutes(ctx context.Context, userID uuid.UUID) ([]*domain.GatewayRoute, error)
	// DeleteRoute removes a route definition.
	DeleteRoute(ctx context.Context, id uuid.UUID) error

	// For the proxy engine

	// GetAllActiveRoutes retrieves every configured route in the system for the load balancer/proxy.
	GetAllActiveRoutes(ctx context.Context) ([]*domain.GatewayRoute, error)
}

// CreateRouteParams holds parameters for creating a new route.
type CreateRouteParams struct {
	Name                    string
	Pattern                 string
	Target                  string
	Methods                 []string
	StripPrefix             bool
	RateLimit               int
	DialTimeout             int64
	ResponseHeaderTimeout   int64
	IdleConnTimeout         int64
	TLSSkipVerify           bool
	RequireTLS              bool
	AllowedCIDRs            []string
	BlockedCIDRs            []string
	MaxBodySize             int64
	CircuitBreakerThreshold int
	CircuitBreakerTimeout   int64
	MaxRetries              int
	RetryTimeout            int64
	Priority                int
}

// GatewayService provides business logic for managing the API gateway and ingress traffic.
type GatewayService interface {
	// CreateRoute establishes a new ingress mapping.
	CreateRoute(ctx context.Context, params CreateRouteParams) (*domain.GatewayRoute, error)
	// ListRoutes returns all ingress rules for the current user.
	ListRoutes(ctx context.Context) ([]*domain.GatewayRoute, error)
	// DeleteRoute decommission an existing ingress rule.
	DeleteRoute(ctx context.Context, id uuid.UUID) error
	// RefreshRoutes reloads all routes and pre-compiles matchers.
	RefreshRoutes(ctx context.Context) error
	// GetProxy finds the appropriate backend for the given path and method.
	// Returns proxy, route, path params, and found flag.
	GetProxy(method, path string) (*httputil.ReverseProxy, *domain.GatewayRoute, map[string]string, bool)
}
