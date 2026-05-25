package csi

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/poyrazk/thecloud/pkg/sdk"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
)

const (
	// DefaultVolumeSizeGB is the fallback size if not specified
	DefaultVolumeSizeGB = 10
)

// Mounter defines OS-level operations for volume management
type Mounter interface {
	FormatDevice(ctx context.Context, device, fsType string) error
	Mount(ctx context.Context, source, target, fsType string) error
	BindMount(ctx context.Context, source, target string) error
	Unmount(ctx context.Context, target string) error
	IsFormatted(ctx context.Context, device string) bool
	MkdirAll(path string, perm os.FileMode) error
}

// LinuxMounter implements Mounter using standard shell commands
type LinuxMounter struct {
	logger *slog.Logger
	execer func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func (m *LinuxMounter) run(ctx context.Context, name string, arg ...string) error {
	cmd := m.execer(ctx, name, arg...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command %s %v failed: %w; output: %s", name, arg, err, string(output))
	}
	return nil
}

func (m *LinuxMounter) IsFormatted(ctx context.Context, device string) bool {
	err := m.run(ctx, "blkid", device)
	return err == nil
}

func (m *LinuxMounter) FormatDevice(ctx context.Context, device, fsType string) error {
	if m.IsFormatted(ctx, device) {
		return nil
	}
	return m.run(ctx, "mkfs", "-t", fsType, device)
}

func (m *LinuxMounter) Mount(ctx context.Context, source, target, fsType string) error {
	return m.run(ctx, "mount", "-t", fsType, source, target)
}

func (m *LinuxMounter) BindMount(ctx context.Context, source, target string) error {
	return m.run(ctx, "mount", "--bind", source, target)
}

func (m *LinuxMounter) Unmount(ctx context.Context, target string) error {
	return m.run(ctx, "umount", target)
}

func (m *LinuxMounter) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Driver implements the CSI services
type Driver struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedControllerServer
	csi.UnimplementedNodeServer

	name     string
	version  string
	nodeID   string
	endpoint string

	logger  *slog.Logger
	cloud   *sdk.Client
	mounter Mounter

	srv *grpc.Server
}

// NewDriver creates a new CSI driver
func NewDriver(name, version, nodeID, endpoint string, cloud *sdk.Client, logger *slog.Logger) *Driver {
	return &Driver{
		name:     name,
		version:  version,
		nodeID:   nodeID,
		endpoint: endpoint,
		cloud:    cloud,
		logger:   logger,
		mounter:  &LinuxMounter{logger: logger, execer: exec.CommandContext},
	}
}

// Run starts the gRPC server
func (d *Driver) Run() error {
	scheme, addr, err := parseEndpoint(d.endpoint)
	if err != nil {
		return err
	}

	if scheme == "unix" {
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove existing socket file %s: %w", addr, err)
		}
	}

	listener, err := net.Listen(scheme, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", d.endpoint, err)
	}

	d.srv = grpc.NewServer()

	csi.RegisterIdentityServer(d.srv, d)
	csi.RegisterControllerServer(d.srv, d)
	csi.RegisterNodeServer(d.srv, d)

	d.logger.Info("CSI driver started", "name", d.name, "version", d.version, "endpoint", d.endpoint, "nodeID", d.nodeID)

	return d.srv.Serve(listener)
}

// Stop stops the gRPC server
func (d *Driver) Stop() {
	if d.srv != nil {
		d.srv.Stop()
		d.srv = nil
	}
}

func parseEndpoint(ep string) (string, string, error) {
	if strings.HasPrefix(ep, "unix://") || strings.HasPrefix(ep, "tcp://") {
		parts := strings.SplitN(ep, "://", 2)
		if len(parts) == 2 && parts[1] != "" {
			return parts[0], parts[1], nil
		}
	}
	return "", "", fmt.Errorf("invalid endpoint: %v", ep)
}

// Identity Server Implementation
func (d *Driver) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          d.name,
		VendorVersion: d.version,
	}, nil
}

func (d *Driver) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}, nil
}

func (d *Driver) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}

