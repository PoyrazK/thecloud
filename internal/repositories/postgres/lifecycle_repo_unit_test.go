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

const (
	selectLifecycleRules = "SELECT .* FROM lifecycle_rules"
	testLifecyclePrefix  = "logs/"
)

func TestLifecycleRepository(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	ctx = appcontext.WithTenantID(ctx, tenantID)
	bucketName := "test-bucket"

	t.Run("Create", func(t *testing.T) {
		t.Parallel()
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		repo := NewLifecycleRepository(mock)
		rule := &domain.LifecycleRule{
			ID:             uuid.New(),
			UserID:         userID,
			BucketName:     bucketName,
			Prefix:         testLifecyclePrefix,
			ExpirationDays: 30,
			Enabled:        true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		mock.ExpectExec("INSERT INTO lifecycle_rules").
			WithArgs(rule.ID, rule.UserID, tenantID, rule.BucketName, rule.Prefix, rule.ExpirationDays, rule.Enabled, rule.CreatedAt, rule.UpdatedAt).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := repo.Create(ctx, rule)
		require.NoError(t, err)
	})

	t.Run("Get", func(t *testing.T) {
		t.Parallel()
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		repo := NewLifecycleRepository(mock)
		id := uuid.New()

		mock.ExpectQuery(selectLifecycleRules).
			WithArgs(id, userID, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "bucket_name", "prefix", "expiration_days", "enabled", "created_at", "updated_at"}).
				AddRow(id, userID, tenantID, bucketName, testLifecyclePrefix, 30, true, time.Now(), time.Now()))

		rule, err := repo.Get(ctx, id)
		require.NoError(t, err)
		assert.NotNil(t, rule)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		repo := NewLifecycleRepository(mock)
		id := uuid.New()

		mock.ExpectExec("DELETE FROM lifecycle_rules").
			WithArgs(id, userID, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		err := repo.Delete(ctx, id)
		require.NoError(t, err)
	})
}
