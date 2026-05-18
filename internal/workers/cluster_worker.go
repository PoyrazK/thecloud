// Package workers provides background worker implementations for various cloud tasks.
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

const (
	clusterQueue          = "k8s_jobs"
	clusterGroup          = "cluster_workers"
	clusterMaxWorkers     = 10
	clusterReclaimMs      = 5 * 60 * 1000 // 5 minutes
	clusterReclaimN       = 10
	clusterStaleThreshold = 15 * time.Minute
	clusterReceiveBackoff = 1 * time.Second
)

// ClusterWorker handles background tasks for Kubernetes cluster lifecycle management.
type ClusterWorker struct {
	repo         ports.ClusterRepository
	provisioner  ports.ClusterProvisioner
	taskQueue    ports.DurableTaskQueue
	ledger       ports.ExecutionLedger
	logger       *slog.Logger
	consumerName string
}

// NewClusterWorker creates a new ClusterWorker.
func NewClusterWorker(repo ports.ClusterRepository, provisioner ports.ClusterProvisioner, taskQueue ports.DurableTaskQueue, ledger ports.ExecutionLedger, logger *slog.Logger) *ClusterWorker {
	hostname, err := os.Hostname()
	if err != nil {
		logger.Warn("failed to get hostname, using fallback", "error", err)
		hostname = "cluster-worker"
	}
	if hostname == "" {
		hostname = "cluster-worker"
	}
	return &ClusterWorker{
		repo:         repo,
		provisioner:  provisioner,
		taskQueue:    taskQueue,
		ledger:       ledger,
		logger:       logger,
		consumerName: hostname,
	}
}

func (w *ClusterWorker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	w.logger.Info("starting cluster worker",
		"consumer", w.consumerName,
		"concurrency", clusterMaxWorkers,
	)

	if err := w.taskQueue.EnsureGroup(ctx, clusterQueue, clusterGroup); err != nil {
		w.logger.Error("failed to ensure cluster consumer group", "error", err)
		return
	}

	sem := make(chan struct{}, clusterMaxWorkers)

	go w.reclaimLoop(ctx, sem)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("stopping cluster worker")
			return
		default:
			msg, err := w.taskQueue.Receive(ctx, clusterQueue, clusterGroup, w.consumerName)
			if err != nil {
				w.logger.Error("failed to receive cluster job", "error", err)
				time.Sleep(clusterReceiveBackoff)
				continue
			}
			if msg == nil {
				continue
			}

			var job domain.ClusterJob
			if err := json.Unmarshal([]byte(msg.Payload), &job); err != nil {
				w.logger.Error("failed to unmarshal cluster job",
					"error", err, "msg_id", msg.ID)
				w.ackWithLog(ctx, msg.ID, "cluster poison message")
				continue
			}

			w.logger.Info("processing cluster job",
				"cluster_id", job.ClusterID,
				"type", job.Type,
				"msg_id", msg.ID,
			)

			sem <- struct{}{}
			go func(m *ports.DurableMessage, j domain.ClusterJob) {
				defer func() { <-sem }()
				w.processJob(ctx, m, j)
			}(msg, job)
		}
	}
}

