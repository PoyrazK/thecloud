package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/poyraz/cloud/internal/core/domain"
	"github.com/poyraz/cloud/internal/core/ports"
	"github.com/poyraz/cloud/internal/errors"
)

type InstanceService struct {
	repo   ports.InstanceRepository
	docker ports.DockerClient
}

func NewInstanceService(repo ports.InstanceRepository, docker ports.DockerClient) *InstanceService {
	return &InstanceService{
		repo:   repo,
		docker: docker,
	}
}

func (s *InstanceService) LaunchInstance(ctx context.Context, name, image, ports string) (*domain.Instance, error) {
	// 1. Validate ports if provided
	portList, err := s.parseAndValidatePorts(ports)
	if err != nil {
		return nil, err
	}

	// 2. Create domain entity
	inst := &domain.Instance{
		ID:        uuid.New(),
		Name:      name,
		Image:     image,
		Status:    domain.StatusStarting,
		Ports:     ports,
		Version:   1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 3. Persist to DB first (Pending state)
	if err := s.repo.Create(ctx, inst); err != nil {
		return nil, err
	}

	// 4. Call Docker to create actual container
	dockerName := fmt.Sprintf("miniaws-%s", inst.ID.String()[:8])

	containerID, err := s.docker.CreateContainer(ctx, dockerName, image, portList)
	if err != nil {
		inst.Status = domain.StatusError
		_ = s.repo.Update(ctx, inst)
		return nil, errors.Wrap(errors.Internal, "failed to launch container", err)
	}

	// 5. Update status and save ContainerID
	inst.Status = domain.StatusRunning
	inst.ContainerID = containerID
	if err := s.repo.Update(ctx, inst); err != nil {
		return nil, err
	}

	return inst, nil
}

func (s *InstanceService) parseAndValidatePorts(ports string) ([]string, error) {
	if ports == "" {
		return nil, nil
	}

	portList := strings.Split(ports, ",")
	if len(portList) > errors.MaxPortsPerInstance {
		return nil, errors.New(errors.TooManyPorts, fmt.Sprintf("max %d ports allowed", errors.MaxPortsPerInstance))
	}

	for _, p := range portList {
		parts := strings.Split(p, ":")
		if len(parts) != 2 {
			return nil, errors.New(errors.InvalidPortFormat, "port format must be host:container")
		}
		// In a more robust version we'd check if ports are integers within range
	}

	return portList, nil
}

func (s *InstanceService) StopInstance(ctx context.Context, id uuid.UUID) error {
	// 1. Get from DB
	inst, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if inst.Status == domain.StatusStopped {
		return nil // Already stopped
	}

	// 2. Call Docker stop
	// We use the stored ContainerID if available, otherwise fallback to Name (legacy support)
	target := inst.ContainerID
	if target == "" {
		// Try to reconstruct older naming scheme or just fail?
		// For now let's assumes legacy containers used Name.
		// But in our new scheme they use miniaws-ID.
		// Actually, let's look for the container by name if ID is missing.
		target = inst.Name
	} else {
		// StopContainer in adapter currently takes 'name', but Docker API supports ID too.
	}

	if err := s.docker.StopContainer(ctx, target); err != nil {
		// If we can't stop it, maybe it is already gone or name changed.
		// Log warning?
		return errors.Wrap(errors.Internal, "failed to stop container", err)
	}

	// 3. Update DB
	inst.Status = domain.StatusStopped
	return s.repo.Update(ctx, inst)
}

func (s *InstanceService) ListInstances(ctx context.Context) ([]*domain.Instance, error) {
	return s.repo.List(ctx)
}
