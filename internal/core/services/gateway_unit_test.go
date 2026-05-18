package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	defaultCircuitBreakerThreshold = 5
	defaultCircuitBreakerTimeout   = 30000 // ms
	defaultMaxRetries              = 2
	defaultRetryTimeout            = 5000 // ms
)

type mockGatewayRepo struct {
	mock.Mock
}

func (m *mockGatewayRepo) CreateRoute(ctx context.Context, r *domain.GatewayRoute) error {
	return m.Called(ctx, r).Error(0)
}
func (m *mockGatewayRepo) GetRouteByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.GatewayRoute, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.GatewayRoute)
	return r0, args.Error(1)
}
func (m *mockGatewayRepo) ListRoutes(ctx context.Context, userID uuid.UUID) ([]*domain.GatewayRoute, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.GatewayRoute)
	return r0, args.Error(1)
}
func (m *mockGatewayRepo) GetAllActiveRoutes(ctx context.Context) ([]*domain.GatewayRoute, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.GatewayRoute)
	return r0, args.Error(1)
}
func (m *mockGatewayRepo) UpdateRoute(ctx context.Context, r *domain.GatewayRoute) error {
	return m.Called(ctx, r).Error(0)
}
func (m *mockGatewayRepo) DeleteRoute(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func TestGatewayService_Unit(t *testing.T) {
	repo := new(mockGatewayRepo)
	auditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// NewGatewayService calls RefreshRoutes during construction, and route mutations refresh again.
	repo.On("GetAllActiveRoutes", mock.Anything).Return([]*domain.GatewayRoute{}, nil).Maybe()
	svc := services.NewGatewayService(repo, rbacSvc, auditSvc, slog.Default())

	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	userID := appcontext.UserIDFromContext(ctx)

	t.Run("CreateRoute applies default resilience values", func(t *testing.T) {
		params := ports.CreateRouteParams{Name: "r1", Pattern: "/r1", Target: "http://t1"}
		repo.On("CreateRoute", ctx, mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "gateway.route_create", "gateway", mock.Anything, mock.Anything).Return(nil).Once()

		res, err := svc.CreateRoute(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, defaultCircuitBreakerThreshold, res.CircuitBreakerThreshold)
		assert.Equal(t, int64(defaultCircuitBreakerTimeout), res.CircuitBreakerTimeout)
		assert.Equal(t, defaultMaxRetries, res.MaxRetries)
		assert.Equal(t, int64(defaultRetryTimeout), res.RetryTimeout)
	})

	t.Run("CreateRoute", func(t *testing.T) {
		params := ports.CreateRouteParams{Name: "r1", Pattern: "/r1", Target: "http://t1"}
		repo.On("CreateRoute", ctx, mock.Anything).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "gateway.route_create", "gateway", mock.Anything, mock.Anything).Return(nil).Once()

		res, err := svc.CreateRoute(ctx, params)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("ListRoutes", func(t *testing.T) {
		repo.On("ListRoutes", mock.Anything, userID).Return([]*domain.GatewayRoute{{ID: uuid.New()}}, nil).Once()
		res, err := svc.ListRoutes(ctx)
		require.NoError(t, err)
		assert.Len(t, res, 1)
	})

	t.Run("RefreshRoutes_Error", func(t *testing.T) {
		errRepo := new(mockGatewayRepo)
		errRBAC := new(MockRBACService)
		errAudit := new(MockAuditService)
		errRepo.On("GetAllActiveRoutes", mock.Anything).Return(nil, fmt.Errorf("db fail")).Maybe()
		errSvc := services.NewGatewayService(errRepo, errRBAC, errAudit, slog.Default())

		err := errSvc.RefreshRoutes(ctx)
		require.Error(t, err)
	})

	t.Run("DeleteRoute", func(t *testing.T) {
		id := uuid.New()
		route := &domain.GatewayRoute{ID: id, UserID: userID, Name: "r1"}

		repo.On("GetRouteByID", mock.Anything, id, userID).Return(route, nil).Once()
		repo.On("DeleteRoute", ctx, id).Return(nil).Once()
		auditSvc.On("Log", mock.Anything, userID, "gateway.route_delete", "gateway", id.String(), mock.Anything).Return(nil).Once()

		err := svc.DeleteRoute(ctx, id)
		require.NoError(t, err)
	})
}
