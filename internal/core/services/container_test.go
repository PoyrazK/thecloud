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

func TestContainerService_CreateDeployment(t *testing.T) {
	repo := new(MockContainerRepo)
	eventSvc := new(MockEventService)
	auditSvc := new(services.MockAuditService)
	svc := services.NewContainerService(repo, eventSvc, auditSvc)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)

	repo.On("CreateDeployment", ctx, mock.AnythingOfType("*domain.Deployment")).Return(nil)
	eventSvc.On("RecordEvent", ctx, "DEPLOYMENT_CREATED", mock.Anything, "DEPLOYMENT", mock.Anything).Return(nil)
	auditSvc.On("Log", ctx, userID, "deployment.create", "deployment", mock.Anything, mock.Anything).Return(nil)

	dep, err := svc.CreateDeployment(ctx, "web-app", "nginx:latest", 3, "80:80")

	assert.NoError(t, err)
	assert.NotNil(t, dep)
	assert.Equal(t, "web-app", dep.Name)
	assert.Equal(t, 3, dep.Replicas)
	repo.AssertExpectations(t)
}

func TestContainerService_ScaleDeployment(t *testing.T) {
	repo := new(MockContainerRepo)
	auditSvc := new(services.MockAuditService)
	svc := services.NewContainerService(repo, nil, auditSvc)

	userID := uuid.New()
	depID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)

	repo.On("GetDeploymentByID", ctx, depID, userID).Return(&domain.Deployment{ID: depID, UserID: userID}, nil)
	repo.On("UpdateDeployment", ctx, mock.MatchedBy(func(d *domain.Deployment) bool {
		return d.Replicas == 5
	})).Return(nil)
	auditSvc.On("Log", ctx, userID, "deployment.scale", "deployment", depID.String(), mock.Anything).Return(nil)

	err := svc.ScaleDeployment(ctx, depID, 5)
	assert.NoError(t, err)
}
