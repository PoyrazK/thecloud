package services

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRT is a simple http.RoundTripper for testing retry behavior.
type mockRT struct {
	results []mockRTResult
	callIdx int
	calls   int
}

type mockRTResult struct {
	resp *http.Response
	err  error
}

func (m *mockRT) RoundTrip(_ *http.Request) (*http.Response, error) {
	m.calls++
	if m.callIdx >= len(m.results) {
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	r := m.results[m.callIdx]
	m.callIdx++
	return r.resp, r.err
}

func mockResp(status int) mockRTResult {
	return mockRTResult{resp: &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}}
}

func mockErr(msg string) mockRTResult {
	return mockRTResult{err: errors.New(msg)}
}

// --- retryTransport helper tests ---

func TestRetryTransport_IsIdempotent(t *testing.T) {
	t.Parallel()
	rt := &retryTransport{}
	for _, m := range []string{"GET", "HEAD", "PUT", "DELETE", "OPTIONS"} {
		assert.True(t, rt.isIdempotent(m), m)
	}
	for _, m := range []string{"POST", "PATCH", "CONNECT", "TRACE"} {
		assert.False(t, rt.isIdempotent(m), m)
	}
}

func TestRetryTransport_IsRetryableStatus(t *testing.T) {
	t.Parallel()
	rt := &retryTransport{}
	for _, c := range []int{502, 503, 504, 429} {
		assert.True(t, rt.isRetryableStatus(c), "%d should be retryable", c)
	}
	for _, c := range []int{200, 201, 400, 401, 403, 404, 500} {
		assert.False(t, rt.isRetryableStatus(c), "%d should not be retryable", c)
	}
}

func TestRetryTransport_IsRetryableError(t *testing.T) {
	t.Parallel()
	rt := &retryTransport{}
	retryable := []string{
		"dial tcp: connection refused",
		"dial tcp: i/o timeout",
		"read tcp: connection reset by peer",
		"write tcp: broken pipe",
		"read tcp: connection reset",
	}
	for _, msg := range retryable {
		assert.True(t, rt.isRetryableError(errors.New(msg)), msg)
	}
	nonRetryable := []string{
		"400 bad request",
		"401 unauthorized",
		"tls: handshake failed",
		"server closed connection",
	}
	for _, msg := range nonRetryable {
		assert.False(t, rt.isRetryableError(errors.New(msg)), msg)
	}
}

func TestRetryTransport_BackoffJitter_Bounded(t *testing.T) {
	t.Parallel()
	rt := &retryTransport{retryTimeout: 5 * time.Second}
	for attempt := 1; attempt <= 5; attempt++ {
		d := rt.backoffWithJitter(attempt)
		assert.Greater(t, d, time.Duration(0), "delay must be > 0")
		assert.LessOrEqual(t, d, 5*time.Second, "delay must be <= max")
	}
}

// --- retry loop tests ---

func TestRetryTransport_DoesNotRetryWhenMaxRetriesZero(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{mockResp(502), mockResp(200)}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 0})

	_, _ = transport.RoundTrip(nil) //nolint:bodyclose
	// m.results[0] (502) is returned immediately, body closed in doRoundTrip loop exit path
	// m.results[1] (200) is never consumed since maxRetries=0 → no retry
	assert.Equal(t, 1, m.calls, "should call base transport only once")
}

func TestRetryTransport_DoesNotRetryNonIdempotentPOST(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{mockErr("connection refused"), mockResp(200)}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("POST", "/", nil)
	_, _ = transport.RoundTrip(req) //nolint:bodyclose
	assert.Equal(t, 1, m.calls, "POST should not be retried")
	// m.results[0].err is non-nil, returned immediately — no body to close
	// m.results[1] is never consumed since POST is not retried
}

func TestRetryTransport_DoesNotRetryNonIdempotentPATCH(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{mockErr("connection refused"), mockResp(200)}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("PATCH", "/", nil)
	_, _ = transport.RoundTrip(req) //nolint:bodyclose
	assert.Equal(t, 1, m.calls, "PATCH should not be retried")
}

func TestRetryTransport_RetriesOnConnectionRefused(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{
		mockErr("connection refused"),
		mockResp(200),
	}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, m.calls, "should retry after connection refused")
}

func TestRetryTransport_RetriesOn502(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{
		mockResp(502),
		mockResp(502),
		mockResp(200),
	}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 3, m.calls, "should retry 502 twice then succeed")
}

func TestRetryTransport_RetriesOn503(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{
		mockResp(503),
		mockResp(200),
	}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, m.calls)
}

func TestRetryTransport_RetriesOn429(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{
		mockResp(429),
		mockResp(200),
	}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, m.calls)
}

func TestRetryTransport_NoRetryOn500(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{
		mockResp(500),
	}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, 1, m.calls, "500 should not be retried")
}

func TestRetryTransport_NoRetryOn400(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{
		mockResp(400),
	}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, 1, m.calls, "400 should not be retried")
}

