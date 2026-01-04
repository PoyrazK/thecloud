package docker

import (
	"context"
	"fmt"
	"io"
	"time"

	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

const (
	// ImagePullTimeout is the maximum time allowed for pulling a Docker image
	ImagePullTimeout = 5 * time.Minute
	// DefaultOperationTimeout is the default timeout for Docker operations
	DefaultOperationTimeout = 30 * time.Second
)

type DockerAdapter struct {
	cli *client.Client
}

func NewDockerAdapter() (*DockerAdapter, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	return &DockerAdapter{cli: cli}, nil
}

// Ping checks if Docker daemon is reachable
func (a *DockerAdapter) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := a.cli.Ping(ctx)
	return err
}

func (a *DockerAdapter) CreateContainer(ctx context.Context, name, imageName string, ports []string, networkID string, volumeBinds []string, env []string, cmd []string) (string, error) {
	// 1. Ensure image exists (pull if not) - with timeout
	pullCtx, pullCancel := context.WithTimeout(ctx, ImagePullTimeout)
	defer pullCancel()

	reader, err := a.cli.ImagePull(pullCtx, imageName, image.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}
	defer func() { _ = reader.Close() }()
	_, _ = io.Copy(io.Discard, reader)

	// 2. Configure container
	config := &container.Config{
		Image:        imageName,
		Env:          env,
		Cmd:          cmd,
		ExposedPorts: make(nat.PortSet),
	}
	hostConfig := &container.HostConfig{
		PortBindings: make(nat.PortMap),
		Binds:        volumeBinds,
	}
	networkingConfig := &network.NetworkingConfig{}

	if networkID != "" {
		networkingConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			networkID: {},
		}
	}

	for _, p := range ports {
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
	resp, err := a.cli.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 4. Start container
	if err := a.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return resp.ID, nil
}

func (a *DockerAdapter) StopContainer(ctx context.Context, name string) error {
	err := a.cli.ContainerStop(ctx, name, container.StopOptions{})
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w", name, err)
	}
	return nil
}

func (a *DockerAdapter) RemoveContainer(ctx context.Context, containerID string) error {
	err := a.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to remove container %s: %w", containerID, err)
	}
	return nil
}

func (a *DockerAdapter) GetLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "2000",
	}

	src, err := a.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %w", err)
	}

	// Use a pipe to clean the stream asynchronously
	r, w := io.Pipe()
	go func() {
		defer func() { _ = w.Close() }()
		defer func() { _ = src.Close() }()
		// stdcopy demultiplexes docker stream into plain text
		_, _ = stdcopy.StdCopy(w, w, src)
	}()

	return r, nil
}

func (a *DockerAdapter) GetContainerStats(ctx context.Context, containerID string) (io.ReadCloser, error) {
	// Stream: false = get one snapshot
	stats, err := a.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	return stats.Body, nil
}

func (a *DockerAdapter) GetContainerPort(ctx context.Context, containerID string, containerPort string) (int, error) {
	inspect, err := a.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return 0, fmt.Errorf("failed to inspect container: %w", err)
	}

	cPort := nat.Port(containerPort + "/tcp")
	bindings, ok := inspect.NetworkSettings.Ports[cPort]
	if !ok || len(bindings) == 0 {
		return 0, fmt.Errorf("no port binding found for %s", containerPort)
	}

	var hostPort int
	_, err = fmt.Sscanf(bindings[0].HostPort, "%d", &hostPort)
	if err != nil {
		return 0, fmt.Errorf("failed to parse host port: %w", err)
	}

	return hostPort, nil
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

func (a *DockerAdapter) RemoveNetwork(ctx context.Context, networkID string) error {
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

func (a *DockerAdapter) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, error) {
	// 1. Ensure image exists
	pullCtx, pullCancel := context.WithTimeout(ctx, ImagePullTimeout)
	defer pullCancel()

	reader, err := a.cli.ImagePull(pullCtx, opts.Image, image.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
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
		SecurityOpt:    []string{"no-new-privileges:true"},
	}

	if opts.PidsLimit != nil {
		hostConfig.Resources.PidsLimit = opts.PidsLimit
	}

	// 3. Create container
	resp, err := a.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create task container: %w", err)
	}

	// 4. Start container
	if err := a.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start task container: %w", err)
	}

	return resp.ID, nil
}

func (a *DockerAdapter) WaitContainer(ctx context.Context, containerID string) (int64, error) {
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
