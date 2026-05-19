// Package postgres provides PostgreSQL-backed repository implementations.
package postgres

import (
	"context"
	stdlib_errors "errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

const (
	tgwColumns           = "id, name, owner_tenant_id, status, arn, created_at, updated_at"
	tgwAttachmentColumns = "id, transit_gateway_id, vpc_id, tenant_id, status, attachment_type, created_at"
	tgwRTColumns         = "id, transit_gateway_id, name, default_route_table, propagation_enabled, created_at"
	tgwRouteColumns      = "id, transit_gateway_rt_id, destination_cidr, target_type, target_id, target_name, created_at"
	tgwRTAssocColumns    = "route_table_id, attachment_id, propagation_enabled"
	errTGNotFound        = "transit gateway not found"
	errAttNotFound       = "transit gateway attachment not found"
	errRTNotFound        = "transit gateway route table not found"
)

// TransitGatewayRepository provides a PostgreSQL implementation for Transit Gateway management.
type TransitGatewayRepository struct {
	db DB
}

// NewTransitGatewayRepository creates a new TransitGatewayRepository.
func NewTransitGatewayRepository(db DB) *TransitGatewayRepository {
	return &TransitGatewayRepository{db: db}
}

// Create inserts a new Transit Gateway record.
func (r *TransitGatewayRepository) Create(ctx context.Context, tg *domain.TransitGateway) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to begin transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
		INSERT INTO transit_gateways (id, name, owner_tenant_id, status, arn, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.Exec(ctx, query, tg.ID, tg.Name, tg.OwnerTenantID, tg.Status, tg.ARN, tg.CreatedAt, tg.UpdatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create transit gateway", err)
	}

	// Create default route table if present
	for _, rt := range tg.RouteTables {
		rtQuery := `
			INSERT INTO transit_gateway_route_tables (id, transit_gateway_id, name, default_route_table, propagation_enabled, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`
		_, err = tx.Exec(ctx, rtQuery, rt.ID, rt.TransitGatewayID, rt.Name, rt.DefaultRouteTable, rt.PropagationEnabled, rt.CreatedAt)
		if err != nil {
			return errors.Wrap(errors.Internal, "failed to create default route table", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.Wrap(errors.Internal, "failed to commit transit gateway", err)
	}
	return nil
}

// GetByID retrieves a Transit Gateway by ID.
func (r *TransitGatewayRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.TransitGateway, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT ` + tgwColumns + `
		FROM transit_gateways
		WHERE id = $1 AND owner_tenant_id = $2
	`
	var tg domain.TransitGateway
	err := r.db.QueryRow(ctx, query, id, tenantID).Scan(
		&tg.ID, &tg.Name, &tg.OwnerTenantID, &tg.Status, &tg.ARN, &tg.CreatedAt, &tg.UpdatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, errTGNotFound)
		}
		return nil, errors.Wrap(errors.Internal, "failed to scan transit gateway", err)
	}

	// Load route tables
	rts, err := r.ListRouteTables(ctx, tg.ID)
	if err != nil {
		return nil, err
	}
	tg.RouteTables = rts

	// Load attachments
	atts, err := r.ListAttachments(ctx, tg.ID)
	if err != nil {
		return nil, err
	}
	tg.Attachments = atts

	return &tg, nil
}

// List returns all Transit Gateways for a tenant.
func (r *TransitGatewayRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.TransitGateway, error) {
	query := `
		SELECT ` + tgwColumns + `
		FROM transit_gateways
		WHERE owner_tenant_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list transit gateways", err)
	}
	defer rows.Close()

	var tgs []*domain.TransitGateway
	for rows.Next() {
		var tg domain.TransitGateway
		if err := rows.Scan(&tg.ID, &tg.Name, &tg.OwnerTenantID, &tg.Status, &tg.ARN, &tg.CreatedAt, &tg.UpdatedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan transit gateway", err)
		}
		tgs = append(tgs, &tg)
	}
	return tgs, rows.Err()
}

// Update updates a Transit Gateway record.
func (r *TransitGatewayRepository) Update(ctx context.Context, tg *domain.TransitGateway) error {
	query := `
		UPDATE transit_gateways
		SET name = $1, status = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, tg.Name, tg.Status, tg.ID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to update transit gateway", err)
	}
	return nil
}

// Delete removes a Transit Gateway and cascades to route tables and attachments via FK.
func (r *TransitGatewayRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `DELETE FROM transit_gateways WHERE id = $1 AND owner_tenant_id = $2`
	cmd, err := r.db.Exec(ctx, query, id, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete transit gateway", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, errTGNotFound)
	}
	return nil
}

// AddAttachment creates a new attachment record.
func (r *TransitGatewayRepository) AddAttachment(ctx context.Context, att *domain.TransitGatewayAttachment) error {
	query := `
		INSERT INTO transit_gateway_attachments (id, transit_gateway_id, vpc_id, tenant_id, status, attachment_type, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query, att.ID, att.TransitGatewayID, att.VPCID, att.TenantID, att.Status, att.AttachmentType, att.CreatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create attachment", err)
	}
	return nil
}

// RemoveAttachment deletes an attachment.
func (r *TransitGatewayRepository) RemoveAttachment(ctx context.Context, id uuid.UUID) error {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `DELETE FROM transit_gateway_attachments WHERE id = $1 AND tenant_id = $2`
	cmd, err := r.db.Exec(ctx, query, id, tenantID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to remove attachment", err)
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, errAttNotFound)
	}
	return nil
}

// ListAttachments returns all attachments for a Transit Gateway.
func (r *TransitGatewayRepository) ListAttachments(ctx context.Context, tgID uuid.UUID) ([]*domain.TransitGatewayAttachment, error) {
	query := `
		SELECT ` + tgwAttachmentColumns + `
		FROM transit_gateway_attachments
		WHERE transit_gateway_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, tgID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list attachments", err)
	}
	defer rows.Close()

	var atts []*domain.TransitGatewayAttachment
	for rows.Next() {
		var att domain.TransitGatewayAttachment
		if err := rows.Scan(&att.ID, &att.TransitGatewayID, &att.VPCID, &att.TenantID, &att.Status, &att.AttachmentType, &att.CreatedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan attachment", err)
		}
		atts = append(atts, &att)
	}
	return atts, rows.Err()
}

// GetAttachment retrieves an attachment by ID.
func (r *TransitGatewayRepository) GetAttachment(ctx context.Context, id uuid.UUID) (*domain.TransitGatewayAttachment, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	query := `
		SELECT ` + tgwAttachmentColumns + `
		FROM transit_gateway_attachments
		WHERE id = $1 AND tenant_id = $2
	`
	var att domain.TransitGatewayAttachment
	err := r.db.QueryRow(ctx, query, id, tenantID).Scan(
		&att.ID, &att.TransitGatewayID, &att.VPCID, &att.TenantID, &att.Status, &att.AttachmentType, &att.CreatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, errAttNotFound)
		}
		return nil, errors.Wrap(errors.Internal, "failed to scan attachment", err)
	}
	return &att, nil
}

// CreateRouteTable inserts a new TGW route table.
func (r *TransitGatewayRepository) CreateRouteTable(ctx context.Context, rt *domain.TransitGatewayRouteTable) error {
	query := `
		INSERT INTO transit_gateway_route_tables (id, transit_gateway_id, name, default_route_table, propagation_enabled, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.Exec(ctx, query, rt.ID, rt.TransitGatewayID, rt.Name, rt.DefaultRouteTable, rt.PropagationEnabled, rt.CreatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create route table", err)
	}
	return nil
}

// GetRouteTable retrieves a TGW route table by ID.
func (r *TransitGatewayRepository) GetRouteTable(ctx context.Context, id uuid.UUID) (*domain.TransitGatewayRouteTable, error) {
	query := `
		SELECT ` + tgwRTColumns + `
		FROM transit_gateway_route_tables
		WHERE id = $1
	`
	var rt domain.TransitGatewayRouteTable
	err := r.db.QueryRow(ctx, query, id).Scan(
		&rt.ID, &rt.TransitGatewayID, &rt.Name, &rt.DefaultRouteTable, &rt.PropagationEnabled, &rt.CreatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, errRTNotFound)
		}
		return nil, errors.Wrap(errors.Internal, "failed to scan route table", err)
	}
	return &rt, nil
}

