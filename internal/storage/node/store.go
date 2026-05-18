// Package node implements storage node services.
package node

import (
	"bytes"
	"encoding/binary"
	stdlib_errors "errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	apperrors "github.com/poyrazk/thecloud/internal/errors"
)

// LocalStore manages file storage on the local disk.
type LocalStore struct {
	rootDir string
	mu      sync.RWMutex
}

const maxObjectSize = 5 * 1024 * 1024 * 1024 // 5 GB

// maxReadBytes is the maximum size for the Read() convenience method.
// Files larger than this should use ReadStream() to avoid memory exhaustion.
const maxReadBytes = 100 * 1024 * 1024 // 100 MB

// NewLocalStore initializes a new local storage backend.
func NewLocalStore(dataDir string) (*LocalStore, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, err
	}
	return &LocalStore{rootDir: dataDir}, nil
}

// WriteStream saves data from a reader to disk.
func (s *LocalStore) WriteStream(bucket, key string, r io.Reader, timestamp int64) (int64, error) {
	s.mu.RLock()
	path, err := s.getObjectPath(bucket, key)
	s.mu.RUnlock()
	if err != nil {
		return 0, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return 0, err
	}

	// Use temporary file for atomic write
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(filepath.Clean(tmpPath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return 0, err
	}

	n, copyErr := io.Copy(f, io.LimitReader(r, maxObjectSize))
	closeErr := f.Close()
	if copyErr != nil && !stdlib_errors.Is(copyErr, io.EOF) {
		_ = os.Remove(tmpPath)
		return n, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return n, closeErr
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return n, err
	}

	// Write metadata (timestamp)
	metaPath := path + ".meta"
	buf := make([]byte, 8)

	var uTimestamp uint64
	if timestamp < 0 {
		uTimestamp = 0
	} else {
		uTimestamp = uint64(timestamp)
	}

	binary.LittleEndian.PutUint64(buf, uTimestamp)
	return n, os.WriteFile(metaPath, buf, 0600)
}

// Write saves data to disk. Overwrites if exists.
func (s *LocalStore) Write(bucket, key string, data []byte, timestamp int64) error {
	_, err := s.WriteStream(bucket, key, bytes.NewReader(data), timestamp)
	return err
}

// ReadStream retrieves a reader for data on disk.
func (s *LocalStore) ReadStream(bucket, key string) (io.ReadCloser, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path, err := s.getObjectPath(bucket, key)
	if err != nil {
		return nil, 0, err
	}

	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, 0, err
	}

	// Read metadata
	var timestamp int64
	metaPath := filepath.Clean(path + ".meta")
	metaBytes, err := os.ReadFile(metaPath)
	if err == nil && len(metaBytes) >= 8 {
		uVal := binary.LittleEndian.Uint64(metaBytes)
		if uVal > math.MaxInt64 {
			timestamp = math.MaxInt64
		} else {
			timestamp = int64(uVal)
		}
	} else {
		// Fallback to file mtime if meta missing
		info, statErr := os.Stat(path)
		if statErr == nil {
			timestamp = info.ModTime().UnixNano()
		}
	}

	return f, timestamp, nil
}

// Read retrieves data from disk.
// Warning: for large files (>maxReadBytes), use ReadStream() instead to avoid memory exhaustion.
func (s *LocalStore) Read(bucket, key string) ([]byte, int64, error) {
	s.mu.RLock()
	path, err := s.getObjectPath(bucket, key)
	s.mu.RUnlock()
	if err != nil {
		return nil, 0, err
	}

	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, 0, err
	}

	// Check size on the opened file to avoid TOCTOU with ReadStream's separate open
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	if info.Size() > maxReadBytes {
		_ = f.Close()
		return nil, 0, fmt.Errorf("file too large (%d bytes, max %d) for Read(), use ReadStream() for large files", info.Size(), maxReadBytes)
	}

	// Read metadata
	var timestamp int64
	metaPath := filepath.Clean(path + ".meta")
	metaBytes, err := os.ReadFile(metaPath)
	if err == nil && len(metaBytes) >= 8 {
		uVal := binary.LittleEndian.Uint64(metaBytes)
		if uVal > math.MaxInt64 {
			timestamp = math.MaxInt64
		} else {
			timestamp = int64(uVal)
		}
	} else {
		timestamp = info.ModTime().UnixNano()
	}

	// Use LimitedReader to prevent reading more than maxReadBytes even if file grows
	lr := io.LimitedReader{R: f, N: maxReadBytes + 1}
	data, err := io.ReadAll(&lr)
	_ = f.Close()
	if err != nil {
		return nil, 0, err
	}

	return data, timestamp, nil
}

