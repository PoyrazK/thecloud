// Package pool implements per-function warm container pools for serverless.
package pool

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

// Pool defaults.
const (
	DefaultMinSize     = 1
	DefaultMaxSize    = 10
	DefaultMaxIdleSecs = 300
)

// FunctionPool manages a pool of warm instances for a single function.
type FunctionPool struct {
	functionID uuid.UUID
	config     ports.PoolConfig
	warm       []*ports.PoolInstance
	busy       map[string]*ports.PoolInstance
	starting   int
	backend    ports.ComputeBackend
	taskOpts   ports.RunTaskOptions
	mu         sync.RWMutex
	reaperStop chan struct{}
	logger     *slog.Logger
}

// PoolManagerImpl manages per-function warm pools.
type PoolManagerImpl struct {
	pools   map[uuid.UUID]*FunctionPool
	mu      sync.RWMutex
	backend ports.ComputeBackend
	logger  *slog.Logger
}

// NewPoolManager creates a new pool manager.
func NewPoolManager(backend ports.ComputeBackend, logger *slog.Logger) *PoolManagerImpl {
	return &PoolManagerImpl{
		pools:   make(map[uuid.UUID]*FunctionPool),
		backend: backend,
		logger:  logger,
	}
}

// Acquire returns a warm instance for the given function.
// It implements backpressure: if no instances are available and we're below
// MaxSize, it spawns a new one. If at MaxSize and all busy, it waits.
func (m *PoolManagerImpl) Acquire(ctx context.Context, functionID uuid.UUID) (*ports.PoolInstance, func(error), error) {
	pool := m.getOrCreatePool(functionID)
	return pool.Acquire(ctx)
}

// GetPoolStats returns current pool utilization for a function.
func (m *PoolManagerImpl) GetPoolStats(ctx context.Context, functionID uuid.UUID) (ports.PoolStats, error) {
	m.mu.RLock()
	pool, ok := m.pools[functionID]
	m.mu.RUnlock()
	if !ok {
		return ports.PoolStats{}, nil
	}
	return pool.Stats(), nil
}

// InvalidateFunction removes all warm instances for a function.
func (m *PoolManagerImpl) InvalidateFunction(ctx context.Context, functionID uuid.UUID) error {
	m.mu.Lock()
	pool, ok := m.pools[functionID]
	if ok {
		delete(m.pools, functionID)
	}
	m.mu.Unlock()

	if pool != nil {
		pool.destroy(ctx)
	}
	return nil
}

// RegisterFunction registers a function with the pool manager,
// associating its pool config and task options.
func (m *PoolManagerImpl) RegisterFunction(functionID uuid.UUID, config ports.PoolConfig, taskOpts ports.RunTaskOptions) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existingPool, ok := m.pools[functionID]; ok {
		existingPool.updateConfig(config, taskOpts)
		return
	}

	newPool := &FunctionPool{
		functionID: functionID,
		config:     config,
		warm:       make([]*ports.PoolInstance, 0),
		busy:       make(map[string]*ports.PoolInstance),
		backend:    m.backend,
		taskOpts:   taskOpts,
		reaperStop: make(chan struct{}),
		logger:     m.logger,
	}
	m.pools[functionID] = newPool
	go newPool.reaper()
}

// UnregisterFunction removes a function from the pool manager.
func (m *PoolManagerImpl) UnregisterFunction(ctx context.Context, functionID uuid.UUID) error {
	return m.InvalidateFunction(ctx, functionID)
}

// Get returns an existing pool or nil.
func (m *PoolManagerImpl) Get(functionID uuid.UUID) *FunctionPool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pools[functionID]
}

func (m *PoolManagerImpl) getOrCreatePool(functionID uuid.UUID) *FunctionPool {
	m.mu.RLock()
	pool, ok := m.pools[functionID]
	m.mu.RUnlock()
	if ok {
		return pool
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Double-check
	if pool, ok := m.pools[functionID]; ok {
		return pool
	}

	pool = &FunctionPool{
		functionID: functionID,
		config:     defaultConfig(),
		warm:       make([]*ports.PoolInstance, 0),
		busy:       make(map[string]*ports.PoolInstance),
		backend:    m.backend,
		taskOpts:   ports.RunTaskOptions{},
		reaperStop: make(chan struct{}),
		logger:     m.logger,
	}
	m.pools[functionID] = pool
	return pool
}

// Acquire attempts to get a warm instance, creating one if needed.
func (p *FunctionPool) Acquire(ctx context.Context) (*ports.PoolInstance, func(error), error) {
	p.mu.Lock()

	// 1. Try to grab a warm instance
	if len(p.warm) > 0 {
		n := len(p.warm)
		inst := p.warm[n-1]
		p.warm = p.warm[:n-1]
		inst.Status = "BUSY"
		p.busy[inst.BackendID] = inst
		p.mu.Unlock()
		return inst, makeReleaseFn(p, inst), nil
	}

	// 2. Try to scale up
	totalActive := len(p.warm) + len(p.busy) + p.starting
	if totalActive < p.config.MaxSize {
		p.starting++
		p.mu.Unlock()
		// Launch instance asynchronously
		go p.launchAsync()
		// Wait for it to become available
		return p.waitForWarmInstance(ctx)
	}

	p.mu.Unlock()

	// 3. Wait for a warm instance with backpressure
	return p.waitWithBackpressure(ctx)
}

func (p *FunctionPool) waitForWarmInstance(ctx context.Context) (*ports.PoolInstance, func(error), error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		p.mu.RLock()
		if len(p.warm) > 0 {
			n := len(p.warm)
			inst := p.warm[n-1]
			p.warm = p.warm[:n-1]
			inst.Status = "BUSY"
			p.busy[inst.BackendID] = inst
			p.mu.RUnlock()
			return inst, makeReleaseFn(p, inst), nil
		}
		// Check if any launch failed
		starting := p.starting
		p.mu.RUnlock()

		if starting == 0 {
			// No launch in progress, we need to trigger one
			p.mu.Lock()
			totalActive := len(p.warm) + len(p.busy) + p.starting
			if totalActive < p.config.MaxSize {
				p.starting++
				p.mu.Unlock()
				go p.launchAsync()
			} else {
				p.mu.Unlock()
			}
		}

		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-ticker.C:
			continue
		}
	}
}

