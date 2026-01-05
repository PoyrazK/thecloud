package ports

import (
	"context"
	"net/http/httputil"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type GatewayRepository interface {
	CreateRoute(ctx context.Context, route *domain.GatewayRoute) error
	GetRouteByID(ctx context.Context, id, userID uuid.UUID) (*domain.GatewayRoute, error)
	ListRoutes(ctx context.Context, userID uuid.UUID) ([]*domain.GatewayRoute, error)
	DeleteRoute(ctx context.Context, id uuid.UUID) error

	// For the proxy engine
	GetAllActiveRoutes(ctx context.Context) ([]*domain.GatewayRoute, error)
}

type GatewayService interface {
	CreateRoute(ctx context.Context, name, prefix, target string, strip bool, rateLimit int) (*domain.GatewayRoute, error)
	ListRoutes(ctx context.Context) ([]*domain.GatewayRoute, error)
	DeleteRoute(ctx context.Context, id uuid.UUID) error
	RefreshRoutes(ctx context.Context) error
	GetProxy(path string) (*httputil.ReverseProxy, bool)
}
