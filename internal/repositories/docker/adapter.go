// Package docker implements the Docker infrastructure adapters.
package docker

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
)

const shellBin = "/bin/sh"

const (
	// ImagePullTimeout is the maximum time allowed for pulling a Docker image
	ImagePullTimeout = 5 * time.Minute
	// DefaultOperationTimeout is the default timeout for Docker operations
	DefaultOperationTimeout = 30 * time.Second
	// tracerName is the name of the tracer for this package
	tracerName = "docker-adapter"
)

// DockerAdapter implements the compute backend using Docker.
type DockerAdapter struct {
	cli    dockerClient
	logger *slog.Logger
	// containerLocks maps container IDs to mutexes for serializing attach/detach operations
	containerLocks sync.Map
}

// deleteLock removes the lock entry for a container after attach/detach completes.
// Must be called while holding the lock.
func (a *DockerAdapter) deleteLock(containerID string) {
	a.containerLocks.Delete(containerID)
}

func (a *DockerAdapter) getContainerLock(containerID string) *sync.Mutex {
	lock, _ := a.containerLocks.LoadOrStore(containerID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// dockerClient is a narrow interface over the Docker SDK client.
// It exists to make DockerAdapter unit-testable without requiring a real Docker daemon.
//
// We keep it in the docker package (not exported) to avoid leaking docker SDK types
// into other packages.
type dockerClient interface {
	Ping(ctx context.Context) (types.Ping, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerPause(ctx context.Context, containerID string) error
	ContainerUnpause(ctx context.Context, containerID string) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error)
	NetworkRemove(ctx context.Context, networkID string) error
	VolumeCreate(ctx context.Context, options volume.CreateOptions) (volume.Volume, error)
	VolumeRemove(ctx context.Context, volumeID string, force bool) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	ContainerExecCreate(ctx context.Context, container string, config container.ExecOptions) (container.ExecCreateResponse, error)
	ContainerExecStart(ctx context.Context, execID string, config container.ExecStartOptions) error
	ContainerExecAttach(ctx context.Context, execID string, config container.ExecStartOptions) (types.HijackedResponse, error)
	ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error)
	ContainerRename(ctx context.Context, containerID string, newName string) error
	ContainerUpdate(ctx context.Context, containerID string, updateConfig container.UpdateConfig) (container.UpdateResponse, error)
}

// NewDockerAdapter constructs a DockerAdapter with a Docker client.
func NewDockerAdapter(logger *slog.Logger) (*DockerAdapter, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerAdapter{cli: cli, logger: logger}, nil
}

// Ping checks if Docker daemon is reachable
func (a *DockerAdapter) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := a.cli.Ping(ctx)
	return err
}

func (a *DockerAdapter) Type() string {
	return "docker"
}

func (a *DockerAdapter) ResizeInstance(ctx context.Context, id string, cpuNanoCPUs, memoryBytes int64) error {
	return platform.Retry(ctx, platform.RetryOpts{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    10 * time.Second,
		Multiplier:  2.0,
		ShouldRetry: dockerResizeShouldRetry,
	}, func(ctx context.Context) error {
		resp, err := a.cli.ContainerUpdate(ctx, id, container.UpdateConfig{
			Resources: container.Resources{
				NanoCPUs:   cpuNanoCPUs,
				Memory:     memoryBytes,
				MemorySwap: memoryBytes, // Must be >= Memory; setting equal disables swap while allowing memory update
			},
		})
		if err != nil {
			return fmt.Errorf("failed to update container %s: %w", id, err)
		}
		if resp.Warnings != nil {
			a.logger.Warn("container update warnings", "container_id", id, "warnings", resp.Warnings)
		}
		return nil
	})
}