// ListRouteTables returns all route tables for a Transit Gateway.
func (r *TransitGatewayRepository) ListRouteTables(ctx context.Context, tgID uuid.UUID) ([]*domain.TransitGatewayRouteTable, error) {
	query := `
		SELECT ` + tgwRTColumns + `
		FROM transit_gateway_route_tables
		WHERE transit_gateway_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, tgID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list route tables", err)
	}
	defer rows.Close()

	var rts []*domain.TransitGatewayRouteTable
	for rows.Next() {
		var rt domain.TransitGatewayRouteTable
		if err := rows.Scan(&rt.ID, &rt.TransitGatewayID, &rt.Name, &rt.DefaultRouteTable, &rt.PropagationEnabled, &rt.CreatedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan route table", err)
		}
		rts = append(rts, &rt)
	}
	return rts, rows.Err()
}

// DeleteRouteTable removes a TGW route table.
func (r *TransitGatewayRepository) DeleteRouteTable(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM transit_gateway_route_tables WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to delete route table", err)
	}
	return nil
}

// AddRoute inserts a route into a TGW route table.
func (r *TransitGatewayRepository) AddRoute(ctx context.Context, rtID uuid.UUID, route *domain.TransitGatewayRoute) error {
	query := `
		INSERT INTO transit_gateway_routes (id, transit_gateway_rt_id, destination_cidr, target_type, target_id, target_name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query, route.ID, route.TransitGatewayRTID, route.DestinationCIDR, route.TargetType, route.TargetID, route.TargetName, route.CreatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to add route", err)
	}
	return nil
}

