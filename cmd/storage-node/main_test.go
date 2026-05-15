package main

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRun_TLSEnabledMissingCertFile(t *testing.T) {
	tmpDir := t.TempDir()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	os.Args = []string{
		"storage-node",
		"--tls=true",
		"--tls-cert=",
		"--tls-key=testdata/tls/test-key.pem",
		"--port=0",
		"--data-dir=" + tmpDir,
	}

	err := run()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--tls-cert and --tls-key are required when --tls is enabled")
}

func TestRun_TLSEnabledMissingKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	os.Args = []string{
		"storage-node",
		"--tls=true",
		"--tls-cert=testdata/tls/test-cert.pem",
		"--tls-key=",
		"--port=0",
		"--data-dir=" + tmpDir,
	}

	err := run()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--tls-cert and --tls-key are required when --tls is enabled")
}

func TestRun_TLSEnabledValidCertAndKey(t *testing.T) {
	tmpDir := t.TempDir()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	os.Args = []string{
		"storage-node",
		"--tls=true",
		"--tls-cert=testdata/tls/test-cert.pem",
		"--tls-key=testdata/tls/test-key.pem",
		"--port=0",
		"--data-dir=" + tmpDir,
	}

	// Start run in background with a timeout so we don't block forever.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- run()
	}()

	select {
	case err := <-errCh:
		// Server returned — verify it didn't fail on cert loading
		if err != nil {
			require.NotContains(t, err.Error(), "failed to load TLS cert")
			require.NotContains(t, err.Error(), "failed to read TLS CA cert")
		}
	case <-ctx.Done():
		// Timeout — this is fine for a "valid cert" test; we just proved
		// the server started and kept running (TLS cert loaded OK).
	}
}

func TestRun_TLSDisabledNoCertRequired(t *testing.T) {
	tmpDir := t.TempDir()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	os.Args = []string{
		"storage-node",
		"--tls=false",
		"--port=0",
		"--data-dir=" + tmpDir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- run()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			require.NotContains(t, err.Error(), "--tls-cert and --tls-key are required")
		}
	case <-ctx.Done():
		// Timeout means server started fine without TLS — good
	}
}