// dockerResizeShouldRetry returns true for Docker-transient errors that are safe to retry.
// It excludes permanent errors such as "not found" so retries do not mask real failures.
func dockerResizeShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if errdefs.IsUnavailable(err) {
		return true
	}
	if errdefs.IsResourceExhausted(err) {
		return true
	}
	if errdefs.IsInternal(err) {
		return true
	}
	msg := err.Error()
	if strings.Contains(msg, "reset") || strings.Contains(msg, "refused") {
		return true
	}
	if strings.Contains(msg, "EOF") {
		return true
	}
	if strings.Contains(msg, "temporary") {
		return true
	}
	return false
}

// CreateSnapshot is a no-op for Docker since Docker resize is hot (no restart required).
func (a *DockerAdapter) CreateSnapshot(ctx context.Context, id, name string) error {
	return nil
}

// RestoreSnapshot is a no-op for Docker since Docker resize is hot (no restart required).
func (a *DockerAdapter) RestoreSnapshot(ctx context.Context, id, name string) error {
	return nil
}

// DeleteSnapshot is a no-op for Docker since Docker resize is hot (no restart required).
func (a *DockerAdapter) DeleteSnapshot(ctx context.Context, id, name string) error {
	return nil
}

func (a *DockerAdapter) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "CreateInstance")
	defer span.End()

	span.SetAttributes(
		attribute.String("docker.image", opts.ImageName),
		attribute.String("docker.name", opts.Name),
		attribute.Bool("docker.network_disabled", opts.NetworkID == ""),
	)

	// 1. Ensure image exists (pull if not) - with timeout
	pullCtx, pullCancel := context.WithTimeout(ctx, ImagePullTimeout)
	defer pullCancel()

	// Create a sub-span for pulling
	pullCtx, pullSpan := otel.Tracer(tracerName).Start(pullCtx, "ImagePull")
	reader, err := a.cli.ImagePull(pullCtx, opts.ImageName, image.PullOptions{})
	pullSpan.End()

	if err != nil {
		return "", nil, fmt.Errorf("failed to pull image: %w", err)
	}
	defer func() { _ = reader.Close() }()
	_, _ = io.Copy(io.Discard, reader)

	// 2. Configure container
	config := &container.Config{
		Image:        opts.ImageName,
		Env:          opts.Env,
		Cmd:          opts.Cmd,
		ExposedPorts: make(nat.PortSet),
	}

	// If no command is specified, use a long-running command to keep container alive
	// This is essential for K8s nodes which need to stay running while we bootstrap them.
	// HACK: For kindest/node, we MUST NOT override the entrypoint/command as it starts systemd.
	isKIND := strings.Contains(opts.ImageName, "kindest/node")
	if len(config.Cmd) == 0 && !isKIND {
		config.Cmd = []string{shellBin, "-c", "tail -f /dev/null"}
	}

	hostConfig := &container.HostConfig{
		PortBindings: make(nat.PortMap),
		Binds:        opts.VolumeBinds,
	}

	// For KIND images, we need privileged mode to support systemd and cgroups.
	// We also need to mount /lib/modules and use anonymous volumes for containerd/kubelet
	// to avoid nested overlayfs issues on Mac.
	if isKIND {
		hostConfig.Privileged = true
		if config.Volumes == nil {
			config.Volumes = make(map[string]struct{})
		}
		config.Volumes["/var/lib/containerd"] = struct{}{}
		config.Volumes["/var/lib/kubelet"] = struct{}{}

		hostConfig.Binds = append(hostConfig.Binds,
			"/lib/modules:/lib/modules:ro",
			"/sys/fs/cgroup:/sys/fs/cgroup:ro",
		)
	}
	networkingConfig := &network.NetworkingConfig{}

	if opts.NetworkID != "" {
		networkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			opts.NetworkID: {},
		}
	}

	for _, p := range opts.Ports {
		parts := strings.Split(p, ":")
		if len(parts) == 2 {
			hostPort := parts[0]
			containerPort := parts[1]

			// We assume TCP for now as per plan
			cPort := nat.Port(containerPort + "/tcp")
			config.ExposedPorts[cPort] = struct{}{}
			hostConfig.PortBindings[cPort] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: hostPort,
				},
			}
		}
	}

	// 3. Create container
	_, createSpan := otel.Tracer(tracerName).Start(ctx, "ContainerCreate")
	resp, err := a.cli.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, opts.Name)
	createSpan.End()
	if err != nil {
		return "", nil, fmt.Errorf("failed to create container: %w", err)
	}

	// 4. Start container
	_, startSpan := otel.Tracer(tracerName).Start(ctx, "ContainerStart")
	if err := a.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		startSpan.End()
		return "", nil, fmt.Errorf("failed to start container: %w", err)
	}
	startSpan.End()

	// 5. Handle UserData (Bootstrap)
	if opts.UserData != "" {
		if err := a.handleUserData(ctx, resp.ID, opts.UserData); err != nil {
			a.logger.Warn("failed to handle user data", "container_id", resp.ID, "error", err)
		}
	}

	return resp.ID, opts.Ports, nil
}