func (p *FunctionPool) waitWithBackpressure(ctx context.Context) (*ports.PoolInstance, func(error), error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		p.mu.RLock()
		if len(p.warm) > 0 {
			n := len(p.warm)
			inst := p.warm[n-1]
			p.warm = p.warm[:n-1]
			inst.Status = "BUSY"
			p.busy[inst.BackendID] = inst
			p.mu.RUnlock()
			return inst, makeReleaseFn(p, inst), nil
		}
		totalActive := len(p.warm) + len(p.busy) + p.starting
		p.mu.RUnlock()

		if totalActive < p.config.MaxSize {
			// Scale up opportunity
			p.mu.Lock()
			totalActive = len(p.warm) + len(p.busy) + p.starting
			if totalActive < p.config.MaxSize {
				p.starting++
				p.mu.Unlock()
				go p.launchAsync()
				continue
			}
			p.mu.Unlock()
		}

		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-ticker.C:
			continue
		}
	}
}

func (p *FunctionPool) launchAsync() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	id, _, err := p.backend.StartPoolInstance(ctx, p.taskOpts)
	if err != nil {
		p.logger.Error("failed to launch warm instance", "function_id", p.functionID, "error", err)
		p.mu.Lock()
		p.starting--
		p.mu.Unlock()
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.starting--

	// Check if already at MaxSize or shutting down
	totalActive := len(p.warm) + len(p.busy) + p.starting
	if totalActive >= p.config.MaxSize {
		// Don't keep it, destroy immediately
		go func() {
			delCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = p.backend.DeleteInstance(delCtx, id)
		}()
		return
	}

	p.warm = append(p.warm, &ports.PoolInstance{
		ID:        id,
		Status:    "WARM",
		LastUsed:  time.Now(),
		BackendID: id,
	})
}

func makeReleaseFn(p *FunctionPool, inst *ports.PoolInstance) func(error) {
	return func(execError error) {
		p.release(inst, execError)
	}
}

func (p *FunctionPool) release(inst *ports.PoolInstance, execError error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.busy, inst.BackendID)

	if execError != nil {
		// Destroy the instance on error
		go func() {
			delCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := p.backend.DeleteInstance(delCtx, inst.BackendID); err != nil {
				p.logger.Warn("failed to delete warm instance after error",
					"instance_id", inst.BackendID, "error", err)
			}
		}()
		return
	}

	// Return to warm pool
	inst.Status = "WARM"
	inst.LastUsed = time.Now()
	p.warm = append(p.warm, inst)
}

// Stats returns current pool statistics.
func (p *FunctionPool) Stats() ports.PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return ports.PoolStats{
		TotalSize: len(p.warm) + len(p.busy) + p.starting,
		WarmCount: len(p.warm),
		BusyCount: len(p.busy),
	}
}

func (p *FunctionPool) reaper() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-p.reaperStop:
			return
		case <-ticker.C:
			p.reapIdleInstances()
		}
	}
}

func (p *FunctionPool) reapIdleInstances() {
	p.mu.Lock()
	defer p.mu.Unlock()

	cutoff := time.Now().Add(-p.config.MaxIdleTime)
	var stillWarm []*ports.PoolInstance

	for _, inst := range p.warm {
		if inst.LastUsed.Before(cutoff) && len(p.warm) > p.config.MinSize {
			// Reap this instance
			go func(id string) {
				delCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := p.backend.DeleteInstance(delCtx, id); err != nil {
					p.logger.Warn("failed to reap warm instance",
						"instance_id", id, "error", err)
				}
			}(inst.BackendID)
		} else {
			stillWarm = append(stillWarm, inst)
		}
	}

	p.warm = stillWarm
}

func (p *FunctionPool) destroy(ctx context.Context) {
	close(p.reaperStop)

	p.mu.Lock()
	warm := p.warm
	p.warm = nil
	p.busy = nil
	p.mu.Unlock()

	for _, inst := range warm {
		delCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := p.backend.DeleteInstance(delCtx, inst.BackendID); err != nil {
			p.logger.Warn("failed to delete warm instance during destroy",
				"instance_id", inst.BackendID, "error", err)
		}
		cancel()
	}
}

func (p *FunctionPool) updateConfig(config ports.PoolConfig, taskOpts ports.RunTaskOptions) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = config
	p.taskOpts = taskOpts
}

func defaultConfig() ports.PoolConfig {
	return ports.PoolConfig{
		MinSize:     DefaultMinSize,
		MaxSize:     DefaultMaxSize,
		MaxIdleTime: DefaultMaxIdleSecs * time.Second,
	}
}

