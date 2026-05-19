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
	"github.com/poyrazk/thecloud/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testVpcName = "test-vpc"
	selectVpc   = "SELECT id, user_id, tenant_id, name, COALESCE\\(cidr_block::text, ''\\), network_id, vxlan_id, status, arn, created_at FROM vpcs"
)

func TestVpcRepositoryCreate(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		vpc := &domain.VPC{
			ID:        uuid.New(),
			UserID:    uuid.New(),
			TenantID:  uuid.New(),
			Name:      testVpcName,
			CIDRBlock: testutil.TestCIDR,
			NetworkID: "net-1",
			VXLANID:   100,
			Status:    "available",
			ARN:       "arn",
			CreatedAt: time.Now(),
		}

		mock.ExpectExec("INSERT INTO vpcs").
			WithArgs(vpc.ID, vpc.UserID, vpc.TenantID, vpc.Name, vpc.CIDRBlock, vpc.NetworkID, vpc.VXLANID, vpc.Status, vpc.ARN, vpc.CreatedAt, vpc.IdempotencyKey).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = repo.Create(context.Background(), vpc)
		require.NoError(t, err)
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		vpc := &domain.VPC{
			ID: uuid.New(),
		}

		mock.ExpectExec("INSERT INTO vpcs").
			WillReturnError(errors.New(testDBError))

		err = repo.Create(context.Background(), vpc)
		require.Error(t, err)
	})
}

func TestVpcRepositoryGetByID(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()

		mock.ExpectQuery(selectVpc).
			WithArgs(id, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "cidr_block", "network_id", "vxlan_id", "status", "arn", "created_at"}).
				AddRow(id, userID, tenantID, testVpcName, testutil.TestCIDR, "net-1", 100, "available", "arn", now))

		vpc, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.NotNil(t, vpc)
		assert.Equal(t, id, vpc.ID)
	})

	t.Run(testNotFound, func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectQuery(selectVpc).
			WithArgs(id, tenantID).
			WillReturnError(pgx.ErrNoRows)

		vpc, err := repo.GetByID(ctx, id)
		require.Error(t, err)
		assert.Nil(t, vpc)
		// Check if the error returned is of type errors.NotFound
		var target theclouderrors.Error
		if errors.As(err, &target) {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
		// If it's wrapped or directly the error we expect (but it seems the repo wraps it)
		// The implementation: return nil, errors.New(errors.NotFound, fmt.Sprintf("vpc %s not found", id))
		// So it should be a custom error.
	})

	t.Run(testDBError, func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectQuery(selectVpc).
			WithArgs(id, tenantID).
			WillReturnError(errors.New(testDBError))

		vpc, err := repo.GetByID(ctx, id)
		require.Error(t, err)
		assert.Nil(t, vpc)
	})
}

func TestVpcRepositoryGetByName(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()
		name := testVpcName

		mock.ExpectQuery(selectVpc).
			WithArgs(name, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "cidr_block", "network_id", "vxlan_id", "status", "arn", "created_at"}).
				AddRow(id, userID, tenantID, name, testutil.TestCIDR, "net-1", 100, "available", "arn", now))

		vpc, err := repo.GetByName(ctx, name)
		require.NoError(t, err)
		assert.NotNil(t, vpc)
		assert.Equal(t, id, vpc.ID)
	})

	t.Run(testNotFound, func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		name := testVpcName

		mock.ExpectQuery(selectVpc).
			WithArgs(name, tenantID).
			WillReturnError(pgx.ErrNoRows)

		vpc, err := repo.GetByName(ctx, name)
		require.Error(t, err)
		assert.Nil(t, vpc)
	})
}

func TestVpcRepositoryList(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()

		mock.ExpectQuery(selectVpc).
			WithArgs(tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "cidr_block", "network_id", "vxlan_id", "status", "arn", "created_at"}).
				AddRow(uuid.New(), userID, tenantID, testVpcName, testutil.TestCIDR, "net-1", 100, "available", "arn", now))

		vpcs, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Len(t, vpcs, 1)
	})

	t.Run(testDBError, func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectQuery(selectVpc).
			WithArgs(tenantID).
			WillReturnError(errors.New(testDBError))

		vpcs, err := repo.List(ctx)
		require.Error(t, err)
		assert.Nil(t, vpcs)
	})

	t.Run("scan error", func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()

		// Return a row with incompatible types to force scan error
		mock.ExpectQuery(selectVpc).
			WithArgs(tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "cidr_block", "network_id", "vxlan_id", "status", "arn", "created_at"}).
				AddRow("invalid-uuid", userID, tenantID, testVpcName, testutil.TestCIDR, "net-1", 100, "available", "arn", now))

		vpcs, err := repo.List(ctx)
		require.Error(t, err)
		assert.Nil(t, vpcs)
	})
}

func TestVpcRepositoryDelete(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectExec("DELETE FROM vpcs").
			WithArgs(id, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		err = repo.Delete(ctx, id)
		require.NoError(t, err)
	})

	t.Run(testNotFound, func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectExec("DELETE FROM vpcs").
			WithArgs(id, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 0))

		err = repo.Delete(ctx, id)
		require.Error(t, err)
	})
}

func TestVpcRepositoryGetByIdempotencyKey(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()
		key := "test-idempotency-key"

		mock.ExpectQuery("SELECT id, user_id, tenant_id, name, COALESCE").
			WithArgs(key, userID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "cidr_block", "network_id", "vxlan_id", "status", "arn", "created_at"}).
				AddRow(id, userID, tenantID, testVpcName, testutil.TestCIDR, "net-1", 100, "available", "arn", now))

		vpc, err := repo.GetByIdempotencyKey(ctx, key)
		require.NoError(t, err)
		assert.NotNil(t, vpc)
		assert.Equal(t, id, vpc.ID)
		assert.Equal(t, testVpcName, vpc.Name)
	})

	t.Run("empty key returns not found", func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		ctx := context.Background()

		vpc, err := repo.GetByIdempotencyKey(ctx, "")
		require.Error(t, err)
		assert.Nil(t, vpc)
		var target theclouderrors.Error
		if errors.As(err, &target) {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})

	t.Run(testNotFound, func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		userID := uuid.New()
		ctx := appcontext.WithUserID(context.Background(), userID)

		mock.ExpectQuery("SELECT id, user_id, tenant_id, name, COALESCE\\(cidr_block::text, ''\\), network_id, vxlan_id, status, arn, created_at FROM vpcs WHERE idempotency_key = $1 AND user_id = $2").
			WithArgs("nonexistent-key", userID).
			WillReturnError(pgx.ErrNoRows)

		vpc, err := repo.GetByIdempotencyKey(ctx, "nonexistent-key")
		require.Error(t, err)
		assert.Nil(t, vpc)
	})

	t.Run(testDBError, func(t *testing.T) {
		t.Parallel()
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewVpcRepository(mock)
		userID := uuid.New()
		ctx := appcontext.WithUserID(context.Background(), userID)

		mock.ExpectQuery("SELECT id, user_id, tenant_id, name, COALESCE\\(cidr_block::text, ''\\), network_id, vxlan_id, status, arn, created_at FROM vpcs WHERE idempotency_key = $1 AND user_id = $2").
			WithArgs("some-key", userID).
			WillReturnError(errors.New(testDBError))

		vpc, err := repo.GetByIdempotencyKey(ctx, "some-key")
		require.Error(t, err)
		assert.Nil(t, vpc)
	})
}
