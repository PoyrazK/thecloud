// Package workers hosts background worker implementations.
package workers

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

// LifecycleWorker periodically enforces bucket lifecycle rules.
type LifecycleWorker struct {
	lifecycleRepo ports.LifecycleRepository
	storageSvc    ports.StorageService
	storageRepo   ports.StorageRepository
	logger        *slog.Logger
	interval      time.Duration
}

// NewLifecycleWorker constructs a LifecycleWorker.
func NewLifecycleWorker(lifecycleRepo ports.LifecycleRepository, storageSvc ports.StorageService, storageRepo ports.StorageRepository, logger *slog.Logger) *LifecycleWorker {
	return &LifecycleWorker{
		lifecycleRepo: lifecycleRepo,
		storageSvc:    storageSvc,
		storageRepo:   storageRepo,
		logger:        logger,
		interval:      24 * time.Hour,
	}
}

func (w *LifecycleWorker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	w.logger.Info("lifecycle worker started")

	// Run immediately on startup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.processRules(ctx)
	}()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("lifecycle worker stopping")
			return
		case <-ticker.C:
			w.processRules(ctx)
		}
	}
}

func (w *LifecycleWorker) processRules(ctx context.Context) {
	w.logger.Info("running lifecycle rule evaluation")
	rules, err := w.lifecycleRepo.GetEnabledRules(ctx)
	if err != nil {
		w.logger.Error("failed to fetch lifecycle rules", "error", err)
		return
	}

	for _, rule := range rules {
		w.processRule(ctx, rule)
	}
}

func (w *LifecycleWorker) processRule(ctx context.Context, rule *domain.LifecycleRule) {
	logger := w.logger.With("rule_id", rule.ID, "bucket", rule.BucketName)

	// Context with rule owner's ID to pass permission checks
	ruleCtx := appcontext.WithUserID(ctx, rule.UserID)

	// List all objects in bucket
	// Note: For production, this should support pagination and prefix filtering at DB level
	objects, err := w.storageSvc.ListObjects(ruleCtx, rule.BucketName)
	if err != nil {
		logger.Error("failed to list objects for lifecycle", "error", err)
		return
	}

	expiration := time.Duration(rule.ExpirationDays) * 24 * time.Hour
	now := time.Now().UTC()
	deletedCount := 0

	for _, obj := range objects {
		// Filter by prefix
		if rule.Prefix != "" && !strings.HasPrefix(obj.Key, rule.Prefix) {
			continue
		}

		// Check expiration. time.Sub is timezone-agnostic (operates on
		// absolute instants), but converting CreatedAt to UTC explicitly
		// keeps log lines and any future Format() calls in a single zone.
		age := now.Sub(obj.CreatedAt.UTC())
		if age > expiration {
			logger.Info("expiring object", "key", obj.Key, "age_days", int(age.Hours()/24))
			if err := w.storageSvc.DeleteObject(ruleCtx, rule.BucketName, obj.Key); err != nil {
				logger.Error("failed to delete expired object", "key", obj.Key, "error", err)
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		logger.Info("lifecycle rule execution completed", "deleted_count", deletedCount)
	}
}
