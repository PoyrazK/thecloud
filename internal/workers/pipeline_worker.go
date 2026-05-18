// Package workers provides background worker implementations.
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

const (
	pipelineQueueName  = "pipeline_build_queue"
	pipelineGroup      = "pipeline_workers"
	pipelineMaxWorkers = 5
	// Must be shorter than pipelineStaleThreshold (35m) to reclaim promptly,
	// but longer than max expected job runtime (30m) to avoid stealing messages
	// from workers that are legitimately still processing.
	pipelineReclaimMs = 32 * 60 * 1000 // 32 minutes
	pipelineReclaimN  = 5
	// Stale threshold for idempotency ledger: builds can take up to 30 min,
	// so a "running" entry older than this is considered abandoned.
	pipelineStaleThreshold = 35 * time.Minute
	maxPayloadSize         = 10 * 1024 * 1024 // 10 MB max pipeline job payload
)

// PipelineWorker processes queued pipeline builds.
type PipelineWorker struct {
	repo         ports.PipelineRepository
	taskQueue    ports.DurableTaskQueue
	ledger       ports.ExecutionLedger
	compute      ports.ComputeBackend
	logger       *slog.Logger
	consumerName string
}

// NewPipelineWorker creates a new PipelineWorker.
// If ledger is nil, idempotency checks are skipped.
func NewPipelineWorker(repo ports.PipelineRepository, taskQueue ports.DurableTaskQueue, ledger ports.ExecutionLedger, compute ports.ComputeBackend, logger *slog.Logger) *PipelineWorker {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "pipeline-worker"
	}
	return &PipelineWorker{
		repo:         repo,
		taskQueue:    taskQueue,
		ledger:       ledger,
		compute:      compute,
		logger:       logger,
		consumerName: hostname,
	}
}

func (w *PipelineWorker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	w.logger.Info("starting pipeline worker",
		"consumer", w.consumerName,
		"concurrency", pipelineMaxWorkers,
	)

	if err := w.taskQueue.EnsureGroup(ctx, pipelineQueueName, pipelineGroup); err != nil {
		w.logger.Error("failed to ensure pipeline consumer group", "error", err)
		return
	}

	sem := make(chan struct{}, pipelineMaxWorkers)

	go w.reclaimLoop(ctx, sem)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("stopping pipeline worker")
			return
		default:
			msg, err := w.taskQueue.Receive(ctx, pipelineQueueName, pipelineGroup, w.consumerName)
			if err != nil {
				w.logger.Error("failed to receive pipeline job", "error", err)
				time.Sleep(1 * time.Second)
				continue
			}
			if msg == nil {
				continue
			}

			var job domain.BuildJob
			if len(msg.Payload) > maxPayloadSize {
				w.logger.Error("pipeline job payload too large", "msg_id", msg.ID, "size", len(msg.Payload))
				w.ackWithLog(ctx, msg.ID, "pipeline payload too large")
				continue
			}
			if err := json.Unmarshal([]byte(msg.Payload), &job); err != nil {
				w.logger.Error("failed to unmarshal build job",
					"error", err, "msg_id", msg.ID)
				w.ackWithLog(ctx, msg.ID, "pipeline poison message")
				continue
			}

			sem <- struct{}{}
			go func(m *ports.DurableMessage, j domain.BuildJob) {
				defer func() { <-sem }()
				w.processJob(ctx, m, j)
			}(msg, job)
		}
	}
}

