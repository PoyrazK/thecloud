package postgres

import (
	"context"
	"errors" // Standard errors for mocking expectations
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

const (
	lbQueryPattern = "SELECT id, user_id, tenant_id, COALESCE.+idempotency_key.+name, vpc_id, port, algorithm, COALESCE.+ip.+status, version, created_at FROM load_balancers"
	errDbMessage   = "db error"
	errNotFound    = "not found"
)

func TestLBRepositoryCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		lb := &domain.LoadBalancer{
			ID:             uuid.New(),
			UserID:         uuid.New(),
			TenantID:       uuid.New(),
			IdempotencyKey: "key-1",
			Name:           "lb-1",
			VpcID:          uuid.New(),
			Port:           80,
			Algorithm:      "round_robin",
			Status:         domain.LBStatusActive,
			Version:        1,
			CreatedAt:      time.Now(),
		}

		mock.ExpectExec("INSERT INTO load_balancers").
			WithArgs(lb.ID, lb.UserID, lb.IdempotencyKey, lb.Name, lb.VpcID, lb.Port, lb.Algorithm, lb.IP, lb.Status, lb.Version, lb.CreatedAt).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = repo.Create(context.Background(), lb)
		require.NoError(t, err)
	})

	t.Run(errDbMessage, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		lb := &domain.LoadBalancer{ID: uuid.New()}

		mock.ExpectExec("INSERT INTO load_balancers").
			WillReturnError(errors.New(errDbMessage))

		err = repo.Create(context.Background(), lb)
		require.Error(t, err)
	})
}

func TestLBRepositoryGetByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		id := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(context.Background(), tenantID)
		now := time.Now()

		mock.ExpectQuery(lbQueryPattern).
			WithArgs(id, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "idempotency_key", "name", "vpc_id", "port", "algorithm", "ip", "status", "version", "created_at"}).
				AddRow(id, uuid.New(), tenantID, "key-1", "lb-1", uuid.New(), 80, "round_robin", "10.0.0.1", string(domain.LBStatusActive), 1, now))

		lb, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		assert.NotNil(t, lb)
		assert.Equal(t, id, lb.ID)
	})

	t.Run(errNotFound, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		id := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(context.Background(), tenantID)

		mock.ExpectQuery(lbQueryPattern).
			WithArgs(id, tenantID).
			WillReturnError(pgx.ErrNoRows)

		lb, err := repo.GetByID(ctx, id)
		require.Error(t, err)
		assert.Nil(t, lb)
		assert.Equal(t, theclouderrors.ErrLBNotFound, err)
	})
}

func TestLBRepositoryList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(context.Background(), tenantID)
		now := time.Now()

		mock.ExpectQuery(lbQueryPattern).
			WithArgs(tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "idempotency_key", "name", "vpc_id", "port", "algorithm", "ip", "status", "version", "created_at"}).
				AddRow(uuid.New(), uuid.New(), tenantID, "key-1", "lb-1", uuid.New(), 80, "round_robin", "10.0.0.1", string(domain.LBStatusActive), 1, now))

		lbs, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Len(t, lbs, 1)
	})

	t.Run(errDbMessage, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(context.Background(), tenantID)

		mock.ExpectQuery(lbQueryPattern).
			WithArgs(tenantID).
			WillReturnError(errors.New(errDbMessage))

		list, err := repo.List(ctx)
		require.Error(t, err)
		assert.Nil(t, list)
	})
}

