package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
)

// InstanceService manages the lifecycle of compute instances,
// coordinating between database persistence, compute backends (Docker/Libvirt),
// and network backends (OVS).
type InstanceService struct {
	repo       ports.InstanceRepository
	vpcRepo    ports.VpcRepository
	subnetRepo ports.SubnetRepository
	volumeRepo ports.VolumeRepository
	compute    ports.ComputeBackend
	network    ports.NetworkBackend
	eventSvc   ports.EventService
	auditSvc   ports.AuditService
	logger     *slog.Logger
}

// NewInstanceService initializes a new InstanceService with required dependencies.
func NewInstanceService(repo ports.InstanceRepository, vpcRepo ports.VpcRepository, subnetRepo ports.SubnetRepository, volumeRepo ports.VolumeRepository, compute ports.ComputeBackend, network ports.NetworkBackend, eventSvc ports.EventService, auditSvc ports.AuditService, logger *slog.Logger) *InstanceService {
	return &InstanceService{
		repo:       repo,
		vpcRepo:    vpcRepo,
		subnetRepo: subnetRepo,
		volumeRepo: volumeRepo,
		compute:    compute,
		network:    network,
		eventSvc:   eventSvc,
		auditSvc:   auditSvc,
		logger:     logger,
	}
}

// LaunchInstance provisions a new instance, sets up its network (if VPC/Subnet provided),
// and attaches any requested volumes.
func (s *InstanceService) LaunchInstance(ctx context.Context, name, image, ports string, vpcID, subnetID *uuid.UUID, volumes []domain.VolumeAttachment) (*domain.Instance, error) {
	// 1. Validate ports if provided
	portList, err := s.parseAndValidatePorts(ports)
	if err != nil {
		return nil, err
	}

	// 2. Create domain entity
	inst := &domain.Instance{
		ID:        uuid.New(),
		UserID:    appcontext.UserIDFromContext(ctx),
		Name:      name,
		Image:     image,
		Status:    domain.StatusStarting,
		Ports:     ports,
		VpcID:     vpcID,
		SubnetID:  subnetID,
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 3. Persist to DB first (Pending state)
	if err := s.repo.Create(ctx, inst); err != nil {
		return nil, err
	}

	// 4. Call Docker to create actual container
	dockerName := fmt.Sprintf("thecloud-%s", inst.ID.String()[:8])

	networkID := ""
	if vpcID != nil {
		vpc, err := s.vpcRepo.GetByID(ctx, *vpcID)
		if err != nil {
			s.logger.Error("failed to get VPC", "vpc_id", vpcID, "error", err)
			return nil, err
		}
		networkID = vpc.NetworkID
	}

	// OVS Networking Setup (Pre-conditional)
	if subnetID != nil && s.network != nil {
		subnet, err := s.subnetRepo.GetByID(ctx, *subnetID)
		if err != nil {
			return nil, errors.Wrap(errors.NotFound, "subnet not found", err)
		}

		// Dynamic IP allocation
		allocatedIP, err := s.allocateIP(ctx, subnet)
		if err != nil {
			return nil, errors.Wrap(errors.ResourceLimitExceeded, "failed to allocate IP in subnet", err)
		}
		inst.PrivateIP = allocatedIP

		vethHost := fmt.Sprintf("veth-%s", inst.ID.String()[:8])
		inst.OvsPort = vethHost
	}

	// 5. Process volume attachments
	var volumeBinds []string
	var attachedVolumes []*domain.Volume
	for _, va := range volumes {
		vol, err := s.getVolumeByIDOrName(ctx, va.VolumeIDOrName)
		if err != nil {
			s.logger.Error("failed to get volume", "volume", va.VolumeIDOrName, "error", err)
			return nil, errors.Wrap(errors.NotFound, fmt.Sprintf("volume %s not found", va.VolumeIDOrName), err)
		}
		if vol.Status != domain.VolumeStatusAvailable {
			return nil, errors.New(errors.InvalidInput, fmt.Sprintf("volume %s is not available", vol.Name))
		}
		dockerVolName := "thecloud-vol-" + vol.ID.String()[:8]
		volumeBinds = append(volumeBinds, fmt.Sprintf("%s:%s", dockerVolName, va.MountPath))
		attachedVolumes = append(attachedVolumes, vol)
	}

	containerID, err := s.compute.CreateInstance(ctx, dockerName, image, portList, networkID, volumeBinds, nil, nil)
	if err != nil {
		platform.InstanceOperationsTotal.WithLabelValues("launch", "failure").Inc()
		s.logger.Error("failed to create docker container", "name", dockerName, "image", image, "error", err)
		inst.Status = domain.StatusError
		if err := s.repo.Update(ctx, inst); err != nil {
			s.logger.Error("failed to update instance status after docker create failure", "instance_id", inst.ID, "error", err)
		}
		_ = s.eventSvc.RecordEvent(ctx, "INSTANCE_LAUNCH_FAILED", inst.ID.String(), "INSTANCE", map[string]interface{}{
			"name":  inst.Name,
			"image": inst.Image,
			"error": err.Error(),
		})
		return nil, errors.Wrap(errors.Internal, "failed to launch container", err)
	}

	// 4a. OVS Post-launch plumb
	if inst.OvsPort != "" && s.network != nil {
		vethContainer := fmt.Sprintf("eth0-%s", inst.ID.String()[:8])
		if err := s.network.CreateVethPair(ctx, inst.OvsPort, vethContainer); err == nil {
			vpc, _ := s.vpcRepo.GetByID(ctx, *inst.VpcID)
			_ = s.network.AttachVethToBridge(ctx, vpc.NetworkID, inst.OvsPort)

			// Set IP on the host simulation "container" end
			if inst.SubnetID != nil {
				subnet, _ := s.subnetRepo.GetByID(ctx, *inst.SubnetID)
				if subnet != nil {
					_, ipNet, _ := net.ParseCIDR(subnet.CIDRBlock)
					ones, _ := ipNet.Mask.Size()
					// In a real cloud, this happens inside the container namespace.
					// For this demo, we do it on the host for the 'peer' end.
					_ = s.network.SetVethIP(ctx, vethContainer, inst.PrivateIP, fmt.Sprintf("%d", ones))

					// Ensure the "container" end is also up on host if simulating
					cmd := exec.CommandContext(ctx, "ip", "link", "set", vethContainer, "up")
					_ = cmd.Run()
				}
			}

			// Note: Moving veth to container namespace requires the container PID,
			// which Docker shim handles in a real cloud. For this local demo,
			// we just create the veth pair on host.
		}
	}

	s.logger.Info("container launched", "instance_id", inst.ID, "container_id", containerID)

	// 5. Update status and save ContainerID
	inst.Status = domain.StatusRunning
	inst.ContainerID = containerID
	if err := s.repo.Update(ctx, inst); err != nil {
		return nil, err
	}

	_ = s.eventSvc.RecordEvent(ctx, "INSTANCE_LAUNCH", inst.ID.String(), "INSTANCE", map[string]interface{}{
		"name":  inst.Name,
		"image": inst.Image,
	})

	_ = s.auditSvc.Log(ctx, inst.UserID, "instance.launch", "instance", inst.ID.String(), map[string]interface{}{
		"name":  inst.Name,
		"image": inst.Image,
	})

	// 6. Update volume statuses
	s.updateVolumesAfterLaunch(ctx, attachedVolumes, inst.ID)

	return inst, nil
}

func (s *InstanceService) parseAndValidatePorts(ports string) ([]string, error) {
	if ports == "" {
		return nil, nil
	}

	portList := strings.Split(ports, ",")
	if len(portList) > domain.MaxPortsPerInstance {
		return nil, errors.New(errors.TooManyPorts, fmt.Sprintf("max %d ports allowed", domain.MaxPortsPerInstance))
	}

	for _, p := range portList {
		parts := strings.Split(p, ":")
		if len(parts) != 2 {
			return nil, errors.New(errors.InvalidPortFormat, "port format must be host:container")
		}

		hostPort, err := parsePort(parts[0])
		if err != nil {
			return nil, errors.New(errors.InvalidPortFormat, fmt.Sprintf("invalid host port: %s", parts[0]))
		}
		containerPort, err := parsePort(parts[1])
		if err != nil {
			return nil, errors.New(errors.InvalidPortFormat, fmt.Sprintf("invalid container port: %s", parts[1]))
		}

		if hostPort < domain.MinPort || hostPort > domain.MaxPort {
			return nil, errors.New(errors.InvalidPortFormat, fmt.Sprintf("host port %d out of range (%d-%d)", hostPort, domain.MinPort, domain.MaxPort))
		}
		if containerPort < domain.MinPort || containerPort > domain.MaxPort {
			return nil, errors.New(errors.InvalidPortFormat, fmt.Sprintf("container port %d out of range (%d-%d)", containerPort, domain.MinPort, domain.MaxPort))
		}
	}

	return portList, nil
}

func parsePort(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty port")
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return port, nil
}

// StopInstance halts a running instance's associated compute resource (e.g., container).
func (s *InstanceService) StopInstance(ctx context.Context, idOrName string) error {
	// 1. Get from DB (handles both Name and UUID)
	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return err
	}

	if inst.Status == domain.StatusStopped {
		return nil // Already stopped
	}

	// 2. Call Docker stop
	target := inst.ContainerID
	if target == "" {
		// Fallback to Reconstruction
		target = fmt.Sprintf("thecloud-%s", inst.ID.String()[:8])
	}

	if err := s.compute.StopInstance(ctx, target); err != nil {
		platform.InstanceOperationsTotal.WithLabelValues("stop", "failure").Inc()
		s.logger.Error("failed to stop docker container", "container_id", target, "error", err)
		return errors.Wrap(errors.Internal, "failed to stop container", err)
	}

	platform.InstancesTotal.WithLabelValues("running", s.compute.Type()).Dec()
	platform.InstancesTotal.WithLabelValues("stopped", s.compute.Type()).Inc()
	platform.InstanceOperationsTotal.WithLabelValues("stop", "success").Inc()

	s.logger.Info("instance stopped", "instance_id", inst.ID)

	// 3. Update DB
	inst.Status = domain.StatusStopped
	if err := s.repo.Update(ctx, inst); err != nil {
		return err
	}

	_ = s.auditSvc.Log(ctx, inst.UserID, "instance.stop", "instance", inst.ID.String(), map[string]interface{}{
		"name": inst.Name,
	})

	return nil
}

