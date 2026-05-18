//go:build integration
// +build integration

package services_test

import (
	"context"
	"log/slog"
	"testing"

	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/repositories/noop"
	"github.com/poyrazk/thecloud/internal/repositories/postgres"
	"github.com/poyrazk/thecloud/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupVPCServiceTest(t *testing.T) (ports.VpcService, ports.VpcRepository, context.Context) {
	t.Helper()
	db := setupDB(t)
	cleanDB(t, db)
	ctx := setupTestUser(t, db)

	repo := postgres.NewVpcRepository(db)
	lbRepo := postgres.NewLBRepository(db)
	network := noop.NewNoopNetworkAdapter(slog.Default())
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	auditRepo := postgres.NewAuditRepository(db)
	auditSvc := services.NewAuditService(services.AuditServiceParams{
		Repo:    auditRepo,
		RBACSvc: rbacSvc,
	})

	logger := slog.Default()
	svc := services.NewVpcService(services.VpcServiceParams{
		Repo:        repo,
		LBRepo:      lbRepo,
		RBACSvc:     rbacSvc,
		Network:     network,
		AuditSvc:    auditSvc,
		Logger:      logger,
		DefaultCIDR: testutil.TestCIDR,
	})

	return svc, repo, ctx
}

func TestVpcService_Integration(t *testing.T) {
	svc, repo, ctx := setupVPCServiceTest(t)
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	t.Run("CreateAndGet", func(t *testing.T) {
		name := "test-vpc"
		vpc, err := svc.CreateVPC(ctx, name, testutil.TestCIDR, "")
		require.NoError(t, err)
		assert.Equal(t, name, vpc.Name)
		assert.Equal(t, userID, vpc.UserID)
		assert.Equal(t, tenantID, vpc.TenantID)

		fetched, err := svc.GetVPC(ctx, vpc.ID.String())
		require.NoError(t, err)
		assert.Equal(t, vpc.ID, fetched.ID)
	})

	t.Run("ListVPCs", func(t *testing.T) {
		_, _ = svc.CreateVPC(ctx, "vpc1", testutil.TestCIDR, "")
		_, _ = svc.CreateVPC(ctx, "vpc2", testutil.TestCIDR, "")

		vpcs, err := svc.ListVPCs(ctx)
		require.NoError(t, err)
		// Including previous test VPC, should be 3
		assert.GreaterOrEqual(t, len(vpcs), 2)
	})

	t.Run("Delete", func(t *testing.T) {
		vpc, _ := svc.CreateVPC(ctx, "to-delete", testutil.TestCIDR, "")

		err := svc.DeleteVPC(ctx, vpc.ID.String())
		require.NoError(t, err)

		_, err = repo.GetByID(ctx, vpc.ID)
		require.Error(t, err)
	})
}