func (w *ClusterWorker) processJob(workerCtx context.Context, msg *ports.DurableMessage, job domain.ClusterJob) {
	jobKey := fmt.Sprintf("cluster:%s:%s", job.Type, job.ClusterID)

	// Idempotency check.
	if w.ledger != nil {
		acquired, err := w.ledger.TryAcquire(workerCtx, jobKey, clusterStaleThreshold)
		if err != nil {
			w.logger.Error("execution ledger error",
				"cluster_id", job.ClusterID, "msg_id", msg.ID, "error", err)
			w.nackWithLog(workerCtx, msg.ID, "ledger try_acquire failed")
			return
		}
		if !acquired {
			// Check if it's already finished or just being processed by someone else.
			status, _, _, getErr := w.ledger.GetStatus(workerCtx, jobKey)
			if getErr == nil && status == "completed" {
				w.logger.Info("skipping already completed cluster job",
					"cluster_id", job.ClusterID, "type", job.Type, "msg_id", msg.ID)
				w.ackWithLog(workerCtx, msg.ID, "cluster already completed")
				return
			}
			w.logger.Info("cluster job is currently being processed by another worker",
				"cluster_id", job.ClusterID, "type", job.Type, "msg_id", msg.ID)
			return // Leave unacked for redelivery/wait.
		}
	}

	ctx := appcontext.WithUserID(appcontext.WithTenantID(workerCtx, job.TenantID), job.UserID)

	cluster, err := w.repo.GetByID(ctx, job.ClusterID)
	if err != nil {
		w.logger.Error("failed to fetch cluster for job",
			"cluster_id", job.ClusterID, "msg_id", msg.ID, "error", err)
		if w.ledger != nil {
			if ledgerErr := w.ledger.MarkFailed(workerCtx, jobKey, err.Error()); ledgerErr != nil {
				w.logger.Warn("failed to mark cluster job failed in ledger",
					"cluster_id", job.ClusterID, "msg_id", msg.ID, "error", ledgerErr)
			}
		}
		w.nackWithLog(workerCtx, msg.ID, "cluster fetch failed")
		return
	}
	if cluster == nil {
		w.logger.Error("cluster not found for job",
			"cluster_id", job.ClusterID, "msg_id", msg.ID)
		// Ack — cluster was deleted, nothing to do.
		if w.ledger != nil {
			if ledgerErr := w.ledger.MarkComplete(workerCtx, jobKey, "cluster_not_found"); ledgerErr != nil {
				w.logger.Warn("failed to mark cluster job complete in ledger",
					"cluster_id", job.ClusterID, "msg_id", msg.ID, "error", ledgerErr)
			}
		}
		w.ackWithLog(workerCtx, msg.ID, "cluster not found")
		return
	}

	var processErr error
	switch job.Type {
	case domain.ClusterJobProvision:
		processErr = w.handleProvision(ctx, cluster)
	case domain.ClusterJobDeprovision:
		processErr = w.handleDeprovision(ctx, cluster)
	case domain.ClusterJobUpgrade:
		processErr = w.handleUpgrade(ctx, cluster, job.Version)
	default:
		processErr = fmt.Errorf("unsupported cluster job type %q for cluster %s", job.Type, job.ClusterID)
	}

	if processErr != nil {
		w.logger.Error("cluster job failed",
			"cluster_id", job.ClusterID, "type", job.Type,
			"msg_id", msg.ID, "error", processErr)
		if w.ledger != nil {
			if ledgerErr := w.ledger.MarkFailed(workerCtx, jobKey, processErr.Error()); ledgerErr != nil {
				w.logger.Warn("failed to mark cluster job failed in ledger",
					"cluster_id", job.ClusterID, "msg_id", msg.ID, "error", ledgerErr)
			}
		}
		w.nackWithLog(workerCtx, msg.ID, "cluster job processing failed")
		return
	}

	if w.ledger != nil {
		if ledgerErr := w.ledger.MarkComplete(workerCtx, jobKey, "ok"); ledgerErr != nil {
			w.logger.Warn("failed to mark cluster job complete in ledger",
				"cluster_id", job.ClusterID, "msg_id", msg.ID, "error", ledgerErr)
		}
	}
	w.ackWithLog(workerCtx, msg.ID, "cluster job success")
}

func (w *ClusterWorker) handleProvision(ctx context.Context, cluster *domain.Cluster) error {
	cluster.Status = domain.ClusterStatusProvisioning
	cluster.UpdatedAt = time.Now()
	if err := w.repo.Update(ctx, cluster); err != nil {
		w.logger.Warn("failed to persist cluster status", "cluster_id", cluster.ID, "status", cluster.Status, "error", err)
	}

	if err := w.provisioner.Provision(ctx, cluster); err != nil {
		w.logger.Error("provisioning failed", "cluster_id", cluster.ID, "error", err)
		cluster.Status = domain.ClusterStatusFailed
		cluster.UpdatedAt = time.Now()
		cluster.JobID = nil
		if updateErr := w.repo.Update(ctx, cluster); updateErr != nil {
			w.logger.Warn("failed to persist cluster status", "cluster_id", cluster.ID, "status", cluster.Status, "error", updateErr)
		}
		return err
	}

	w.logger.Info("provisioning succeeded", "cluster_id", cluster.ID)
	cluster.Status = domain.ClusterStatusRunning
	cluster.UpdatedAt = time.Now()
	cluster.JobID = nil
	if err := w.repo.Update(ctx, cluster); err != nil {
		w.logger.Warn("failed to persist cluster status", "cluster_id", cluster.ID, "status", cluster.Status, "error", err)
	}
	return nil
}