func (w *PipelineWorker) processJob(workerCtx context.Context, msg *ports.DurableMessage, job domain.BuildJob) {
	jobKey := fmt.Sprintf("pipeline:%s", job.BuildID)

	// Idempotency check: skip if already completed or actively being processed.
	if w.ledger != nil {
		acquired, err := w.ledger.TryAcquire(workerCtx, jobKey, pipelineStaleThreshold)
		if err != nil {
			w.logger.Error("execution ledger error",
				"build_id", job.BuildID, "msg_id", msg.ID, "error", err)
			w.nackWithLog(workerCtx, msg.ID, "ledger try_acquire failed")
			return
		}
		if !acquired {
			// Check if it's already finished or just being processed by someone else.
			status, _, _, getErr := w.ledger.GetStatus(workerCtx, jobKey)
			if getErr == nil && status == "completed" {
				w.logger.Info("skipping already completed pipeline job",
					"build_id", job.BuildID, "msg_id", msg.ID)
				w.ackWithLog(workerCtx, msg.ID, "pipeline already completed")
				return
			}
			w.logger.Info("pipeline job is currently being processed by another worker",
				"build_id", job.BuildID, "msg_id", msg.ID)
			return // Leave unacked for redelivery/wait.
		}
	}

	ctx, cancel := context.WithTimeout(workerCtx, 30*time.Minute)
	defer cancel()
	ctx = appcontext.WithUserID(ctx, job.UserID)

	build, pipeline, err := w.loadBuildAndPipeline(ctx, job)
	if err != nil {
		// Transient error loading build/pipeline — nack and retry.
		w.logger.Error("transient error loading build/pipeline",
			"build_id", job.BuildID, "error", err)
		if w.ledger != nil {
			if ledgerErr := w.ledger.MarkFailed(workerCtx, jobKey, "transient load error"); ledgerErr != nil {
				w.logger.Warn("failed to mark pipeline job failed in ledger",
					"build_id", job.BuildID, "msg_id", msg.ID, "error", ledgerErr)
			}
		}
		w.nackWithLog(workerCtx, msg.ID, "transient pipeline load error")
		return
	}

	if build == nil || pipeline == nil {
		// Build or pipeline truly not found — ack to avoid infinite retries.
		if w.ledger != nil {
			if ledgerErr := w.ledger.MarkComplete(workerCtx, jobKey, "not_found"); ledgerErr != nil {
				w.logger.Warn("failed to mark pipeline job complete in ledger",
					"build_id", job.BuildID, "msg_id", msg.ID, "error", ledgerErr)
			}
		}
		w.ackWithLog(workerCtx, msg.ID, "pipeline build/pipeline not found")
		return
	}

	if !w.markBuildRunning(ctx, build) {
		if w.ledger != nil {
			if ledgerErr := w.ledger.MarkFailed(workerCtx, jobKey, "failed to mark build running"); ledgerErr != nil {
				w.logger.Warn("failed to mark pipeline job failed in ledger",
					"build_id", job.BuildID, "msg_id", msg.ID, "error", ledgerErr)
			}
		}
		w.nackWithLog(workerCtx, msg.ID, "mark build running failed")
		return
	}

	if len(pipeline.Config.Stages) == 0 {
		w.failBuild(ctx, build, "pipeline has no stages")
		if w.ledger != nil {
			if ledgerErr := w.ledger.MarkComplete(workerCtx, jobKey, "no_stages"); ledgerErr != nil {
				w.logger.Warn("failed to mark pipeline job complete in ledger",
					"build_id", job.BuildID, "msg_id", msg.ID, "error", ledgerErr)
			}
		}
		w.ackWithLog(workerCtx, msg.ID, "pipeline has no stages")
		return
	}

	if !w.executePipeline(ctx, build, pipeline) {
		// Build failed but was processed — ack the message.
		if w.ledger != nil {
			if ledgerErr := w.ledger.MarkComplete(workerCtx, jobKey, "build_failed"); ledgerErr != nil {
				w.logger.Warn("failed to mark pipeline job complete in ledger",
					"build_id", job.BuildID, "msg_id", msg.ID, "error", ledgerErr)
			}
		}
		w.ackWithLog(workerCtx, msg.ID, "pipeline execution failed")
		return
	}

	w.markBuildSucceeded(ctx, build)

	if w.ledger != nil {
		if ledgerErr := w.ledger.MarkComplete(workerCtx, jobKey, "ok"); ledgerErr != nil {
			w.logger.Warn("failed to mark pipeline job complete in ledger",
				"build_id", job.BuildID, "msg_id", msg.ID, "error", ledgerErr)
		}
	}
	w.ackWithLog(workerCtx, msg.ID, "pipeline job success")
}

func (w *PipelineWorker) loadBuildAndPipeline(ctx context.Context, job domain.BuildJob) (*domain.Build, *domain.Pipeline, error) {
	build, err := w.repo.GetBuild(ctx, job.BuildID, job.UserID)
	if err != nil {
		w.logger.Error("failed to load build", "build_id", job.BuildID, "error", err)
		return nil, nil, err
	}
	if build == nil {
		return nil, nil, nil
	}

	pipeline, err := w.repo.GetPipeline(ctx, job.PipelineID, job.UserID)
	if err != nil {
		w.logger.Error("failed to load pipeline", "pipeline_id", job.PipelineID, "error", err)
		w.failBuild(ctx, build, "pipeline load error: "+err.Error())
		return nil, nil, err
	}
	if pipeline == nil {
		w.failBuild(ctx, build, "pipeline not found")
		return build, nil, nil
	}

	return build, pipeline, nil
}

