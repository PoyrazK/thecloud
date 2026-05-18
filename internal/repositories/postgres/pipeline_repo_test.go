package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pipelineTestPipeline() *domain.Pipeline {
	return &domain.Pipeline{
		ID:            uuid.New(),
		UserID:        uuid.New(),
		TenantID:      uuid.New(),
		Name:          "test-pipeline",
		RepositoryURL: "https://github.com/test/repo",
		Branch:        "main",
		WebhookSecret: "secret123",
		Config: domain.PipelineConfig{
			Environment: map[string]string{"KEY": "value"},
		},
		Status:    domain.PipelineStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func pipelineTestBuild() *domain.Build {
	started := time.Now()
	finished := started.Add(2 * time.Minute)
	return &domain.Build{
		ID:          uuid.New(),
		PipelineID:  uuid.New(),
		UserID:      uuid.New(),
		TenantID:    uuid.New(),
		CommitHash:  "abc123def456",
		TriggerType: domain.BuildTriggerManual,
		Status:      domain.BuildStatusQueued,
		StartedAt:   &started,
		FinishedAt:  &finished,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func pipelineTestBuildStep() *domain.BuildStep {
	started := time.Now()
	finished := started.Add(1 * time.Minute)
	exitCode := 0
	return &domain.BuildStep{
		ID:         uuid.New(),
		BuildID:    uuid.New(),
		Name:       "build-step",
		Image:      "golang:1.21",
		Commands:   []string{"go build ./..."},
		Status:     domain.BuildStatusQueued,
		ExitCode:   &exitCode,
		StartedAt:  &started,
		FinishedAt: &finished,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func pipelineTestBuildLog() *domain.BuildLog {
	return &domain.BuildLog{
		ID:        uuid.New(),
		BuildID:   uuid.New(),
		StepID:    uuid.New(),
		Content:   "Build log output here",
		CreatedAt: time.Now(),
	}
}

// Pipeline CRUD tests

func TestPipelineRepository_CreatePipeline(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	p := pipelineTestPipeline()

	mock.ExpectExec("INSERT INTO pipelines").
		WithArgs(p.ID, p.UserID, p.TenantID, p.Name, p.RepositoryURL, p.Branch, p.WebhookSecret, pgxmock.AnyArg(), p.Status, p.CreatedAt, p.UpdatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.CreatePipeline(context.Background(), p)
	require.NoError(t, err)
}

func TestPipelineRepository_GetPipelineByID(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	p := pipelineTestPipeline()
	configJSON := []byte(`{}`)

	mock.ExpectQuery("SELECT id, user_id, tenant_id, name, repository_url, branch, webhook_secret, config, status, created_at, updated_at FROM pipelines WHERE id").
		WithArgs(p.ID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "repository_url", "branch", "webhook_secret", "config", "status", "created_at", "updated_at"}).
			AddRow(p.ID, p.UserID, p.TenantID, p.Name, p.RepositoryURL, p.Branch, p.WebhookSecret, configJSON, string(p.Status), p.CreatedAt, p.UpdatedAt))

	result, err := repo.GetPipelineByID(context.Background(), p.ID)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, p.Name, result.Name)
}

func TestPipelineRepository_GetPipelineByID_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	id := uuid.New()

	mock.ExpectQuery("SELECT id, user_id, tenant_id, name, repository_url, branch, webhook_secret, config, status, created_at, updated_at FROM pipelines WHERE id").
		WithArgs(id).
		WillReturnError(pgx.ErrNoRows)

	result, err := repo.GetPipelineByID(context.Background(), id)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestPipelineRepository_GetPipeline(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	p := pipelineTestPipeline()
	configJSON := []byte(`{}`)

	mock.ExpectQuery("SELECT id, user_id, tenant_id, name, repository_url, branch, webhook_secret, config, status, created_at, updated_at FROM pipelines WHERE id = .+AND tenant_id = .+").
		WithArgs(p.ID, p.TenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "repository_url", "branch", "webhook_secret", "config", "status", "created_at", "updated_at"}).
			AddRow(p.ID, p.UserID, p.TenantID, p.Name, p.RepositoryURL, p.Branch, p.WebhookSecret, configJSON, string(p.Status), p.CreatedAt, p.UpdatedAt))

	result, err := repo.GetPipeline(context.Background(), p.ID, p.TenantID)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPipelineRepository_ListPipelines(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	p := pipelineTestPipeline()
	configJSON := []byte(`{}`)

	mock.ExpectQuery("SELECT id, user_id, tenant_id, name, repository_url, branch, webhook_secret, config, status, created_at, updated_at FROM pipelines WHERE tenant_id = .+ORDER BY created_at DESC").
		WithArgs(p.TenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "repository_url", "branch", "webhook_secret", "config", "status", "created_at", "updated_at"}).
			AddRow(p.ID, p.UserID, p.TenantID, p.Name, p.RepositoryURL, p.Branch, p.WebhookSecret, configJSON, string(p.Status), p.CreatedAt, p.UpdatedAt))

	result, err := repo.ListPipelines(context.Background(), p.TenantID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestPipelineRepository_UpdatePipeline(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	p := pipelineTestPipeline()

	mock.ExpectExec("UPDATE pipelines SET name = .+, repository_url = .+, branch = .+, webhook_secret = .+, config = .+, status = .+, updated_at = NOW\\(\\) WHERE id = .+AND tenant_id = .+").
		WithArgs(p.Name, p.RepositoryURL, p.Branch, p.WebhookSecret, pgxmock.AnyArg(), p.Status, p.ID, p.TenantID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.UpdatePipeline(context.Background(), p)
	require.NoError(t, err)
}

func TestPipelineRepository_DeletePipeline(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()

	mock.ExpectExec("DELETE FROM pipelines WHERE id = .+AND tenant_id = .+").
		WithArgs(id, tenantID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.DeletePipeline(context.Background(), id, tenantID)
	require.NoError(t, err)
}

// Build tests

func TestPipelineRepository_CreateBuild(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	b := pipelineTestBuild()

	mock.ExpectExec("INSERT INTO builds").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.CreateBuild(context.Background(), b)
	require.NoError(t, err)
}

func TestPipelineRepository_GetBuild(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	b := pipelineTestBuild()

	mock.ExpectQuery("SELECT id, pipeline_id, user_id, tenant_id, commit_hash, trigger_type, status, started_at, finished_at, created_at, updated_at FROM builds WHERE id = .+AND tenant_id = .+").
		WithArgs(b.ID, b.TenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "pipeline_id", "user_id", "tenant_id", "commit_hash", "trigger_type", "status", "started_at", "finished_at", "created_at", "updated_at"}).
			AddRow(b.ID, b.PipelineID, b.UserID, b.TenantID, b.CommitHash, string(b.TriggerType), string(b.Status), b.StartedAt, b.FinishedAt, b.CreatedAt, b.UpdatedAt))

	result, err := repo.GetBuild(context.Background(), b.ID, b.TenantID)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, b.CommitHash, result.CommitHash)
}

func TestPipelineRepository_GetBuild_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()

	mock.ExpectQuery("SELECT id, pipeline_id, user_id, tenant_id, commit_hash, trigger_type, status, started_at, finished_at, created_at, updated_at FROM builds WHERE id = .+AND tenant_id = .+").
		WithArgs(id, tenantID).
		WillReturnError(pgx.ErrNoRows)

	result, err := repo.GetBuild(context.Background(), id, tenantID)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestPipelineRepository_ListBuildsByPipeline(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	b := pipelineTestBuild()

	mock.ExpectQuery("SELECT id, pipeline_id, user_id, tenant_id, commit_hash, trigger_type, status, started_at, finished_at, created_at, updated_at FROM builds WHERE pipeline_id = .+AND tenant_id = .+ORDER BY created_at DESC").
		WithArgs(b.PipelineID, b.TenantID).
		WillReturnRows(pgxmock.NewRows([]string{"id", "pipeline_id", "user_id", "tenant_id", "commit_hash", "trigger_type", "status", "started_at", "finished_at", "created_at", "updated_at"}).
			AddRow(b.ID, b.PipelineID, b.UserID, b.TenantID, b.CommitHash, string(b.TriggerType), string(b.Status), b.StartedAt, b.FinishedAt, b.CreatedAt, b.UpdatedAt))

	result, err := repo.ListBuildsByPipeline(context.Background(), b.PipelineID, b.TenantID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestPipelineRepository_UpdateBuild(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	b := pipelineTestBuild()

	mock.ExpectExec("UPDATE builds SET status = .+, started_at = .+, finished_at = .+, updated_at = NOW\\(\\) WHERE id = .+AND user_id = .+").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.UpdateBuild(context.Background(), b)
	require.NoError(t, err)
}

// BuildStep tests

func TestPipelineRepository_CreateBuildStep(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	s := pipelineTestBuildStep()

	mock.ExpectExec("INSERT INTO build_steps").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.CreateBuildStep(context.Background(), s)
	require.NoError(t, err)
}

func TestPipelineRepository_ListBuildSteps(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	s := pipelineTestBuildStep()

	mock.ExpectQuery("SELECT s.id, s.build_id, s.name, s.image, s.commands, s.status, s.exit_code, s.started_at, s.finished_at, s.created_at, s.updated_at FROM build_steps s INNER JOIN builds b ON b.id = s.build_id WHERE s.build_id = .+AND b.user_id = .+ORDER BY s.created_at ASC").
		WithArgs(s.BuildID, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "build_id", "name", "image", "commands", "status", "exit_code", "started_at", "finished_at", "created_at", "updated_at"}).
			AddRow(s.ID, s.BuildID, s.Name, s.Image, nil, string(s.Status), nil, nil, nil, s.CreatedAt, s.UpdatedAt))

	result, err := repo.ListBuildSteps(context.Background(), s.BuildID, uuid.New())
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestPipelineRepository_UpdateBuildStep(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	s := pipelineTestBuildStep()

	mock.ExpectExec("UPDATE build_steps SET status = .+, exit_code = .+, started_at = .+, finished_at = .+, updated_at = NOW\\(\\) WHERE id = .+").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.UpdateBuildStep(context.Background(), s)
	require.NoError(t, err)
}

// BuildLog tests

func TestPipelineRepository_AppendBuildLog(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	log := pipelineTestBuildLog()

	mock.ExpectExec("INSERT INTO build_logs").
		WithArgs(log.ID, log.BuildID, log.StepID, log.Content, log.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.AppendBuildLog(context.Background(), log)
	require.NoError(t, err)
}

func TestPipelineRepository_ListBuildLogs(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	log := pipelineTestBuildLog()

	mock.ExpectQuery("SELECT l.id, l.build_id, l.step_id, l.content, l.created_at FROM build_logs l INNER JOIN builds b ON b.id = l.build_id WHERE l.build_id = .+AND b.user_id = .+ORDER BY l.created_at ASC LIMIT .+").
		WithArgs(log.BuildID, pgxmock.AnyArg(), 200).
		WillReturnRows(pgxmock.NewRows([]string{"id", "build_id", "step_id", "content", "created_at"}).
			AddRow(log.ID, log.BuildID, log.StepID, log.Content, log.CreatedAt))

	result, err := repo.ListBuildLogs(context.Background(), log.BuildID, uuid.New(), 0)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestPipelineRepository_ListBuildLogs_DefaultLimit(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	log := pipelineTestBuildLog()

	mock.ExpectQuery("SELECT l.id, l.build_id, l.step_id, l.content, l.created_at FROM build_logs l INNER JOIN builds b ON b.id = l.build_id WHERE l.build_id = .+AND b.user_id = .+ORDER BY l.created_at ASC LIMIT \\$3").
		WithArgs(log.BuildID, pgxmock.AnyArg(), 200).
		WillReturnRows(pgxmock.NewRows([]string{"id", "build_id", "step_id", "content", "created_at"}).
			AddRow(log.ID, log.BuildID, log.StepID, log.Content, log.CreatedAt))

	result, err := repo.ListBuildLogs(context.Background(), log.BuildID, uuid.New(), -1)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// Webhook delivery tests

func TestPipelineRepository_ReserveWebhookDelivery_Success(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	pipelineID := uuid.New()

	mock.ExpectExec("INSERT INTO pipeline_webhook_deliveries").
		WithArgs(pgxmock.AnyArg(), pipelineID, "github", "push", "delivery-123").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	reserved, err := repo.ReserveWebhookDelivery(context.Background(), pipelineID, "github", "push", "delivery-123")
	require.NoError(t, err)
	assert.True(t, reserved)
}

func TestPipelineRepository_ReserveWebhookDelivery_Duplicate(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	pipelineID := uuid.New()

	pgErr := &pgconn.PgError{Code: "23505"}
	mock.ExpectExec("INSERT INTO pipeline_webhook_deliveries").
		WithArgs(pgxmock.AnyArg(), pipelineID, "github", "push", "delivery-123").
		WillReturnError(pgErr)

	reserved, err := repo.ReserveWebhookDelivery(context.Background(), pipelineID, "github", "push", "delivery-123")
	require.NoError(t, err)
	assert.False(t, reserved)
}

func TestPipelineRepository_ReserveWebhookDelivery_OtherError(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)
	pipelineID := uuid.New()

	mock.ExpectExec("INSERT INTO pipeline_webhook_deliveries").
		WithArgs(pgxmock.AnyArg(), pipelineID, "github", "push", "delivery-123").
		WillReturnError(assert.AnError)

	_, err = repo.ReserveWebhookDelivery(context.Background(), pipelineID, "github", "push", "delivery-123")
	require.Error(t, err)
}

func TestPipelineRepository_GetBuild_ScanError(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewPipelineRepository(mock)

	mock.ExpectQuery("SELECT id, pipeline_id, user_id, tenant_id, commit_hash, trigger_type, status, started_at, finished_at, created_at, updated_at FROM builds WHERE id = .+AND tenant_id = .+").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(assert.AnError)

	result, err := repo.GetBuild(context.Background(), uuid.New(), uuid.New())
	require.Error(t, err)
	assert.Nil(t, result)
	assert.NotErrorIs(t, err, pgx.ErrNoRows)
}
