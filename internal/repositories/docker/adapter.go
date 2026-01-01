package docker

import (
	"context"
	"fmt"
	"io"

	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
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

func (a *DockerAdapter) CreateContainer(ctx context.Context, name, imageName string, ports []string, networkID string) (string, error) {
	// 1. Ensure image exists (pull if not)
	reader, err := a.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader)

	// 2. Configure container
	config := &container.Config{
		Image:        imageName,
		ExposedPorts: make(nat.PortSet),
	}
	hostConfig := &container.HostConfig{
		PortBindings: make(nat.PortMap),
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
		defer w.Close()
		defer src.Close()
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