// Delete removes data from disk.
func (s *LocalStore) Delete(bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.getObjectPath(bucket, key)
	if err != nil {
		return err
	}

	_ = os.Remove(path + ".meta")
	return os.Remove(path)
}

// Assemble combines multiple parts into a single object.
func (s *LocalStore) Assemble(bucket, key string, parts []string) (int64, error) {
	s.mu.RLock()
	destPath, err := s.getObjectPath(bucket, key)
	s.mu.RUnlock()
	if err != nil {
		return 0, err
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return 0, err
	}

	tmpPath := destPath + ".tmp"
	f, err := os.OpenFile(filepath.Clean(tmpPath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return 0, err
	}

	var totalSize int64
	var assembleErr error
	for _, partKey := range parts {
		s.mu.RLock()
		partPath, err := s.getObjectPath(bucket, partKey)
		s.mu.RUnlock()
		if err != nil {
			assembleErr = err
			break
		}

		pf, err := os.Open(filepath.Clean(partPath))
		if err != nil {
			assembleErr = err
			break
		}
		partInfo, err := pf.Stat()
		if err != nil {
			_ = pf.Close()
			assembleErr = err
			break
		}
		partSize := partInfo.Size()
		if totalSize+partSize > maxObjectSize {
			_ = pf.Close()
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return totalSize + partSize, apperrors.New(apperrors.ObjectTooLarge, fmt.Sprintf("assembled object exceeds max size: %d bytes (max %d)", totalSize+partSize, maxObjectSize))
		}
		n, err := io.Copy(f, pf)
		_ = pf.Close()
		if err != nil {
			assembleErr = err
			break
		}
		totalSize += n
		if totalSize > maxObjectSize {
			_ = f.Close()
			_ = os.Remove(tmpPath)
			return totalSize, apperrors.New(apperrors.ObjectTooLarge, fmt.Sprintf("assembled object exceeds max size: %d bytes (max %d)", totalSize, maxObjectSize))
		}
	}

	closeErr := f.Close()
	if assembleErr != nil {
		_ = os.Remove(tmpPath)
		return 0, assembleErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return 0, closeErr
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, err
	}

	// Cleanup parts after successful rename
	for _, partKey := range parts {
		if partPath, err := s.getObjectPath(bucket, partKey); err == nil {
			_ = os.Remove(partPath)
			_ = os.Remove(partPath + ".meta")
		}
	}

	// Write final meta with current timestamp
	metaPath := destPath + ".meta"
	buf := make([]byte, 8)

	now := time.Now().UnixNano()
	var uNow uint64
	if now < 0 {
		uNow = 0
	} else {
		uNow = uint64(now)
	}

	binary.LittleEndian.PutUint64(buf, uNow)
	_ = os.WriteFile(metaPath, buf, 0600)

	return totalSize, nil
}

func (s *LocalStore) getObjectPath(bucket, key string) (string, error) {
	// Clean the inputs
	cleanBucket := filepath.Base(filepath.Clean(bucket))
	cleanKey := filepath.Clean(key)

	if filepath.IsAbs(cleanKey) {
		return "", os.ErrInvalid
	}

	bucketDir := filepath.Join(s.rootDir, cleanBucket)
	fullPath := filepath.Join(bucketDir, cleanKey)

	// Verify it's within bucketDir (strict isolation)
	// Must be a child path - not the directory itself, not parent
	rel, err := filepath.Rel(bucketDir, fullPath)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", os.ErrPermission
	}

	return fullPath, nil
}
