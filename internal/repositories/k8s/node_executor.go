package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"golang.org/x/crypto/ssh"
)

// NodeExecutor defines an interface for executing commands on a cluster node.
type NodeExecutor interface {
	Run(ctx context.Context, cmd string) (string, error)
	WriteFile(ctx context.Context, path string, data io.Reader) error
	WaitForReady(ctx context.Context, timeout time.Duration) error
}

// ServiceExecutor uses the InstanceService.Exec (for Docker backend).
type ServiceExecutor struct {
	svc    ports.InstanceService
	instID uuid.UUID
}

func NewServiceExecutor(svc ports.InstanceService, instID uuid.UUID) *ServiceExecutor {
	return &ServiceExecutor{svc: svc, instID: instID}
}

func (e *ServiceExecutor) Run(ctx context.Context, cmd string) (string, error) {
	return e.svc.Exec(ctx, e.instID.String(), []string{"sh", "-c", cmd})
}

func (e *ServiceExecutor) WriteFile(ctx context.Context, path string, data io.Reader) error {
	content, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	b64Data := base64.StdEncoding.EncodeToString(content)
	_, err = e.Run(ctx, fmt.Sprintf("printf '%%s' %s | base64 -d > %s", shellQuote(b64Data), shellQuote(path)))
	return err
}

func (e *ServiceExecutor) WaitForReady(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for node to be ready: %w", ctx.Err())
		case <-ticker.C:
			_, err := e.Run(ctx, "echo ready")
			if err == nil {
				return nil
			}
		}
	}
}

// SSHExecutor uses SSH to run commands on a node.
type SSHExecutor struct {
	ip            string
	user          string
	key           string
	hostPublicKey string
}

func NewSSHExecutor(ip, user, key, hostPublicKey string) *SSHExecutor {
	return &SSHExecutor{ip: ip, user: user, key: key, hostPublicKey: hostPublicKey}
}

func (e *SSHExecutor) hostKeyCallback() (ssh.HostKeyCallback, error) {
	if strings.TrimSpace(e.hostPublicKey) == "" {
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec // intentional fallback when no host key provided
	}
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(e.hostPublicKey))
	if err != nil {
		return nil, fmt.Errorf("failed to parse host public key: %w", err)
	}
	return ssh.FixedHostKey(pubKey), nil
}

func (e *SSHExecutor) Run(ctx context.Context, cmd string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(e.key))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}
	hostKeyCallback, err := e.hostKeyCallback()
	if err != nil {
		return "", err
	}

	config := &ssh.ClientConfig{
		User: e.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(e.ip, "22")
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("failed to dial ssh: %w", err)
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w (output: %s)", err, string(output))
	}

	return string(output), nil
}

func (e *SSHExecutor) WriteFile(ctx context.Context, path string, data io.Reader) error {
	signer, err := ssh.ParsePrivateKey([]byte(e.key))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}
	hostKeyCallback, err := e.hostKeyCallback()
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User: e.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(e.ip, "22")
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to dial ssh: %w", err)
	}
	defer func() { _ = client.Close() }()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	session.Stdin = data
	err = session.Run(fmt.Sprintf("cat > %s", shellQuote(path)))
	return err
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (e *SSHExecutor) WaitForReady(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for node to be ready: %w", ctx.Err())
		case <-ticker.C:
			_, err := e.Run(ctx, "echo ready")
			if err == nil {
				return nil
			}
		}
	}
}