func (a *DockerAdapter) handleUserData(ctx context.Context, containerID string, userData string) error {
	// For Docker, we simulate Cloud-Init by writing the script to the container and executing it.
	if strings.HasPrefix(userData, "#cloud-config") {
		return a.processCloudConfig(ctx, containerID, userData)
	}

	// Default fallback: treat as shell script
	scriptPath := "/tmp/bootstrap.sh"

	// Write file to container
	// Note: We use base64 encoding to avoid escaping issues with complex scripts
	encoded := base64.StdEncoding.EncodeToString([]byte(userData))
	writeCmd := []string{"sh", "-c", fmt.Sprintf("mkdir -p %s && echo %s | base64 -d > %s && chmod +x %s",
		filepath.Dir(scriptPath), encoded, scriptPath, scriptPath)}

	if _, err := a.Exec(ctx, containerID, writeCmd); err != nil {
		return fmt.Errorf("failed to write userdata to container: %w", err)
	}

	// Synchronous execution with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	out, err := a.Exec(ctxWithTimeout, containerID, []string{shellBin, scriptPath})
	if err != nil {
		a.logger.Error("failed to execute userdata script", "container_id", containerID, "error", err, "output", out)
		return fmt.Errorf("userdata script execution failed: %w", err)
	}

	return nil
}

type cloudConfig struct {
	PackageUpdate     bool              `yaml:"package_update"`
	PackageUpgrade    bool              `yaml:"package_upgrade"`
	Packages          []string          `yaml:"packages"`
	WriteFiles        []cloudConfigFile `yaml:"write_files"`
	SSHAuthorizedKeys []string          `yaml:"ssh_authorized_keys"`
	RunCmd            []interface{}     `yaml:"runcmd"` // Can be string or list
}

type cloudConfigFile struct {
	Path        string `yaml:"path"`
	Content     string `yaml:"content"`
	Permissions string `yaml:"permissions"`
}

