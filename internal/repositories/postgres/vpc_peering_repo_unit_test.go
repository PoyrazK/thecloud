package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	theclouderrors "github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVPCPeeringRepository_Create(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVPCPeeringRepository(mock)
	peering := &domain.VPCPeering{
		ID:                uuid.New(),
		RequesterVPCID:    uuid.New(),
		AccepterVPCID:     uuid.New(),
		RequesterTenantID: uuid.New(),
		AccepterTenantID:  uuid.New(),
		Status:            domain.PeeringStatusPendingAcceptance,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	mock.ExpectExec("INSERT INTO vpc_peerings").
		WithArgs(peering.ID, peering.RequesterVPCID, peering.AccepterVPCID, peering.RequesterTenantID, peering.AccepterTenantID, peering.Status, peering.ARN, peering.CreatedAt, peering.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Create(context.Background(), peering)
	require.NoError(t, err)
}

func TestVPCPeeringRepository_GetByID(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVPCPeeringRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT id, requester_vpc_id, accepter_vpc_id, requester_tenant_id, accepter_tenant_id, status, arn, created_at, updated_at FROM vpc_peerings").
		WithArgs(id, tenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "requester_vpc_id", "accepter_vpc_id", "requester_tenant_id", "accepter_tenant_id", "status", "arn", "created_at", "updated_at"}).
			AddRow(id, uuid.New(), uuid.New(), tenantID, tenantID, domain.PeeringStatusActive, "", time.Now(), time.Now()))

	peering, err := repo.GetByID(ctx, id)
	require.NoError(t, err)
	assert.NotNil(t, peering)
	assert.Equal(t, id, peering.ID)
}

func TestVPCPeeringRepository_UpdateStatus(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVPCPeeringRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("UPDATE vpc_peerings").
		WithArgs(domain.PeeringStatusActive, id, tenantID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.UpdateStatus(ctx, id, domain.PeeringStatusActive)
	require.NoError(t, err)
}

func TestVPCPeeringRepository_Delete(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVPCPeeringRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("DELETE FROM vpc_peerings").
		WithArgs(id, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(ctx, id)
	require.NoError(t, err)
}

func TestVPCPeeringRepository_GetActiveByVPCPair(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVPCPeeringRepository(mock)
	vpc1 := uuid.New()
	vpc2 := uuid.New()
	tenantID := uuid.New()

	mock.ExpectQuery("SELECT id, requester_vpc_id, accepter_vpc_id, requester_tenant_id, accepter_tenant_id, status, arn, created_at, updated_at FROM vpc_peerings").
		WithArgs(vpc1, vpc2, domain.PeeringStatusPendingAcceptance, domain.PeeringStatusActive).
		WillReturnRows(pgxmock.NewRows([]string{"id", "requester_vpc_id", "accepter_vpc_id", "requester_tenant_id", "accepter_tenant_id", "status", "arn", "created_at", "updated_at"}).
			AddRow(uuid.New(), vpc1, vpc2, tenantID, tenantID, domain.PeeringStatusActive, "", time.Now(), time.Now()))

	peering, err := repo.GetActiveByVPCPair(context.Background(), vpc1, vpc2)
	require.NoError(t, err)
	assert.NotNil(t, peering)
}

func TestVPCPeeringRepository_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVPCPeeringRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT id, requester_vpc_id, accepter_vpc_id, requester_tenant_id, accepter_tenant_id, status, arn, created_at, updated_at FROM vpc_peerings").
		WithArgs(id, tenantID).
		WillReturnError(pgx.ErrNoRows)

	peering, err := repo.GetByID(ctx, id)
	require.Error(t, err)
	assert.Nil(t, peering)

	var target theclouderrors.Error
	if errors.As(err, &target) {
		assert.Equal(t, theclouderrors.NotFound, target.Type)
	}
}