func (w *ClusterWorker) handleDeprovision(ctx context.Context, cluster *domain.Cluster) error {
	cluster.Status = domain.ClusterStatusDeleting
	cluster.UpdatedAt = time.Now()
	if err := w.repo.Update(ctx, cluster); err != nil {
		w.logger.Warn("failed to persist cluster status", "cluster_id", cluster.ID, "status", cluster.Status, "error", err)
	}

	if err := w.provisioner.Deprovision(ctx, cluster); err != nil {
		w.logger.Error("deprovisioning failed", "cluster_id", cluster.ID, "error", err)
		cluster.UpdatedAt = time.Now()
		cluster.JobID = nil
		if updateErr := w.repo.Update(ctx, cluster); updateErr != nil {
			w.logger.Warn("failed to persist cluster status", "cluster_id", cluster.ID, "status", cluster.Status, "error", updateErr)
		}
		return err
	}

	w.logger.Info("deprovisioning succeeded", "cluster_id", cluster.ID)
	if err := w.repo.Delete(ctx, cluster.ID); err != nil {
		w.logger.Warn("failed to delete cluster after deprovision", "cluster_id", cluster.ID, "error", err)
	}
	return nil
}

func (w *ClusterWorker) handleUpgrade(ctx context.Context, cluster *domain.Cluster, version string) error {
	cluster.Status = domain.ClusterStatusUpgrading
	cluster.UpdatedAt = time.Now()
	if err := w.repo.Update(ctx, cluster); err != nil {
		w.logger.Warn("failed to persist cluster status", "cluster_id", cluster.ID, "status", cluster.Status, "error", err)
	}

	if err := w.provisioner.Upgrade(ctx, cluster, version); err != nil {
		w.logger.Error("upgrade failed", "cluster_id", cluster.ID, "error", err)
		cluster.Status = domain.ClusterStatusRunning
		cluster.UpdatedAt = time.Now()
		cluster.JobID = nil
		if updateErr := w.repo.Update(ctx, cluster); updateErr != nil {
			w.logger.Warn("failed to persist cluster status", "cluster_id", cluster.ID, "status", cluster.Status, "error", updateErr)
		}
		return err
	}

	w.logger.Info("upgrade succeeded", "cluster_id", cluster.ID)
	cluster.Status = domain.ClusterStatusRunning
	cluster.Version = version
	cluster.UpdatedAt = time.Now()
	cluster.JobID = nil
	if err := w.repo.Update(ctx, cluster); err != nil {
		w.logger.Warn("failed to persist cluster status", "cluster_id", cluster.ID, "status", cluster.Status, "error", err)
	}
	return nil
}

func (w *ClusterWorker) reclaimLoop(ctx context.Context, sem chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msgs, err := w.taskQueue.ReclaimStale(ctx, clusterQueue, clusterGroup, w.consumerName, clusterReclaimMs, clusterReclaimN)
			if err != nil {
				w.logger.Warn("cluster reclaim error", "error", err)
				continue
			}
			for _, m := range msgs {
				var job domain.ClusterJob
				if err := json.Unmarshal([]byte(m.Payload), &job); err != nil {
					w.logger.Error("failed to unmarshal reclaimed cluster job",
						"msg_id", m.ID, "error", err)
					w.ackWithLog(ctx, m.ID, "reclaimed cluster poison message")
					continue
				}
				w.logger.Info("reclaimed stale cluster job",
					"cluster_id", job.ClusterID, "msg_id", m.ID)

				m := m
				sem <- struct{}{}
				go func() {
					defer func() { <-sem }()
					w.processJob(ctx, &m, job)
				}()
			}
		}
	}
}

func (w *ClusterWorker) ackWithLog(ctx context.Context, messageID string, reason string) {
	if err := w.taskQueue.Ack(ctx, clusterQueue, clusterGroup, messageID); err != nil {
		w.logger.Warn("failed to ack cluster job",
			"msg_id", messageID, "reason", reason, "error", err)
	}
}

func (w *ClusterWorker) nackWithLog(ctx context.Context, messageID string, reason string) {
	if err := w.taskQueue.Nack(ctx, clusterQueue, clusterGroup, messageID); err != nil {
		w.logger.Warn("failed to nack cluster job",
			"msg_id", messageID, "reason", reason, "error", err)
	}
}
