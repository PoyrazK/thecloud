// Package filesystem implements the local filesystem infrastructure adapters.
package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

// LocalFileStore stores objects on the local filesystem.
type LocalFileStore struct {
	basePath string
}

// NewLocalFileStore creates a LocalFileStore and ensures the base path exists.
func NewLocalFileStore(basePath string) (*LocalFileStore, error) {
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create storage base path: %w", err)
	}
	return &LocalFileStore{basePath: basePath}, nil
}

const (
	errTraversal  = "invalid path: traversal detected"
	maxObjectSize = 5 * 1024 * 1024 * 1024 // 5 GB - prevents memory exhaustion during writes
)

func (s *LocalFileStore) Write(ctx context.Context, bucket, key string, r io.Reader) (int64, error) {
	bucketPath := filepath.Join(s.basePath, filepath.Clean(bucket))
	filePath := filepath.Join(bucketPath, filepath.Clean(key))

	if !strings.HasPrefix(filePath, filepath.Clean(s.basePath)) {
		return 0, errors.New(errors.InvalidInput, errTraversal)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0750); err != nil {
		return 0, errors.Wrap(errors.Internal, "failed to create directories", err)
	}

	f, err := os.OpenFile(filepath.Clean(filePath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return 0, errors.Wrap(errors.Internal, "failed to create file", err)
	}
	defer func() { _ = f.Close() }()

	n, err := io.Copy(f, io.LimitReader(r, maxObjectSize))
	if err != nil {
		return 0, errors.Wrap(errors.Internal, "failed to write file", err)
	}

	return n, nil
}

func (s *LocalFileStore) Read(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	filePath := filepath.Join(s.basePath, filepath.Clean(bucket), filepath.Clean(key))
	if !strings.HasPrefix(filePath, filepath.Clean(s.basePath)) {
		return nil, errors.New(errors.InvalidInput, errTraversal)
	}
	// filepath.Clean is used above, but calling it again inside Open to satisfy gosec G304 explicitly
	f, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.ObjectNotFound, "object not found on disk")
		}
		return nil, errors.Wrap(errors.Internal, "failed to open file", err)
	}
	return f, nil
}

func (s *LocalFileStore) Delete(ctx context.Context, bucket, key string) error {
	filePath := filepath.Join(s.basePath, filepath.Clean(bucket), filepath.Clean(key))
	if !strings.HasPrefix(filePath, filepath.Clean(s.basePath)) {
		return errors.New(errors.InvalidInput, errTraversal)
	}
	err := os.Remove(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already gone
		}
		return errors.Wrap(errors.Internal, "failed to delete file", err)
	}
	return nil
}
func (s *LocalFileStore) GetClusterStatus(ctx context.Context) (*domain.StorageCluster, error) {
	// Local storage acts as a single-node "cluster"
	return &domain.StorageCluster{
		Nodes: []domain.StorageNode{
			{
				ID:      "local-node",
				Address: "localhost",
				Status:  "alive",
			},
		},
	}, nil
}

func (s *LocalFileStore) Assemble(ctx context.Context, bucket, key string, parts []string) (int64, error) {
	bucketPath := filepath.Join(s.basePath, filepath.Clean(bucket))
	destPath := filepath.Join(bucketPath, filepath.Clean(key))

	if !strings.HasPrefix(destPath, filepath.Clean(s.basePath)) {
		return 0, errors.New(errors.InvalidInput, errTraversal)
	}

	// G304: Explicitly clean path before creation
	f, err := os.Create(filepath.Clean(destPath))
	if err != nil {
		return 0, errors.Wrap(errors.Internal, "failed to create dest file", err)
	}
	defer func() { _ = f.Close() }()

	var totalSize int64
	for _, partKey := range parts {
		partPath := filepath.Join(bucketPath, filepath.Clean(partKey))
		if !strings.HasPrefix(partPath, filepath.Clean(s.basePath)) {
			return 0, errors.New(errors.InvalidInput, errTraversal)
		}
		// G304: Explicitly clean path before opening
		pf, err := os.Open(filepath.Clean(partPath))
		if err != nil {
			return 0, errors.Wrap(errors.Internal, "failed to open part file", err)
		}
		partInfo, err := pf.Stat()
		if err != nil {
			_ = pf.Close()
			return 0, errors.Wrap(errors.Internal, "failed to stat part file", err)
		}
		partSize := partInfo.Size()
		if totalSize+partSize > maxObjectSize {
			_ = pf.Close()
			_ = os.Remove(destPath)
			return totalSize + partSize, errors.New(errors.ObjectTooLarge, fmt.Sprintf("assembled object exceeds max size: %d bytes (max %d)", totalSize+partSize, maxObjectSize))
		}
		n, err := io.Copy(f, pf)
		_ = pf.Close()
		if err != nil {
			return 0, errors.Wrap(errors.Internal, "failed to copy part", err)
		}
		totalSize += n
		if totalSize > maxObjectSize {
			_ = os.Remove(partPath)
			return totalSize, errors.New(errors.ObjectTooLarge, fmt.Sprintf("assembled object exceeds max size: %d bytes (max %d)", totalSize, maxObjectSize))
		}
		_ = os.Remove(partPath) // Cleanup part
	}

	return totalSize, nil
}
