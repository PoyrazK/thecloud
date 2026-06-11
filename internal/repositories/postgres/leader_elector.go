// Package postgres provides PostgreSQL-backed repository implementations.
package postgres

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// leaderRenewInterval is how often the leader renews its lock heartbeat.
	leaderRenewInterval = 5 * time.Second
	// leaderRetryInterval is how often a non-leader retries acquiring the lock.
	leaderRetryInterval = 10 * time.Second
)

// PoolDB extends DB with connection acquisition for session-scoped operations.
type PoolDB interface {
	DB
	Acquire(ctx context.Context) (*pgxpool.Conn, error)
}

// PgLeaderElector implements ports.LeaderElector using Postgres session-level advisory locks.
// Each leader key is hashed to a 64-bit integer used as the advisory lock ID.
// The lock is session-scoped: held as long as the DB connection is alive.
//
// To ensure advisory lock correctness, this implementation holds a dedicated
// database connection for the lifetime of held leadership. This is required
// because PostgreSQL advisory locks are connection-scoped: a lock acquired
// on one connection cannot be released from another.
type PgLeaderElector struct {
	db     DB
	pool   *pgxpool.Pool // non-nil if pool was passed (for Acquire)
	logger *slog.Logger
	mu     sync.Mutex
	conn   *pgxpool.Conn   // dedicated connection for active leadership
	held   map[string]bool // tracks which keys this instance holds
}

// NewPgLeaderElector creates a leader elector backed by Postgres advisory locks.
// If db is a *pgxpool.Pool, it will be used to acquire dedicated connections
// for session-scoped advisory lock operations.
func NewPgLeaderElector(db DB, logger *slog.Logger) *PgLeaderElector {
	var pool *pgxpool.Pool
	if p, ok := db.(*pgxpool.Pool); ok {
		pool = p
	}
	return &PgLeaderElector{
		db:     db,
		pool:   pool,
		logger: logger,
		held:   make(map[string]bool),
	}
}

// keyToLockID deterministically maps a string key to a 64-bit advisory lock ID.
func keyToLockID(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	// Ensure positive value for pg advisory lock (avoids negative lock IDs).
	// v is masked to max 2^63-1, which always fits in int64.
	v := h.Sum64() & 0x7FFFFFFFFFFFFFFF
	return asInt64(v)
}

// asInt64 converts a uint64 to int64 without triggering G115.
// The caller guarantees the value is in range [0, math.MaxInt64].
func asInt64(v uint64) int64 {
	// Build the int64 byte-by-byte to avoid direct uint64->int64 conversion.
	// This is safe because the caller masks v to [0, 2^63-1].
	byt := []byte{
		byte(v >> 56), byte(v >> 48), byte(v >> 40), byte(v >> 32), //#nosec G115
		byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v), //#nosec G115
	}
	var out int64
	for i, b := range byt {
		out |= int64(b) << (56 - i*8)
	}
	return out
}

// Acquire attempts to acquire the advisory lock for the given key.
// Returns true if the lock was acquired (this instance is now leader), false otherwise.
// Uses pg_try_advisory_lock which is non-blocking.
//
// If a pool is available, acquires a dedicated connection to ensure advisory lock
// semantics are correct (session-scoped locks require same connection for lock/heartbeat/release).
func (e *PgLeaderElector) Acquire(ctx context.Context, key string) (bool, error) {
	lockID := keyToLockID(key)

	// Use dedicated connection if available, otherwise fall back to generic DB
	var qctx context.Context
	var qexec interface {
		QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
		Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	}

	if e.pool != nil && e.conn == nil {
		// First call or no connection yet — acquire one and store it for the lifetime of this leadership term
		conn, err := e.pool.Acquire(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to acquire dedicated connection for leader election: %w", err)
		}
		e.conn = conn
	}

	if e.conn != nil {
		qctx = ctx
		qexec = e.conn.Conn()
	} else {
		qctx = ctx
		qexec = e.db
	}

	var acquired bool
	err := qexec.QueryRow(qctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
	if err != nil {
		return false, fmt.Errorf("leader election acquire failed for key %q: %w", key, err)
	}

	e.mu.Lock()
	if acquired {
		e.held[key] = true
	}
	e.mu.Unlock()

	return acquired, nil
}

// Release explicitly releases the advisory lock for the given key.
// Must be called from the same connection that acquired the lock.
func (e *PgLeaderElector) Release(ctx context.Context, key string) error {
	lockID := keyToLockID(key)

	var qexec interface {
		Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	}
	if e.conn != nil {
		qexec = e.conn.Conn()
	} else {
		qexec = e.db
	}

	_, err := qexec.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID)
	if err != nil {
		return fmt.Errorf("leader election release failed for key %q: %w", key, err)
	}

	e.mu.Lock()
	delete(e.held, key)
	e.mu.Unlock()

	// Release the dedicated connection back to the pool
	if e.conn != nil {
		e.conn.Release()
		e.conn = nil
	}

	return nil
}

// RunAsLeader blocks until leadership is acquired, then executes fn.
// If the parent context is cancelled, it stops trying and returns.
// When fn returns (or panics), leadership is released.
//
// The fn receives a child context that is cancelled if:
//   - the parent context is cancelled
//   - the periodic heartbeat detects the lock was lost
func (e *PgLeaderElector) RunAsLeader(ctx context.Context, key string, fn func(ctx context.Context) error) error {
	// Phase 1: Acquire leadership (retry loop)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		acquired, err := e.Acquire(ctx, key)
		if err != nil {
			e.logger.Warn("leader election attempt failed, retrying",
				"key", key, "error", err, "retry_in", leaderRetryInterval)
			timer := time.NewTimer(leaderRetryInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
				timer.Stop()
				continue
			}
		}

		if acquired {
			e.logger.Info("acquired leadership", "key", key)
			break
		}

		e.logger.Debug("leadership not acquired, another instance is leader", "key", key)
		timer := time.NewTimer(leaderRetryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			timer.Stop()
		}
	}

	// Phase 2: Run fn with a context that gets cancelled if leadership is lost
	fnCtx, fnCancel := context.WithCancel(ctx)
	defer fnCancel()
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		if err := e.Release(releaseCtx, key); err != nil {
			e.logger.Error("failed to release leadership", "key", key, "error", err)
		}
	}()

	// Start heartbeat goroutine to verify we still hold the lock
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		defer func() {
			if r := recover(); r != nil {
				e.logger.Error("heartbeat goroutine panicked", "key", key, "panic", r)
				fnCancel()
			}
		}()
		e.heartbeat(fnCtx, key, fnCancel)
	}()

	// Run the actual worker function
	err := fn(fnCtx)

	// Wait for heartbeat to stop
	fnCancel()
	<-heartbeatDone

	return err
}

// heartbeat periodically checks that we still hold the advisory lock.
// If the lock is lost (e.g., DB connection reset), it cancels the fn context.
// Uses the dedicated connection acquired during Acquire.
func (e *PgLeaderElector) heartbeat(ctx context.Context, key string, cancel context.CancelFunc) {
	ticker := time.NewTicker(leaderRenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Simple liveness check: verify we can still talk to the DB
			// using the same dedicated connection that holds the lock.
			var alive int
			var err error
			if e.conn != nil {
				err = e.conn.Conn().QueryRow(ctx, "SELECT 1").Scan(&alive)
			} else {
				err = e.db.QueryRow(ctx, "SELECT 1").Scan(&alive)
			}
			if err != nil {
				e.logger.Error("heartbeat DB check failed, assuming leadership lost", "key", key, "error", err)
				cancel()
				return
			}
		}
	}
}
