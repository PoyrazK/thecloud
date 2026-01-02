package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

type InstanceService struct {
	repo       ports.InstanceRepository
	vpcRepo    ports.VpcRepository
	volumeRepo ports.VolumeRepository
	docker     ports.DockerClient
	eventSvc   ports.EventService
	logger     *slog.Logger
}

func NewInstanceService(repo ports.InstanceRepository, vpcRepo ports.VpcRepository, volumeRepo ports.VolumeRepository, docker ports.DockerClient, eventSvc ports.EventService, logger *slog.Logger) *InstanceService {
	return &InstanceService{
		repo:       repo,
		vpcRepo:    vpcRepo,
		volumeRepo: volumeRepo,
		docker:     docker,
		eventSvc:   eventSvc,
		logger:     logger,
	}
}

func (s *InstanceService) LaunchInstance(ctx context.Context, name, image, ports string, vpcID *uuid.UUID, volumes []domain.VolumeAttachment) (*domain.Instance, error) {
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

	containerID, err := s.docker.CreateContainer(ctx, dockerName, image, portList, networkID, volumeBinds)
	if err != nil {
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

	if err := s.docker.StopContainer(ctx, target); err != nil {
		s.logger.Error("failed to stop docker container", "container_id", target, "error", err)
		return errors.Wrap(errors.Internal, "failed to stop container", err)
	}
	s.logger.Info("instance stopped", "instance_id", inst.ID)

	// 3. Update DB
	inst.Status = domain.StatusStopped
	return s.repo.Update(ctx, inst)
}

func (s *InstanceService) ListInstances(ctx context.Context) ([]*domain.Instance, error) {
	return s.repo.List(ctx)
}

func (s *InstanceService) GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error) {
	// 1. Try to parse as UUID
	id, uuidErr := uuid.Parse(idOrName)
	if uuidErr == nil {
		return s.repo.GetByID(ctx, id)
	}
	// 2. Fallback to name lookup
	return s.repo.GetByName(ctx, idOrName)
}

func (s *InstanceService) GetInstanceLogs(ctx context.Context, idOrName string) (string, error) {
	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return "", err
	}

	if inst.ContainerID == "" {
		return "", errors.New(errors.InstanceNotRunning, "instance has no active container")
	}

	stream, err := s.docker.GetLogs(ctx, inst.ContainerID)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	bytes, err := io.ReadAll(stream)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to read logs", err)
	}

	return string(bytes), nil
}

func (s *InstanceService) TerminateInstance(ctx context.Context, idOrName string) error {
	// 1. Get from DB (handles both Name and UUID)
	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return err
	}

	// 2. Remove from Docker (force remove handles running containers)
	if err := s.removeInstanceContainer(ctx, inst); err != nil {
		return err
	}

	// 3. Release attached volumes after container removal
	if err := s.releaseAttachedVolumes(ctx, inst.ID); err != nil {
		s.logger.Warn("failed to release volumes during termination", "instance_id", inst.ID, "error", err)
	}

	// 4. Delete from DB
	if err := s.repo.Delete(ctx, inst.ID); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "INSTANCE_TERMINATE", inst.ID.String(), "INSTANCE", map[string]interface{}{})
	return nil
}

func (s *InstanceService) removeInstanceContainer(ctx context.Context, inst *domain.Instance) error {
	containerID := inst.ContainerID
	if containerID == "" {
		// Fallback to Reconstruction for legacy or missing ID
		containerID = fmt.Sprintf("thecloud-%s", inst.ID.String()[:8])
	}

	if err := s.docker.RemoveContainer(ctx, containerID); err != nil {
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

func (s *InstanceService) GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error) {
	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return nil, err
	}

	if inst.ContainerID == "" {
		return nil, errors.New(errors.InstanceNotRunning, "instance not running")
	}

	stream, err := s.docker.GetContainerStats(ctx, inst.ContainerID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to get stats stream", err)
	}
	defer stream.Close()

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
