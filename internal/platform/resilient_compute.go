package platform

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/poyrazk/thecloud/internal/core/ports"
)

// ResilientComputeOpts configures the resilient compute wrapper.
type ResilientComputeOpts struct {
	// CallTimeout is the per-call context timeout for normal operations.
	// Default: 2 minutes.
	CallTimeout time.Duration
	// LongCallTimeout is the timeout for operations that are expected to take
	// longer (e.g., LaunchInstanceWithOptions, RunTask). Default: 10 minutes.
	LongCallTimeout time.Duration
	// CBThreshold is the number of consecutive failures before the circuit
	// opens. Default: 5.
	CBThreshold int
	// CBResetTimeout is how long the circuit stays open before attempting a
	// half-open probe. Default: 30s.
	CBResetTimeout time.Duration
	// BulkheadMaxConc is the max concurrent calls to the backend. Default: 20.
	BulkheadMaxConc int
	// BulkheadWait is how long to wait for a bulkhead slot. Default: 10s.
	BulkheadWait time.Duration
}

func (o ResilientComputeOpts) withDefaults() ResilientComputeOpts {
	if o.CallTimeout <= 0 {
		o.CallTimeout = 2 * time.Minute
	}
	if o.LongCallTimeout <= 0 {
		o.LongCallTimeout = 10 * time.Minute
	}
	if o.CBThreshold <= 0 {
		o.CBThreshold = 5
	}
	if o.CBResetTimeout <= 0 {
		o.CBResetTimeout = 30 * time.Second
	}
	if o.BulkheadMaxConc <= 0 {
		o.BulkheadMaxConc = 20
	}
	if o.BulkheadWait <= 0 {
		o.BulkheadWait = 10 * time.Second
	}
	return o
}

// ResilientCompute wraps a ComputeBackend with circuit breaker, bulkhead,
// and per-call timeouts. It implements the ports.ComputeBackend interface.
//
// Note: since v2, the circuit breaker is per-operation (one breaker guards
// each operation type on a single compute backend). If one operation fails
// repeatedly, only that operation type is blocked — others continue working.
type ResilientCompute struct {
	inner    ports.ComputeBackend
	cb       map[string]*CircuitBreaker
	cbMu     sync.Mutex
	bulkhead *Bulkhead
	logger   *slog.Logger
	opts     ResilientComputeOpts
}

// NewResilientCompute decorates inner with resilience primitives.
func NewResilientCompute(inner ports.ComputeBackend, logger *slog.Logger, opts ResilientComputeOpts) *ResilientCompute {
	opts = opts.withDefaults()
	name := fmt.Sprintf("compute-%s", inner.Type())

	bh := NewBulkhead(BulkheadOpts{
		Name:        name,
		MaxConc:     opts.BulkheadMaxConc,
		WaitTimeout: opts.BulkheadWait,
	})

	return &ResilientCompute{
		inner:    inner,
		cb:       make(map[string]*CircuitBreaker),
		bulkhead: bh,
		logger:   logger.With("adapter", name),
		opts:     opts,
	}
}

// ---------- helpers ----------

// getOrCreateBreaker returns an existing per-operation breaker or creates
// a new one. Thread-safe.
func (r *ResilientCompute) getOrCreateBreaker(op string) *CircuitBreaker {
	r.cbMu.Lock()
	defer r.cbMu.Unlock()
	if cb, ok := r.cb[op]; ok {
		return cb
	}
	name := fmt.Sprintf("compute-%s-%s", r.inner.Type(), op)
	cb := NewCircuitBreakerWithOpts(CircuitBreakerOpts{
		Name:            name,
		Threshold:       r.opts.CBThreshold,
		ResetTimeout:    r.opts.CBResetTimeout,
		SuccessRequired: 2,
		OnStateChange: func(n string, from, to State) {
			r.logger.Warn("circuit breaker state change",
				"breaker", n, "from", from.String(), "to", to.String())
		},
	})
	r.cb[op] = cb
	return cb
}

