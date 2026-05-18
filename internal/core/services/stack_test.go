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
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	stackType      = "*domain.Stack"
	stackTestVpc   = "test-vpc"
	stackTestInst  = "test-inst"
	stackTestStack = "test-stack"
)

func setupStackServiceTest(_ *testing.T) (*MockStackRepo, *MockInstanceService, *MockVpcService, *MockVolumeService, *MockSnapshotService, ports.StackService) {
	repo := new(MockStackRepo)
	instanceSvc := new(MockInstanceService)
	vpcSvc := new(MockVpcService)
	volumeSvc := new(MockVolumeService)
	snapshotSvc := new(MockSnapshotService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := services.NewStackService(repo, rbacSvc, instanceSvc, vpcSvc, volumeSvc, snapshotSvc, logger)
	return repo, instanceSvc, vpcSvc, volumeSvc, snapshotSvc, svc
}

func TestCreateStackSuccess(t *testing.T) {
	t.Parallel()
	repo, instanceSvc, vpcSvc, _, _, svc := setupStackServiceTest(t)
	defer repo.AssertExpectations(t)
	defer instanceSvc.AssertExpectations(t)
	defer vpcSvc.AssertExpectations(t)

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

	repo.On("Create", ctx, mock.AnythingOfType(stackType)).Return(nil)

	// Async expectations
	vpcID := uuid.New()
	vpc := &domain.VPC{ID: vpcID, Name: stackTestVpc}
	vpcSvc.On("CreateVPC", mock.Anything, stackTestVpc, "", "").Return(vpc, nil)
	repo.On("AddResource", mock.Anything, mock.MatchedBy(func(r *domain.StackResource) bool {
		return r.LogicalID == "MyVPC" && r.ResourceType == "VPC"
	})).Return(nil)

	instID := uuid.New()
	inst := &domain.Instance{ID: instID, Name: stackTestInst}
	instanceSvc.On("LaunchInstance", mock.Anything, mock.MatchedBy(func(p ports.LaunchParams) bool {
		return p.Name == stackTestInst && p.Image == "alpine" && p.VpcID != nil && *p.VpcID == vpcID
	})).Return(inst, nil)
	repo.On("AddResource", mock.Anything, mock.MatchedBy(func(r *domain.StackResource) bool {
		return r.LogicalID == "MyInstance" && r.ResourceType == "Instance"
	})).Return(nil)

	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusCreateComplete
	})).Return(nil)

	stack, err := svc.CreateStack(ctx, stackTestStack, template, nil)

	require.NoError(t, err)
	assert.NotNil(t, stack)
	assert.Equal(t, stackTestStack, stack.Name)
	assert.Equal(t, domain.StackStatusCreateInProgress, stack.Status)

	// Wait for background processing
	time.Sleep(150 * time.Millisecond)
}

func TestDeleteStackSuccess(t *testing.T) {
	t.Parallel()
	repo, instanceSvc, vpcSvc, _, _, svc := setupStackServiceTest(t)
	defer repo.AssertExpectations(t)
	defer instanceSvc.AssertExpectations(t)
	defer vpcSvc.AssertExpectations(t)

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
	vpcSvc.On("DeleteVPC", mock.Anything, resources[0].PhysicalID, true).Return(nil)
	repo.On("Delete", mock.Anything, stackID).Return(nil)

	err := svc.DeleteStack(ctx, stackID)

	require.NoError(t, err)

	// Wait for background processing
	time.Sleep(150 * time.Millisecond)
}

func TestCreateStackRollback(t *testing.T) {
	t.Parallel()
	repo, instanceSvc, vpcSvc, _, _, svc := setupStackServiceTest(t)
	defer repo.AssertExpectations(t)
	defer instanceSvc.AssertExpectations(t)
	defer vpcSvc.AssertExpectations(t)

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

	repo.On("Create", ctx, mock.AnythingOfType(stackType)).Return(nil)

	// 1. VPC Success
	vpcID := uuid.New()
	vpc := &domain.VPC{ID: vpcID, Name: stackTestVpc}
	vpcSvc.On("CreateVPC", mock.Anything, stackTestVpc, "", "").Return(vpc, nil)
	repo.On("AddResource", mock.Anything, mock.MatchedBy(func(r *domain.StackResource) bool {
		return r.LogicalID == "MyVPC" && r.ResourceType == "VPC"
	})).Return(nil)

	// 2. Instance Fail
	instanceSvc.On("LaunchInstance", mock.Anything, mock.MatchedBy(func(p ports.LaunchParams) bool {
		return p.Name == stackTestInst && p.Image == "alpine" && p.VpcID != nil && *p.VpcID == vpcID
	})).Return(nil, fmt.Errorf("launch failed"))

	// 3. Rollback
	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusRollbackInProgress
	})).Return(nil)

	// stackID unused

	// Mock ListResources for rollback (simulating DB state)
	// Since we mock AddResource, the repo doesn't actually store it. We have to mock ListResources to return what would have been there.
	repo.On("ListResources", mock.Anything, mock.Anything).Return([]domain.StackResource{
		{
			LogicalID:    "MyVPC",
			PhysicalID:   vpcID.String(),
			ResourceType: "VPC",
		},
	}, nil)

	vpcSvc.On("DeleteVPC", mock.Anything, vpcID.String(), true).Return(nil)
	repo.On("DeleteResources", mock.Anything, mock.Anything).Return(nil)

	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusRollbackComplete
	})).Return(nil)

	_, err := svc.CreateStack(ctx, "test-stack-rb", template, nil)
	require.NoError(t, err)

	// Wait for background processing
	time.Sleep(150 * time.Millisecond)
}

