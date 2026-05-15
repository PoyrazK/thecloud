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

func igwTestGateway() *domain.InternetGateway {
	return &domain.InternetGateway{
		ID:        uuid.New(),
		VPCID:     nil,
		UserID:    uuid.New(),
		TenantID:  uuid.New(),
		Status:    domain.IGWStatusDetached,
		ARN:       "arn:aws:ec2::123456789012:internet-gateway/igw-1234567890",
		CreatedAt: time.Now(),
	}
}

func TestIGWRepository_Create(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIGWRepository(mock)
	igw := igwTestGateway()

	mock.ExpectExec("INSERT INTO internet_gateways").
		WithArgs(igw.ID, igw.VPCID, igw.UserID, igw.TenantID, igw.Status, igw.ARN, igw.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.Create(context.Background(), igw)
	require.NoError(t, err)
}

func TestIGWRepository_GetByID(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIGWRepository(mock)
	igw := igwTestGateway()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT .+ FROM internet_gateways WHERE id = .+AND tenant_id = .+").
		WithArgs(igw.ID, tenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "vpc_id", "user_id", "tenant_id", "status", "arn", "created_at"}).
			AddRow(igw.ID, igw.VPCID, igw.UserID, igw.TenantID, "detached", igw.ARN, igw.CreatedAt))

	result, err := repo.GetByID(ctx, igw.ID)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestIGWRepository_GetByID_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIGWRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT .+ FROM internet_gateways WHERE id = .+AND tenant_id = .+").
		WithArgs(id, tenantID).
		WillReturnError(context.DeadlineExceeded)

	result, err := repo.GetByID(ctx, id)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestIGWRepository_GetByVPC(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIGWRepository(mock)
	igw := igwTestGateway()
	vpcID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT .+ FROM internet_gateways WHERE vpc_id = .+AND tenant_id = .+").
		WithArgs(vpcID, tenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "vpc_id", "user_id", "tenant_id", "status", "arn", "created_at"}).
			AddRow(igw.ID, igw.VPCID, igw.UserID, igw.TenantID, "detached", igw.ARN, igw.CreatedAt))

	result, err := repo.GetByVPC(ctx, vpcID)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestIGWRepository_GetByVPC_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIGWRepository(mock)
	vpcID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT .+ FROM internet_gateways WHERE vpc_id = .+AND tenant_id = .+").
		WithArgs(vpcID, tenantID).
		WillReturnError(context.DeadlineExceeded)

	result, err := repo.GetByVPC(ctx, vpcID)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestIGWRepository_Update(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIGWRepository(mock)
	igw := igwTestGateway()
	vpcID := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("UPDATE internet_gateways SET vpc_id = .+, status = .+ WHERE id = .+AND tenant_id = .+").
		WithArgs(pgxmock.AnyArg(), igw.Status, igw.ID, tenantID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	igw.VPCID = &vpcID
	err = repo.Update(ctx, igw)
	require.NoError(t, err)
}

func TestIGWRepository_Update_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIGWRepository(mock)
	igw := igwTestGateway()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("UPDATE internet_gateways SET vpc_id = .+, status = .+ WHERE id = .+AND tenant_id = .+").
		WithArgs(pgxmock.AnyArg(), igw.Status, igw.ID, tenantID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err = repo.Update(ctx, igw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestIGWRepository_Delete(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIGWRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("DELETE FROM internet_gateways WHERE id = .+AND tenant_id = .+").
		WithArgs(id, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.Delete(ctx, id)
	require.NoError(t, err)
}

func TestIGWRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIGWRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectExec("DELETE FROM internet_gateways WHERE id = .+AND tenant_id = .+").
		WithArgs(id, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err = repo.Delete(ctx, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestIGWRepository_ListAll skipped: IGWStatus scan requires string→custom-type
// conversion that pgxmock cannot mock for batch scans. Use integration tests.
func TestIGWRepository_ListAll(t *testing.T) {
	t.Skip("IGWStatus custom scan unsupported by pgxmock for batch scans")
}

func TestIGWRepository_ListAll_Empty(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewIGWRepository(mock)
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery("SELECT .+ FROM internet_gateways WHERE tenant_id = .+ORDER BY created_at DESC").
		WithArgs(tenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "vpc_id", "user_id", "tenant_id", "status", "arn", "created_at"}))

	result, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 0)
}
