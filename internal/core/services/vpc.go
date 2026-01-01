package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/poyraz/cloud/internal/core/domain"
	"github.com/poyraz/cloud/internal/core/ports"
)

type VpcService struct {
	repo   ports.VpcRepository
	docker ports.DockerClient
}

func NewVpcService(repo ports.VpcRepository, docker ports.DockerClient) *VpcService {
	return &VpcService{
		repo:   repo,
		docker: docker,
	}
}

func (s *VpcService) CreateVPC(ctx context.Context, name string) (*domain.VPC, error) {
	// 1. Create Docker network first
	networkName := fmt.Sprintf("miniaws-vpc-%s", uuid.New().String()[:8])
	dockerNetworkID, err := s.docker.CreateNetwork(ctx, networkName)
	if err != nil {
		return nil, err
	}

	// 2. Persist to DB
	vpc := &domain.VPC{
		ID:        uuid.New(),
		Name:      name,
		NetworkID: dockerNetworkID,
		CreatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, vpc); err != nil {
		// Cleanup Docker network if DB fails
		_ = s.docker.RemoveNetwork(ctx, dockerNetworkID)
		return nil, err
	}

	return vpc, nil
}

func (s *VpcService) GetVPC(ctx context.Context, idOrName string) (*domain.VPC, error) {
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.repo.GetByID(ctx, id)
	}
	return s.repo.GetByName(ctx, idOrName)
}

func (s *VpcService) ListVPCs(ctx context.Context) ([]*domain.VPC, error) {
	return s.repo.List(ctx)
}

func (s *VpcService) DeleteVPC(ctx context.Context, idOrName string) error {
	vpc, err := s.GetVPC(ctx, idOrName)
	if err != nil {
		return err
	}

	// 1. Remove Docker network
	if err := s.docker.RemoveNetwork(ctx, vpc.NetworkID); err != nil {
		return err
	}

	// 2. Delete from DB
	return s.repo.Delete(ctx, vpc.ID)
}
