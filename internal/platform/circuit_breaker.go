// Package platform provides shared infrastructure utilities.
package platform

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is in OPEN state.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// State represents the state of the circuit breaker.
type State int

const (
	// StateClosed allows all requests.
	StateClosed State = iota
	// StateOpen blocks all requests.
	StateOpen
	// StateHalfOpen allows limited requests to test recovery.
	StateHalfOpen
)

// String returns a human-readable name for the circuit breaker state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// StateChangeFunc is called when the circuit breaker transitions between states.
// The old and new states are provided. Implementations must not block.
type StateChangeFunc func(name string, from, to State)

// CircuitBreakerOpts configures the circuit breaker. All fields are optional
// and have sensible defaults; use the functional options to override.
type CircuitBreakerOpts struct {
	Name            string          // Identifies this breaker in logs/metrics.
	Threshold       int             // Consecutive failures to trip open. Default 5.
	ResetTimeout    time.Duration   // Time in open before trying half-open. Default 30s.
	SuccessRequired int             // Successes in half-open to close. Default 1.
	OnStateChange   StateChangeFunc // Optional callback.
}

// CircuitBreaker implements the circuit breaker pattern with proper
// half-open single-flight: only one probe request is allowed while open
// transitions to half-open.
type CircuitBreaker struct {
	mu sync.Mutex

	name             string
	state            State
	failureCount     int
	successCount     int // successes in half-open
	threshold        int
	successRequired  int
	resetTimeout     time.Duration
	lastFailure      time.Time
	halfOpenInFlight bool // true while a half-open probe is executing
	onStateChange    StateChangeFunc
}

// NewCircuitBreaker creates a circuit breaker. The two positional args
// (threshold, resetTimeout) are kept for backward compatibility with existing
// callers. Use NewCircuitBreakerWithOpts for full configuration.
func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return NewCircuitBreakerWithOpts(CircuitBreakerOpts{
		Threshold:    threshold,
		ResetTimeout: resetTimeout,
	})
}

// NewCircuitBreakerWithOpts creates a circuit breaker with full options.
func NewCircuitBreakerWithOpts(opts CircuitBreakerOpts) *CircuitBreaker {
	if opts.Threshold <= 0 {
		opts.Threshold = 5
	}
	if opts.ResetTimeout <= 0 {
		opts.ResetTimeout = 30 * time.Second
	}
	if opts.SuccessRequired <= 0 {
		opts.SuccessRequired = 1
	}
	return &CircuitBreaker{
		name:            opts.Name,
		state:           StateClosed,
		threshold:       opts.Threshold,
		successRequired: opts.SuccessRequired,
		resetTimeout:    opts.ResetTimeout,
		onStateChange:   opts.OnStateChange,
	}
}

// Execute wraps a function call with circuit breaker logic.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn()
	if err != nil {
		cb.recordFailure()
		return err
	}

	cb.recordSuccess()
	return nil
}

func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	var cbFunc StateChangeFunc
	var name string
	var from, to State
	var changed bool
	allowed := false

	switch cb.state {
	case StateClosed:
		allowed = true
	case StateOpen:
		if time.Since(cb.lastFailure) <= cb.resetTimeout {
			break
		}
		// Transition to half-open; only allow one probe at a time.
		if cb.halfOpenInFlight {
			break
		}
		cbFunc, name, from, to, changed = cb.transitionLocked(StateHalfOpen)
		cb.halfOpenInFlight = true
		cb.successCount = 0
		allowed = true
	case StateHalfOpen:
		// Allow additional requests only if no probe is in flight.
		if cb.halfOpenInFlight {
			break
		}
		cb.halfOpenInFlight = true
		allowed = true
	}
	cb.mu.Unlock()

	if changed && cbFunc != nil {
		cbFunc(name, from, to)
	}

	return allowed
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	var cbFunc StateChangeFunc
	var name string
	var from, to State
	var changed bool

	cb.halfOpenInFlight = false
	cb.failureCount++
	cb.lastFailure = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.threshold {
			cbFunc, name, from, to, changed = cb.transitionLocked(StateOpen)
		}
	case StateHalfOpen:
		// Probe failed — go back to open.
		cbFunc, name, from, to, changed = cb.transitionLocked(StateOpen)
	}
	cb.mu.Unlock()

	if changed && cbFunc != nil {
		cbFunc(name, from, to)
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	var cbFunc StateChangeFunc
	var name string
	var from, to State
	var changed bool

	cb.halfOpenInFlight = false

	switch cb.state {
	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.successRequired {
			cb.failureCount = 0
			cb.successCount = 0
			cbFunc, name, from, to, changed = cb.transitionLocked(StateClosed)
		}
	default:
		cb.failureCount = 0
		cb.state = StateClosed
	}
	cb.mu.Unlock()

	if changed && cbFunc != nil {
		cbFunc(name, from, to)
	}
}

// transitionLocked changes state and fires the callback. Must be called
// with cb.mu held. The callback is invoked synchronously; implementations
// must not block or acquire cb.mu.
func (cb *CircuitBreaker) transitionLocked(to State) (StateChangeFunc, string, State, State, bool) {
	from := cb.state
	if from == to {
		return nil, "", from, to, false
	}
	cb.state = to
	return cb.onStateChange, cb.name, from, to, true
}

// Reset clears the circuit breaker state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	cbFunc, name, from, to, changed := cb.transitionLocked(StateClosed)
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenInFlight = false
	cb.mu.Unlock()

	if changed && cbFunc != nil {
		cbFunc(name, from, to)
	}
}

// RecordFailure immediately records a failure and trips the circuit breaker
// if it is currently closed. This is used for fast-fail scenarios where
// we want to trip the CB immediately on specific errors.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	var cbFunc StateChangeFunc
	var name string
	var from, to State
	var changed bool

	cb.halfOpenInFlight = false
	cb.failureCount++
	cb.lastFailure = time.Now()

	switch cb.state {
	case StateClosed:
		cbFunc, name, from, to, changed = cb.transitionLocked(StateOpen)
	case StateHalfOpen:
		cbFunc, name, from, to, changed = cb.transitionLocked(StateOpen)
	}
	cb.mu.Unlock()

	if changed && cbFunc != nil {
		cbFunc(name, from, to)
	}
}

// GetState returns the current state of the circuit breaker.
func (cb *CircuitBreaker) GetState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Name returns the configured name of this circuit breaker.
func (cb *CircuitBreaker) Name() string {
	return cb.name
}