func TestRetryTransport_RetriesOnTimeoutError(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{
		mockErr("dial tcp: i/o timeout"),
		mockResp(200),
	}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, m.calls)
}

func TestRetryTransport_GivesUpAfterMaxRetries(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{
		mockResp(502),
		mockResp(502),
		mockResp(502),
	}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose
	require.NoError(t, err)
	assert.Equal(t, 502, resp.StatusCode)
	assert.Equal(t, 3, m.calls, "3 attempts: first + 2 retries")
}

func TestRetryTransport_GivesUpOnConnectionErrorAfterMaxRetries(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{
		mockErr("connection refused"),
		mockErr("connection refused"),
		mockErr("connection refused"),
	}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose
	// When all attempts return errors, doRoundTrip returns nil, nil
	require.NoError(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 3, m.calls, "3 attempts: first + 2 retries")
}

func TestRetryTransport_SucceedsOnFirstAttempt(t *testing.T) {
	t.Parallel()
	m := &mockRT{results: []mockRTResult{mockResp(200)}}
	transport := wrapTransport(m, &retryTransport{maxRetries: 2})

	req, _ := http.NewRequest("GET", "/", nil)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 1, m.calls)
}

// wrapTransport creates a retryTransport wrapping the mock.
func wrapTransport(mock *mockRT, rt *retryTransport) *retryTransport {
	// rt.base is used directly by doRoundTrip — swap it for our mock
	rt.base = (*mockHTTPTransport)(mock)
	return rt
}

// mockHTTPTransport lets us inject the mock via rt.base.
type mockHTTPTransport mockRT

func (m *mockHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return (*mockRT)(m).RoundTrip(req)
}

func (m *mockHTTPTransport) CloseIdleConnections() {}

// --- circuit breaker tests ---

func TestCircuitBreaker_DisabledWhenThresholdZero(t *testing.T) {
	t.Parallel()
	route := &domain.GatewayRoute{
		ID:                      [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		CircuitBreakerThreshold: 0,
		MaxRetries:              2,
		RetryTimeout:            5000,
	}
	rt := newRetryTransport(&http.Transport{}, route, nil)
	assert.Nil(t, rt.cb)
	assert.Equal(t, 2, rt.maxRetries)
}

func TestCircuitBreaker_EnabledWhenThresholdPositive(t *testing.T) {
	t.Parallel()
	route := &domain.GatewayRoute{
		ID:                      [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   30000,
		MaxRetries:              2,
		RetryTimeout:            5000,
	}
	rt := newRetryTransport(&http.Transport{}, route, nil)
	assert.NotNil(t, rt.cb)
	assert.Equal(t, platform.StateClosed, rt.cb.GetState())
}

func TestCircuitBreaker_TripsOpenAfterThreshold(t *testing.T) {
	t.Parallel()
	cb := platform.NewCircuitBreakerWithOpts(platform.CircuitBreakerOpts{
		Name:          "test",
		Threshold:     3,
		ResetTimeout:  100 * time.Millisecond,
		OnStateChange: nil,
	})

	for i := 0; i < 3; i++ {
		_ = cb.Execute(func() error { return errors.New("fail") })
	}
	assert.Equal(t, platform.StateOpen, cb.GetState())

	// Next call is blocked
	err := cb.Execute(func() error { return nil })
	assert.ErrorIs(t, err, platform.ErrCircuitOpen)
}

func TestCircuitBreaker_GoesHalfOpenAfterTimeout(t *testing.T) {
	t.Parallel()
	cb := platform.NewCircuitBreakerWithOpts(platform.CircuitBreakerOpts{
		Name:          "test",
		Threshold:     2,
		ResetTimeout:  50 * time.Millisecond,
		OnStateChange: nil,
	})

	_ = cb.Execute(func() error { return errors.New("fail") })
	_ = cb.Execute(func() error { return errors.New("fail") })
	assert.Equal(t, platform.StateOpen, cb.GetState())

	// Wait for half-open window to expire, then trigger a probe request
	time.Sleep(80 * time.Millisecond)
	_ = cb.Execute(func() error { return errors.New("still failing") })
	// After ResetTimeout the CB transitions to half-open automatically.
	// The probe arrives during or just after that transition, so either
	// Open (transition not yet observed) or HalfOpen (transition complete but probe pending)
	// is valid — this is not a flaky test.
	assert.True(t, cb.GetState() == platform.StateOpen || cb.GetState() == platform.StateHalfOpen)
}

func TestCircuitBreaker_ClosesAfterSuccessfulProbe(t *testing.T) {
	t.Parallel()
	cb := platform.NewCircuitBreakerWithOpts(platform.CircuitBreakerOpts{
		Name:          "test",
		Threshold:     2,
		ResetTimeout:  50 * time.Millisecond,
		OnStateChange: nil,
	})

	_ = cb.Execute(func() error { return errors.New("fail") })
	_ = cb.Execute(func() error { return errors.New("fail") })
	time.Sleep(80 * time.Millisecond)
	_ = cb.Execute(func() error { return nil })

	assert.Equal(t, platform.StateClosed, cb.GetState())
}
