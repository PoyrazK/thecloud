package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/mock"
)

func setupContainerWorkerTest(t *testing.T) (*MockContainerRepo, *MockInstanceService, *MockEventService, *services.ContainerWorker) {
	repo := new(MockContainerRepo)
	instSvc := new(MockInstanceService)
	eventSvc := new(MockEventService)
	worker := services.NewContainerWorker(repo, instSvc, eventSvc)
	return repo, instSvc, eventSvc, worker
}

func TestContainerWorker_Reconcile_ScaleUp(t *testing.T) {
	repo, instSvc, _, worker := setupContainerWorkerTest(t)
	defer repo.AssertExpectations(t)
	defer instSvc.AssertExpectations(t)

	depID := uuid.New()
	userID := uuid.New()
	dep := &domain.Deployment{
		ID:       depID,
		UserID:   userID,
		Name:     "dep-1",
		Replicas: 2,
		Image:    "nginx",
		Status:   domain.DeploymentStatusReady,
	}

	repo.On("ListAllDeployments", mock.Anything).Return([]*domain.Deployment{dep}, nil)
	repo.On("GetContainers", mock.Anything, depID).Return([]uuid.UUID{}, nil)

	instID1 := uuid.New()
	instID2 := uuid.New()

	inst1 := &domain.Instance{ID: instID1}
	inst2 := &domain.Instance{ID: instID2}

	instSvc.On("LaunchInstance", mock.Anything, mock.AnythingOfType("string"), dep.Image, dep.Ports, (*uuid.UUID)(nil), (*uuid.UUID)(nil), ([]domain.VolumeAttachment)(nil)).Return(inst1, nil).Once()
	repo.On("AddContainer", mock.Anything, depID, instID1).Return(nil).Once()

	instSvc.On("LaunchInstance", mock.Anything, mock.AnythingOfType("string"), dep.Image, dep.Ports, (*uuid.UUID)(nil), (*uuid.UUID)(nil), ([]domain.VolumeAttachment)(nil)).Return(inst2, nil).Once()
	repo.On("AddContainer", mock.Anything, depID, instID2).Return(nil).Once()

	repo.On("UpdateDeployment", mock.Anything, mock.MatchedBy(func(d *domain.Deployment) bool {
		return d.ID == depID
	})).Return(nil)

	worker.Reconcile(context.Background())
}

func TestContainerWorker_Reconcile_ScaleDown(t *testing.T) {
	repo, instSvc, _, worker := setupContainerWorkerTest(t)
	defer repo.AssertExpectations(t)
	defer instSvc.AssertExpectations(t)

	depID := uuid.New()
	userID := uuid.New()
	dep := &domain.Deployment{
		ID:       depID,
		UserID:   userID,
		Name:     "dep-1",
		Replicas: 1,
	}

	containerIDs := []uuid.UUID{uuid.New(), uuid.New()}

	repo.On("ListAllDeployments", mock.Anything).Return([]*domain.Deployment{dep}, nil)
	repo.On("GetContainers", mock.Anything, depID).Return(containerIDs, nil)

	repo.On("RemoveContainer", mock.Anything, depID, containerIDs[0]).Return(nil)
	instSvc.On("TerminateInstance", mock.Anything, containerIDs[0].String()).Return(nil)

	repo.On("UpdateDeployment", mock.Anything, mock.Anything).Return(nil)

	worker.Reconcile(context.Background())
}