// callProtected runs fn through bulkhead → per-operation circuit breaker → timeout.
func (r *ResilientCompute) callProtected(ctx context.Context, timeout time.Duration, op string, fn func(ctx context.Context) error) error {
	return r.bulkhead.Execute(ctx, func() error {
		return r.getOrCreateBreaker(op).Execute(func() error {
			ctx2, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			return fn(ctx2)
		})
	})
}

// ---------- Instance Lifecycle ----------

func (r *ResilientCompute) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	var id string
	var ps []string
	err := r.callProtected(ctx, r.opts.LongCallTimeout, "launch", func(ctx context.Context) error {
		var e error
		id, ps, e = r.inner.LaunchInstanceWithOptions(ctx, opts)
		return e
	})
	return id, ps, err
}

func (r *ResilientCompute) StartInstance(ctx context.Context, id string) error {
	return r.callProtected(ctx, r.opts.CallTimeout, "start", func(ctx context.Context) error {
		return r.inner.StartInstance(ctx, id)
	})
}

func (r *ResilientCompute) StopInstance(ctx context.Context, id string) error {
	return r.callProtected(ctx, r.opts.CallTimeout, "stop", func(ctx context.Context) error {
		return r.inner.StopInstance(ctx, id)
	})
}

func (r *ResilientCompute) DeleteInstance(ctx context.Context, id string) error {
	return r.callProtected(ctx, r.opts.CallTimeout, "delete", func(ctx context.Context) error {
		return r.inner.DeleteInstance(ctx, id)
	})
}

func (r *ResilientCompute) ResizeInstance(ctx context.Context, id string, cpuNano, memoryBytes int64) error {
	return r.callProtected(ctx, r.opts.CallTimeout, "resize", func(ctx context.Context) error {
		return r.inner.ResizeInstance(ctx, id, cpuNano, memoryBytes)
	})
}

func (r *ResilientCompute) CreateSnapshot(ctx context.Context, id, name string) error {
	return r.callProtected(ctx, r.opts.CallTimeout, "create_snapshot", func(ctx context.Context) error {
		return r.inner.CreateSnapshot(ctx, id, name)
	})
}

func (r *ResilientCompute) RestoreSnapshot(ctx context.Context, id, name string) error {
	return r.callProtected(ctx, r.opts.CallTimeout, "restore_snapshot", func(ctx context.Context) error {
		return r.inner.RestoreSnapshot(ctx, id, name)
	})
}

func (r *ResilientCompute) DeleteSnapshot(ctx context.Context, id, name string) error {
	return r.callProtected(ctx, r.opts.CallTimeout, "delete_snapshot", func(ctx context.Context) error {
		return r.inner.DeleteSnapshot(ctx, id, name)
	})
}

func (r *ResilientCompute) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	var rc io.ReadCloser
	err := r.callProtected(ctx, r.opts.CallTimeout, "get_logs", func(ctx context.Context) error {
		var e error
		rc, e = r.inner.GetInstanceLogs(ctx, id)
		return e
	})
	return rc, err
}

func (r *ResilientCompute) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	var rc io.ReadCloser
	err := r.callProtected(ctx, r.opts.CallTimeout, "stats", func(ctx context.Context) error {
		var e error
		rc, e = r.inner.GetInstanceStats(ctx, id)
		return e
	})
	return rc, err
}

func (r *ResilientCompute) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	var port int
	err := r.callProtected(ctx, r.opts.CallTimeout, "get_port", func(ctx context.Context) error {
		var e error
		port, e = r.inner.GetInstancePort(ctx, id, internalPort)
		return e
	})
	return port, err
}

func (r *ResilientCompute) GetInstanceIP(ctx context.Context, id string) (string, error) {
	var ip string
	err := r.callProtected(ctx, r.opts.CallTimeout, "get_ip", func(ctx context.Context) error {
		var e error
		ip, e = r.inner.GetInstanceIP(ctx, id)
		return e
	})
	return ip, err
}

