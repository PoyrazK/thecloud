package sshutil

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// TestMain switches the package into insecure-host-key mode for the duration
// of the tests. The library itself refuses to skip verification by default.
func TestMain(m *testing.M) {
	if err := os.Setenv(envInsecureHostKey, "1"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to set %s: %v\n", envInsecureHostKey, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// generateTestKey generates a private key for testing
func generateTestKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return string(keyPEM)
}

const (
	testLocalhostSSH  = "localhost:22"
	testLoopbackAddr  = "127.0.0.1:0"
	testLocalhostIP   = "127.0.0.1"
	testUnreachableIP = "192.0.2.1"
	testErrTimedOut   = "timed out"
	testCmdEcho       = "echo hello"
	testTmpFile       = "/tmp/test"
	testScpCmdPrefix  = "/usr/bin/scp -t "
)

func TestNewClientWithKey(t *testing.T) {
	// 1. Valid Key
	privKey := generateTestKey(t)
	client, err := NewClientWithKey(testLocalhostSSH, "user", privKey)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, testLocalhostSSH, client.Host)
	assert.Equal(t, "user", client.User)
	assert.NotEmpty(t, client.Auth)

	// 2. Invalid Key
	_, err = NewClientWithKey(testLocalhostSSH, "user", "invalid-key")
	require.Error(t, err)
}

func TestNewClientWithKey_NoHostKeyVerification(t *testing.T) {
	// When THECLOUD_SSH_INSECURE_IGNORE_HOSTKEY is NOT set and no known_hosts
	// file exists, NewClientWithKey must return an error (not silently use insecure defaults).
	// Use a temp dir as HOME so there's no ~/.ssh/known_hosts fallback.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv(envInsecureHostKey, "")
	t.Setenv(envKnownHostsPath, "")

	privKey := generateTestKey(t)
	_, err := NewClientWithKey(testLocalhostSSH, "user", privKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ssh host key verification not configured")
}

func TestNewClientWithKeyAndCallback(t *testing.T) {
	privKey := generateTestKey(t)

	t.Run("NilCallback", func(t *testing.T) {
		_, err := NewClientWithKeyAndCallback(testLocalhostSSH, "user", privKey, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "host key callback is required")
	})

	t.Run("ValidCallback", func(t *testing.T) {
		cb := ssh.InsecureIgnoreHostKey()
		client, err := NewClientWithKeyAndCallback(testLocalhostSSH, "user", privKey, cb)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, testLocalhostSSH, client.Host)
	})

	t.Run("InvalidKey", func(t *testing.T) {
		cb := ssh.InsecureIgnoreHostKey()
		_, err := NewClientWithKeyAndCallback(testLocalhostSSH, "user", "invalid-key", cb)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse private key")
	})
}

func TestWaitForSSH(t *testing.T) {
	// Start a dummy TCP server
	l, err := net.Listen("tcp", testLoopbackAddr)
	require.NoError(t, err)
	defer func() { _ = l.Close() }()

	tcpAddr, ok := l.Addr().(*net.TCPAddr)
	require.True(t, ok)
	port := tcpAddr.Port
	host := fmt.Sprintf("%s:%d", testLocalhostIP, port)

	client := &Client{Host: host}

	// Should connect successfully
	err = client.WaitForSSH(context.Background(), 2*time.Second)
	require.NoError(t, err)
}

func TestWaitForSSHTimeout(t *testing.T) {
	// Pick a random port (hopefully unused)
	client := &Client{Host: fmt.Sprintf("%s:54321", testLocalhostIP)} // Unlikely to be a valid SSH server immediately

	// Should timeout
	// Use small timeout for test speed
	err := client.WaitForSSH(context.Background(), 100*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), testErrTimedOut)
}

func TestWaitForSSHContextCanceled(t *testing.T) {
	client := &Client{Host: fmt.Sprintf("%s:54321", testLocalhostIP)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.WaitForSSH(ctx, 2*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), testErrTimedOut)
}

func TestWaitForSSHHostWithoutPortTimeout(t *testing.T) {
	client := &Client{Host: testUnreachableIP}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.WaitForSSH(ctx, 100*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), testErrTimedOut)
}

func TestRunConnectionRefused(t *testing.T) {
	// Ensure we pick a port that rejects connection
	l, err := net.Listen("tcp", testLoopbackAddr)
	require.NoError(t, err)
	tcpAddr, ok := l.Addr().(*net.TCPAddr)
	require.True(t, ok)
	port := tcpAddr.Port
	_ = l.Close() // Close immediately to ensure connection refused

	host := fmt.Sprintf("%s:%d", testLocalhostIP, port)
	privKey := generateTestKey(t)
	client, _ := NewClientWithKey(host, "user", privKey)

	_, err = client.Run(context.Background(), testCmdEcho)
	require.Error(t, err)
	// Error message format depends on OS, usually "connection refused" or "dial tcp"
}

func TestRunContextTimeout(t *testing.T) {
	privKey := generateTestKey(t)
	client, err := NewClientWithKey(testLoopbackAddr, "user", privKey)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = client.Run(ctx, testCmdEcho)
	require.Error(t, err)
}

