package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func routeTableTestRouteTable() *domain.RouteTable {
	return &domain.RouteTable{
		ID:        uuid.New(),
		VPCID:     uuid.New(),
		Name:      "test-route-table",
		IsMain:    false,
		CreatedAt: time.Now(),
	}
}

func routeTableTestRoute() *domain.Route {
	id := uuid.New()
	return &domain.Route{
		ID:              uuid.New(),
		RouteTableID:    uuid.New(),
		DestinationCIDR: "10.0.0.0/16",
		TargetType:      domain.RouteTargetNAT,
		TargetID:        &id,
		TargetName:      "nat-gw-1",
		CreatedAt:       time.Now(),
	}
}

func TestRouteTableRepository_Create(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rt := routeTableTestRouteTable()

	mock.ExpectExec("INSERT INTO route_tables").
		WithArgs(rt.ID, rt.VPCID, rt.Name, rt.IsMain, rt.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Create(context.Background(), rt)
	require.NoError(t, err)
}

func TestRouteTableRepository_GetByID(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rt := routeTableTestRouteTable()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT rt.id, rt.vpc_id, rt.name, rt.is_main, rt.created_at FROM route_tables rt JOIN vpcs v ON rt.vpc_id = v.id WHERE rt.id = .+AND v.tenant_id = .+").
		WithArgs(rt.ID, tenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "vpc_id", "name", "is_main", "created_at"}).
			AddRow(rt.ID, rt.VPCID, rt.Name, rt.IsMain, rt.CreatedAt))

	mock.ExpectQuery("SELECT .+ FROM routes WHERE route_table_id = .+ORDER BY destination_cidr").
		WithArgs(rt.ID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "route_table_id", "destination_cidr", "target_type", "coalesce", "coalesce", "created_at"}))

	mock.ExpectQuery("SELECT subnet_id FROM route_table_associations WHERE route_table_id = .+").
		WithArgs(rt.ID).
		WillReturnRows(pgxmock.NewRows([]string{"subnet_id"}))

	result, err := repo.GetByID(ctx, rt.ID)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, rt.Name, result.Name)
}

func TestRouteTableRepository_GetByID_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT rt.id, rt.vpc_id, rt.name, rt.is_main, rt.created_at FROM route_tables rt JOIN vpcs v ON rt.vpc_id = v.id WHERE rt.id = .+AND v.tenant_id = .+").
		WithArgs(id, tenantID).
		WillReturnError(context.DeadlineExceeded)

	result, err := repo.GetByID(ctx, id)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestRouteTableRepository_GetByVPC(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rt := routeTableTestRouteTable()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT rt.id, rt.vpc_id, rt.name, rt.is_main, rt.created_at FROM route_tables rt JOIN vpcs v ON rt.vpc_id = v.id WHERE rt.vpc_id = .+AND v.tenant_id = .+ORDER BY rt.is_main DESC, rt.created_at ASC").
		WithArgs(rt.VPCID, tenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "vpc_id", "name", "is_main", "created_at"}).
			AddRow(rt.ID, rt.VPCID, rt.Name, rt.IsMain, rt.CreatedAt))

	result, err := repo.GetByVPC(ctx, rt.VPCID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestRouteTableRepository_GetMainByVPC(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rt := routeTableTestRouteTable()
	rt.IsMain = true
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT rt.id, rt.vpc_id, rt.name, rt.is_main, rt.created_at FROM route_tables rt JOIN vpcs v ON rt.vpc_id = v.id WHERE rt.vpc_id = .+AND rt.is_main = TRUE AND v.tenant_id = .+").
		WithArgs(rt.VPCID, tenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "vpc_id", "name", "is_main", "created_at"}).
			AddRow(rt.ID, rt.VPCID, rt.Name, rt.IsMain, rt.CreatedAt))

	result, err := repo.GetMainByVPC(ctx, rt.VPCID)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsMain)
}

