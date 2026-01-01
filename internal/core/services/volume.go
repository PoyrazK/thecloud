package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/poyraz/cloud/internal/core/domain"
	"github.com/poyraz/cloud/internal/core/ports"
	"github.com/poyraz/cloud/internal/errors"
)

type VolumeService struct {
	repo     ports.VolumeRepository
	docker   ports.DockerClient
	eventSvc ports.EventService
	logger   *slog.Logger
}

func NewVolumeService(repo ports.VolumeRepository, docker ports.DockerClient, eventSvc ports.EventService, logger *slog.Logger) *VolumeService {
	return &VolumeService{
		repo:     repo,
		docker:   docker,
		eventSvc: eventSvc,
		logger:   logger,
	}
}

func (s *VolumeService) CreateVolume(ctx context.Context, name string, sizeGB int) (*domain.Volume, error) {
	// 1. Create domain entity
	vol := &domain.Volume{
		ID:        uuid.New(),
		Name:      name,
		SizeGB:    sizeGB,
		Status:    domain.VolumeStatusAvailable,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 2. Create Docker Volume
	dockerName := "miniaws-vol-" + vol.ID.String()[:8]
	if err := s.docker.CreateVolume(ctx, dockerName); err != nil {
		s.logger.Error("failed to create docker volume", "name", dockerName, "error", err)
		return nil, errors.Wrap(errors.Internal, "failed to create volume", err)
	}

	// 3. Persist to DB
	if err := s.repo.Create(ctx, vol); err != nil {
		// Rollback Docker Volume
		_ = s.docker.DeleteVolume(ctx, dockerName)
		return nil, err
	}

	_ = s.eventSvc.RecordEvent(ctx, "VOLUME_CREATE", vol.ID.String(), "VOLUME", map[string]interface{}{
		"name":    vol.Name,
		"size_gb": vol.SizeGB,
	})

	s.logger.Info("volume created", "volume_id", vol.ID, "name", vol.Name)
	return vol, nil
}

func (s *VolumeService) ListVolumes(ctx context.Context) ([]*domain.Volume, error) {
	return s.repo.List(ctx)
}

func (s *VolumeService) GetVolume(ctx context.Context, idOrName string) (*domain.Volume, error) {
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.repo.GetByID(ctx, id)
	}
	return s.repo.GetByName(ctx, idOrName)
}

func (s *VolumeService) DeleteVolume(ctx context.Context, idOrName string) error {
	vol, err := s.GetVolume(ctx, idOrName)
	if err != nil {
		return err
	}

	if vol.Status == domain.VolumeStatusInUse {
		return errors.New(errors.InvalidInput, "cannot delete volume that is in use")
	}

	// 1. Delete Docker Volume
	dockerName := "miniaws-vol-" + vol.ID.String()[:8]
	if err := s.docker.DeleteVolume(ctx, dockerName); err != nil {
		s.logger.Warn("failed to delete docker volume", "name", dockerName, "error", err)
	}

	// 2. Delete from DB
	if err := s.repo.Delete(ctx, vol.ID); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "VOLUME_DELETE", vol.ID.String(), "VOLUME", map[string]interface{}{})

	s.logger.Info("volume deleted", "volume_id", vol.ID)
	return nil
}
