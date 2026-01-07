package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateDeployment_Success(t *testing.T) {
	repo := new(MockContainerRepository)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	// logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Note: NewContainerService signature might vary, assuming it takes these
	// Checking the file content for NewContainerService: repo, eventSvc, auditSvc
	svc := services.NewContainerService(repo, eventSvc, auditSvc)

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	name := "test-deploy"
	image := "nginx:latest"
	replicas := 3
	ports := "80:80"

	repo.On("CreateDeployment", ctx, mock.MatchedBy(func(d *domain.Deployment) bool {
		return d.Name == name && d.Replicas == replicas && d.UserID == userID
	})).Return(nil)

	eventSvc.On("RecordEvent", ctx, "DEPLOYMENT_CREATED", mock.Anything, "DEPLOYMENT", mock.Anything).Return(nil)
	auditSvc.On("Log", ctx, userID, "container.deployment_create", "deployment", mock.Anything, mock.Anything).Return(nil)

	dep, err := svc.CreateDeployment(ctx, name, image, replicas, ports)

	assert.NoError(t, err)
	assert.NotNil(t, dep)
	assert.Equal(t, name, dep.Name)
	assert.Equal(t, replicas, dep.Replicas)

	repo.AssertExpectations(t)
	eventSvc.AssertExpectations(t)
	auditSvc.AssertExpectations(t)
}

func TestScaleDeployment_Success(t *testing.T) {
	repo := new(MockContainerRepository)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	svc := services.NewContainerService(repo, eventSvc, auditSvc)

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	deployID := uuid.New()

	existing := &domain.Deployment{
		ID:       deployID,
		UserID:   userID,
		Replicas: 1,
	}

	repo.On("GetDeploymentByID", ctx, deployID, userID).Return(existing, nil)
	repo.On("UpdateDeployment", ctx, mock.MatchedBy(func(d *domain.Deployment) bool {
		return d.Replicas == 5 && d.ID == deployID
	})).Return(nil)

	auditSvc.On("Log", ctx, userID, "container.deployment_scale", "deployment", deployID.String(), mock.Anything).Return(nil)

	err := svc.ScaleDeployment(ctx, deployID, 5)

	assert.NoError(t, err)

	repo.AssertExpectations(t)
}

func TestDeleteDeployment_Success(t *testing.T) {
	repo := new(MockContainerRepository)
	eventSvc := new(MockEventService)
	auditSvc := new(MockAuditService)
	svc := services.NewContainerService(repo, eventSvc, auditSvc)

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)
	deployID := uuid.New()

	existing := &domain.Deployment{
		ID:     deployID,
		UserID: userID,
		Status: domain.DeploymentStatusReady,
	}

	repo.On("GetDeploymentByID", ctx, deployID, userID).Return(existing, nil)
	repo.On("UpdateDeployment", ctx, mock.MatchedBy(func(d *domain.Deployment) bool {
		return d.Status == domain.DeploymentStatusDeleting
	})).Return(nil)

	auditSvc.On("Log", ctx, userID, "container.deployment_delete", "deployment", deployID.String(), mock.Anything).Return(nil)

	err := svc.DeleteDeployment(ctx, deployID)

	assert.NoError(t, err)

	repo.AssertExpectations(t)
}