func (w *PipelineWorker) markBuildRunning(ctx context.Context, build *domain.Build) bool {
	now := time.Now()
	build.Status = domain.BuildStatusRunning
	build.StartedAt = &now
	build.UpdatedAt = now
	if err := w.repo.UpdateBuild(ctx, build); err != nil {
		w.logger.Error("failed to mark build running", "build_id", build.ID, "error", err)
		return false
	}
	return true
}

func (w *PipelineWorker) executePipeline(ctx context.Context, build *domain.Build, pipeline *domain.Pipeline) bool {
	for _, stage := range pipeline.Config.Stages {
		for _, stageStep := range stage.Steps {
			if !w.executeStageStep(ctx, build, stageStep) {
				return false
			}
		}
	}
	return true
}

func (w *PipelineWorker) executeStageStep(ctx context.Context, build *domain.Build, stageStep domain.PipelineStep) bool {
	step := w.newRunningStep(build.ID, stageStep)
	if err := w.repo.CreateBuildStep(ctx, step); err != nil {
		w.failBuild(ctx, build, "failed creating build step: "+err.Error())
		return false
	}

	if stageStep.Image == "" {
		w.failStepAndBuild(ctx, step, build, "step image is required")
		return false
	}

	if len(stageStep.Commands) == 0 {
		w.markStepSucceeded(ctx, step)
		return true
	}

	exitCode, logs, execErr := w.runTaskForStep(ctx, build.ID, stageStep)
	if execErr != nil {
		w.failStepAndBuild(ctx, step, build, execErr.Error())
		return false
	}

	if logs != "" {
		if err := w.repo.AppendBuildLog(ctx, &domain.BuildLog{ID: uuid.New(), BuildID: build.ID, StepID: step.ID, Content: logs, CreatedAt: time.Now()}); err != nil {
			w.logger.Warn("failed to append build log", "build_id", build.ID, "step_id", step.ID, "error", err)
		}
	}

	if exitCode != 0 {
		w.markStepFinished(ctx, step, domain.BuildStatusFailed, int(exitCode))
		w.failBuild(ctx, build, "step failed with non-zero exit code")
		return false
	}

	w.markStepFinished(ctx, step, domain.BuildStatusSucceeded, int(exitCode))
	return true
}

func (w *PipelineWorker) runTaskForStep(ctx context.Context, buildID uuid.UUID, stageStep domain.PipelineStep) (int64, string, error) {
	containerID, _, runErr := w.compute.RunTask(ctx, ports.RunTaskOptions{
		Name:    "pipeline-" + buildID.String(),
		Image:   stageStep.Image,
		Command: []string{"/bin/sh", "-lc", strings.Join(stageStep.Commands, "\n")},
	})
	if runErr != nil {
		return 0, "", runErr
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := w.compute.DeleteInstance(cleanupCtx, containerID); err != nil {
			w.logger.Warn("failed to delete pipeline task container", "container_id", containerID, "build_id", buildID, "error", err)
		}
	}()

	exitCode, waitErr := w.compute.WaitTask(ctx, containerID)
	if waitErr != nil {
		return 0, "", waitErr
	}

	logs, logsErr := w.collectTaskLogs(ctx, containerID)
	if logsErr != nil {
		w.logger.Warn("failed to collect task logs", "container_id", containerID, "error", logsErr)
		return exitCode, "", fmt.Errorf("failed to collect logs: %w", logsErr)
	}

	return exitCode, logs, nil
}

