// Package sshutil provides SSH helpers for remote operations.
package sshutil

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// envInsecureHostKey, when set to "1"/"true", makes NewClientWithKey accept any host key.
// This exists only as an explicit escape hatch for tests and one-shot bootstrap flows.
// Production callers should provide their own HostKeyCallback (e.g. via known_hosts).
const envInsecureHostKey = "THECLOUD_SSH_INSECURE_IGNORE_HOSTKEY"

// envKnownHostsPath overrides the default ~/.ssh/known_hosts lookup.
const envKnownHostsPath = "THECLOUD_SSH_KNOWN_HOSTS"

// Client represents an SSH client for remote execution.
type Client struct {
	Host            string
	User            string
	Auth            []ssh.AuthMethod
	HostKeyCallback ssh.HostKeyCallback
}

// NewClientWithKey constructs an SSH client using a private key.
// The HostKeyCallback is derived in order from:
//  1. THECLOUD_SSH_INSECURE_IGNORE_HOSTKEY=1 → ssh.InsecureIgnoreHostKey() (test/bootstrap only)
//  2. THECLOUD_SSH_KNOWN_HOSTS → known_hosts file at that path
//  3. ~/.ssh/known_hosts if it exists
//
// If none of the above yield a callback, an error is returned so callers cannot
// silently fall back to InsecureIgnoreHostKey.
func NewClientWithKey(host, user, privateKey string) (*Client, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	cb, err := resolveHostKeyCallback()
	if err != nil {
		return nil, err
	}

	return &Client{
		Host: host,
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: cb,
	}, nil
}

// NewClientWithKeyAndCallback is the explicit constructor for callers that
// already have a HostKeyCallback (e.g. a known_hosts-backed callback they
// share with other SSH consumers).
func NewClientWithKeyAndCallback(host, user, privateKey string, cb ssh.HostKeyCallback) (*Client, error) {
	if cb == nil {
		return nil, fmt.Errorf("host key callback is required")
	}
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	return &Client{
		Host:            host,
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: cb,
	}, nil
}

func resolveHostKeyCallback() (ssh.HostKeyCallback, error) {
	if v := os.Getenv(envInsecureHostKey); v == "1" || v == "true" {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	candidates := make([]string, 0, 2)
	if p := os.Getenv(envKnownHostsPath); p != "" {
		candidates = append(candidates, p)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".ssh", "known_hosts"))
	}
	for _, p := range candidates {
		if _, statErr := os.Stat(p); statErr != nil {
			continue
		}
		cb, err := knownhosts.New(p)
		if err != nil {
			return nil, fmt.Errorf("failed to load known_hosts %q: %w", p, err)
		}
		return cb, nil
	}

	return nil, fmt.Errorf("ssh host key verification not configured: set %s to a known_hosts file or %s=1 to bypass (insecure)", envKnownHostsPath, envInsecureHostKey)
}

// Run executes a command and returns its output.
func (c *Client) Run(ctx context.Context, cmd string) (string, error) {
	config := &ssh.ClientConfig{
		User:            c.User,
		Auth:            c.Auth,
		HostKeyCallback: c.HostKeyCallback,
		Timeout:         10 * time.Second,
	}

	addr := c.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "22")
	}

	d := net.Dialer{Timeout: config.Timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("failed to dial: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()
		return "", fmt.Errorf("failed to create ssh client conn: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	// Capture stdout and stderr separately to avoid race conditions on bytes.Buffer
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		// Combine output for error context
		output := stdout.String() + stderr.String()
		return output, fmt.Errorf("failed to run command %q: %w", cmd, err)
	}

	return stdout.String() + stderr.String(), nil
}

// WriteFile writes content to a remote file.
func (c *Client) WriteFile(ctx context.Context, path string, content []byte, mode string) error {
	config := &ssh.ClientConfig{
		User:            c.User,
		Auth:            c.Auth,
		HostKeyCallback: c.HostKeyCallback,
		Timeout:         10 * time.Second,
	}

	addr := c.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "22")
	}

	d := net.Dialer{Timeout: config.Timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to create ssh client conn: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	w, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)
		defer func() { _ = w.Close() }()

		filename := filepath.Base(path)
		if _, err := fmt.Fprintf(w, "C%s %d %s\n", mode, len(content), filename); err != nil {
			errCh <- fmt.Errorf("failed to write SCP header: %w", err)
			return
		}
		if _, err := w.Write(content); err != nil {
			errCh <- fmt.Errorf("failed to write content: %w", err)
			return
		}
		if _, err := fmt.Fprint(w, "\x00"); err != nil {
			errCh <- fmt.Errorf("failed to write null byte: %w", err)
			return
		}
	}()

	if err := session.Run("/usr/bin/scp -t " + path); err != nil {
		// Check if we had a goroutine error first
		select {
		case pipeErr := <-errCh:
			if pipeErr != nil {
				return pipeErr
			}
		default:
		}
		return fmt.Errorf("failed to scp: %w", err)
	}

	// Double check for any pipe errors after run completes
	if err := <-errCh; err != nil {
		return err
	}

	return nil
}

const (
	sshWaitInterval = 2 * time.Second
	sshDialTimeout  = 2 * time.Second
)

// WaitForSSH waits for the SSH port to be open and accepting connections.
func (c *Client) WaitForSSH(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(sshWaitInterval)
	defer ticker.Stop()

	// Determine address to dial once
	addr := c.Host
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "22")
	}

	// Check immediately once
	d := net.Dialer{Timeout: sshDialTimeout}
	if conn, err := d.DialContext(ctx, "tcp", addr); err == nil {
		_ = conn.Close()
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for SSH on %s: %w", c.Host, ctx.Err())
		case <-ticker.C:
			d := net.Dialer{Timeout: sshDialTimeout}
			conn, err := d.DialContext(ctx, "tcp", addr)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}
