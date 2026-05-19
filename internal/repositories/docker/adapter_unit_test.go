package docker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	apierrors "github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/testutil"
)

func TestDockerAdapterType(t *testing.T) {
	a := &DockerAdapter{}
	require.Equal(t, "docker", a.Type())
}

func TestDockerAdapterDeleteInstanceNotFoundIsNil(t *testing.T) {
	a := &DockerAdapter{cli: &fakeDockerClient{removeErr: errdefs.ErrNotFound}}
	require.NoError(t, a.DeleteInstance(context.Background(), "missing"))
}

func TestDockerAdapterGetInstancePortNoBinding(t *testing.T) {
	inspect := container.InspectResponse{}
	inspect.NetworkSettings = &container.NetworkSettings{}

	a := &DockerAdapter{cli: &fakeDockerClient{inspect: inspect}}
	_, err := a.GetInstancePort(context.Background(), "cid", "8080")
	require.Error(t, err)
}

// Note: port parsing is best covered via integration tests because docker SDK
// types for NetworkSettings/Ports are tricky to construct across versions.

func TestDockerAdapterGetInstanceIPNoNetworks(t *testing.T) {
	inspect := container.InspectResponse{}
	inspect.NetworkSettings = &container.NetworkSettings{Networks: map[string]*network.EndpointSettings{}}

	a := &DockerAdapter{cli: &fakeDockerClient{inspect: inspect}}
	// Use short timeout to avoid waiting full retry duration (15s) in test
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := a.GetInstanceIP(ctx, "cid")
	require.Error(t, err)
}

func TestDockerAdapterGetInstanceIPFirstIP(t *testing.T) {
	inspect := container.InspectResponse{}
	inspect.NetworkSettings = &container.NetworkSettings{
		Networks: map[string]*network.EndpointSettings{
			"n1": {IPAddress: testutil.TestDockerInstanceIP},
		},
	}

	a := &DockerAdapter{cli: &fakeDockerClient{inspect: inspect}}
	ip, err := a.GetInstanceIP(context.Background(), "cid")
	require.NoError(t, err)
	require.Equal(t, testutil.TestDockerInstanceIP, ip)
}

func TestDockerAdapterGetInstanceStatsReturnsBody(t *testing.T) {
	a := &DockerAdapter{cli: &fakeDockerClient{statsRC: io.NopCloser(strings.NewReader("{}"))}}
	rc, err := a.GetInstanceStats(context.Background(), "cid")
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
}

func TestDockerAdapterGetInstanceStatsError(t *testing.T) {
	a := &DockerAdapter{cli: &fakeDockerClient{statsErr: apierrors.New("Internal", "docker stats failed")}}
	_, err := a.GetInstanceStats(context.Background(), "cid")
	require.Error(t, err)
	require.True(t, apierrors.Is(err, apierrors.Internal))
}

func TestDockerAdapterExecNonZeroExit(t *testing.T) {
	cli := &fakeDockerClient{
		execAttachRead: strings.NewReader("out"),
		execInspect:    container.ExecInspect{ExitCode: 2},
	}
	adapter := &DockerAdapter{cli: cli}

	out, err := adapter.Exec(context.Background(), "cid", []string{"false"})
	require.Error(t, err)
	// Exec output is demultiplexed with stdcopy, which expects Docker's multiplexed
	// stream format. Our unit tests don't simulate that format, so output may be empty.
	_ = out
}

func TestDockerAdapterExecInspectErrorReturnsOutput(t *testing.T) {
	cli := &fakeDockerClient{
		execAttachRead: strings.NewReader("out"),
		execInspectErr: errFakeNotFound,
	}
	adapter := &DockerAdapter{cli: cli}

	out, err := adapter.Exec(context.Background(), "cid", []string{"echo"})
	require.Error(t, err)
	_ = out
}

func TestDockerAdapterCreateVolume(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	err := adapter.CreateVolume(context.Background(), "test-volume")
	require.NoError(t, err)
}

func TestDockerAdapterDeleteVolume(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	err := adapter.DeleteVolume(context.Background(), "test-volume")
	require.NoError(t, err)
}

func TestDockerAdapterAttachVolume(t *testing.T) {
	cli := &fakeDockerClient{
		inspect: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				Name: "/test-container",
				HostConfig: &container.HostConfig{
					Binds: []string{"/host/data:/container/data:rw"},
				},
			},
			NetworkSettings: &container.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{},
			},
		},
	}
	adapter := &DockerAdapter{cli: cli}
	containerPath, newContainerID, err := adapter.AttachVolume(context.Background(), "cid", "/new/volume:/mnt/new:rw")
	require.NoError(t, err)
	require.Equal(t, "/mnt/new", containerPath)
	require.NotEmpty(t, newContainerID)
}

