// Package workers hosts background worker implementations.
package workers

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/poyrazk/thecloud/internal/core/ports"
)

// MetricsCollectorWorker collects and exports storage metrics occasionally.
type MetricsCollectorWorker struct {
	storageRepo ports.StorageRepository
	storageSvc  ports.StorageService
	logger      *slog.Logger
	interval    time.Duration
}

// NewMetricsCollectorWorker constructs a MetricsCollectorWorker.
func NewMetricsCollectorWorker(storageRepo ports.StorageRepository, storageSvc ports.StorageService, logger *slog.Logger) *MetricsCollectorWorker {
	return &MetricsCollectorWorker{
		storageRepo: storageRepo,
		storageSvc:  storageSvc,
		logger:      logger,
		interval:    5 * time.Minute,
	}
}

func (w *MetricsCollectorWorker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	w.logger.Info("metrics collector worker started")

	// Initial run — call synchronously before the ticker loop
	w.collectMetrics(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("metrics collector worker stopping")
			return
		case <-ticker.C:
			w.collectMetrics(ctx)
		}
	}
}

func (w *MetricsCollectorWorker) collectMetrics(ctx context.Context) {
	w.logger.Info("collecting storage metrics")

	// We need a way to list all buckets (or system-wide stats)
	// Currently ListBuckets is per-user.
	// For metrics, we ideally need a system-admin capability or repository method "ListAllBuckets"
	// Lacking that, if this is a worker with system privileges, we'd need to bypass or iterate users.

	// For this implementation, we will update the repository interface to support ListAllBuckets
	// or we will query stats directly if efficient.

	// Assuming we might add ListAllBuckets later, for now we will skip bucket-level exact stats scan
	// if it is too expensive (listing all objects).

	// However, we should at least check cluster status
	status, err := w.storageSvc.GetClusterStatus(ctx)
	if err == nil {
		upNodes := 0
		for _, node := range status.Nodes {
			if node.Status == "up" || node.Status == "alive" {
				upNodes++
			}
		}
		// We could export this as a metric if we had a gauge for it.
		// platform.StorageClusterNodesUp.Set(float64(upNodes))
	}

	// NOTE: Real implementation would query DB for total size/objects per bucket efficiently via COUNT/SUM.
}
