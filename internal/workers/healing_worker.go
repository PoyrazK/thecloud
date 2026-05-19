// Package workers provides background worker implementations.
package workers

import (
	"context"
	"log/slog"
	"sync"
	"time"

	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

const (
	defaultHealingTickInterval = 1 * time.Minute
	defaultHealingDelay        = 5 * time.Second
	maxConcurrentHeals         = 10
)

// HealingWorker periodically checks for instances in ERROR state and attempts to recover them.
type HealingWorker struct {
	instSvc ports.InstanceService
	repo    ports.InstanceRepository
	logger  *slog.Logger

	// Testing hooks
	reconcileWG  *sync.WaitGroup
	healingDelay time.Duration
	tickInterval time.Duration

	// Concurrency control
	semaphore chan struct{}
}

// NewHealingWorker constructs a HealingWorker.
func NewHealingWorker(instSvc ports.InstanceService, repo ports.InstanceRepository, logger *slog.Logger) *HealingWorker {
	return &HealingWorker{
		instSvc:      instSvc,
		repo:         repo,
		logger:       logger,
		reconcileWG:  &sync.WaitGroup{},
		healingDelay: defaultHealingDelay,
		tickInterval: defaultHealingTickInterval,
		semaphore:    make(chan struct{}, maxConcurrentHeals),
	}
}

// Run starts the healing loop.
func (w *HealingWorker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	w.logger.Info("starting healing worker", "interval", w.tickInterval, "concurrency_limit", maxConcurrentHeals)
	ticker := time.NewTicker(w.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("stopping healing worker, waiting for active healing tasks...")
			w.reconcileWG.Wait()
			return
		case <-ticker.C:
			w.healERRORInstances(ctx)
		}
	}
}

func (w *HealingWorker) healERRORInstances(ctx context.Context) {
	instances, err := w.repo.ListAll(ctx)
	if err != nil {
		w.logger.Error("failed to list all instances for healing", "error", err)
		return
	}

	for _, inst := range instances {
		if inst.Status != domain.StatusError {
			continue
		}

		// Skip managed instances if possible (heuristic: name prefix or labels if we start using them)
		// For now, we try to heal all ERROR instances. Overlapping with other workers is mostly safe
		// because of optimistic locking and status checks in InstanceService.

		w.logger.Info("attempting to heal instance", "instance_id", inst.ID, "name", inst.Name)

		// Acquire semaphore token
		select {
		case w.semaphore <- struct{}{}:
		case <-ctx.Done():
			return
		}

		// Inject context for the instance owner
		hCtx := appcontext.WithUserID(ctx, inst.UserID)
		hCtx = appcontext.WithTenantID(hCtx, inst.TenantID)

		w.reconcileWG.Add(1)
		// Self-healing: Stop then Start
		go func(id string) {
			defer w.reconcileWG.Done()
			defer func() { <-w.semaphore }() // Release semaphore token

			// Wait a bit to avoid race conditions with recent failures
			timer := time.NewTimer(w.healingDelay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}

			w.logger.Info("initiating healing restart", "instance_id", id)
			if err := w.instSvc.StopInstance(hCtx, id); err != nil {
				w.logger.Warn("healing: stop failed", "instance_id", id, "error", err)
				// Continue anyway, maybe it was already half-stopped
			}

			if err := w.instSvc.StartInstance(hCtx, id); err != nil {
				w.logger.Error("healing: start failed", "instance_id", id, "error", err)
			} else {
				w.logger.Info("successfully healed instance", "instance_id", id)
			}
		}(inst.ID.String())
	}
}
