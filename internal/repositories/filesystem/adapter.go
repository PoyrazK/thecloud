package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/poyrazk/thecloud/internal/errors"
)

type LocalFileStore struct {
	basePath string
}

func NewLocalFileStore(basePath string) (*LocalFileStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage base path: %w", err)
	}
	return &LocalFileStore{basePath: basePath}, nil
}

func (s *LocalFileStore) Write(ctx context.Context, bucket, key string, r io.Reader) (int64, error) {
	bucketPath := filepath.Join(s.basePath, bucket)
	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		return 0, errors.Wrap(errors.Internal, "failed to create bucket directory", err)
	}

	filePath := filepath.Join(bucketPath, key)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return 0, errors.Wrap(errors.Internal, "failed to create directories", err)
	}

	f, err := os.Create(filePath)
	if err != nil {
		return 0, errors.Wrap(errors.Internal, "failed to create file", err)
	}
	defer func() { _ = f.Close() }()

	n, err := io.Copy(f, r)
	if err != nil {
		return 0, errors.Wrap(errors.Internal, "failed to write file", err)
	}

	return n, nil
}

func (s *LocalFileStore) Read(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	filePath := filepath.Join(s.basePath, bucket, key)
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New(errors.ObjectNotFound, "object not found on disk")
		}
		return nil, errors.Wrap(errors.Internal, "failed to open file", err)
	}
	return f, nil
}

func (s *LocalFileStore) Delete(ctx context.Context, bucket, key string) error {
	filePath := filepath.Join(s.basePath, bucket, key)
	err := os.Remove(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already gone
		}
		return errors.Wrap(errors.Internal, "failed to delete file", err)
	}
	return nil
}
