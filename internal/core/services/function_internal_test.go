package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSecretSvc is a test double for SecretService.
type testSecretSvc struct {
	val string
	err error
}

// testComputeBackend is a minimal test double for ComputeBackend.
type testComputeBackend struct{}

func (t *testComputeBackend) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	return "", nil, nil
}
func (t *testComputeBackend) StartInstance(ctx context.Context, id string) error  { return nil }
func (t *testComputeBackend) StopInstance(ctx context.Context, id string) error   { return nil }
func (t *testComputeBackend) DeleteInstance(ctx context.Context, id string) error { return nil }
func (t *testComputeBackend) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, nil
}
func (t *testComputeBackend) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, nil
}
func (t *testComputeBackend) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	return 0, nil
}
func (t *testComputeBackend) GetInstanceIP(ctx context.Context, id string) (string, error) {
	return "", nil
}
func (t *testComputeBackend) GetConsoleURL(ctx context.Context, id string) (string, error) {
	return "", nil
}
func (t *testComputeBackend) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", nil
}
func (t *testComputeBackend) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	return "", nil, nil
}
func (t *testComputeBackend) WaitTask(ctx context.Context, id string) (int64, error) { return 0, nil }
func (t *testComputeBackend) CreateNetwork(ctx context.Context, name string) (string, error) {
	return "", nil
}
func (t *testComputeBackend) DeleteNetwork(ctx context.Context, id string) error { return nil }
func (t *testComputeBackend) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	return "", "", nil
}
func (t *testComputeBackend) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	return "", nil
}
func (t *testComputeBackend) Ping(ctx context.Context) error { return nil }
func (t *testComputeBackend) Type() string                   { return "test" }
func (t *testComputeBackend) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	return nil
}
func (t *testComputeBackend) CreateSnapshot(ctx context.Context, id, name string) error  { return nil }
func (t *testComputeBackend) RestoreSnapshot(ctx context.Context, id, name string) error { return nil }
func (t *testComputeBackend) DeleteSnapshot(ctx context.Context, id, name string) error  { return nil }
func (t *testComputeBackend) PauseInstance(ctx context.Context, id string) error         { return nil }
func (t *testComputeBackend) ResumeInstance(ctx context.Context, id string) error        { return nil }
func (t *testComputeBackend) ResetCircuitBreaker()                                       {}

// compile-time check that testComputeBackend satisfies ports.ComputeBackend
var _ ports.ComputeBackend = (*testComputeBackend)(nil)

func (t *testSecretSvc) CreateSecret(ctx context.Context, name, value, desc string) (*domain.Secret, error) {
	return &domain.Secret{ID: uuid.New(), Name: name}, nil
}
func (t *testSecretSvc) GetSecret(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	return &domain.Secret{ID: id}, nil
}
func (t *testSecretSvc) GetSecretByName(ctx context.Context, name string) (*domain.Secret, error) {
	if t.err != nil {
		return nil, t.err
	}
	return &domain.Secret{ID: uuid.New(), Name: name, EncryptedValue: t.val}, nil
}
func (t *testSecretSvc) ListSecrets(ctx context.Context) ([]*domain.Secret, error) { return nil, nil }
func (t *testSecretSvc) DeleteSecret(ctx context.Context, id uuid.UUID) error      { return nil }
func (t *testSecretSvc) Encrypt(ctx context.Context, userID uuid.UUID, plain string) (string, error) {
	return plain, nil
}
func (t *testSecretSvc) Decrypt(ctx context.Context, userID uuid.UUID, cipher string) (string, error) {
	return cipher, nil
}

// compile-time check that testSecretSvc satisfies ports.SecretService
var _ ports.SecretService = (*testSecretSvc)(nil)