func (w *PipelineWorker) newRunningStep(buildID uuid.UUID, stageStep domain.PipelineStep) *domain.BuildStep {
	now := time.Now()
	return &domain.BuildStep{
		ID:        uuid.New(),
		BuildID:   buildID,
		Name:      stageStep.Name,
		Image:     stageStep.Image,
		Commands:  stageStep.Commands,
		Status:    domain.BuildStatusRunning,
		StartedAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (w *PipelineWorker) markStepSucceeded(ctx context.Context, step *domain.BuildStep) {
	w.markStepFinished(ctx, step, domain.BuildStatusSucceeded, 0)
}

func (w *PipelineWorker) markStepFinished(ctx context.Context, step *domain.BuildStep, status domain.BuildStatus, exitCode int) {
	finished := time.Now()
	step.Status = status
	step.FinishedAt = &finished
	step.UpdatedAt = finished
	step.ExitCode = &exitCode
	if err := w.repo.UpdateBuildStep(ctx, step); err != nil {
		w.logger.Warn("failed to update build step", "step_id", step.ID, "build_id", step.BuildID, "error", err)
	}
}

func (w *PipelineWorker) markBuildSucceeded(ctx context.Context, build *domain.Build) {
	finish := time.Now()
	build.Status = domain.BuildStatusSucceeded
	build.FinishedAt = &finish
	build.UpdatedAt = finish
	if err := w.repo.UpdateBuild(ctx, build); err != nil {
		w.logger.Warn("failed to update build", "build_id", build.ID, "error", err)
	}
}

func (w *PipelineWorker) failStepAndBuild(ctx context.Context, step *domain.BuildStep, build *domain.Build, message string) {
	end := time.Now()
	step.Status = domain.BuildStatusFailed
	step.FinishedAt = &end
	step.UpdatedAt = end
	if err := w.repo.UpdateBuildStep(ctx, step); err != nil {
		w.logger.Warn("failed to update build step", "step_id", step.ID, "build_id", step.BuildID, "error", err)
	}
	if err := w.repo.AppendBuildLog(ctx, &domain.BuildLog{ID: uuid.New(), BuildID: build.ID, StepID: step.ID, Content: message, CreatedAt: end}); err != nil {
		w.logger.Warn("failed to append build log", "build_id", build.ID, "step_id", step.ID, "error", err)
	}
	w.failBuild(ctx, build, message)
}

func (w *PipelineWorker) failBuild(ctx context.Context, build *domain.Build, message string) {
	finish := time.Now()
	build.Status = domain.BuildStatusFailed
	build.FinishedAt = &finish
	build.UpdatedAt = finish
	if err := w.repo.UpdateBuild(ctx, build); err != nil {
		w.logger.Warn("failed to update build", "build_id", build.ID, "error", err)
	}
	if err := w.repo.AppendBuildLog(ctx, &domain.BuildLog{ID: uuid.New(), BuildID: build.ID, Content: message, CreatedAt: finish}); err != nil {
		w.logger.Warn("failed to append build log", "build_id", build.ID, "error", err)
	}
}

func (w *PipelineWorker) collectTaskLogs(ctx context.Context, taskID string) (string, error) {
	reader, err := w.compute.GetInstanceLogs(ctx, taskID)
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (w *PipelineWorker) reclaimLoop(ctx context.Context, sem chan struct{}) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msgs, err := w.taskQueue.ReclaimStale(ctx, pipelineQueueName, pipelineGroup, w.consumerName, pipelineReclaimMs, pipelineReclaimN)
			if err != nil {
				w.logger.Warn("pipeline reclaim error", "error", err)
				continue
			}
			for _, m := range msgs {
				var job domain.BuildJob
				if len(m.Payload) > maxPayloadSize {
					w.logger.Error("reclaimed pipeline job payload too large", "msg_id", m.ID, "size", len(m.Payload))
					w.ackWithLog(ctx, m.ID, "pipeline payload too large")
					continue
				}
				if err := json.Unmarshal([]byte(m.Payload), &job); err != nil {
					w.logger.Error("failed to unmarshal reclaimed pipeline job",
						"msg_id", m.ID, "error", err)
					w.ackWithLog(ctx, m.ID, "reclaimed pipeline poison message")
					continue
				}
				w.logger.Info("reclaimed stale pipeline job",
					"build_id", job.BuildID, "msg_id", m.ID)

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

func (w *PipelineWorker) ackWithLog(ctx context.Context, messageID string, reason string) {
	if err := w.taskQueue.Ack(ctx, pipelineQueueName, pipelineGroup, messageID); err != nil {
		w.logger.Warn("failed to ack pipeline job",
			"msg_id", messageID, "reason", reason, "error", err)
	}
}

func (w *PipelineWorker) nackWithLog(ctx context.Context, messageID string, reason string) {
	if err := w.taskQueue.Nack(ctx, pipelineQueueName, pipelineGroup, messageID); err != nil {
		w.logger.Warn("failed to nack pipeline job",
			"msg_id", messageID, "reason", reason, "error", err)
	}
}