func TestLBRepositoryUpdate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		tenantID := uuid.New()
		lb := &domain.LoadBalancer{
			ID:        uuid.New(),
			TenantID:  tenantID,
			Name:      "lb-updated",
			Port:      8080,
			Algorithm: "least_conn",
			Status:    domain.LBStatusActive,
			Version:   1,
		}
		ctx := appcontext.WithTenantID(context.Background(), tenantID)

		mock.ExpectExec("UPDATE load_balancers").
			WithArgs(lb.Name, lb.Port, lb.Algorithm, lb.IP, lb.Status, lb.ID, lb.Version, lb.TenantID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err = repo.Update(ctx, lb)
		require.NoError(t, err)
		assert.Equal(t, 2, lb.Version)
	})

	t.Run("conflict", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		tenantID := uuid.New()
		lb := &domain.LoadBalancer{
			ID:       uuid.New(),
			TenantID: tenantID,
			Version:  1,
		}
		ctx := appcontext.WithTenantID(context.Background(), tenantID)

		mock.ExpectExec("UPDATE load_balancers").
			WithArgs(lb.Name, lb.Port, lb.Algorithm, lb.IP, lb.Status, lb.ID, lb.Version, lb.TenantID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 0))

		err = repo.Update(ctx, lb)
		require.Error(t, err)
		var theCloudErr *theclouderrors.Error
		if errors.As(err, &theCloudErr) {
			assert.Equal(t, theclouderrors.Conflict, theCloudErr.Type)
		}
	})
}

func TestLBRepositoryDelete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		id := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(context.Background(), tenantID)

		mock.ExpectExec("DELETE FROM load_balancers").
			WithArgs(id, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		err = repo.Delete(ctx, id)
		require.NoError(t, err)
	})

	t.Run(errNotFound, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		id := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(context.Background(), tenantID)

		mock.ExpectExec("DELETE FROM load_balancers").
			WithArgs(id, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 0))

		err = repo.Delete(ctx, id)
		require.Error(t, err)
		assert.Equal(t, theclouderrors.ErrLBNotFound, err)
	})
}

func TestLBRepositoryAddTarget(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		target := &domain.LBTarget{
			ID:         uuid.New(),
			LBID:       uuid.New(),
			InstanceID: uuid.New(),
			Port:       80,
			Weight:     1,
			Health:     "healthy",
		}

		mock.ExpectExec("INSERT INTO lb_targets").
			WithArgs(target.ID, target.LBID, target.InstanceID, target.Port, target.Weight, target.Health).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = repo.AddTarget(context.Background(), target)
		require.NoError(t, err)
	})
}

func TestLBRepositoryRemoveTarget(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		lbID := uuid.New()
		instanceID := uuid.New()
		tenantID := uuid.New()

		mock.ExpectExec("DELETE FROM lb_targets").
			WithArgs(lbID, instanceID, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		err = repo.RemoveTarget(appcontext.WithTenantID(context.Background(), tenantID), lbID, instanceID)
		require.NoError(t, err)
	})

	t.Run(errNotFound, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		lbID := uuid.New()
		instanceID := uuid.New()
		tenantID := uuid.New()

		mock.ExpectExec("DELETE FROM lb_targets").
			WithArgs(lbID, instanceID, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 0))

		err = repo.RemoveTarget(appcontext.WithTenantID(context.Background(), tenantID), lbID, instanceID)
		require.Error(t, err)
	})
}

func TestLBRepositoryListTargets(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		lbID := uuid.New()
		tenantID := uuid.New()

		mock.ExpectQuery("SELECT t.id, t.lb_id, t.instance_id, t.port, t.weight, t.health FROM lb_targets t").
			WithArgs(lbID, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "lb_id", "instance_id", "port", "weight", "health"}).
				AddRow(uuid.New(), lbID, uuid.New(), 80, 1, "healthy"))

		targets, err := repo.ListTargets(appcontext.WithTenantID(context.Background(), tenantID), lbID)
		require.NoError(t, err)
		assert.Len(t, targets, 1)
	})
}

func TestLBRepositoryUpdateTargetHealth(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewLBRepository(mock)
		lbID := uuid.New()
		instanceID := uuid.New()
		tenantID := uuid.New()
		health := "unhealthy"

		mock.ExpectExec("UPDATE lb_targets").
			WithArgs(health, lbID, instanceID, tenantID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err = repo.UpdateTargetHealth(appcontext.WithTenantID(context.Background(), tenantID), lbID, instanceID, health)
		require.NoError(t, err)
	})
}
