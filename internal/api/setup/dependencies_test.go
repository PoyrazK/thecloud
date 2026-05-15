package setup

import (
	"log/slog"
	"os"
	"testing"

	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitIdentityServices(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := &platform.Config{}
	repos := &Repositories{}

	// This will likely need mocks in Repositories
	// But let's see if we can test the branching logic
	svc := initIdentityServices(ServiceConfig{
		Config: cfg,
		Repos:  repos,
		Logger: logger,
	}, nil, nil)

	assert.NotNil(t, svc)
}

func TestInitRBACServices(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := &platform.Config{}
	repos := &Repositories{}

	svc := initRBACServices(ServiceConfig{
		Config: cfg,
		Repos:  repos,
		Logger: logger,
	})

	assert.NotNil(t, svc)
}

func TestBuildStorageDialOpts_InsecureDefault(t *testing.T) {
	cfg := &platform.Config{StorageTLSEnabled: false}
	dialOpts, err := buildStorageDialOpts(cfg)
	require.NoError(t, err)
	require.Len(t, dialOpts, 1)
	// Should use insecure credentials - just verify it doesn't error
}

func TestBuildStorageDialOpts_TLSEnabled(t *testing.T) {
	cfg := &platform.Config{
		StorageTLSEnabled: true,
		StorageTLSCertFile: "testdata/tls/test-cert.pem",
		StorageTLSKeyFile:  "testdata/tls/test-key.pem",
	}
	dialOpts, err := buildStorageDialOpts(cfg)
	require.NoError(t, err)
	require.Len(t, dialOpts, 1)
}

func TestBuildStorageDialOpts_TLSEnabledWithCA(t *testing.T) {
	cfg := &platform.Config{
		StorageTLSEnabled:   true,
		StorageTLSCertFile:  "testdata/tls/test-cert.pem",
		StorageTLSKeyFile:   "testdata/tls/test-key.pem",
		StorageTLSCACertFile: "testdata/tls/ca-cert.pem",
	}
	dialOpts, err := buildStorageDialOpts(cfg)
	require.NoError(t, err)
	require.Len(t, dialOpts, 1)
}

func TestBuildStorageDialOpts_TLSEnabledWithSkipVerify(t *testing.T) {
	cfg := &platform.Config{
		StorageTLSEnabled:   true,
		StorageTLSCertFile:  "testdata/tls/test-cert.pem",
		StorageTLSKeyFile:   "testdata/tls/test-key.pem",
		StorageTLSSkipVerify: true,
	}
	dialOpts, err := buildStorageDialOpts(cfg)
	require.NoError(t, err)
	require.Len(t, dialOpts, 1)
}
