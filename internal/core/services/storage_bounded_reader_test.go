package services

import (
	"io"
	"testing"

	apperrors "github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingReader returns at most len(p) bytes per Read, respecting buffer size.
type countingReader struct {
	data []byte
	pos  int
}

func (c *countingReader) Read(p []byte) (n int, err error) {
	if c.pos >= len(c.data) {
		return 0, io.EOF
	}
	n = copy(p, c.data[c.pos:])
	c.pos += n
	return n, nil
}

func TestBoundedReaderUnderLimit(t *testing.T) {
	// 5 bytes of data, limit is 10 — should succeed without cleanup
	r := &countingReader{data: []byte("hello")}
	cleanupCalled := false
	br := &boundedReader{
		r:         r,
		limit:     10,
		cleanupFn: func() { cleanupCalled = true },
	}

	buf := make([]byte, 100)
	n, err := br.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", string(buf[:n]))
	assert.False(t, cleanupCalled, "cleanup should not be called when under limit")
}

func TestBoundedReaderAtLimit(t *testing.T) {
	// 5 bytes of data, limit is exactly 5 — first read returns 0 + EOF after probe,
	// second read returns 0 + EOF (underlying exhausted)
	r := &countingReader{data: []byte("hello")}
	cleanupCalled := false
	br := &boundedReader{
		r:         r,
		limit:     5,
		cleanupFn: func() { cleanupCalled = true },
	}

	buf := make([]byte, 100)
	n1, err1 := br.Read(buf)
	// first Read: data transferred, probe finds underlying EOF → clean EOF
	assert.Equal(t, 0, n1)
	require.ErrorIs(t, err1, io.EOF)
	assert.False(t, cleanupCalled)

	// Second Read: count == limit, underlying already exhausted → clean EOF
	n2, err2 := br.Read(buf)
	assert.Equal(t, 0, n2)
	require.ErrorIs(t, err2, io.EOF)
	assert.False(t, cleanupCalled, "cleanup should not be called for exact-limit clean EOF")
}

func TestBoundedReaderExceedsLimit(t *testing.T) {
	// 10 bytes of data, limit is 5 — first read hits limit, probe finds extra data,
	// boundedReader returns ObjectTooLarge immediately (no partial data returned)
	r := &countingReader{data: make([]byte, 10)}
	cleanupCalled := false
	br := &boundedReader{
		r:         r,
		limit:     5,
		cleanupFn: func() { cleanupCalled = true },
	}

	buf := make([]byte, 100)
	n1, err1 := br.Read(buf)
	assert.Equal(t, 0, n1)
	assert.True(t, cleanupCalled, "cleanup must be called on oversize")
	var appErr1 apperrors.Error
	require.ErrorAs(t, err1, &appErr1)
	assert.Equal(t, apperrors.ObjectTooLarge, appErr1.Type)
}

func TestBoundedReaderExceedsLimitInSingleRead(t *testing.T) {
	// 10 bytes of data, limit is 5 — boundedReader reads up to remaining (5)
	// and probe finds extra data, so it returns ObjectTooLarge immediately
	// on the first Read (no partial data returned to caller).
	r := &countingReader{data: make([]byte, 10)}
	br := &boundedReader{
		r:         r,
		limit:     5,
		cleanupFn: func() {},
	}

	buf := make([]byte, 100)
	n, err := br.Read(buf)
	assert.Equal(t, 0, n)
	var appErr apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.ObjectTooLarge, appErr.Type)
}

func TestBoundedReaderNilCleanupFn(t *testing.T) {
	// cleanupFn is nil — overflow must not panic.
	r := &countingReader{data: make([]byte, 10)}
	br := &boundedReader{
		r:     r,
		limit: 5,
		// cleanupFn intentionally nil
	}

	buf := make([]byte, 100)
	n, err := br.Read(buf)
	assert.Equal(t, 0, n)
	var appErr apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.ObjectTooLarge, appErr.Type)
}
