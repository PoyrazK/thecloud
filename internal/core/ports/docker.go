package ports

import (
	"context"
	"io"
)

// DockerClient defines the interface for interacting with the container engine.
type DockerClient interface {
	CreateContainer(ctx context.Context, name, image string, ports []string, networkID string, volumeBinds []string) (string, error)
	StopContainer(ctx context.Context, containerID string) error
	RemoveContainer(ctx context.Context, containerID string) error
	GetLogs(ctx context.Context, containerID string) (io.ReadCloser, error)
	GetContainerStats(ctx context.Context, containerID string) (io.ReadCloser, error)
	CreateNetwork(ctx context.Context, name string) (string, error)
	RemoveNetwork(ctx context.Context, networkID string) error
	CreateVolume(ctx context.Context, name string) error
	DeleteVolume(ctx context.Context, name string) error
}
