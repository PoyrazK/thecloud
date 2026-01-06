package services_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateStack_Success(t *testing.T) {
	repo := new(MockStackRepo)
	instanceSvc := new(MockInstanceService)
	vpcSvc := new(MockVpcService)
	volumeSvc := new(MockVolumeService)
	snapshotSvc := new(MockSnapshotService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewStackService(repo, instanceSvc, vpcSvc, volumeSvc, snapshotSvc, logger)

	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	template := `
Resources:
  MyVPC:
    Type: VPC
    Properties:
      Name: test-vpc
  MyInstance:
    Type: Instance
    Properties:
      Name: test-inst
      Image: alpine
      VpcID: 
        Ref: MyVPC
`

	repo.On("Create", ctx, mock.AnythingOfType("*domain.Stack")).Return(nil)

	// Async expectations
	vpcID := uuid.New()
	vpc := &domain.VPC{ID: vpcID, Name: "test-vpc"}
	vpcSvc.On("CreateVPC", mock.Anything, "test-vpc").Return(vpc, nil)
	repo.On("AddResource", mock.Anything, mock.MatchedBy(func(r *domain.StackResource) bool {
		return r.LogicalID == "MyVPC" && r.ResourceType == "VPC"
	})).Return(nil)

	instID := uuid.New()
	inst := &domain.Instance{ID: instID, Name: "test-inst"}
	instanceSvc.On("LaunchInstance", mock.Anything, "test-inst", "alpine", "80", &vpcID, mock.Anything).Return(inst, nil)
	repo.On("AddResource", mock.Anything, mock.MatchedBy(func(r *domain.StackResource) bool {
		return r.LogicalID == "MyInstance" && r.ResourceType == "Instance"
	})).Return(nil)

	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusCreateComplete
	})).Return(nil)

	stack, err := svc.CreateStack(ctx, "test-stack", template, nil)

	assert.NoError(t, err)
	assert.NotNil(t, stack)
	assert.Equal(t, "test-stack", stack.Name)
	assert.Equal(t, domain.StackStatusCreateInProgress, stack.Status)

	// Wait for background processing
	time.Sleep(150 * time.Millisecond)

	repo.AssertExpectations(t)
	vpcSvc.AssertExpectations(t)
	instanceSvc.AssertExpectations(t)
}

func TestDeleteStack_Success(t *testing.T) {
	repo := new(MockStackRepo)
	instanceSvc := new(MockInstanceService)
	vpcSvc := new(MockVpcService)
	volumeSvc := new(MockVolumeService)
	snapshotSvc := new(MockSnapshotService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewStackService(repo, instanceSvc, vpcSvc, volumeSvc, snapshotSvc, logger)

	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	stackID := uuid.New()
	stack := &domain.Stack{ID: stackID, UserID: appcontext.UserIDFromContext(ctx)}

	repo.On("GetByID", ctx, stackID).Return(stack, nil)

	resources := []domain.StackResource{
		{LogicalID: "MyVPC", PhysicalID: uuid.New().String(), ResourceType: "VPC"},
		{LogicalID: "MyInstance", PhysicalID: uuid.New().String(), ResourceType: "Instance"},
	}
	repo.On("ListResources", mock.Anything, stackID).Return(resources, nil)

	instanceSvc.On("TerminateInstance", mock.Anything, resources[1].PhysicalID).Return(nil)
	vpcSvc.On("DeleteVPC", mock.Anything, resources[0].PhysicalID).Return(nil)
	repo.On("Delete", mock.Anything, stackID).Return(nil)

	err := svc.DeleteStack(ctx, stackID)

	assert.NoError(t, err)

	// Wait for background processing
	time.Sleep(150 * time.Millisecond)

	repo.AssertExpectations(t)
	instanceSvc.AssertExpectations(t)
	vpcSvc.AssertExpectations(t)
}

func TestCreateStack_Rollback(t *testing.T) {
	repo := new(MockStackRepo)
	instanceSvc := new(MockInstanceService)
	vpcSvc := new(MockVpcService)
	volumeSvc := new(MockVolumeService)
	snapshotSvc := new(MockSnapshotService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewStackService(repo, instanceSvc, vpcSvc, volumeSvc, snapshotSvc, logger)

	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	template := `
Resources:
  MyVPC:
    Type: VPC
    Properties:
      Name: test-vpc
  MyInstance:
    Type: Instance
    Properties:
      Name: test-inst
      Image: alpine
      VpcID: 
        Ref: MyVPC
`

	repo.On("Create", ctx, mock.AnythingOfType("*domain.Stack")).Return(nil)

	// 1. VPC Success
	vpcID := uuid.New()
	vpc := &domain.VPC{ID: vpcID, Name: "test-vpc"}
	vpcSvc.On("CreateVPC", mock.Anything, "test-vpc").Return(vpc, nil)
	repo.On("AddResource", mock.Anything, mock.MatchedBy(func(r *domain.StackResource) bool {
		return r.LogicalID == "MyVPC" && r.ResourceType == "VPC"
	})).Return(nil)

	// 2. Instance Fail
	instanceSvc.On("LaunchInstance", mock.Anything, "test-inst", "alpine", "80", &vpcID, mock.Anything).Return(nil, fmt.Errorf("launch failed"))

	// 3. Rollback
	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusRollbackInProgress
	})).Return(nil)

	// Mock ListResources for rollback (simulating DB state)
	// Since we mock AddResource, the repo doesn't actually store it. We have to mock ListResources to return what would have been there.
	repo.On("ListResources", mock.Anything, mock.Anything).Return([]domain.StackResource{
		{
			LogicalID:    "MyVPC",
			PhysicalID:   vpcID.String(),
			ResourceType: "VPC",
		},
	}, nil)

	vpcSvc.On("DeleteVPC", mock.Anything, vpcID.String()).Return(nil)
	repo.On("DeleteResources", mock.Anything, mock.Anything).Return(nil)

	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusRollbackComplete
	})).Return(nil)

	_, err := svc.CreateStack(ctx, "test-stack-rb", template, nil)
	assert.NoError(t, err)

	// Wait for background processing
	time.Sleep(150 * time.Millisecond)

	repo.AssertExpectations(t)
	vpcSvc.AssertExpectations(t)
	instanceSvc.AssertExpectations(t)
}