func (a *DockerAdapter) processCloudConfig(ctx context.Context, containerID string, userData string) error {
	var cfg cloudConfig
	if err := yaml.Unmarshal([]byte(userData), &cfg); err != nil {
		return fmt.Errorf("failed to parse cloud-config: %w", err)
	}

	// We'll build a single giant shell script to execute these steps
	var scriptBuilder strings.Builder
	scriptBuilder.WriteString("#!" + shellBin + "\n")
	scriptBuilder.WriteString("set -e\n") // Exit on error
	scriptBuilder.WriteString("export DEBIAN_FRONTEND=noninteractive\n")

	// 1. Packages
	if cfg.PackageUpdate {
		scriptBuilder.WriteString("apt-get update -y\n")
	}
	if cfg.PackageUpgrade {
		scriptBuilder.WriteString("apt-get upgrade -y\n")
	}
	if len(cfg.Packages) > 0 {
		scriptBuilder.WriteString("apt-get install -y " + strings.Join(cfg.Packages, " ") + "\n")
	}

	// 2. Write Files
	for _, f := range cfg.WriteFiles {
		encodedContent := base64.StdEncoding.EncodeToString([]byte(f.Content))
		scriptBuilder.WriteString(fmt.Sprintf("mkdir -p $(dirname %s)\n", f.Path))
		scriptBuilder.WriteString(fmt.Sprintf("echo %s | base64 -d > %s\n", encodedContent, f.Path))
		if f.Permissions != "" {
			scriptBuilder.WriteString(fmt.Sprintf("chmod %s %s\n", f.Permissions, f.Path))
		}
	}

	// 2.5 SSH Keys
	if len(cfg.SSHAuthorizedKeys) > 0 {
		scriptBuilder.WriteString("mkdir -p /root/.ssh\n")
		scriptBuilder.WriteString("chmod 700 /root/.ssh\n")
		for _, key := range cfg.SSHAuthorizedKeys {
			encodedKey := base64.StdEncoding.EncodeToString([]byte(key + "\n"))
			scriptBuilder.WriteString(fmt.Sprintf("echo %s | base64 -d >> /root/.ssh/authorized_keys\n", encodedKey))
		}
		scriptBuilder.WriteString("chmod 600 /root/.ssh/authorized_keys\n")
	}

	// 3. RunCmd
	for _, cmd := range cfg.RunCmd {
		switch v := cmd.(type) {
		case string:
			scriptBuilder.WriteString(v + "\n")
		case []interface{}:
			// Convert ["echo", "hello"] to "echo hello"
			var parts []string
			for _, p := range v {
				parts = append(parts, fmt.Sprintf("%v", p))
			}
			scriptBuilder.WriteString(strings.Join(parts, " ") + "\n")
		}
	}

	// Upload and Execute the generated script
	finalScript := scriptBuilder.String()
	bootstrapPath := "/tmp/cloud-init-bootstrap.sh"

	encodedScript := base64.StdEncoding.EncodeToString([]byte(finalScript))
	writeCmd := []string{"sh", "-c", fmt.Sprintf("echo %s | base64 -d > %s && chmod +x %s",
		encodedScript, bootstrapPath, bootstrapPath)}

	if _, err := a.Exec(ctx, containerID, writeCmd); err != nil {
		return fmt.Errorf("failed to upload cloud-init bootstrap: %w", err)
	}

	// Synchronous execution with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// We stream output to logs ideally, but here just run it
	out, err := a.Exec(ctxWithTimeout, containerID, []string{shellBin, bootstrapPath})
	if err != nil {
		a.logger.Error("cloud-init execution failed", "container_id", containerID, "error", err, "output", out)
		return fmt.Errorf("cloud-init execution failed: %w", err)
	}

	a.logger.Info("cloud-init execution success", "container_id", containerID)

	return nil
}

func (a *DockerAdapter) StartInstance(ctx context.Context, id string) error {
	if err := a.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container %s: %w", id, err)
	}
	return nil
}

func (a *DockerAdapter) StopInstance(ctx context.Context, name string) error {
	timeout := 30
	err := a.cli.ContainerStop(ctx, name, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w", name, err)
	}
	return nil
}

func (a *DockerAdapter) PauseInstance(ctx context.Context, name string) error {
	if err := a.cli.ContainerPause(ctx, name); err != nil {
		return fmt.Errorf("failed to pause container %s: %w", name, err)
	}
	return nil
}

func (a *DockerAdapter) ResumeInstance(ctx context.Context, name string) error {
	if err := a.cli.ContainerUnpause(ctx, name); err != nil {
		return fmt.Errorf("failed to resume container %s: %w", name, err)
	}
	return nil
}

func (a *DockerAdapter) DeleteInstance(ctx context.Context, containerID string) error {
	err := a.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to remove container %s: %w", containerID, err)
	}
	return nil
}

func (a *DockerAdapter) GetInstanceLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "2000",
	}

	src, err := a.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}

	// Use a pipe to clean the stream asynchronously.
	// The defers below run on every exit path (including a panic out of
	// stdcopy.StdCopy), so src is always closed even if StdCopy errors
	// before consuming the stream. src.Close runs first (LIFO) so the
	// underlying reader stops producing before the pipe writer closes,
	// avoiding races where the reader keeps trying to push frames into a
	// closed writer.
	r, w := io.Pipe()
	go func() {
		defer func() { _ = w.Close() }()
		defer func() { _ = src.Close() }()
		// stdcopy demultiplexes docker stream into plain text
		_, _ = stdcopy.StdCopy(w, w, src)
	}()

	return r, nil
}