func TestRunHostWithoutPort(t *testing.T) {
	privKey := generateTestKey(t)
	client, err := NewClientWithKey(testLocalhostIP, "user", privKey)
	require.NoError(t, err)

	_, err = client.Run(context.Background(), testCmdEcho)
	require.Error(t, err)
}

// Full SSH server mock for Run and WriteFile validates the Client logic, Key parsing, and Network dialing.

func TestWriteFileConnectionRefused(t *testing.T) {
	// Pick a random port (hopefully unused)
	client := &Client{Host: testLoopbackAddr}

	err := client.WriteFile(context.Background(), testTmpFile, []byte("data"), "0644")
	require.Error(t, err)
	// Expect dial error
}

func TestWriteFileContextTimeout(t *testing.T) {
	client := &Client{Host: testLoopbackAddr}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.WriteFile(ctx, testTmpFile, []byte("data"), "0644")
	require.Error(t, err)
}

func TestWriteFileHostWithoutPort(t *testing.T) {
	privKey := generateTestKey(t)
	client, err := NewClientWithKey(testLocalhostIP, "user", privKey)
	require.NoError(t, err)

	err = client.WriteFile(context.Background(), testTmpFile, []byte("data"), "0644")
	require.Error(t, err)
}

func TestRunSuccess(t *testing.T) {
	addr, stop := startTestSSHServer(t)
	defer stop()

	privKey := generateTestKey(t)
	client, err := NewClientWithKey(addr, "user", privKey)
	require.NoError(t, err)

	out, err := client.Run(context.Background(), testCmdEcho)
	require.NoError(t, err)
	assert.Contains(t, out, "hello")
}

func TestWriteFileSuccess(t *testing.T) {
	addr, stop := startTestSSHServer(t)
	defer stop()

	privKey := generateTestKey(t)
	client, err := NewClientWithKey(addr, "user", privKey)
	require.NoError(t, err)

	err = client.WriteFile(context.Background(), "/tmp/test.txt", []byte("data"), "0644")
	require.NoError(t, err)
}

func TestWriteFileScpError(t *testing.T) {
	addr, stop := startTestSSHServerWithHandler(t, func(cmd string, ch ssh.Channel) error {
		if strings.HasPrefix(cmd, testScpCmdPrefix) {
			return fmt.Errorf("scp failed")
		}
		if cmd == testCmdEcho {
			_, err := ch.Write([]byte("hello\n"))
			return err
		}
		return fmt.Errorf("unsupported command: %s", cmd)
	})
	defer stop()

	privKey := generateTestKey(t)
	client, err := NewClientWithKey(addr, "user", privKey)
	require.NoError(t, err)

	err = client.WriteFile(context.Background(), "/tmp/test.txt", []byte("data"), "0644")
	require.Error(t, err)
}

func startTestSSHServer(t *testing.T) (string, func()) {
	t.Helper()
	return startTestSSHServerWithHandler(t, handleExecCommand)
}

func startTestSSHServerWithHandler(t *testing.T, handler func(cmd string, ch ssh.Channel) error) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", testLoopbackAddr)
	require.NoError(t, err)

	signer, err := newTestSigner()
	require.NoError(t, err)

	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				handleSSHConn(c, config, handler)
			}(conn)
		}
	}()

	return listener.Addr().String(), func() {
		close(stop)
		_ = listener.Close()
		wg.Wait()
	}
}

func newTestSigner() (ssh.Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(key)
}

func handleSSHConn(conn net.Conn, config *ssh.ServerConfig, handler func(cmd string, ch ssh.Channel) error) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer func() { _ = sshConn.Close() }()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		go handleSessionRequests(channel, requests, handler)
	}
}

func handleSessionRequests(ch ssh.Channel, in <-chan *ssh.Request, handler func(cmd string, ch ssh.Channel) error) {
	defer func() { _ = ch.Close() }()
	for req := range in {
		if req.Type != "exec" {
			_ = req.Reply(false, nil)
			continue
		}

		var payload struct{ Command string }
		_ = ssh.Unmarshal(req.Payload, &payload)
		_ = req.Reply(true, nil)

		status := uint32(0)
		if err := handler(payload.Command, ch); err != nil {
			status = 1
		}

		_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: status}))
		return
	}
}

func handleExecCommand(cmd string, ch ssh.Channel) error {
	if cmd == testCmdEcho {
		_, err := ch.Write([]byte("hello\n"))
		return err
	}

	if len(cmd) >= len(testScpCmdPrefix) && cmd[:len(testScpCmdPrefix)] == testScpCmdPrefix {
		return handleScp(ch)
	}

	return fmt.Errorf("unsupported command: %s", cmd)
}

func handleScp(r io.Reader) error {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if err != nil {
		return err
	}

	var mode string
	var size int
	var filename string
	_, err = fmt.Sscanf(line, "C%s %d %s", &mode, &size, &filename)
	if err != nil {
		return err
	}

	if size > 0 {
		if _, err := io.CopyN(io.Discard, br, int64(size)); err != nil {
			return err
		}
	}

	_, err = br.ReadByte() // null byte
	return err
}
