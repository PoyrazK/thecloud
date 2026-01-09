package services_test

import (
	"context"
	"testing"

	"log/slog"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/repositories/noop"
)

func BenchmarkInstanceServiceList(b *testing.B) {
	// Setup
	repo := &noop.NoopInstanceRepository{}
	vpcRepo := &noop.NoopVpcRepository{}
	subnetRepo := &noop.NoopSubnetRepository{}
	volumeRepo := &noop.NoopVolumeRepository{}
	compute := &noop.NoopComputeBackend{}
	network := &noop.NoopNetworkAdapter{}
	eventSvc := &noop.NoopEventService{}
	auditSvc := &noop.NoopAuditService{}
	logger := slog.Default()

	svc := services.NewInstanceService(services.InstanceServiceParams{
		Repo:       repo,
		VpcRepo:    vpcRepo,
		SubnetRepo: subnetRepo,
		VolumeRepo: volumeRepo,
		Compute:    compute,
		Network:    network,
		EventSvc:   eventSvc,
		AuditSvc:   auditSvc,
		Logger:     logger,
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.ListInstances(ctx)
	}
}

func BenchmarkVPCServiceGet(b *testing.B) {
	repo := &noop.NoopVpcRepository{}
	network := &noop.NoopNetworkAdapter{}
	auditSvc := &noop.NoopAuditService{}
	logger := slog.Default()
	svc := services.NewVpcService(repo, network, auditSvc, logger, "10.0.0.0/16")

	ctx := context.Background()
	id := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.GetVPC(ctx, id.String())
	}
}
