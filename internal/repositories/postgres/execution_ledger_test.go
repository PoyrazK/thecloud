package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionLedger_TryAcquire_NewJob(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	mock.ExpectQuery("INSERT INTO job_executions").
		WithArgs("job/test").
		WillReturnRows(pgxmock.NewRows([]string{"inserted"}).AddRow(true))

	acquired, err := ledger.TryAcquire(context.Background(), "job/test", 30*time.Second)
	require.NoError(t, err)
	assert.True(t, acquired)
}

func TestExecutionLedger_TryAcquire_AlreadyCompleted(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	// INSERT returns no rows (ON CONFLICT DO NOTHING)
	mock.ExpectQuery("INSERT INTO job_executions").
		WithArgs("job/test").
		WillReturnError(pgx.ErrNoRows)

	// Query returns completed status
	now := time.Now()
	mock.ExpectQuery("SELECT status, started_at FROM job_executions WHERE job_key").
		WithArgs("job/test").
		WillReturnRows(pgxmock.NewRows([]string{"status", "started_at"}).
			AddRow("completed", now))

	acquired, err := ledger.TryAcquire(context.Background(), "job/test", 30*time.Second)
	require.NoError(t, err)
	assert.False(t, acquired)
}

func TestExecutionLedger_TryAcquire_InsertError(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	mock.ExpectQuery("INSERT INTO job_executions").
		WithArgs("job/test").
		WillReturnError(assert.AnError)

	acquired, err := ledger.TryAcquire(context.Background(), "job/test", 30*time.Second)
	require.Error(t, err)
	assert.False(t, acquired)
	assert.Contains(t, err.Error(), "execution ledger insert")
}

func TestExecutionLedger_TryAcquire_CheckQueryError(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	// INSERT returns no rows
	mock.ExpectQuery("INSERT INTO job_executions").
		WithArgs("job/test").
		WillReturnError(pgx.ErrNoRows)

	// Query returns error
	mock.ExpectQuery("SELECT status, started_at FROM job_executions WHERE job_key").
		WithArgs("job/test").
		WillReturnError(assert.AnError)

	acquired, err := ledger.TryAcquire(context.Background(), "job/test", 30*time.Second)
	require.Error(t, err)
	assert.False(t, acquired)
	assert.Contains(t, err.Error(), "execution ledger check")
}

func TestExecutionLedger_TryAcquire_UnknownStatus(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	mock.ExpectQuery("INSERT INTO job_executions").
		WithArgs("job/test").
		WillReturnError(pgx.ErrNoRows)

	now := time.Now()
	mock.ExpectQuery("SELECT status, started_at FROM job_executions WHERE job_key").
		WithArgs("job/test").
		WillReturnRows(pgxmock.NewRows([]string{"status", "started_at"}).
			AddRow("unknown_status", now))

	_, err = ledger.TryAcquire(context.Background(), "job/test", 30*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown status")
}

func TestExecutionLedger_MarkComplete_Success(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	mock.ExpectExec("UPDATE job_executions SET status = 'completed'").
		WithArgs("job/test", "success-result").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = ledger.MarkComplete(context.Background(), "job/test", "success-result")
	require.NoError(t, err)
}

func TestExecutionLedger_MarkComplete_NoRunningRow(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	mock.ExpectExec("UPDATE job_executions SET status = 'completed'").
		WithArgs("job/test", "result").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err = ledger.MarkComplete(context.Background(), "job/test", "result")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no running row updated")
}

func TestExecutionLedger_MarkFailed_Success(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	mock.ExpectExec("UPDATE job_executions SET status = 'failed'").
		WithArgs("job/test", "failed-reason").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = ledger.MarkFailed(context.Background(), "job/test", "failed-reason")
	require.NoError(t, err)
}

func TestExecutionLedger_MarkFailed_NoRunningRow(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	mock.ExpectExec("UPDATE job_executions SET status = 'failed'").
		WithArgs("job/test", "reason").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err = ledger.MarkFailed(context.Background(), "job/test", "reason")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no running row updated")
}

func TestExecutionLedger_GetStatus_Success(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	now := time.Now()
	mock.ExpectQuery("SELECT status, COALESCE").
		WithArgs("job/test").
		WillReturnRows(pgxmock.NewRows([]string{"status", "result", "started_at"}).
			AddRow("completed", "done", now))

	status, result, startedAt, err := ledger.GetStatus(context.Background(), "job/test")
	require.NoError(t, err)
	assert.Equal(t, "completed", status)
	assert.Equal(t, "done", result)
	assert.Equal(t, now, startedAt)
}

func TestExecutionLedger_GetStatus_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	mock.ExpectQuery("SELECT status, COALESCE").
		WithArgs("job/unknown").
		WillReturnError(pgx.ErrNoRows)

	status, result, startedAt, err := ledger.GetStatus(context.Background(), "job/unknown")
	require.NoError(t, err)
	assert.Equal(t, "", status)
	assert.Equal(t, "", result)
	assert.True(t, startedAt.IsZero())
}

func TestExecutionLedger_GetStatus_QueryError(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	ledger := NewExecutionLedger(mock)

	mock.ExpectQuery("SELECT status, COALESCE").
		WithArgs("job/test").
		WillReturnError(assert.AnError)

	_, _, _, err = ledger.GetStatus(context.Background(), "job/test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "execution ledger get status")
}
