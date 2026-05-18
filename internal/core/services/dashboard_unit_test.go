package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDashboardService_Unit(t *testing.T) {
	t.Run("GetStats", testDashboardServiceGetStats)
	t.Run("GetSummary", testDashboardServiceGetSummary)
	t.Run("GetRecentEvents", testDashboardServiceGetRecentEvents)
}

func testDashboardServiceGetStats(t *testing.T) {
	instRepo := new(MockInstanceRepo)
	volRepo := new(MockVolumeRepo)
	vpcRepo := new(MockVpcRepo)
	eventRepo := new(MockEventRepo)
	rbacSvc := new(MockRBACService)

	svc := services.NewDashboardService(rbacSvc, instRepo, volRepo, vpcRepo, eventRepo, slog.Default())
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		instID := uuid.New()
		instances := []*domain.Instance{
			{ID: uuid.New(), Status: domain.StatusRunning},
			{ID: uuid.New(), Status: domain.StatusStopped},
		}
		volumes := []*domain.Volume{
			{ID: uuid.New(), SizeGB: 10, InstanceID: &instID},
			{ID: uuid.New(), SizeGB: 20},
		}
		vpcs := []*domain.VPC{{ID: uuid.New()}, {ID: uuid.New()}, {ID: uuid.New()}}
		events := []*domain.Event{{ID: uuid.New(), Action: "test.action"}}

		rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		instRepo.On("List", ctx, []string(nil)).Return(instances, nil).Once()
		volRepo.On("List", ctx).Return(volumes, nil).Once()
		vpcRepo.On("List", ctx).Return(vpcs, nil).Once()
		eventRepo.On("List", ctx, 10).Return(events, nil).Once()

		stats, err := svc.GetStats(ctx)
		require.NoError(t, err)
		assert.NotNil(t, stats)

		assert.Equal(t, 2, stats.Summary.TotalInstances)
		assert.Equal(t, 1, stats.Summary.RunningInstances)
		assert.Equal(t, 1, stats.Summary.StoppedInstances)
		assert.Equal(t, 2, stats.Summary.TotalVolumes)
		assert.Equal(t, 1, stats.Summary.AttachedVolumes)
		assert.Equal(t, 30*1024, stats.Summary.TotalStorageMB)
		assert.Equal(t, 3, stats.Summary.TotalVPCs)
		assert.Len(t, stats.RecentEvents, 1)
	})

	t.Run("RepoError", func(t *testing.T) {
		instRepo.On("List", ctx, []string(nil)).Return(nil, fmt.Errorf("db fail")).Once()

		_, err := svc.GetStats(ctx)
		require.Error(t, err)
	})
}

func testDashboardServiceGetSummary(t *testing.T) {
	instRepo := new(MockInstanceRepo)
	volRepo := new(MockVolumeRepo)
	vpcRepo := new(MockVpcRepo)
	eventRepo := new(MockEventRepo)
	rbacSvc := new(MockRBACService)

	svc := services.NewDashboardService(rbacSvc, instRepo, volRepo, vpcRepo, eventRepo, slog.Default())

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithUserID(context.Background(), userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionDashboardRead, "*").Return(nil)

		instances := []*domain.Instance{
			{ID: uuid.New(), Status: domain.StatusRunning},
			{ID: uuid.New(), Status: domain.StatusStopped},
		}
		volumes := []*domain.Volume{
			{ID: uuid.New(), SizeGB: 10, InstanceID: &instances[0].ID},
			{ID: uuid.New(), SizeGB: 20},
		}
		vpcs := []*domain.VPC{{ID: uuid.New()}, {ID: uuid.New()}}

		instRepo.On("List", ctx, []string(nil)).Return(instances, nil).Once()
		volRepo.On("List", ctx).Return(volumes, nil).Once()
		vpcRepo.On("List", ctx).Return(vpcs, nil).Once()

		summary, err := svc.GetSummary(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, summary.TotalInstances)
		assert.Equal(t, 1, summary.RunningInstances)
		assert.Equal(t, 1, summary.StoppedInstances)
		assert.Equal(t, 2, summary.TotalVolumes)
		assert.Equal(t, 1, summary.AttachedVolumes)
		assert.Equal(t, 30*1024, summary.TotalStorageMB)
		assert.Equal(t, 2, summary.TotalVPCs)
	})

	t.Run("InstanceRepoError", func(t *testing.T) {
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithUserID(context.Background(), userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionDashboardRead, "*").Return(nil)

		instRepo.On("List", ctx, []string(nil)).Return(nil, fmt.Errorf("db fail")).Once()

		_, err := svc.GetSummary(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db fail")
	})

	t.Run("Unauthorized", func(t *testing.T) {
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithUserID(context.Background(), userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionDashboardRead, "*").Return(fmt.Errorf("forbidden")).Once()

		_, err := svc.GetSummary(ctx)
		require.Error(t, err)
	})
}

func testDashboardServiceGetRecentEvents(t *testing.T) {
	instRepo := new(MockInstanceRepo)
	volRepo := new(MockVolumeRepo)
	vpcRepo := new(MockVpcRepo)
	eventRepo := new(MockEventRepo)
	rbacSvc := new(MockRBACService)

	svc := services.NewDashboardService(rbacSvc, instRepo, volRepo, vpcRepo, eventRepo, slog.Default())

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithUserID(context.Background(), userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionDashboardRead, "*").Return(nil)

		events := []*domain.Event{
			{ID: uuid.New(), Action: "action1"},
			{ID: uuid.New(), Action: "action2"},
		}
		eventRepo.On("List", ctx, 5).Return(events, nil).Once()

		result, err := svc.GetRecentEvents(ctx, 5)
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("RepoError", func(t *testing.T) {
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithUserID(context.Background(), userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionDashboardRead, "*").Return(nil)

		eventRepo.On("List", ctx, 10).Return(nil, fmt.Errorf("db fail")).Once()

		_, err := svc.GetRecentEvents(ctx, 10)
		require.Error(t, err)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithUserID(context.Background(), userID)
		ctx = appcontext.WithTenantID(ctx, tenantID)
		rbacSvc.On("Authorize", mock.Anything, userID, tenantID, domain.PermissionDashboardRead, "*").Return(fmt.Errorf("forbidden")).Once()

		_, err := svc.GetRecentEvents(ctx, 10)
		require.Error(t, err)
	})
}