func (a *DockerAdapter) GetInstanceStats(ctx context.Context, containerID string) (io.ReadCloser, error) {
	// Stream: false = get one snapshot
	stats, err := a.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to get container stats", err)
	}
	return stats.Body, nil
}

func (a *DockerAdapter) GetInstancePort(ctx context.Context, containerID string, containerPort string) (int, error) {
	// Retry up to 30 times with 500ms backoff (15 seconds total)
	for i := 0; i < 30; i++ {
		inspect, err := a.cli.ContainerInspect(ctx, containerID)
		if err != nil {
			return 0, fmt.Errorf("failed to inspect container: %w", err)
		}

		cPort := nat.Port(containerPort + "/tcp")
		bindings, ok := inspect.NetworkSettings.Ports[cPort]
		if ok && len(bindings) > 0 {
			var hostPort int
			_, err = fmt.Sscanf(bindings[0].HostPort, "%d", &hostPort)
			if err == nil && hostPort != 0 {
				return hostPort, nil
			}
		}

		// Wait and retry
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0, ctx.Err()
		case <-timer.C:
			timer.Stop()
			continue
		}
	}

	return 0, fmt.Errorf("no port binding found for %s after retries", containerPort)
}

func (a *DockerAdapter) CreateNetwork(ctx context.Context, name string) (string, error) {
	resp, err := a.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return "", fmt.Errorf("failed to create network %s: %w", name, err)
	}
	return resp.ID, nil
}

func (a *DockerAdapter) DeleteNetwork(ctx context.Context, networkID string) error {
	err := a.cli.NetworkRemove(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to remove network %s: %w", networkID, err)
	}
	return nil
}

func (a *DockerAdapter) CreateVolume(ctx context.Context, name string) error {
	_, err := a.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
	})
	if err != nil {
		return fmt.Errorf("failed to create volume %s: %w", name, err)
	}
	return nil
}

func (a *DockerAdapter) DeleteVolume(ctx context.Context, name string) error {
	if err := a.cli.VolumeRemove(ctx, name, true); err != nil {
		return fmt.Errorf("failed to delete volume %s: %w", name, err)
	}
	return nil
}

func (a *DockerAdapter) ResizeVolume(ctx context.Context, name string, newSizeGB int) error {
	// Docker doesn't support resizing existing volumes easily without manual steps
	// or specific volume drivers. For this simulator, we'll just log it and return success
	// as it satisfies the interface and doesn't break the logical flow.
	a.logger.Info("docker volume resize simulated (no-op)", "name", name, "new_size", newSizeGB)
	return nil
}

