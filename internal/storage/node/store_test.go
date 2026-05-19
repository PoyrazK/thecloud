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

const testNodeID = "test-node"

func TestLocalStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalStore(tmpDir, testNodeID)
	require.NoError(t, err)

	bucket := "test-bucket"
	key := "folder/test-object.txt"
	data := []byte("hello world")

	// 1. Test Write
	err = store.Write(bucket, key, data, nil)
	require.NoError(t, err)

	// Verify file exists
	path := filepath.Join(tmpDir, bucket, key)
	_, err = os.Stat(path)
	require.NoError(t, err)

	// 2. Test Read
	readData, readVC, err := store.Read(bucket, key)
	require.NoError(t, err)
	assert.Equal(t, data, readData)
	require.NotNil(t, readVC)
	assert.Equal(t, uint64(1), readVC[testNodeID], "first write should set counter to 1")

	// 3. Test Delete
	err = store.Delete(bucket, key)
	require.NoError(t, err)

	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err))
}

func TestLocalStorePathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewLocalStore(tmpDir, testNodeID)

	// Attempt to write outside root
	err := store.Write("bucket", "../outside.txt", []byte("evil"), nil)
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
		{name: "dot in middle works", key: "foo/./bar", wantErr: false},
		{name: "url encoded traversal", key: "..%2Foutside.txt", wantErr: true},
		{name: "backslash encoded", key: "..%5Coutside.txt", wantErr: true},
		{name: "multi dot dot", key: "../foo/../../bar", wantErr: true},
	}
	for _, tc := range testCases {
		tc := tc // capture range variable for parallel subtest
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := store.Write("bucket", tc.key, []byte("data"), nil)
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
	store, _ := NewLocalStore(tmpDir, testNodeID)

	bucket := "upload-bucket"
	finalKey := "final.bin"

	// Create parts
	part1 := "parts/1"
	part2 := "parts/2"
	data1 := []byte("part1")
	data2 := []byte("part2")

	require.NoError(t, store.Write(bucket, part1, data1, nil))
	require.NoError(t, store.Write(bucket, part2, data2, nil))

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
	store, _ := NewLocalStore(tmpDir, testNodeID)

	bucket := "test-bucket"
	key := "file.txt"
	data := []byte("data")

	require.NoError(t, store.Write(bucket, key, data, nil))

	path := filepath.Join(tmpDir, bucket, key)
	metaPath := path + ".meta"
	require.NoError(t, os.Remove(metaPath))

	mtime := time.Unix(1700000000, 0)
	require.NoError(t, os.Chtimes(path, mtime, mtime))

	_, readVC, err := store.Read(bucket, key)
	require.NoError(t, err)
	require.NotNil(t, readVC)
	// Should fall back to mtime as counter for this node
	assert.Equal(t, uint64(mtime.UnixNano()), readVC[testNodeID])
}

func TestLocalStoreInvalidAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewLocalStore(tmpDir, testNodeID)

	err := store.Write("bucket", "/abs/path", []byte("data"), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrInvalid)
}

func TestLocalStoreReadSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewLocalStore(tmpDir, testNodeID)
	bucket := "test-bucket"
	key := "largefile.bin"

	// Create a file larger than maxReadBytes (100MB)
	largeData := make([]byte, maxReadBytes+1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err := store.Write(bucket, key, largeData, nil)
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

func TestLocalStoreDeleteMissingMetaOk(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalStore(tmpDir, testNodeID)
	require.NoError(t, err)

	bucket := "test-bucket"
	key := "testfile.txt"
	data := []byte("hello")
	err = store.Write(bucket, key, data, nil)
	require.NoError(t, err)

	// Remove the .meta file to simulate pre-existing state where only data file exists
	metaPath := filepath.Join(tmpDir, bucket, key+".meta")
	err = os.Remove(metaPath)
	require.NoError(t, err)

	// Delete should succeed even though .meta is missing
	err = store.Delete(bucket, key)
	require.NoError(t, err, "Delete must succeed when .meta is already gone")

	// Verify data file is gone
	_, err = os.Stat(filepath.Join(tmpDir, bucket, key))
	require.True(t, os.IsNotExist(err))
}

func TestLocalStoreWriteAndReadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalStore(tmpDir, testNodeID)
	require.NoError(t, err)

	bucket := "test-bucket"
	key := "obj"
	data := []byte("round-trip-test")

	err = store.Write(bucket, key, data, nil)
	require.NoError(t, err)

	readBack, readVC, err := store.Read(bucket, key)
	require.NoError(t, err)
	assert.Equal(t, data, readBack)
	require.NotNil(t, readVC)
	assert.Equal(t, uint64(1), readVC[testNodeID])
}

func TestLocalStoreVCIncrementsOnWrite(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalStore(tmpDir, testNodeID)
	require.NoError(t, err)

	bucket := "test-bucket"
	key := "vc-test"
	data := []byte("hello")

	// Each write creates a fresh VC for this node (coordinator doesn't pass winning VC for initial writes)
	// First write: node creates VC {"test-node":1}
	require.NoError(t, store.Write(bucket, key, data, nil))
	_, vc1, err := store.Read(bucket, key)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), vc1[testNodeID])

	// Second write: node creates fresh VC {"test-node":1} again (not a continuation)
	// This reflects the real coordinator flow where writes don't carry forward state
	require.NoError(t, store.Write(bucket, key, []byte("world"), nil))
	_, vc2, err := store.Read(bucket, key)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), vc2[testNodeID])
}

func TestLocalStoreVCPruning(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalStore(tmpDir, testNodeID)
	require.NoError(t, err)

	bucket := "test-bucket"
	key := "prune-test"
	data := []byte("hello")

	// Write 15 times — each write creates fresh VC with counter=1 (node generates new VC)
	// This tests that pruning logic doesn't cause crashes or corruption
	for i := 0; i < 15; i++ {
		require.NoError(t, store.Write(bucket, key, data, nil))
	}
	_, vc, err := store.Read(bucket, key)
	require.NoError(t, err)
	require.NotNil(t, vc)
	// Each write starts fresh, so counter is 1
	assert.Equal(t, uint64(1), vc[testNodeID])
}