func (r *ResilientCompute) GetConsoleURL(ctx context.Context, id string) (string, error) {
	var url string
	err := r.callProtected(ctx, r.opts.CallTimeout, "console_url", func(ctx context.Context) error {
		var e error
		url, e = r.inner.GetConsoleURL(ctx, id)
		return e
	})
	return url, err
}

// ---------- Execution ----------

func (r *ResilientCompute) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	var out string
	err := r.callProtected(ctx, r.opts.CallTimeout, "exec", func(ctx context.Context) error {
		var e error
		out, e = r.inner.Exec(ctx, id, cmd)
		return e
	})
	return out, err
}

func (r *ResilientCompute) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	var id string
	var ps []string
	err := r.callProtected(ctx, r.opts.LongCallTimeout, "run_task", func(ctx context.Context) error {
		var e error
		id, ps, e = r.inner.RunTask(ctx, opts)
		return e
	})
	return id, ps, err
}

func (r *ResilientCompute) WaitTask(ctx context.Context, id string) (int64, error) {
	var code int64
	err := r.callProtected(ctx, r.opts.LongCallTimeout, "wait_task", func(ctx context.Context) error {
		var e error
		code, e = r.inner.WaitTask(ctx, id)
		return e
	})
	return code, err
}

// ---------- Network Management ----------

func (r *ResilientCompute) CreateNetwork(ctx context.Context, name string) (string, error) {
	var id string
	err := r.callProtected(ctx, r.opts.CallTimeout, "create_network", func(ctx context.Context) error {
		var e error
		id, e = r.inner.CreateNetwork(ctx, name)
		return e
	})
	return id, err
}

func (r *ResilientCompute) DeleteNetwork(ctx context.Context, id string) error {
	return r.callProtected(ctx, r.opts.CallTimeout, "delete_network", func(ctx context.Context) error {
		return r.inner.DeleteNetwork(ctx, id)
	})
}

// ---------- Volume Attachment ----------

func (r *ResilientCompute) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	var devPath, containerID string
	err := r.callProtected(ctx, r.opts.CallTimeout, "attach_volume", func(ctx context.Context) error {
		var e error
		devPath, containerID, e = r.inner.AttachVolume(ctx, id, volumePath)
		return e
	})
	if err != nil {
		return "", "", err
	}
	return devPath, containerID, nil
}

func (r *ResilientCompute) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	var containerID string
	err := r.callProtected(ctx, r.opts.CallTimeout, "detach_volume", func(ctx context.Context) error {
		var e error
		containerID, e = r.inner.DetachVolume(ctx, id, volumePath)
		return e
	})
	if err != nil {
		return "", err
	}
	return containerID, nil
}

// ---------- Health ----------

// Ping bypasses the bulkhead (low cost, used for health checks) but still
// goes through the circuit breaker so a broken backend trips the circuit.
func (r *ResilientCompute) Ping(ctx context.Context) error {
	return r.getOrCreateBreaker("ping").Execute(func() error {
		ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return r.inner.Ping(ctx2)
	})
}

func (r *ResilientCompute) PauseInstance(ctx context.Context, id string) error {
	return r.callProtected(ctx, r.opts.CallTimeout, "pause", func(ctx context.Context) error {
		return r.inner.PauseInstance(ctx, id)
	})
}

func (r *ResilientCompute) ResumeInstance(ctx context.Context, id string) error {
	return r.callProtected(ctx, r.opts.CallTimeout, "resume", func(ctx context.Context) error {
		return r.inner.ResumeInstance(ctx, id)
	})
}

// Type delegates directly — no protection needed.
func (r *ResilientCompute) Type() string {
	return r.inner.Type()
}

// Unwrap returns the underlying ComputeBackend (useful for tests).
func (r *ResilientCompute) Unwrap() ports.ComputeBackend {
	return r.inner
}

// ResetCircuitBreaker resets all per-operation circuit breaker states.
// Useful for E2E tests that need clean state between test suites.
func (r *ResilientCompute) ResetCircuitBreaker() {
	r.cbMu.Lock()
	defer r.cbMu.Unlock()
	for _, cb := range r.cb {
		cb.Reset()
	}
}