// Controller Server Implementation
func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "Name is required")
	}

	cap := req.GetCapacityRange()
	sizeGB := DefaultVolumeSizeGB
	if cap != nil {
		sizeGB = int(cap.GetRequiredBytes() / 1024 / 1024 / 1024)
		if sizeGB < 1 {
			sizeGB = 1
		}
	}

	d.logger.Info("Creating volume", "name", req.Name, "sizeGB", sizeGB)
	vol, err := d.cloud.CreateVolume(req.Name, sizeGB)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create volume: %v", err)
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      vol.ID.String(),
			CapacityBytes: int64(vol.SizeGB) * 1024 * 1024 * 1024,
		},
	}, nil
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId is required")
	}

	d.logger.Info("Deleting volume", "volumeID", req.VolumeId)
	if err := d.cloud.DeleteVolume(req.VolumeId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete volume: %v", err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if req.VolumeId == "" || req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId and NodeId are required")
	}

	d.logger.Info("Attaching volume", "volumeID", req.VolumeId, "nodeID", req.NodeId)

	// 1. Resolve NodeId (Kubernetes Node Name) to Instance UUID
	instance, err := d.cloud.GetInstance(req.NodeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resolve node name %s to instance: %v", req.NodeId, err)
	}

	// 2. Attach via API using the UUID
	devicePath, err := d.cloud.AttachVolume(req.VolumeId, instance.ID, "/dev/vdb")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to attach volume: %v", err)
	}

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			"device": devicePath,
		},
	}, nil
}

func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId is required")
	}

	d.logger.Info("Detaching volume", "volumeID", req.VolumeId, "nodeID", req.NodeId)
	if err := d.cloud.DetachVolume(req.VolumeId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to detach volume: %v", err)
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.VolumeCapabilities,
		},
	}, nil
}

func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	newCap := func(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
		return &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	rpcCaps := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
	}
	caps := make([]*csi.ControllerServiceCapability, 0, len(rpcCaps))
	for _, c := range rpcCaps {
		caps = append(caps, newCap(c))
	}

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}, nil
}

func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) GetSnapshot(ctx context.Context, req *csi.GetSnapshotRequest) (*csi.GetSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Node Server Implementation
func (d *Driver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	if req.VolumeId == "" || req.StagingTargetPath == "" || req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "VolumeId, StagingTargetPath and VolumeCapability are required")
	}

	devicePath := req.PublishContext["device"]
	if devicePath == "" {
		return nil, status.Error(codes.InvalidArgument, "device path missing in publish context")
	}

	d.logger.Info("NodeStageVolume", "volumeID", req.VolumeId, "stagingPath", req.StagingTargetPath, "device", devicePath)

	if err := d.mounter.FormatDevice(ctx, devicePath, "ext4"); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to format device: %v", err)
	}

	if err := d.mounter.MkdirAll(req.StagingTargetPath, 0750); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create staging path: %v", err)
	}

	if err := d.mounter.Mount(ctx, devicePath, req.StagingTargetPath, "ext4"); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mount device: %v", err)
	}

	return &csi.NodeStageVolumeResponse{}, nil
}

func (d *Driver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	if req.VolumeId == "" || req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId and StagingTargetPath are required")
	}

	if err := d.mounter.Unmount(ctx, req.StagingTargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount: %v", err)
	}

	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (d *Driver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if req.VolumeId == "" || req.TargetPath == "" || req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId, TargetPath and StagingTargetPath are required")
	}

	d.logger.Info("NodePublishVolume (Bind Mount)", "volumeID", req.VolumeId, "targetPath", req.TargetPath)

	if err := d.mounter.MkdirAll(req.TargetPath, 0750); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create target path: %v", err)
	}

	if err := d.mounter.BindMount(ctx, req.StagingTargetPath, req.TargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to bind mount: %v", err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (d *Driver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if req.VolumeId == "" || req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "VolumeId and TargetPath are required")
	}

	d.logger.Info("NodeUnpublishVolume", "volumeID", req.VolumeId, "targetPath", req.TargetPath)

	if err := d.mounter.Unmount(ctx, req.TargetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmount target: %v", err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (d *Driver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: d.nodeID,
	}, nil
}

func (d *Driver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
					},
				},
			},
		},
	}, nil
}

func (d *Driver) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
