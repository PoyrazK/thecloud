//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRBACRepository_Integration(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	repo := NewRBACRepository(db)
	ctx := context.Background()

	// Cleanup
	_, _ = db.Exec(ctx, "DELETE FROM role_permissions")
	_, _ = db.Exec(ctx, "DELETE FROM roles")

	t.Run("CreateAndGetRole", func(t *testing.T) {
		roleID := uuid.New()
		role := &domain.Role{
			ID:          roleID,
			Name:        "test-role",
			Description: "Test role description",
			Permissions: []domain.Permission{
				domain.PermissionInstanceRead,
				domain.PermissionInstanceLaunch,
			},
		}

		err := repo.CreateRole(ctx, role)
		require.NoError(t, err)

		fetched, err := repo.GetRoleByID(ctx, roleID)
		require.NoError(t, err)
		assert.Equal(t, role.Name, fetched.Name)
		assert.Equal(t, role.Description, fetched.Description)
		assert.Len(t, fetched.Permissions, 2)
		assert.Contains(t, fetched.Permissions, domain.PermissionInstanceRead)
	})

	t.Run("GetRoleByName", func(t *testing.T) {
		fetched, err := repo.GetRoleByName(ctx, "test-role")
		require.NoError(t, err)
		assert.Equal(t, "test-role", fetched.Name)
		assert.NotEmpty(t, fetched.Permissions)
	})

	t.Run("ListRoles", func(t *testing.T) {
		roles, err := repo.ListRoles(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, roles)

		found := false
		for _, r := range roles {
			if r.Name == "test-role" {
				found = true
				assert.NotEmpty(t, r.Permissions)
			}
		}
		assert.True(t, found)
	})

	t.Run("UpdateRole", func(t *testing.T) {
		role, err := repo.GetRoleByName(ctx, "test-role")
		require.NoError(t, err)

		role.Description = "Updated description"
		role.Permissions = []domain.Permission{domain.PermissionFullAccess}

		err = repo.UpdateRole(ctx, role)
		require.NoError(t, err)

		fetched, err := repo.GetRoleByID(ctx, role.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", fetched.Description)
		assert.Len(t, fetched.Permissions, 1)
		assert.Equal(t, domain.PermissionFullAccess, fetched.Permissions[0])
	})

	t.Run("AddPermissionToRole", func(t *testing.T) {
		role, err := repo.GetRoleByName(ctx, "test-role")
		require.NoError(t, err)

		err = repo.AddPermissionToRole(ctx, role.ID, domain.PermissionVpcCreate)
		require.NoError(t, err)

		perms, err := repo.GetPermissionsForRole(ctx, role.ID)
		require.NoError(t, err)
		assert.Contains(t, perms, domain.PermissionVpcCreate)
	})

	t.Run("RemovePermissionFromRole", func(t *testing.T) {
		role, err := repo.GetRoleByName(ctx, "test-role")
		require.NoError(t, err)

		err = repo.RemovePermissionFromRole(ctx, role.ID, domain.PermissionVpcCreate)
		require.NoError(t, err)

		perms, err := repo.GetPermissionsForRole(ctx, role.ID)
		require.NoError(t, err)
		assert.NotContains(t, perms, domain.PermissionVpcCreate)
	})

	t.Run("DeleteRole", func(t *testing.T) {
		role, err := repo.GetRoleByName(ctx, "test-role")
		require.NoError(t, err)

		err = repo.DeleteRole(ctx, role.ID)
		require.NoError(t, err)

		_, err = repo.GetRoleByID(ctx, role.ID)
		assert.Error(t, err)
	})
}