func TestDockerAdapterDetachVolume(t *testing.T) {
	cli := &fakeDockerClient{
		inspect: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				Name: "/test-container",
				HostConfig: &container.HostConfig{
					Binds: []string{"/host/data:/mnt/data:rw"},
				},
			},
			NetworkSettings: &container.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{},
			},
		},
	}
	adapter := &DockerAdapter{cli: cli}
	newContainerID, err := adapter.DetachVolume(context.Background(), "cid", "/mnt/data")
	require.NoError(t, err)
	require.NotEmpty(t, newContainerID)
}

func TestDockerAdapterGetConsoleURL(t *testing.T) {
	adapter := &DockerAdapter{}
	_, err := adapter.GetConsoleURL(context.Background(), "cid")
	// Console is not supported for docker
	require.Error(t, err)
	require.Contains(t, err.Error(), "console not supported")
}

func TestDockerAdapterCreateVolumeSnapshot(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	// Should succeed with fake client
	err := adapter.CreateVolumeSnapshot(context.Background(), "vol-id", "/tmp/backup.tar.gz")
	require.NoError(t, err)
}

func TestDockerAdapterRestoreVolumeSnapshot(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	// Should succeed with fake client
	err := adapter.RestoreVolumeSnapshot(context.Background(), "vol-id", "/tmp/backup.tar.gz")
	require.NoError(t, err)
}

func TestDockerAdapterRunTask(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	opts := ports.RunTaskOptions{
		Image:   "alpine",
		Command: []string{"echo", "hello"},
	}
	id, _, err := adapter.RunTask(context.Background(), opts)
	require.NoError(t, err)
	require.Equal(t, "cid", id)
}

func TestDockerAdapterWaitTask(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	exitCode, err := adapter.WaitTask(context.Background(), "cid")
	require.NoError(t, err)
	require.Equal(t, int64(0), exitCode)
}

func TestDockerAdapterWaitTaskError(t *testing.T) {
	cli := &fakeDockerClient{waitErr: errors.New("wait failed")}
	adapter := &DockerAdapter{cli: cli}

	_, err := adapter.WaitTask(context.Background(), "cid")
	require.Error(t, err)
}

func TestDockerAdapterCreateNetwork(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	id, err := adapter.CreateNetwork(context.Background(), "net1")
	require.NoError(t, err)
	require.Equal(t, "nid", id)
}

func TestDockerAdapterDeleteNetwork(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	err := adapter.DeleteNetwork(context.Background(), "net1")
	require.NoError(t, err)
}

func TestDockerAdapterCreateInstance(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	opts := ports.CreateInstanceOptions{
		Name:      "inst1",
		ImageName: "alpine",
		Cmd:       []string{"/bin/sh"},
		Ports:     []string{"8080:80"},
	}
	id, _, err := adapter.LaunchInstanceWithOptions(context.Background(), opts)
	require.NoError(t, err)
	require.Equal(t, "cid", id)
}

func TestDockerAdapterStopInstance(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	err := adapter.StopInstance(context.Background(), "cid")
	require.NoError(t, err)
}

func TestDockerAdapterGetInstanceLogs(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}

	rc, err := adapter.GetInstanceLogs(context.Background(), "cid")
	require.NoError(t, err)
	_ = rc.Close()
}

func TestDockerAdapterPing(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &DockerAdapter{cli: cli}
	err := adapter.Ping(context.Background())
	require.NoError(t, err)

	cli.pingErr = errors.New("ping failed")
	err = adapter.Ping(context.Background())
	require.Error(t, err)
}

func TestDockerAdapterNewDockerAdapterError(t *testing.T) {
	// Triggering error by setting an invalid DOCKER_HOST
	t.Setenv("DOCKER_HOST", "invalid-proto://")
	_, err := NewDockerAdapter(slog.Default())
	require.Error(t, err)
}

type mockVpcRepository struct {
	ports.VpcRepository
	getByIDFunc func(ctx context.Context, id uuid.UUID) (*domain.VPC, error)
}

func (m *mockVpcRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.VPC, error) {
	return m.getByIDFunc(ctx, id)
}

type mockInstanceRepoUnit struct {
	ports.InstanceRepository
	getByIDFunc func(ctx context.Context, id uuid.UUID) (*domain.Instance, error)
}

func (m *mockInstanceRepoUnit) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	return m.getByIDFunc(ctx, id)
}