// ListInstances returns all instances owned by the current user.
func (s *InstanceService) ListInstances(ctx context.Context) ([]*domain.Instance, error) {
	return s.repo.List(ctx)
}

// GetInstance retrieves an instance by its UUID or name.
func (s *InstanceService) GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error) {
	// 1. Try to parse as UUID
	id, uuidErr := uuid.Parse(idOrName)
	if uuidErr == nil {
		return s.repo.GetByID(ctx, id)
	}
	// 2. Fallback to name lookup
	return s.repo.GetByName(ctx, idOrName)
}

// GetInstanceLogs retrieves the execution logs from the instance's compute resource.
func (s *InstanceService) GetInstanceLogs(ctx context.Context, idOrName string) (string, error) {
	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return "", err
	}

	if inst.ContainerID == "" {
		return "", errors.New(errors.InstanceNotRunning, "instance has no active container")
	}

	stream, err := s.compute.GetInstanceLogs(ctx, inst.ContainerID)
	if err != nil {
		return "", err
	}
	defer func() { _ = stream.Close() }()

	bytes, err := io.ReadAll(stream)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to read logs", err)
	}

	return string(bytes), nil
}

// TerminateInstance permanently removes an instance and its associated compute resources.
func (s *InstanceService) TerminateInstance(ctx context.Context, idOrName string) error {
	// 1. Get from DB (handles both Name and UUID)
	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return err
	}

	// 2. Remove from Docker (force remove handles running containers)
	if err := s.removeInstanceContainer(ctx, inst); err != nil {
		platform.InstanceOperationsTotal.WithLabelValues("terminate", "failure").Inc()
		return err
	}

	switch inst.Status {
	case domain.StatusRunning:
		platform.InstancesTotal.WithLabelValues("running", s.compute.Type()).Dec()
	case domain.StatusStopped:
		platform.InstancesTotal.WithLabelValues("stopped", s.compute.Type()).Dec()
	}
	platform.InstanceOperationsTotal.WithLabelValues("terminate", "success").Inc()

	// 3. Release attached volumes after container removal
	if err := s.releaseAttachedVolumes(ctx, inst.ID); err != nil {
		s.logger.Warn("failed to release volumes during termination", "instance_id", inst.ID, "error", err)
	}

	// 4. Delete from DB
	if err := s.repo.Delete(ctx, inst.ID); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "INSTANCE_TERMINATE", inst.ID.String(), "INSTANCE", map[string]interface{}{})

	_ = s.auditSvc.Log(ctx, inst.UserID, "instance.terminate", "instance", inst.ID.String(), map[string]interface{}{
		"name": inst.Name,
	})

	return nil
}