// RemoveRoute deletes a route from a TGW route table.
func (r *TransitGatewayRepository) RemoveRoute(ctx context.Context, rtID, routeID uuid.UUID) error {
	query := `DELETE FROM transit_gateway_routes WHERE id = $1 AND transit_gateway_rt_id = $2`
	_, err := r.db.Exec(ctx, query, routeID, rtID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to remove route", err)
	}
	return nil
}

// ListRoutes returns all routes in a TGW route table.
func (r *TransitGatewayRepository) ListRoutes(ctx context.Context, rtID uuid.UUID) ([]*domain.TransitGatewayRoute, error) {
	query := `
		SELECT ` + tgwRouteColumns + `
		FROM transit_gateway_routes
		WHERE transit_gateway_rt_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, rtID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list routes", err)
	}
	defer rows.Close()

	var routes []*domain.TransitGatewayRoute
	for rows.Next() {
		var route domain.TransitGatewayRoute
		if err := rows.Scan(&route.ID, &route.TransitGatewayRTID, &route.DestinationCIDR, &route.TargetType, &route.TargetID, &route.TargetName, &route.CreatedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan route", err)
		}
		routes = append(routes, &route)
	}
	return routes, rows.Err()
}

// AssociateAttachment links an attachment to a route table.
func (r *TransitGatewayRepository) AssociateAttachment(ctx context.Context, rtID, attID uuid.UUID) error {
	query := `
		INSERT INTO transit_gateway_rt_associations (route_table_id, attachment_id, propagation_enabled)
		VALUES ($1, $2, FALSE)
		ON CONFLICT (route_table_id, attachment_id) DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, rtID, attID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to associate attachment", err)
	}
	return nil
}

// EnablePropagation enables route propagation from an attachment to a route table.
func (r *TransitGatewayRepository) EnablePropagation(ctx context.Context, rtID, attID uuid.UUID) error {
	query := `
		INSERT INTO transit_gateway_rt_associations (route_table_id, attachment_id, propagation_enabled)
		VALUES ($1, $2, TRUE)
		ON CONFLICT (route_table_id, attachment_id) DO UPDATE SET propagation_enabled = TRUE
	`
	_, err := r.db.Exec(ctx, query, rtID, attID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to enable propagation", err)
	}
	return nil
}
