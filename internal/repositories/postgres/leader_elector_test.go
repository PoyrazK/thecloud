package postgres

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
)

func TestKeyToLockIDDeterministic(t *testing.T) {
	key := "singleton:lb"
	id1 := keyToLockID(key)
	id2 := keyToLockID(key)
	if id1 != id2 {
		t.Fatalf("expected same lock ID for same key, got %d and %d", id1, id2)
	}
}

func TestKeyToLockIDUnique(t *testing.T) {
	keys := []string{
		"singleton:lb",
		"singleton:cron",
		"singleton:autoscaling",
		"singleton:container",
		"singleton:healing",
		"singleton:db-failover",
		"singleton:cluster-reconciler",
		"singleton:replica-monitor",
		"singleton:lifecycle",
		"singleton:log",
		"singleton:accounting",
	}

	seen := make(map[int64]string)
	for _, k := range keys {
		id := keyToLockID(k)
		if id <= 0 {
			t.Fatalf("expected positive lock ID for key %q, got %d", k, id)
		}
		if existing, ok := seen[id]; ok {
			t.Fatalf("lock ID collision: key %q and %q both map to %d", k, existing, id)
		}
		seen[id] = k
	}
}

func TestKeyToLockIDPositive(t *testing.T) {
	testKeys := []string{"a", "b", "test", "singleton:anything", ""}
	for _, k := range testKeys {
		id := keyToLockID(k)
		if id < 0 {
			t.Fatalf("expected non-negative lock ID for key %q, got %d", k, id)
		}
	}
}

func TestAsInt64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		val  uint64
	}{
		{"zero", 0},
		{"one", 1},
		{"max int64", 0x7FFFFFFFFFFFFFFF},
		{"random positive", 0x1234567890ABCDEF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asInt64(tt.val)
			assert.GreaterOrEqual(t, got, int64(0), "asInt64 should return non-negative int64")
			assert.Equal(t, int64(tt.val), got, "asInt64(%d)", tt.val)
		})
	}
}

func TestNewPgLeaderElector(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	logger := slog.Default()
	elector := NewPgLeaderElector(mock, logger)
	assert.NotNil(t, elector)
	assert.Nil(t, elector.conn)
}

func TestPgLeaderElector_Acquire_Success(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	logger := slog.Default()
	elector := NewPgLeaderElector(mock, logger)

	mock.ExpectQuery("SELECT pg_try_advisory_lock").
		WithArgs(keyToLockID("test/key")).
		WillReturnRows(pgxmock.NewRows([]string{"acquired"}).AddRow(true))

	acquired, err := elector.Acquire(context.Background(), "test/key")
	require.NoError(t, err)
	assert.True(t, acquired)

	elector.mu.Lock()
	assert.True(t, elector.held["test/key"])
	elector.mu.Unlock()
}

func TestPgLeaderElector_Acquire_AlreadyHeld(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	logger := slog.Default()
	elector := NewPgLeaderElector(mock, logger)

	mock.ExpectQuery("SELECT pg_try_advisory_lock").
		WithArgs(keyToLockID("test/key")).
		WillReturnRows(pgxmock.NewRows([]string{"acquired"}).AddRow(false))

	acquired, err := elector.Acquire(context.Background(), "test/key")
	require.NoError(t, err)
	assert.False(t, acquired)
}

func TestPgLeaderElector_Acquire_QueryError(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	logger := slog.Default()
	elector := NewPgLeaderElector(mock, logger)

	mock.ExpectQuery("SELECT pg_try_advisory_lock").
		WithArgs(keyToLockID("test/key")).
		WillReturnError(assert.AnError)

	acquired, err := elector.Acquire(context.Background(), "test/key")
	require.Error(t, err)
	assert.False(t, acquired)
	assert.Contains(t, err.Error(), "leader election acquire failed")
}

func TestPgLeaderElector_Release_Success(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	logger := slog.Default()
	elector := NewPgLeaderElector(mock, logger)

	mock.ExpectExec("SELECT pg_advisory_unlock").
		WithArgs(keyToLockID("test/key")).
		WillReturnResult(pgxmock.NewResult("SELECT", 0))

	err = elector.Release(context.Background(), "test/key")
	require.NoError(t, err)

	elector.mu.Lock()
	_, held := elector.held["test/key"]
	assert.False(t, held, "key should be removed from held map")
	elector.mu.Unlock()
}

func TestPgLeaderElector_Release_NotHeld(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	logger := slog.Default()
	elector := NewPgLeaderElector(mock, logger)

	mock.ExpectExec("SELECT pg_advisory_unlock").
		WithArgs(keyToLockID("unknown/key")).
		WillReturnError(assert.AnError)

	err = elector.Release(context.Background(), "unknown/key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "leader election release failed")
}

func TestPgLeaderElector_Release_WithPoolConnection(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	logger := slog.Default()
	elector := &PgLeaderElector{
		db:     mock,
		pool:   nil,
		logger: logger,
		held:   make(map[string]bool),
	}
	elector.held["test/key"] = true

	mock.ExpectExec("SELECT pg_advisory_unlock").
		WithArgs(keyToLockID("test/key")).
		WillReturnResult(pgxmock.NewResult("SELECT", 0))

	err = elector.Release(context.Background(), "test/key")
	require.NoError(t, err)
}