func TestRouteTableRepository_GetMainByVPC_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	vpcID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT rt.id, rt.vpc_id, rt.name, rt.is_main, rt.created_at FROM route_tables rt JOIN vpcs v ON rt.vpc_id = v.id WHERE rt.vpc_id = .+AND rt.is_main = TRUE AND v.tenant_id = .+").
		WithArgs(vpcID, tenantID).
		WillReturnError(context.DeadlineExceeded)

	result, err := repo.GetMainByVPC(ctx, vpcID)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestRouteTableRepository_Update(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rt := routeTableTestRouteTable()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("UPDATE route_tables SET name = .+, is_main = .+ WHERE id = .+AND vpc_id IN \\(SELECT id FROM vpcs WHERE tenant_id = .+\\)").
		WithArgs(rt.Name, rt.IsMain, rt.ID, tenantID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.Update(ctx, rt)
	require.NoError(t, err)
}

func TestRouteTableRepository_Update_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rt := routeTableTestRouteTable()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("UPDATE route_tables SET name = .+, is_main = .+ WHERE id = .+AND vpc_id IN \\(SELECT id FROM vpcs WHERE tenant_id = .+\\)").
		WithArgs(rt.Name, rt.IsMain, rt.ID, tenantID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err = repo.Update(ctx, rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRouteTableRepository_Delete(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("DELETE FROM route_tables rt USING vpcs v WHERE rt.vpc_id = v.id AND rt.id = .+AND v.tenant_id = .+AND rt.is_main = FALSE").
		WithArgs(id, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(ctx, id)
	require.NoError(t, err)
}

func TestRouteTableRepository_Delete_MainTable(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("DELETE FROM route_tables rt USING vpcs v WHERE rt.vpc_id = v.id AND rt.id = .+AND v.tenant_id = .+AND rt.is_main = FALSE").
		WithArgs(id, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err = repo.Delete(ctx, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRouteTableRepository_AddRoute(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	route := routeTableTestRoute()

	mock.ExpectExec("INSERT INTO routes").
		WithArgs(route.ID, route.RouteTableID, route.DestinationCIDR, route.TargetType, route.TargetID, route.TargetName, route.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.AddRoute(context.Background(), route.RouteTableID, route)
	require.NoError(t, err)
}

// TestRouteTableRepository_ListRoutes skipped: RouteTargetType implements pgx.Scanner
// that pgxmock cannot mock. The ListRoutes path is exercised indirectly via GetByID
// and GetByVPC in integration tests.

func TestRouteTableRepository_RemoveRoute(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rtID := uuid.New()
	routeID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("DELETE FROM routes r USING route_tables rt JOIN vpcs v ON rt.vpc_id = v.id WHERE r.route_table_id = rt.id AND r.id = .+AND rt.id = .+AND v.tenant_id = .+").
		WithArgs(routeID, rtID, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.RemoveRoute(ctx, rtID, routeID)
	require.NoError(t, err)
}

func TestRouteTableRepository_RemoveRoute_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rtID := uuid.New()
	routeID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("DELETE FROM routes r USING route_tables rt JOIN vpcs v ON rt.vpc_id = v.id WHERE r.route_table_id = rt.id AND r.id = .+AND rt.id = .+AND v.tenant_id = .+").
		WithArgs(routeID, rtID, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err = repo.RemoveRoute(ctx, rtID, routeID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRouteTableRepository_AssociateSubnet(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rtID := uuid.New()
	subnetID := uuid.New()

	mock.ExpectExec("INSERT INTO route_table_associations").
		WithArgs(pgxmock.AnyArg(), rtID, subnetID, "now").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.AssociateSubnet(context.Background(), rtID, subnetID)
	require.NoError(t, err)
}

func TestRouteTableRepository_DisassociateSubnet(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rtID := uuid.New()
	subnetID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("DELETE FROM route_table_associations rta USING route_tables rt JOIN vpcs v ON rt.vpc_id = v.id WHERE rta.route_table_id = rt.id AND rta.subnet_id = .+AND rt.id = .+AND v.tenant_id = .+").
		WithArgs(subnetID, rtID, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.DisassociateSubnet(ctx, rtID, subnetID)
	require.NoError(t, err)
}

func TestRouteTableRepository_DisassociateSubnet_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rtID := uuid.New()
	subnetID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("DELETE FROM route_table_associations rta USING route_tables rt JOIN vpcs v ON rt.vpc_id = v.id WHERE rta.route_table_id = rt.id AND rta.subnet_id = .+AND rt.id = .+AND v.tenant_id = .+").
		WithArgs(subnetID, rtID, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err = repo.DisassociateSubnet(ctx, rtID, subnetID)
	require.NoError(t, err)
}

func TestRouteTableRepository_ListAssociatedSubnets(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rtID := uuid.New()
	subnetID := uuid.New()

	mock.ExpectQuery("SELECT subnet_id FROM route_table_associations WHERE route_table_id = .+").
		WithArgs(rtID).
		WillReturnRows(pgxmock.NewRows([]string{"subnet_id"}).AddRow(subnetID))

	result, err := repo.ListAssociatedSubnets(context.Background(), rtID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, subnetID, result[0])
}

func TestRouteTableRepository_ListAssociatedSubnets_Empty(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewRouteTableRepository(mock)
	rtID := uuid.New()

	mock.ExpectQuery("SELECT subnet_id FROM route_table_associations WHERE route_table_id = .+").
		WithArgs(rtID).
		WillReturnRows(pgxmock.NewRows([]string{"subnet_id"}))

	result, err := repo.ListAssociatedSubnets(context.Background(), rtID)
	require.NoError(t, err)
	assert.Len(t, result, 0)
}