func TestCreateStackRollbackDeletesResourcesAllTypes(t *testing.T) {
	t.Parallel()
	repo, instanceSvc, vpcSvc, volumeSvc, snapshotSvc, svc := setupStackServiceTest(t)
	defer repo.AssertExpectations(t)
	defer instanceSvc.AssertExpectations(t)
	defer vpcSvc.AssertExpectations(t)
	defer volumeSvc.AssertExpectations(t)
	defer snapshotSvc.AssertExpectations(t)

	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	template := `
Resources:
  MyVPC:
    Type: VPC
    Properties:
      Name: test-vpc
  MyVolume:
    Type: Volume
    Properties:
      Name: vol1
      Size: 20
`

	repo.On("Create", ctx, mock.AnythingOfType(stackType)).Return(nil)

	vpcID := uuid.New()
	vpc := &domain.VPC{ID: vpcID, Name: stackTestVpc}
	vpcSvc.On("CreateVPC", mock.Anything, stackTestVpc, "", "").Return(vpc, nil)
	repo.On("AddResource", mock.Anything, mock.MatchedBy(func(r *domain.StackResource) bool {
		return r.LogicalID == "MyVPC" && r.ResourceType == "VPC"
	})).Return(nil)

	volumeSvc.On("CreateVolume", mock.Anything, "vol1", 20).Return(nil, fmt.Errorf("volume failed"))

	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusRollbackInProgress
	})).Return(nil)

	instanceID := uuid.New()
	volumeID := uuid.New()
	snapshotID := uuid.New()
	repo.On("ListResources", mock.Anything, mock.Anything).Return([]domain.StackResource{
		{LogicalID: "MyVPC", PhysicalID: vpcID.String(), ResourceType: "VPC"},
		{LogicalID: "MyVolume", PhysicalID: volumeID.String(), ResourceType: "Volume"},
		{LogicalID: "MyInstance", PhysicalID: instanceID.String(), ResourceType: "Instance"},
		{LogicalID: "MySnapshot", PhysicalID: snapshotID.String(), ResourceType: "Snapshot"},
	}, nil)

	instanceSvc.On("TerminateInstance", mock.Anything, instanceID.String()).Return(nil)
	vpcSvc.On("DeleteVPC", mock.Anything, vpcID.String(), true).Return(nil)
	volumeSvc.On("DeleteVolume", mock.Anything, volumeID.String()).Return(nil)
	snapshotSvc.On("DeleteSnapshot", mock.Anything, snapshotID).Return(nil)
	repo.On("DeleteResources", mock.Anything, mock.Anything).Return(nil)

	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusRollbackComplete
	})).Return(nil)

	_, err := svc.CreateStack(ctx, "test-stack-rollback-all", template, nil)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
}

