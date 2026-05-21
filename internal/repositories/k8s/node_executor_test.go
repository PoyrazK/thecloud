package k8s

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestServiceExecutor(t *testing.T) {
	ctx := context.Background()
	instID := uuid.New()
	mockSvc := new(mockInstanceService)
	executor := NewServiceExecutor(mockSvc, instID)

	t.Run("Run", func(t *testing.T) {
		cmd := "echo hello"
		mockSvc.On("Exec", ctx, instID.String(), []string{"sh", "-c", cmd}).Return("hello", nil).Once()

		out, err := executor.Run(ctx, cmd)
		require.NoError(t, err)
		assert.Equal(t, "hello", out)
	})

	t.Run("WriteFile", func(t *testing.T) {
		path := "/tmp/test.txt"
		data := "hello world"

		mockSvc.On("Exec", ctx, instID.String(), mock.MatchedBy(func(cmd []string) bool {
			return strings.Contains(cmd[2], "base64 -d > '"+path+"'")
		})).Return("", nil).Once()

		err := executor.WriteFile(ctx, path, strings.NewReader(data))
		require.NoError(t, err)
	})

	t.Run("WaitForReady", func(t *testing.T) {
		mockSvc.On("Exec", mock.Anything, instID.String(), []string{"sh", "-c", "echo ready"}).Return("ready", nil).Once()

		err := executor.WaitForReady(ctx, 2*time.Second)
		require.NoError(t, err)
	})
}

func TestSSHExecutor(t *testing.T) {
	// Setup a simple SSH server for testing
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(pk),
	})

	signer, err := ssh.ParsePrivateKey(privateKeyPEM)
	require.NoError(t, err)

	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	ip := "127.0.0.1"

	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				return
			}
			_, _, reqs, err := ssh.NewServerConn(nConn, config)
			if err != nil {
				continue
			}
			go ssh.DiscardRequests(reqs)
		}
	}()

	t.Run("NewSSHExecutor", func(t *testing.T) {
		executor := NewSSHExecutor(ip, "user", string(privateKeyPEM), "")
		assert.NotNil(t, executor)
		assert.Equal(t, ip, executor.ip)
	})

	t.Run("Run_FailConn", func(t *testing.T) {
		executor := NewSSHExecutor("127.0.0.1", "user", string(privateKeyPEM), "")
		// Use a wrong port or something that will definitely fail fast
		executor.ip = "127.0.0.1:1"
		_, err := executor.Run(context.Background(), "ls")
		assert.Error(t, err)
	})

	t.Run("HostKeyCallback_UsesPinnedHostKey", func(t *testing.T) {
		hostPub := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
		executor := NewSSHExecutor(ip, "user", string(privateKeyPEM), hostPub)

		callback, err := executor.hostKeyCallback()
		require.NoError(t, err)

		err = callback("127.0.0.1:22", nil, signer.PublicKey())
		assert.NoError(t, err)
	})

	t.Run("HostKeyCallback_InvalidPinnedKey", func(t *testing.T) {
		executor := NewSSHExecutor(ip, "user", string(privateKeyPEM), "not-a-valid-key")

		_, err := executor.hostKeyCallback()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse host public key")
	})
}
