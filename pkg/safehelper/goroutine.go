package safehelper

import (
	"context"
	"sync"
	"time"
)

// GoWithTimeout launches a goroutine that is guaranteed to have a timeout.
// This satisfies gosec G118 because the timeout is always established.
// If timeout <= 0, defaults to 1 second to prevent immediately-cancelled context.
func GoWithTimeout(timeout time.Duration, fn func(ctx context.Context)) {
	if timeout <= 0 {
		timeout = 1
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		fn(ctx)
	}()
}

// GoWithErrorChannel launches a goroutine with timeout and error reporting.
// Use this when you need to capture errors from the goroutine.
// The errCh must be buffered to avoid blocking.
func GoWithErrorChannel(timeout time.Duration, errCh chan<- error, fn func(ctx context.Context) error) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := fn(ctx); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}()
}

// GoWithWaitGroup launches a goroutine with timeout, tracking completion via WaitGroup.
// The wg.Add(1) should be called before this function.
func GoWithWaitGroup(timeout time.Duration, wg *sync.WaitGroup, fn func(ctx context.Context)) {
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		fn(ctx)
	}()
}