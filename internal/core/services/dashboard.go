// Package services implements core business workflows.
package services

import (
	"context"
	"log/slog"

	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type dashboardService struct {
	rbacSvc   ports.RBACService
	instances ports.InstanceRepository
	volumes   ports.VolumeRepository
	vpcs      ports.VpcRepository
	events    ports.EventRepository
	logger    *slog.Logger
}

// NewDashboardService creates a new dashboard service with all required repositories.
func NewDashboardService(
	rbacSvc ports.RBACService,
	instances ports.InstanceRepository,
	volumes ports.VolumeRepository,
	vpcs ports.VpcRepository,
	events ports.EventRepository,
	logger *slog.Logger,
) ports.DashboardService {
	return &dashboardService{
		rbacSvc:   rbacSvc,
		instances: instances,
		volumes:   volumes,
		vpcs:      vpcs,
		events:    events,
		logger:    logger,
	}
}

// GetSummary aggregates resource counts from all repositories.
func (s *dashboardService) GetSummary(ctx context.Context) (*domain.ResourceSummary, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDashboardRead, "*"); err != nil {
		return nil, err
	}

	summary := &domain.ResourceSummary{}

	// Count instances
	instances, err := s.instances.List(ctx, nil)
	if err != nil {
		s.logger.Error("failed to list instances", slog.String("error", err.Error()))
		return nil, err
	}
	summary.TotalInstances = len(instances)
	for _, inst := range instances {
		switch inst.Status {
		case domain.StatusRunning:
			summary.RunningInstances++
		case domain.StatusStopped:
			summary.StoppedInstances++
		}
	}

	// Count volumes
	volumes, err := s.volumes.List(ctx)
	if err != nil {
		s.logger.Error("failed to list volumes", slog.String("error", err.Error()))
		return nil, err
	}
	summary.TotalVolumes = len(volumes)
	for _, vol := range volumes {
		summary.TotalStorageMB += vol.SizeGB * 1024
		if vol.InstanceID != nil {
			summary.AttachedVolumes++
		}
	}

	// Count VPCs
	vpcs, err := s.vpcs.List(ctx)
	if err != nil {
		s.logger.Error("failed to list vpcs", slog.String("error", err.Error()))
		return nil, err
	}
	summary.TotalVPCs = len(vpcs)

	s.logger.Debug("dashboard summary generated",
		slog.Int("instances", summary.TotalInstances),
		slog.Int("volumes", summary.TotalVolumes),
		slog.Int("vpcs", summary.TotalVPCs),
	)

	return summary, nil
}

// GetRecentEvents returns the most recent audit events.
func (s *dashboardService) GetRecentEvents(ctx context.Context, limit int) ([]*domain.Event, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDashboardRead, "*"); err != nil {
		return nil, err
	}

	events, err := s.events.List(ctx, limit)
	if err != nil {
		s.logger.Error("failed to list events", slog.String("error", err.Error()))
		return nil, err
	}
	return events, nil
}

// GetStats returns the full dashboard statistics including summary and recent events.
func (s *dashboardService) GetStats(ctx context.Context) (*domain.DashboardStats, error) {
	// RBAC checked in GetSummary and GetRecentEvents
	summary, err := s.GetSummary(ctx)
	if err != nil {
		return nil, err
	}

	events, err := s.GetRecentEvents(ctx, 10)
	if err != nil {
		return nil, err
	}

	// Convert event pointers to values for the response
	eventValues := make([]domain.Event, len(events))
	for i, e := range events {
		eventValues[i] = *e
	}

	return &domain.DashboardStats{
		Summary:       *summary,
		RecentEvents:  eventValues,
		CPUHistory:    []domain.MetricPoint{}, // Will be populated by metrics service
		MemoryHistory: []domain.MetricPoint{},
	}, nil
}
