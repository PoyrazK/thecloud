// Package services implements core business workflows.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const routeTableTracer = "route-table-service"

// RouteTableService manages the lifecycle of route tables within a VPC.
type RouteTableService struct {
	repo     ports.RouteTableRepository
	vpcRepo  ports.VpcRepository
	rbacSvc  ports.RBACService
	network  ports.NetworkBackend
	auditSvc ports.AuditService
	logger   *slog.Logger
}

// RouteTableServiceParams holds dependencies for RouteTableService.
type RouteTableServiceParams struct {
	Repo     ports.RouteTableRepository
	VpcRepo  ports.VpcRepository
	RBACSvc  ports.RBACService
	Network  ports.NetworkBackend
	AuditSvc ports.AuditService
	Logger   *slog.Logger
}

// NewRouteTableService constructs a RouteTableService with its dependencies.
func NewRouteTableService(params RouteTableServiceParams) *RouteTableService {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &RouteTableService{
		repo:     params.Repo,
		vpcRepo:  params.VpcRepo,
		rbacSvc:  params.RBACSvc,
		network:  params.Network,
		auditSvc: params.AuditSvc,
		logger:   logger,
	}
}

// CreateRouteTable creates a new custom route table for a VPC.
// If isMain is true, this becomes the main route table (replacing existing main).
func (s *RouteTableService) CreateRouteTable(ctx context.Context, vpcID uuid.UUID, name string, isMain bool) (*domain.RouteTable, error) {
	ctx, span := otel.Tracer(routeTableTracer).Start(ctx, "CreateRouteTable")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	span.SetAttributes(
		attribute.String("vpc_id", vpcID.String()),
		attribute.String("name", name),
		attribute.Bool("is_main", isMain),
	)

	// Verify VPC exists and user has access
	if _, err := s.vpcRepo.GetByID(ctx, vpcID); err != nil {
		return nil, errors.Wrap(errors.NotFound, "VPC not found", err)
	}

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcUpdate, vpcID.String()); err != nil {
		return nil, err
	}

	rtID := uuid.New()
	rt := &domain.RouteTable{
		ID:        rtID,
		VPCID:     vpcID,
		Name:      name,
		IsMain:    isMain,
		Routes:    []domain.Route{},
		CreatedAt: time.Now().UTC(),
	}

	if err := rt.Validate(); err != nil {
		return nil, errors.New(errors.InvalidInput, err.Error())
	}

	if err := s.repo.Create(ctx, rt); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create route table", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "route_table.create", "route_table", rtID.String(), map[string]interface{}{
		"vpc_id":  vpcID.String(),
		"name":    name,
		"is_main": isMain,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("route table created", "id", rtID, "vpc_id", vpcID, "name", name, "is_main", isMain)
	return rt, nil
}

// GetRouteTable retrieves a route table by ID.
func (s *RouteTableService) GetRouteTable(ctx context.Context, id uuid.UUID) (*domain.RouteTable, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcRead, id.String()); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}

// ListRouteTables returns all route tables for a VPC.
func (s *RouteTableService) ListRouteTables(ctx context.Context, vpcID uuid.UUID) ([]*domain.RouteTable, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcRead, vpcID.String()); err != nil {
		return nil, err
	}

	return s.repo.GetByVPC(ctx, vpcID)
}

// DeleteRouteTable removes a custom route table (cannot delete main route table).
func (s *RouteTableService) DeleteRouteTable(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcDelete, id.String()); err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete route table", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "route_table.delete", "route_table", id.String(), nil); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("route table deleted", "id", id)
	return nil
}

