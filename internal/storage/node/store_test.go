package node

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalStore(tmpDir)
	require.NoError(t, err)

	bucket := "test-bucket"
	key := "folder/test-object.txt"
	data := []byte("hello world")
	timestamp := time.Now().UnixNano()

	// 1. Test Write
	err = store.Write(bucket, key, data, timestamp)
	require.NoError(t, err)

	// Verify file exists
	path := filepath.Join(tmpDir, bucket, key)
	_, err = os.Stat(path)
	require.NoError(t, err)

	// 2. Test Read
	readData, readTs, err := store.Read(bucket, key)
	require.NoError(t, err)
	assert.Equal(t, data, readData)
	assert.Equal(t, timestamp, readTs)

	// 3. Test Delete
	err = store.Delete(bucket, key)
	require.NoError(t, err)

	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err))
}

func TestLocalStorePathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewLocalStore(tmpDir)

	// Attempt to write outside root
	err := store.Write("bucket", "../outside.txt", []byte("evil"), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied") // os.ErrPermission

	// Attempt to read outside root
	_, _, err = store.Read("bucket", "../outside.txt")
	require.Error(t, err)

	// Table-driven tests for path isolation edge cases
	testCases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "dot key", key: ".", wantErr: true},
		{name: "dot slash", key: "./", wantErr: true},
		{name: "dot dot dot", key: "./.", wantErr: true},
		// Dot in middle is allowed: filepath.Clean normalizes "foo/./bar" to "foo/bar"
		{name: "dot in middle works", key: "foo/./bar", wantErr: false},
		{name: "url encoded traversal", key: "..%2Foutside.txt", wantErr: true},
		{name: "backslash encoded", key: "..%5Coutside.txt", wantErr: true},
		{name: "multi dot dot", key: "../foo/../../bar", wantErr: true},
	}
	for _, tc := range testCases {
		tc := tc // capture range variable for parallel subtest
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := store.Write("bucket", tc.key, []byte("data"), 0)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLocalStoreAssemble(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewLocalStore(tmpDir)

	bucket := "upload-bucket"
	finalKey := "final.bin"

	// Create parts
	part1 := "parts/1"
	part2 := "parts/2"
	data1 := []byte("part1")
	data2 := []byte("part2")

	require.NoError(t, store.Write(bucket, part1, data1, 0))
	require.NoError(t, store.Write(bucket, part2, data2, 0))

	// Assemble
	totalSize, err := store.Assemble(bucket, finalKey, []string{part1, part2})
	require.NoError(t, err)
	assert.Equal(t, int64(len(data1)+len(data2)), totalSize)

	// Verify final content
	content, _, err := store.Read(bucket, finalKey)
	require.NoError(t, err)
	assert.Equal(t, []byte("part1part2"), content)

	// Verify parts are deleted
	_, _, err = store.Read(bucket, part1)
	require.Error(t, err)
}

func TestLocalStoreReadFallbackToMtime(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewLocalStore(tmpDir)

	bucket := "test-bucket"
	key := "file.txt"
	data := []byte("data")

	require.NoError(t, store.Write(bucket, key, data, 1))

	path := filepath.Join(tmpDir, bucket, key)
	metaPath := path + ".meta"
	require.NoError(t, os.Remove(metaPath))

	mtime := time.Unix(1700000000, 0)
	require.NoError(t, os.Chtimes(path, mtime, mtime))

	_, ts, err := store.Read(bucket, key)
	require.NoError(t, err)
	assert.Equal(t, mtime.UnixNano(), ts)
}

func TestLocalStoreInvalidAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewLocalStore(tmpDir)

	err := store.Write("bucket", "/abs/path", []byte("data"), 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrInvalid)
}

func TestLocalStoreReadSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewLocalStore(tmpDir)
	bucket := "test-bucket"
	key := "largefile.bin"

	// Create a file larger than maxReadBytes (100MB)
	// Use WriteStream which doesn't have the size check
	largeData := make([]byte, maxReadBytes+1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err := store.Write(bucket, key, largeData, 0)
	require.NoError(t, err)

	// Read() should fail due to size limit
	_, _, err = store.Read(bucket, key)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file too large")

	// But ReadStream() should still work
	rc, _, err := store.ReadStream(bucket, key)
	require.NoError(t, err)
	defer rc.Close()

	// Verify we can read the data via stream
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, len(largeData), len(data))
}