func TestLBProxyAdapterDeployProxy(t *testing.T) {
	cli := &fakeDockerClient{}
	vpcRepo := &mockVpcRepository{
		getByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.VPC, error) {
			return &domain.VPC{ID: id, NetworkID: "net1"}, nil
		},
	}
	instRepo := &mockInstanceRepoUnit{
		getByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
			return &domain.Instance{ID: id}, nil
		},
	}
	adapter := &LBProxyAdapter{
		cli:          cli,
		vpcRepo:      vpcRepo,
		instanceRepo: instRepo,
	}

	lb := &domain.LoadBalancer{
		ID:    uuid.New(),
		Port:  80,
		VpcID: uuid.New(),
	}
	targets := []*domain.LBTarget{
		{InstanceID: uuid.New(), Port: 8080, Weight: 1},
	}

	id, err := adapter.DeployProxy(context.Background(), lb, targets)
	require.NoError(t, err)
	require.Equal(t, "cid", id)
}

func TestLBProxyAdapterRemoveProxy(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter := &LBProxyAdapter{cli: cli}
	err := adapter.RemoveProxy(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestLBProxyAdapterUpdateProxyConfig(t *testing.T) {
	cli := &fakeDockerClient{}
	instRepo := &mockInstanceRepoUnit{
		getByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
			return &domain.Instance{ID: id}, nil
		},
	}
	adapter := &LBProxyAdapter{
		cli:          cli,
		instanceRepo: instRepo,
	}

	lb := &domain.LoadBalancer{
		ID:   uuid.New(),
		Port: 80,
	}
	targets := []*domain.LBTarget{
		{InstanceID: uuid.New(), Port: 8080},
	}

	// Ensure directory exists for test
	configPath := filepath.Join("/tmp", "thecloud", "lb", lb.ID.String())
	_ = os.MkdirAll(configPath, 0750)
	defer func() { _ = os.RemoveAll(configPath) }()

	err := adapter.UpdateProxyConfig(context.Background(), lb, targets)
	require.NoError(t, err)
}

func TestDockerAdapterStopInstanceError(t *testing.T) {
	cli := &fakeDockerClient{stopErr: errors.New("stop error")}
	adapter := &DockerAdapter{cli: cli}
	err := adapter.StopInstance(context.Background(), "cid")
	require.Error(t, err)
}

func TestDockerAdapterCreateNetworkError(t *testing.T) {
	cli := &fakeDockerClient{networkCreateErr: errors.New("network create error")}
	adapter := &DockerAdapter{cli: cli}
	_, err := adapter.CreateNetwork(context.Background(), "net")
	require.Error(t, err)
}
func TestDockerAdapterCreateInstanceError(t *testing.T) {
	cli := &fakeDockerClient{containerCreateErr: errors.New("create error")}
	adapter := &DockerAdapter{cli: cli}

	opts := ports.CreateInstanceOptions{
		Name:      "inst1",
		ImageName: "alpine",
	}
	_, _, err := adapter.LaunchInstanceWithOptions(context.Background(), opts)
	require.Error(t, err)
}

func TestDockerAdapterDeleteNetworkError(t *testing.T) {
	cli := &fakeDockerClient{networkRemoveErr: errors.New("network remove error")}
	adapter := &DockerAdapter{cli: cli}

	err := adapter.DeleteNetwork(context.Background(), "net1")
	require.Error(t, err)
}
func TestDockerAdapterLaunchWithUserData(t *testing.T) {
	cli := &fakeDockerClient{}
	adapter, err := NewDockerAdapter(slog.Default())
	require.NoError(t, err)
	adapter.cli = cli

	opts := ports.CreateInstanceOptions{
		Name:      "test-userdata",
		ImageName: "alpine",
		UserData:  "#!/bin/sh\necho 'hello world' > /tmp/output",
	}

	id, _, err := adapter.LaunchInstanceWithOptions(context.Background(), opts)
	require.NoError(t, err)
	require.Equal(t, "cid", id)

	// UserData execution occurs asynchronously in a background goroutine.
	// A brief pause ensures that the goroutine has sufficient time to initiate its Exec calls.
	time.Sleep(50 * time.Millisecond)

	// Verify the two-stage bootstrap sequence:
	// Stage 1: Payload delivery (writing the bootstrap script to the container filesystem).
	// Stage 2: Bootstrap execution (invoking the script via a background Exec operation).
	// Each 'Exec' operation sequentially triggers ContainerExecCreate, ContainerExecAttach,
	// and ContainerExecInspect according to the adapter's implementation.
	require.Equal(t, 2, cli.CallCount("ContainerExecCreate"), "Expected dual Stage (delivery + execution) Exec invocations")
	require.Equal(t, 2, cli.CallCount("ContainerExecAttach"), "Expected associated ExecAttach calls for I/O")
	require.Equal(t, 2, cli.CallCount("ContainerExecInspect"), "Expected ExecInspect calls to verify termination state")
}