func (s *InstanceService) removeInstanceContainer(ctx context.Context, inst *domain.Instance) error {
	containerID := inst.ContainerID
	if containerID == "" {
		// Fallback to Reconstruction for legacy or missing ID
		containerID = fmt.Sprintf("thecloud-%s", inst.ID.String()[:8])
	}

	if err := s.compute.DeleteInstance(ctx, containerID); err != nil {
		s.logger.Warn("failed to remove docker container", "container_id", containerID, "error", err)
		return errors.Wrap(errors.Internal, "failed to remove container", err)
	}

	s.logger.Info("instance terminated", "instance_id", inst.ID)
	return nil
}

// releaseAttachedVolumes marks all volumes attached to an instance as available
func (s *InstanceService) releaseAttachedVolumes(ctx context.Context, instanceID uuid.UUID) error {
	volumes, err := s.volumeRepo.ListByInstanceID(ctx, instanceID)
	if err != nil {
		return err
	}

	for _, vol := range volumes {
		vol.Status = domain.VolumeStatusAvailable
		vol.InstanceID = nil
		vol.MountPath = ""
		vol.UpdatedAt = time.Now()

		if err := s.volumeRepo.Update(ctx, vol); err != nil {
			s.logger.Warn("failed to release volume", "volume_id", vol.ID, "error", err)
			continue
		}
		s.logger.Info("volume released during instance termination", "volume_id", vol.ID, "instance_id", instanceID)
	}

	return nil
}

