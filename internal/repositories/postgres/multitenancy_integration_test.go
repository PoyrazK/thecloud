//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiTenancy_ResourceIsolation(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	// Clear tables
	ctx := context.Background()
	_, _ = db.Exec(ctx, "DELETE FROM instances")
	_, _ = db.Exec(ctx, "DELETE FROM load_balancers")
	_, _ = db.Exec(ctx, "DELETE FROM vpcs")
	_, _ = db.Exec(ctx, "DELETE FROM volumes")
	_, _ = db.Exec(ctx, "DELETE FROM scaling_groups")

	// Create user IDs
	userA := uuid.New()
	userB := uuid.New()

	// User contexts
	ctxA := appcontext.WithUserID(ctx, userA)
	ctxB := appcontext.WithUserID(ctx, userB)

	// Repositories
	userRepo := NewUserRepo(db)
	vpcRepo := NewVpcRepository(db)
	instRepo := NewInstanceRepository(db)
	lbRepo := NewLBRepository(db)
	volRepo := NewVolumeRepository(db)

	// Create Users
	err := userRepo.Create(ctx, &domain.User{
		ID:        userA,
		Email:     "usera@example.com",
		Name:      "User A",
		Role:      "user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	require.NoError(t, err)

	err = userRepo.Create(ctx, &domain.User{
		ID:        userB,
		Email:     "userb@example.com",
		Name:      "User B",
		Role:      "user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	require.NoError(t, err)

	t.Run("VPC Isolation", func(t *testing.T) {
		vpcA := &domain.VPC{
			ID:        uuid.New(),
			UserID:    userA,
			Name:      "vpc-a",
			NetworkID: "net-123",
			CreatedAt: time.Now(),
		}
		require.NoError(t, vpcRepo.Create(ctxA, vpcA))

		// User A can see it
		fetched, err := vpcRepo.GetByID(ctxA, vpcA.ID)
		require.NoError(t, err)
		assert.Equal(t, vpcA.ID, fetched.ID)

		// User B cannot see it
		_, err = vpcRepo.GetByID(ctxB, vpcA.ID)
		assert.Error(t, err)

		// List for User B should be empty
		listB, err := vpcRepo.List(ctxB)
		require.NoError(t, err)
		assert.Empty(t, listB)
	})

	t.Run("Instance Isolation", func(t *testing.T) {
		instA := &domain.Instance{
			ID:        uuid.New(),
			UserID:    userA,
			Name:      "inst-a",
			Image:     "alpine",
			Status:    domain.StatusRunning,
			CreatedAt: time.Now(),
		}
		require.NoError(t, instRepo.Create(ctxA, instA))

		// User A can see it
		fetched, err := instRepo.GetByID(ctxA, instA.ID)
		require.NoError(t, err)
		assert.Equal(t, instA.ID, fetched.ID)

		// User B cannot see it
		_, err = instRepo.GetByID(ctxB, instA.ID)
		assert.Error(t, err)

		// List for User B should be empty
		listB, err := instRepo.List(ctxB)
		require.NoError(t, err)
		assert.Empty(t, listB)
	})

	t.Run("Volume Isolation", func(t *testing.T) {
		volA := &domain.Volume{
			ID:        uuid.New(),
			UserID:    userA,
			Name:      "vol-a",
			SizeGB:    10,
			Status:    domain.VolumeStatusAvailable,
			CreatedAt: time.Now(),
		}
		require.NoError(t, volRepo.Create(ctxA, volA))

		// User A can see it
		fetched, err := volRepo.GetByID(ctxA, volA.ID)
		require.NoError(t, err)
		assert.Equal(t, volA.ID, fetched.ID)

		// User B cannot see it
		_, err = volRepo.GetByID(ctxB, volA.ID)
		assert.Error(t, err)

		// List for User B should be empty
		listB, err := volRepo.List(ctxB)
		require.NoError(t, err)
		assert.Empty(t, listB)
	})

	t.Run("Load Balancer Isolation", func(t *testing.T) {
		// Need a VPC first
		vpcID := uuid.New()
		_ = vpcRepo.Create(ctxA, &domain.VPC{ID: vpcID, UserID: userA, Name: "lb-vpc", NetworkID: "net-lb", CreatedAt: time.Now()})

		lbA := &domain.LoadBalancer{
			ID:        uuid.New(),
			UserID:    userA,
			Name:      "lb-a",
			VpcID:     vpcID,
			Port:      80,
			Algorithm: "round-robin",
			Status:    domain.LBStatusActive,
			CreatedAt: time.Now(),
		}
		require.NoError(t, lbRepo.Create(ctxA, lbA))

		// User A can see it
		fetched, err := lbRepo.GetByID(ctxA, lbA.ID)
		require.NoError(t, err)
		assert.Equal(t, lbA.ID, fetched.ID)

		// User B cannot see it
		_, err = lbRepo.GetByID(ctxB, lbA.ID)
		assert.Error(t, err)

		// List for User B should be empty
		listB, err := lbRepo.List(ctxB)
		require.NoError(t, err)
		assert.Empty(t, listB)
	})
}