func TestCreateStackRollbackFailureUpdatesStatus(t *testing.T) {
	t.Parallel()
	repo, instanceSvc, vpcSvc, volumeSvc, _, svc := setupStackServiceTest(t)
	defer repo.AssertExpectations(t)
	defer instanceSvc.AssertExpectations(t)
	defer vpcSvc.AssertExpectations(t)
	defer volumeSvc.AssertExpectations(t)

	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	template := `
Resources:
  MyVPC:
    Type: VPC
    Properties:
      Name: test-vpc
  MyVolume:
    Type: Volume
    Properties:
      Name: vol1
      Size: 20
`

	repo.On("Create", ctx, mock.AnythingOfType(stackType)).Return(nil)

	vpcID := uuid.New()
	vpc := &domain.VPC{ID: vpcID, Name: stackTestVpc}
	vpcSvc.On("CreateVPC", mock.Anything, stackTestVpc, "", "").Return(vpc, nil)
	repo.On("AddResource", mock.Anything, mock.MatchedBy(func(r *domain.StackResource) bool {
		return r.LogicalID == "MyVPC" && r.ResourceType == "VPC"
	})).Return(nil)

	volumeSvc.On("CreateVolume", mock.Anything, "vol1", 20).Return(nil, fmt.Errorf("volume failed"))

	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusRollbackInProgress
	})).Return(nil)
	repo.On("ListResources", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("list error"))
	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusRollbackFailed
	})).Return(nil)

	_, err := svc.CreateStack(ctx, "test-stack-rollback-fail", template, nil)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
}
func TestGetStack(t *testing.T) {
	t.Parallel()
	repo, _, _, _, _, svc := setupStackServiceTest(t)
	defer repo.AssertExpectations(t)

	ctx := context.Background()
	stackID := uuid.New()
	stack := &domain.Stack{ID: stackID, Name: stackTestStack}

	repo.On("GetByID", ctx, stackID).Return(stack, nil)

	res, err := svc.GetStack(ctx, stackID)
	require.NoError(t, err)
	assert.Equal(t, stack, res)
}

func TestListStacks(t *testing.T) {
	t.Parallel()
	repo, _, _, _, _, svc := setupStackServiceTest(t)
	defer repo.AssertExpectations(t)

	userID := uuid.New()
	ctx := appcontext.WithUserID(context.Background(), userID)
	stacks := []*domain.Stack{{ID: uuid.New(), Name: "s1"}}

	repo.On("ListByUserID", ctx, userID).Return(stacks, nil)

	res, err := svc.ListStacks(ctx)
	require.NoError(t, err)
	assert.Equal(t, stacks, res)
}

func TestValidateTemplate(t *testing.T) {
	t.Parallel()
	_, _, _, _, _, svc := setupStackServiceTest(t)

	t.Run("valid template", func(t *testing.T) {
		template := `
Resources:
  MyVPC:
    Type: VPC
    Properties:
      Name: test-vpc
`
		res, err := svc.ValidateTemplate(context.Background(), template)
		require.NoError(t, err)
		assert.True(t, res.Valid)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		template := `
Resources:
  MyVPC:
    Type: VPC
  - invalid
`
		res, err := svc.ValidateTemplate(context.Background(), template)
		require.NoError(t, err) // It returns error in response object
		assert.False(t, res.Valid)
		assert.NotEmpty(t, res.Errors)
	})

	t.Run("missing resources", func(t *testing.T) {
		template := "Parameters: {}"
		res, err := svc.ValidateTemplate(context.Background(), template)
		require.NoError(t, err)
		assert.False(t, res.Valid)
	})
}

func TestCreateStackComplex(t *testing.T) {
	t.Parallel()
	repo, instanceSvc, vpcSvc, volumeSvc, snapshotSvc, svc := setupStackServiceTest(t)
	defer repo.AssertExpectations(t)
	defer instanceSvc.AssertExpectations(t)
	defer vpcSvc.AssertExpectations(t)
	defer volumeSvc.AssertExpectations(t)
	defer snapshotSvc.AssertExpectations(t)

	ctx := appcontext.WithUserID(context.Background(), uuid.New())
	template := `
Resources:
  MyVolume:
    Type: Volume
    Properties:
      Name: vol1
      Size: 20
  MySnapshot:
    Type: Snapshot
    Properties:
      Name: snap1
      VolumeID: 
        Ref: MyVolume
`
	repo.On("Create", ctx, mock.AnythingOfType(stackType)).Return(nil)

	volID := uuid.New()
	vol := &domain.Volume{ID: volID, Name: "vol1"}
	volumeSvc.On("CreateVolume", mock.Anything, "vol1", 20).Return(vol, nil)
	repo.On("AddResource", mock.Anything, mock.MatchedBy(func(r *domain.StackResource) bool {
		return r.LogicalID == "MyVolume" && r.ResourceType == "Volume"
	})).Return(nil)

	snapID := uuid.New()
	snap := &domain.Snapshot{ID: snapID, Description: "snap1"}
	snapshotSvc.On("CreateSnapshot", mock.Anything, volID, "snap1").Return(snap, nil)
	repo.On("AddResource", mock.Anything, mock.MatchedBy(func(r *domain.StackResource) bool {
		return r.LogicalID == "MySnapshot" && r.ResourceType == "Snapshot"
	})).Return(nil)

	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *domain.Stack) bool {
		return s.Status == domain.StackStatusCreateComplete
	})).Return(nil)

	_, err := svc.CreateStack(ctx, "complex-stack", template, nil)
	require.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
}
