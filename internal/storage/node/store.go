// Package node implements storage node services.
package node

import (
	"bytes"
	"encoding/binary"
	stdlib_errors "errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	apperrors "github.com/poyrazk/thecloud/internal/errors"
)

// LocalStore manages file storage on the local disk.
type LocalStore struct {
	rootDir string
	nodeID  string
	mu      sync.RWMutex
}

const maxObjectSize = 5 * 1024 * 1024 * 1024 // 5 GB

// maxReadBytes is the maximum size for the Read() convenience method.
// Files larger than this should use ReadStream() to avoid memory exhaustion.
const maxReadBytes = 100 * 1024 * 1024 // 100 MB

// NewLocalStore initializes a new local storage backend.
// nodeID identifies this node in vector clocks.
func NewLocalStore(dataDir, nodeID string) (*LocalStore, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, err
	}
	return &LocalStore{rootDir: dataDir, nodeID: nodeID}, nil
}

// WriteStream saves data from a reader to disk.
// If vc is nil, creates a new VC with just this node's counter (first write).
// The VC is incremented for this node before writing.
func (s *LocalStore) WriteStream(bucket, key string, r io.Reader, vc VectorClock) (int64, error) {
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

	// Write metadata (versioned vector clock)
	metaPath := path + ".meta"
	if vc == nil {
		vc = NewVectorClock()
	}
	vc.Increment(s.nodeID)
	vc.prune()

	vcBytes, err := vc.Serialize()
	if err != nil {
		return n, err
	}
	// Version byte 0x01 = versioned VC format
	metaData := append([]byte{0x01}, vcBytes...)
	return n, os.WriteFile(metaPath, metaData, 0600)
}

// Write saves data to disk. Overwrites if exists. Uses vector clock.
func (s *LocalStore) Write(bucket, key string, data []byte, vc VectorClock) error {
	_, err := s.WriteStream(bucket, key, bytes.NewReader(data), vc)
	return err
}

// ReadStream retrieves a reader and vector clock for data on disk.
func (s *LocalStore) ReadStream(bucket, key string) (io.ReadCloser, VectorClock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path, err := s.getObjectPath(bucket, key)
	if err != nil {
		return nil, nil, err
	}

	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, err
	}

	// Read metadata
	var vc VectorClock
	metaPath := filepath.Clean(path + ".meta")
	metaBytes, err := os.ReadFile(metaPath)
	if err == nil && len(metaBytes) >= 1 {
		if metaBytes[0] == 0x01 && len(metaBytes) > 1 {
			// Versioned VC format (JSON)
			vc, _ = DeserializeVC(metaBytes[1:])
		}
	}
	if vc == nil {
		// Legacy format: 8-byte timestamp or no meta file.
		// Construct synthetic VC from timestamp or file mtime.
		vc = NewVectorClock()
		if len(metaBytes) == 8 {
			// Legacy timestamp — use as this node's counter
			vc[s.nodeID] = binary.LittleEndian.Uint64(metaBytes)
		} else if info, statErr := os.Stat(path); statErr == nil {
			// No meta file — fall back to mtime as counter

			vc[s.nodeID] = uint64(info.ModTime().UnixNano())
		}
	}

	return f, vc, nil
}

// Read retrieves data from disk and its vector clock.
// Warning: for large files (>maxReadBytes), use ReadStream() instead to avoid memory exhaustion.
func (s *LocalStore) Read(bucket, key string) ([]byte, VectorClock, error) {
	s.mu.RLock()
	path, err := s.getObjectPath(bucket, key)
	s.mu.RUnlock()
	if err != nil {
		return nil, nil, err
	}

	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, err
	}

	// Check size on the opened file to avoid TOCTOU with ReadStream's separate open
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	if info.Size() > maxReadBytes {
		_ = f.Close()
		return nil, nil, fmt.Errorf("file too large (%d bytes, max %d) for Read(), use ReadStream() for large files", info.Size(), maxReadBytes)
	}

	// Read metadata — same versioned format as ReadStream
	var vc VectorClock
	metaPath := filepath.Clean(path + ".meta")
	metaBytes, err := os.ReadFile(metaPath)
	if err == nil && len(metaBytes) >= 1 {
		if metaBytes[0] == 0x01 && len(metaBytes) > 1 {
			vc, _ = DeserializeVC(metaBytes[1:])
		}
	}
	if vc == nil {
		vc = NewVectorClock()
		if len(metaBytes) == 8 {
			vc[s.nodeID] = binary.LittleEndian.Uint64(metaBytes)
		} else {

			vc[s.nodeID] = uint64(info.ModTime().UnixNano())
		}
	}

	// Use LimitedReader to prevent reading more than maxReadBytes even if file grows
	lr := io.LimitedReader{R: f, N: maxReadBytes + 1}
	data, err := io.ReadAll(&lr)
	_ = f.Close()
	if err != nil {
		return nil, nil, err
	}

	return data, vc, nil
}

// Delete removes data from disk.
func (s *LocalStore) Delete(bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.getObjectPath(bucket, key)
	if err != nil {
		return err
	}

	if err := os.Remove(path + ".meta"); err != nil && !os.IsNotExist(err) {
		slog.Error("failed to remove meta file", "path", path+".meta", "error", err)
	}
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

	// Write final meta with VC
	metaPath := destPath + ".meta"
	vc := NewVectorClock()
	vc.Increment(s.nodeID)
	vc.prune()
	vcBytes, _ := vc.Serialize()
	metaData := append([]byte{0x01}, vcBytes...)
	_ = os.WriteFile(metaPath, metaData, 0600)

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

// ListKeys returns all keys stored in a bucket.
func (s *LocalStore) ListKeys(bucket string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bucketDir := filepath.Join(s.rootDir, filepath.Base(filepath.Clean(bucket)))

	entries, err := os.ReadDir(bucketDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Skip meta files
		name := entry.Name()
		if strings.HasSuffix(name, ".meta") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		keys = append(keys, name)
	}
	return keys, nil
}