// AddRoute adds a route to an existing route table.
func (s *RouteTableService) AddRoute(ctx context.Context, rtID uuid.UUID, destinationCIDR string, targetType domain.RouteTargetType, targetID *uuid.UUID) (*domain.Route, error) {
	ctx, span := otel.Tracer(routeTableTracer).Start(ctx, "AddRoute")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcUpdate, rtID.String()); err != nil {
		return nil, err
	}

	span.SetAttributes(
		attribute.String("rt_id", rtID.String()),
		attribute.String("destination_cidr", destinationCIDR),
		attribute.String("target_type", string(targetType)),
	)

	// Get the route table to find VPC
	rt, err := s.repo.GetByID(ctx, rtID)
	if err != nil {
		return nil, err
	}

	// Get VPC to find bridge name
	vpc, err := s.vpcRepo.GetByID(ctx, rt.VPCID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to get VPC", err)
	}

	route := &domain.Route{
		ID:              uuid.New(),
		RouteTableID:    rtID,
		DestinationCIDR: destinationCIDR,
		TargetType:      targetType,
		TargetID:        targetID,
		TargetName:      string(targetType),
		CreatedAt:       time.Now().UTC(),
	}

	if err := route.Validate(); err != nil {
		return nil, errors.New(errors.InvalidInput, err.Error())
	}

	// Add route to database
	if err := s.repo.AddRoute(ctx, rtID, route); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to add route", err)
	}

	// Program OVS flow for the route
	flow := ports.FlowRule{
		Priority: 300,
		Match:    fmt.Sprintf("ip,nw_dst=%s", destinationCIDR),
		Actions:  "NORMAL",
	}
	if err := s.network.AddFlowRule(ctx, vpc.NetworkID, flow); err != nil {
		s.logger.Error("failed to add OVS flow for route", "route_id", route.ID, "error", err)
		// Don't fail the operation - DB is source of truth
	}

	if err := s.auditSvc.Log(ctx, userID, "route_table.add_route", "route_table", rtID.String(), map[string]interface{}{
		"route_id":         route.ID.String(),
		"destination_cidr": destinationCIDR,
		"target_type":      string(targetType),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("route added", "id", route.ID, "rt_id", rtID, "dest", destinationCIDR, "target", targetType)
	return route, nil
}

// RemoveRoute removes a route from a route table.
func (s *RouteTableService) RemoveRoute(ctx context.Context, rtID, routeID uuid.UUID) error {
	ctx, span := otel.Tracer(routeTableTracer).Start(ctx, "RemoveRoute")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcUpdate, rtID.String()); err != nil {
		return err
	}

	// Get the route to find destination CIDR for OVS cleanup
	routes, err := s.repo.ListRoutes(ctx, rtID)
	if err != nil {
		return err
	}

	var destinationCIDR string
	for _, r := range routes {
		if r.ID == routeID {
			destinationCIDR = r.DestinationCIDR
			break
		}
	}

	// Get VPC for OVS cleanup
	rt, err := s.repo.GetByID(ctx, rtID)
	if err != nil {
		return err
	}

	vpc, err := s.vpcRepo.GetByID(ctx, rt.VPCID)
	if err != nil {
		return err
	}

	// Remove from DB
	if err := s.repo.RemoveRoute(ctx, rtID, routeID); err != nil {
		return errors.Wrap(errors.Internal, "failed to remove route", err)
	}

	// Remove OVS flow
	if destinationCIDR != "" {
		match := fmt.Sprintf("ip,nw_dst=%s", destinationCIDR)
		if err := s.network.DeleteFlowRule(ctx, vpc.NetworkID, match); err != nil {
			s.logger.Error("failed to remove OVS flow for route", "route_id", routeID, "error", err)
		}
	}

	if err := s.auditSvc.Log(ctx, userID, "route_table.remove_route", "route_table", rtID.String(), map[string]interface{}{
		"route_id": routeID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	return nil
}

// AssociateSubnet links a subnet to a route table.
func (s *RouteTableService) AssociateSubnet(ctx context.Context, rtID, subnetID uuid.UUID) error {
	ctx, span := otel.Tracer(routeTableTracer).Start(ctx, "AssociateSubnet")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcUpdate, rtID.String()); err != nil {
		return err
	}

	if err := s.repo.AssociateSubnet(ctx, rtID, subnetID); err != nil {
		return errors.Wrap(errors.Internal, "failed to associate subnet", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "route_table.associate", "route_table", rtID.String(), map[string]interface{}{
		"subnet_id": subnetID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("subnet associated with route table", "rt_id", rtID, "subnet_id", subnetID)
	return nil
}

// DisassociateSubnet removes a subnet's association with a route table.
// The subnet will then use the main route table.
func (s *RouteTableService) DisassociateSubnet(ctx context.Context, rtID, subnetID uuid.UUID) error {
	ctx, span := otel.Tracer(routeTableTracer).Start(ctx, "DisassociateSubnet")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionVpcUpdate, rtID.String()); err != nil {
		return err
	}

	if err := s.repo.DisassociateSubnet(ctx, rtID, subnetID); err != nil {
		return errors.Wrap(errors.Internal, "failed to disassociate subnet", err)
	}

	if err := s.auditSvc.Log(ctx, userID, "route_table.disassociate", "route_table", rtID.String(), map[string]interface{}{
		"subnet_id": subnetID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "error", err)
	}

	s.logger.Info("subnet disassociated from route table", "rt_id", rtID, "subnet_id", subnetID)
	return nil
}

// ReplaceRoute replaces an existing route with a new target.
func (s *RouteTableService) ReplaceRoute(ctx context.Context, rtID, routeID uuid.UUID, newTargetID *uuid.UUID) error {
	// This would require getting the route, removing it, and adding a new one
	// For now, just a placeholder - implementation would follow similar pattern
	return errors.New(errors.NotImplemented, "ReplaceRoute not yet implemented")
}