func (a *DockerAdapter) CreateVolumeSnapshot(ctx context.Context, volumeID string, destinationPath string) error {
	// volumeID is the docker volume name
	// destinationPath is on the host

	cleanedDest, err := validateSnapshotPath(destinationPath)
	if err != nil {
		return fmt.Errorf("invalid destination path: %w", err)
	}
	destinationPath = cleanedDest

	// Ensure parent dir of destinationPath exists
	// We assume destinationPath is accessible to the docker daemon (bind mount)

	// Create a temp container to do the work
	imageName := "alpine"
	// Ensure image exists
	if _, err := a.cli.ImagePull(ctx, imageName, image.PullOptions{}); err != nil {
		return fmt.Errorf("failed to pull alpine: %w", err)
	}

	cmd := []string{"tar", "czf", "/snapshot/snapshot.tar.gz", "-C", "/data", "."}

	config := &container.Config{
		Image: imageName,
		Cmd:   cmd,
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			volumeID + ":/data:ro",
			filepath.Dir(destinationPath) + ":/snapshot",
		},
	}

	// We need to name the file correctly inside.
	// We bind the PARENT directory of destinationPath to /snapshot
	// And write to /snapshot/<filename>
	filename := filepath.Base(destinationPath)
	config.Cmd = []string{"tar", "czf", "/snapshot/" + filename, "-C", "/data", "."}

	resp, err := a.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create snapshot task: %w", err)
	}
	defer func() { _ = a.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}) }()

	if err := a.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start snapshot task: %w", err)
	}

	statusCh, errCh := a.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for snapshot task: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("snapshot task failed with exit code %d", status.StatusCode)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (a *DockerAdapter) RestoreVolumeSnapshot(ctx context.Context, volumeID string, sourcePath string) error {
	// volumeID is the docker volume name
	// sourcePath is the .tar.gz on host

	cleanedSource, err := validateSnapshotPath(sourcePath)
	if err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}
	sourcePath = cleanedSource

	imageName := "alpine"
	if _, err := a.cli.ImagePull(ctx, imageName, image.PullOptions{}); err != nil {
		return fmt.Errorf("failed to pull alpine: %w", err)
	}

	filename := filepath.Base(sourcePath)
	config := &container.Config{
		Image: imageName,
		Cmd:   []string{"tar", "xzf", "/snapshot/" + filename, "-C", "/data"},
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			volumeID + ":/data",
			filepath.Dir(sourcePath) + ":/snapshot:ro",
		},
	}

	resp, err := a.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create restore task: %w", err)
	}
	defer func() { _ = a.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}) }()

	if err := a.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start restore task: %w", err)
	}

	statusCh, errCh := a.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for restore task: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("restore task failed with exit code %d", status.StatusCode)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (a *DockerAdapter) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	lock := a.getContainerLock(id)
	lock.Lock()
	defer a.deleteLock(id) // Clean up lock entry after operation
	defer lock.Unlock()

	// 1. Inspect current container to get configuration
	inspect, err := a.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", "", fmt.Errorf("failed to inspect container %s: %w", id, err)
	}

	// Parse volumePath: if it contains ":", it's already in hostPath:containerPath[:mode] format
	// Otherwise, it's a backend volume name to be mounted at a generated path
	bindSpec := volumePath
	if !strings.Contains(volumePath, ":") {
		// Backend volume name - mount as read-write at generated path
		bindSpec = volumePath + ":/mnt/cloud-volume-" + uuid.New().String()[:8] + ":rw"
	}

	// 2. Stop the container gracefully
	timeout := 30
	if err := a.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout}); err != nil {
		return "", "", fmt.Errorf("failed to stop container %s: %w", id, err)
	}

	// 3. Build new HostConfig with updated binds
	oldHostConfig := inspect.HostConfig
	oldHostConfig.Binds = append(oldHostConfig.Binds, bindSpec)

	config := inspect.Config
	hostConfig := &container.HostConfig{
		PortBindings: oldHostConfig.PortBindings,
		Binds:        oldHostConfig.Binds,
		Privileged:   oldHostConfig.Privileged,
		Resources:    oldHostConfig.Resources,
		NetworkMode:  oldHostConfig.NetworkMode,
	}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: inspect.NetworkSettings.Networks,
	}

	// Reconstruct container name from inspect response
	containerName := ""
	if inspect.Name != "" {
		containerName = strings.TrimPrefix(inspect.Name, "/")
	}

	// 4. Rename old container to free the name before creating new one
	oldContainerTmpName := ""
	if containerName != "" {
		oldContainerTmpName = id + "-old"
		if err := a.cli.ContainerRename(ctx, id, oldContainerTmpName); err != nil {
			return "", "", fmt.Errorf("failed to rename old container: %w", err)
		}
	}

	// 5. Create new container with updated binds
	createResp, err := a.cli.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, containerName)
	if err != nil {
		// Rollback: rename old container back and restart it
		if containerName != "" {
			_ = a.cli.ContainerRename(ctx, oldContainerTmpName, containerName)
		}
		_ = a.cli.ContainerStart(ctx, id, container.StartOptions{})
		return "", "", fmt.Errorf("failed to recreate container with volume: %w", err)
	}

	// 6. Start the new container
	if err := a.cli.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		// Cleanup failed container and rollback
		_ = a.cli.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
		if containerName != "" {
			_ = a.cli.ContainerRename(ctx, oldContainerTmpName, containerName)
		}
		_ = a.cli.ContainerStart(ctx, id, container.StartOptions{})
		return "", "", fmt.Errorf("failed to start container: %w", err)
	}

	// 7. Remove old container (best effort - no error if this fails)
	if err := a.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		a.logger.Warn("failed to remove old container after volume attach",
			"old_id", id, "new_id", createResp.ID, "error", err)
	}

	// Return the container-side mount path and the new container ID
	// bindSpec format: "devicePath:containerPath[:mode]"
	parts := strings.Split(bindSpec, ":")
	if len(parts) < 2 {
		return "", createResp.ID, fmt.Errorf("invalid bind spec %q: expected hostPath:containerPath[:mode]", bindSpec)
	}
	containerPath := parts[1]
	return containerPath, createResp.ID, nil
}