// GetInstanceStats retrieves real-time CPU and Memory usage for an instance.
func (s *InstanceService) GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error) {
	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return nil, err
	}

	if inst.ContainerID == "" {
		return nil, errors.New(errors.InstanceNotRunning, "instance not running")
	}

	stream, err := s.compute.GetInstanceStats(ctx, inst.ContainerID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to get stats stream", err)
	}
	defer func() { _ = stream.Close() }()

	// Parse JSON
	var stats struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
	}

	if err := json.NewDecoder(stream).Decode(&stats); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to decode stats", err)
	}

	// Calculate CPU %
	// (total - pre_total) / (system - pre_system) * number_cpus * 100
	// For simplicity, we assume single core or simple calc for now
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemCPUUsage) - float64(stats.PreCPUStats.SystemCPUUsage)

	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * 100.0
	}

	memUsage := float64(stats.MemoryStats.Usage)
	memLimit := float64(stats.MemoryStats.Limit)
	memPercent := 0.0
	if memLimit > 0 {
		memPercent = (memUsage / memLimit) * 100.0
	}

	return &domain.InstanceStats{
		CPUPercentage:    cpuPercent,
		MemoryUsageBytes: memUsage,
		MemoryLimitBytes: memLimit,
		MemoryPercentage: memPercent,
	}, nil
}

func (s *InstanceService) getVolumeByIDOrName(ctx context.Context, idOrName string) (*domain.Volume, error) {
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.volumeRepo.GetByID(ctx, id)
	}
	return s.volumeRepo.GetByName(ctx, idOrName)
}

func (s *InstanceService) updateVolumesAfterLaunch(ctx context.Context, volumes []*domain.Volume, instanceID uuid.UUID) {
	for _, vol := range volumes {
		vol.Status = domain.VolumeStatusInUse
		vol.InstanceID = &instanceID
		vol.UpdatedAt = time.Now()
		if err := s.volumeRepo.Update(ctx, vol); err != nil {
			s.logger.Warn("failed to update volume status", "volume_id", vol.ID, "error", err)
		}
	}
}
func (s *InstanceService) allocateIP(ctx context.Context, subnet *domain.Subnet) (string, error) {
	_, ipNet, err := net.ParseCIDR(subnet.CIDRBlock)
	if err != nil {
		return "", err
	}

	instances, err := s.repo.ListBySubnet(ctx, subnet.ID)
	if err != nil {
		return "", err
	}

	usedIPs := make(map[string]bool)
	for _, inst := range instances {
		if inst.PrivateIP != "" {
			usedIPs[inst.PrivateIP] = true
		}
	}
	usedIPs[subnet.GatewayIP] = true

	// Find first available IP
	ip := make(net.IP, len(ipNet.IP))
	copy(ip, ipNet.IP)

	for {
		// Increment IP
		for i := len(ip) - 1; i >= 0; i-- {
			ip[i]++
			if ip[i] > 0 {
				break
			}
		}

		if !ipNet.Contains(ip) {
			break
		}

		if !usedIPs[ip.String()] && s.isValidHostIP(ip, ipNet) {
			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("no available IPs in subnet")
}

func (s *InstanceService) isValidHostIP(ip net.IP, n *net.IPNet) bool {
	// Simple check: not network address and not broadcast address (if /30 or larger)
	// For simplicity in this demo, we just ensure it's in range and not gateway
	return n.Contains(ip)
}
