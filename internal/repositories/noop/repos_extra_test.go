package noop

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestNoopRepositoriesExtra(t *testing.T) {
	ctx := context.Background()
	uID := uuid.New()
	id := uuid.New()

	t.Run("InstanceRepository", func(t *testing.T) {
		repo := &NoopInstanceRepository{}
		list, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, list)

		list, err = repo.ListAll(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)

		require.NoError(t, repo.Update(ctx, &domain.Instance{}))

		list, err = repo.ListBySubnet(ctx, id)
		require.NoError(t, err)
		assert.Empty(t, list)

		list, err = repo.ListByUserID(ctx, uID)
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("VpcRepository", func(t *testing.T) {
		repo := &NoopVpcRepository{}
		require.NoError(t, repo.Create(ctx, &domain.VPC{}))
		_, err := repo.GetByName(ctx, "name")
		require.NoError(t, err)
		list, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)
		list, err = repo.ListByUserID(ctx, uID)
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("SubnetRepository", func(t *testing.T) {
		repo := &NoopSubnetRepository{}
		require.NoError(t, repo.Create(ctx, &domain.Subnet{}))
		list, err := repo.ListByVPC(ctx, id)
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	t.Run("VolumeRepository", func(t *testing.T) {
		repo := &NoopVolumeRepository{}
		require.NoError(t, repo.Create(ctx, &domain.Volume{}))
		_, err := repo.GetByName(ctx, "name")
		require.NoError(t, err)
		list, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)
		list, err = repo.ListByUserID(ctx, uID)
		require.NoError(t, err)
		assert.Empty(t, list)
		list, err = repo.ListByInstanceID(ctx, id)
		require.NoError(t, err)
		assert.Empty(t, list)
		require.NoError(t, repo.Update(ctx, &domain.Volume{}))
	})

	t.Run("DatabaseRepository", func(t *testing.T) {
		repo := &NoopDatabaseRepository{}
		require.NoError(t, repo.Create(ctx, &domain.Database{}))
		_, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		list, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)
		require.NoError(t, repo.Update(ctx, &domain.Database{}))
		require.NoError(t, repo.Delete(ctx, id))
	})

	t.Run("CacheRepository", func(t *testing.T) {
		repo := &NoopCacheRepository{}
		require.NoError(t, repo.Create(ctx, &domain.Cache{}))
		_, err := repo.GetByID(ctx, id, uID)
		require.NoError(t, err)
		_, err = repo.GetByName(ctx, uID, "name")
		require.NoError(t, err)
		list, err := repo.List(ctx, uID)
		require.NoError(t, err)
		assert.Empty(t, list)
		require.NoError(t, repo.Update(ctx, &domain.Cache{}))
		require.NoError(t, repo.Delete(ctx, id, uID))
	})

	t.Run("FunctionRepository", func(t *testing.T) {
		repo := &NoopFunctionRepository{}
		require.NoError(t, repo.Create(ctx, &domain.Function{}))
		_, err := repo.GetByName(ctx, uID, "name")
		require.NoError(t, err)
		list, err := repo.List(ctx, uID)
		require.NoError(t, err)
		assert.Empty(t, list)
		require.NoError(t, repo.Delete(ctx, id))
	})

	t.Run("UserRepository", func(t *testing.T) {
		repo := &NoopUserRepository{}
		require.NoError(t, repo.Create(ctx, &domain.User{}))
		user, err := repo.GetByEmail(ctx, "test@example.com")
		require.NoError(t, err)
		assert.Nil(t, user)
		_, err = repo.GetByID(ctx, id)
		require.NoError(t, err)
		require.NoError(t, repo.Update(ctx, &domain.User{}))
		list, err := repo.List(ctx)
		require.NoError(t, err)
		assert.Empty(t, list)
		require.NoError(t, repo.Delete(ctx, id))
	})
}