func (a *DockerAdapter) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	lock := a.getContainerLock(id)
	lock.Lock()
	defer a.deleteLock(id) // Clean up lock entry after operation
	defer lock.Unlock()

	// 1. Inspect current container
	inspect, err := a.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container %s: %w", id, err)
	}

	// 2. Find and remove the bind mount matching volumePath
	currentBinds := inspect.HostConfig.Binds
	newBinds := make([]string, 0, len(currentBinds))
	found := false

	for _, bind := range currentBinds {
		parts := strings.Split(bind, ":")
		if len(parts) >= 2 {
			// Match by host path prefix (volumePath) or container path
			if strings.HasPrefix(bind, volumePath+":") || parts[1] == volumePath {
				found = true
				continue // skip this bind (remove it)
			}
		}
		newBinds = append(newBinds, bind)
	}

	if !found {
		return "", fmt.Errorf("volume path %s not found in container binds", volumePath)
	}

	// 3. Stop the container
	timeout := 30
	if err := a.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout}); err != nil {
		return "", fmt.Errorf("failed to stop container %s: %w", id, err)
	}

	// 4. Recreate without the volume bind
	config := inspect.Config
	hostConfig := &container.HostConfig{
		PortBindings: inspect.HostConfig.PortBindings,
		Binds:        newBinds,
		Privileged:   inspect.HostConfig.Privileged,
		Resources:    inspect.HostConfig.Resources,
		NetworkMode:  inspect.HostConfig.NetworkMode,
	}
	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: inspect.NetworkSettings.Networks,
	}

	containerName := ""
	if inspect.Name != "" {
		containerName = strings.TrimPrefix(inspect.Name, "/")
	}

	// 4b. Rename old container to free the name before creating new one
	oldContainerTmpName := ""
	if containerName != "" {
		oldContainerTmpName = id + "-old"
		if err := a.cli.ContainerRename(ctx, id, oldContainerTmpName); err != nil {
			return "", fmt.Errorf("failed to rename old container: %w", err)
		}
	}

	createResp, err := a.cli.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, containerName)
	if err != nil {
		// Rollback: rename old container back and restart
		if containerName != "" {
			_ = a.cli.ContainerRename(ctx, oldContainerTmpName, containerName)
		}
		_ = a.cli.ContainerStart(ctx, id, container.StartOptions{})
		return "", fmt.Errorf("failed to recreate container: %w", err)
	}

	// 5. Start new container
	if err := a.cli.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		_ = a.cli.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
		if containerName != "" {
			_ = a.cli.ContainerRename(ctx, oldContainerTmpName, containerName)
		}
		_ = a.cli.ContainerStart(ctx, id, container.StartOptions{})
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	// 6. Remove old container (best effort)
	if err := a.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		a.logger.Warn("failed to remove old container after volume detach",
			"old_id", id, "new_id", createResp.ID, "error", err)
	}

	return createResp.ID, nil
}