func TestFunctionService_InternalExtract(t *testing.T) {
	s := &FunctionService{logger: slog.Default()}
	tmpDir, _ := os.MkdirTemp("", "test-extract-*")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	t.Run("extractZip successful", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		f, _ := zw.Create("hello.txt")
		_, _ = f.Write([]byte("hello world"))
		_ = zw.Close()

		err := s.extractZip(bytes.NewReader(buf.Bytes()), tmpDir)
		require.NoError(t, err)

		content, _ := os.ReadFile(filepath.Join(tmpDir, "hello.txt"))
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("extractZip traversal attempt", func(t *testing.T) {
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		// Zip file with relative path attempting traversal
		_, err := zw.Create("../traversal.txt")
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		err = s.extractZip(bytes.NewReader(buf.Bytes()), tmpDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid file path")
	})

	// Regression for #237: simple HasPrefix checks miss several traversal
	// payload classes — absolute paths, real `..` segments, mixed separators
	// (Windows-authored archives), and NUL bytes. Notably we do NOT include
	// `..././../etc/passwd` here: filepath.Clean reduces it to `etc/passwd`,
	// a perfectly local relative path that is meant to extract as
	// `<tmpDir>/etc/passwd` rather than escape. The hardening targets real
	// escape attempts, not benign canonicalization.
	traversalPayloads := []string{
		"foo/../../bar.txt",
		"./../etc/passwd",
		"/abs/etc/passwd",      // absolute path
		"..\\windows\\sys.ini", // backslash separator
		"a\x00b.txt",           // NUL byte
	}
	for _, name := range traversalPayloads {
		name := name
		t.Run("extractZip rejects "+name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			zw := zip.NewWriter(buf)
			_, err := zw.Create(name)
			require.NoError(t, err)
			require.NoError(t, zw.Close())

			err = s.extractZip(bytes.NewReader(buf.Bytes()), tmpDir)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid file path")
		})
	}
}

func TestFunctionService_BuildTaskOptions(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		s := &FunctionService{logger: slog.Default()}
		f := &domain.Function{
			Runtime:  "nodejs20",
			Handler:  "index.handler",
			MemoryMB: 256,
		}
		tmpDir := "/tmp/fn-123"
		payload := []byte(`{"foo":"bar"}`)

		opts := s.buildTaskOptions(context.Background(), f, tmpDir, payload)
		assert.Equal(t, "node:20-alpine", opts.Image)
		assert.Equal(t, []string{"node", "./index.handler"}, opts.Command)
		assert.Contains(t, opts.Env[0], "PAYLOAD=")
		assert.Equal(t, int64(256), opts.MemoryMB)
	})

	t.Run("withSecretRef", func(t *testing.T) {
		secretSvc := &testSecretSvc{val: "s3cr3t"}
		s := &FunctionService{
			secretSvc: secretSvc,
			logger:    slog.Default(),
		}
		f := &domain.Function{
			Runtime:  "nodejs20",
			Handler:  "index.handler",
			MemoryMB: 256,
			EnvVars:  []*domain.EnvVar{{Key: "API_KEY", SecretRef: "@my-secret"}},
		}

		opts := s.buildTaskOptions(context.Background(), f, "/tmp/test", []byte("{}"))

		var apiKeyEnv string
		for _, e := range opts.Env {
			if strings.HasPrefix(e, "API_KEY=") {
				apiKeyEnv = e
				break
			}
		}
		assert.NotEmpty(t, apiKeyEnv)
		assert.Equal(t, "API_KEY=s3cr3t", apiKeyEnv)
	})

	t.Run("secretResolutionFailure", func(t *testing.T) {
		secretSvc := &testSecretSvc{err: errors.New("secret not found")}
		s := &FunctionService{
			secretSvc: secretSvc,
			logger:    slog.Default(),
		}
		f := &domain.Function{
			Runtime:  "nodejs20",
			Handler:  "index.handler",
			MemoryMB: 256,
			EnvVars:  []*domain.EnvVar{{Key: "API_KEY", SecretRef: "@missing"}},
		}

		opts := s.buildTaskOptions(context.Background(), f, "/tmp/test", []byte("{}"))

		for _, e := range opts.Env {
			assert.NotContains(t, e, "API_KEY")
		}
	})
}

func TestFunctionService_CaptureInvocationResults(t *testing.T) {
	s := &FunctionService{logger: slog.Default(), compute: &testComputeBackend{}}

	t.Run("non-zero exit code", func(t *testing.T) {
		i := &domain.Invocation{ID: uuid.New(), Status: "RUNNING"}
		s.captureInvocationResults(i, "task-1", 127, nil)
		assert.Equal(t, "FAILED", i.Status)
		assert.Equal(t, 127, i.StatusCode)
	})

	t.Run("error during wait", func(t *testing.T) {
		i := &domain.Invocation{ID: uuid.New(), Status: "RUNNING"}
		s.captureInvocationResults(i, "task-1", 0, errors.New("connection lost"))
		assert.Equal(t, "FAILED", i.Status)
		assert.Contains(t, i.Logs, "connection lost")
	})

	t.Run("timeout", func(t *testing.T) {
		i := &domain.Invocation{ID: uuid.New(), Status: "RUNNING"}
		s.captureInvocationResults(i, "task-1", 0, context.DeadlineExceeded)
		assert.Equal(t, "FAILED", i.Status)
		assert.Contains(t, i.Logs, "timed out")
	})

	t.Run("success", func(t *testing.T) {
		i := &domain.Invocation{ID: uuid.New(), Status: "RUNNING"}
		s.captureInvocationResults(i, "task-1", 0, nil)
		assert.Equal(t, "SUCCESS", i.Status)
		assert.Equal(t, 0, i.StatusCode)
	})

	t.Run("logs sanitized", func(t *testing.T) {
		i := &domain.Invocation{ID: uuid.New(), Status: "RUNNING"}
		s.captureInvocationResults(i, "task-1", 0, nil)
		assert.NotContains(t, i.Logs, "\x00")
	})
}

func TestFunctionService_NormalizeHandler(t *testing.T) {
	s := &FunctionService{}

	tests := []struct {
		name     string
		runtime  string
		handler  string
		expected string
	}{
		{"Node no prefix", "nodejs20", "index", "./index.js"},
		{"Node with dot", "nodejs20", "main.handler", "./main.handler"},
		{"Node with prefix", "nodejs20", "./app.js", "./app.js"},
		{"Python no prefix", "python312", "main", "./main.py"},
		{"Go no prefix", "go122", "main", "main.go"},
		{"Unsupported runtime", "unknown", "main", "main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.normalizeHandler(tt.runtime, tt.handler)
			assert.Equal(t, tt.expected, result)
		})
	}
}