func (a *DockerAdapter) GetConsoleURL(ctx context.Context, id string) (string, error) {
	return "", errors.New(errors.NotImplemented, "console not supported for docker instances")
}

func (a *DockerAdapter) GetInstanceIP(ctx context.Context, id string) (string, error) {
	// Inspect container with retries to allow time for IP assignment
	var json container.InspectResponse
	var err error

	// Retry up to 30 times with 500ms backoff (15 seconds total)
	for i := 0; i < 30; i++ {
		json, err = a.cli.ContainerInspect(ctx, id)
		if err != nil {
			return "", fmt.Errorf("failed to inspect container: %w", err)
		}

		// Add nil check for NetworkSettings
		if json.NetworkSettings == nil {
			goto retry
		}

		// Add nil check for Networks map
		if len(json.NetworkSettings.Networks) == 0 {
			goto retry
		}

		// Try to get IP from first network
		for _, net := range json.NetworkSettings.Networks {
			if net != nil && net.IPAddress != "" {
				return net.IPAddress, nil
			}
		}

	retry:
		// Wait and retry
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
			timer.Stop()
			continue
		}
	}

	return "", fmt.Errorf("no IP address found for container %s after retries", id)
}

func (a *DockerAdapter) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	// 1. Ensure image exists
	pullCtx, pullCancel := context.WithTimeout(ctx, ImagePullTimeout)
	defer pullCancel()

	reader, err := a.cli.ImagePull(pullCtx, opts.Image, image.PullOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("failed to pull image: %w", err)
	}
	defer func() { _ = reader.Close() }()
	_, _ = io.Copy(io.Discard, reader)

	// 2. Configure container with security defaults
	config := &container.Config{
		Image:           opts.Image,
		Cmd:             opts.Command,
		Env:             opts.Env,
		WorkingDir:      opts.WorkingDir,
		NetworkDisabled: opts.NetworkDisabled,
	}

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:   opts.MemoryMB * 1024 * 1024,
			NanoCPUs: int64(opts.CPUs * 1e9),
		},
		Binds:          opts.Binds,
		ReadonlyRootfs: opts.ReadOnlyRootfs,
		// SecurityOpt:    []string{"no-new-privileges:true"}, // Removed to allow privileged operations (e.g. kube-proxy)
	}

	if opts.PidsLimit != nil {
		hostConfig.PidsLimit = opts.PidsLimit
	}

	// 3. Create container
	resp, err := a.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, opts.Name)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create task container: %w", err)
	}

	// 4. Start container
	if err := a.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", nil, fmt.Errorf("failed to start task container: %w", err)
	}

	return resp.ID, nil, nil
}

func (a *DockerAdapter) WaitTask(ctx context.Context, containerID string) (int64, error) {
	statusCh, errCh := a.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return 0, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		return status.StatusCode, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	return 0, nil
}

func (a *DockerAdapter) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	config := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execResp, err := a.cli.ContainerExecCreate(ctx, containerID, config)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := a.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach exec: %w", err)
	}
	defer resp.Close()

	// Capture output
	var outBuf strings.Builder
	if _, err := stdcopy.StdCopy(&outBuf, &outBuf, resp.Reader); err != nil {
		return "", fmt.Errorf("failed to read exec output: %w", err)
	}

	// Wait for completion to get exit code?
	// The attach waits for the stream to close, which happens when the process exits.
	// But to be sure, we can inspect.
	execInspect, err := a.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return outBuf.String(), fmt.Errorf("failed to inspect exec result: %w", err)
	}

	if execInspect.ExitCode != 0 {
		return outBuf.String(), fmt.Errorf("command execution failed with exit code %d: %s", execInspect.ExitCode, outBuf.String())
	}

	return outBuf.String(), nil
}

// ResetCircuitBreaker is a no-op for the raw Docker adapter.
// The circuit breaker lives in ResilientCompute wrapping this backend.
func (a *DockerAdapter) ResetCircuitBreaker() {}
