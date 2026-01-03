package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
)

func TestCreateGroup_SecurityLimits(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockVpcRepo := new(MockVpcRepo)
	svc := services.NewAutoScalingService(mockRepo, mockVpcRepo)
	ctx := context.Background()
	vpcID := uuid.New()

	mockVpcRepo.On("GetByID", ctx, vpcID).Return(&domain.VPC{ID: vpcID}, nil)

	t.Run("ExceedsMaxInstances", func(t *testing.T) {
		_, err := svc.CreateGroup(ctx, "test", vpcID, "img", "80:80", 1, 1000, 1, nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max_instances cannot exceed")
	})

	t.Run("ExceedsVPCLimit", func(t *testing.T) {
		mockRepo.On("CountGroupsByVPC", ctx, vpcID).Return(10, nil)
		_, err := svc.CreateGroup(ctx, "test", vpcID, "img", "80:80", 1, 5, 1, nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "VPC already has")
	})
}

func TestCreateGroup_Success(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockVpcRepo := new(MockVpcRepo)
	svc := services.NewAutoScalingService(mockRepo, mockVpcRepo)
	ctx := context.Background()
	vpcID := uuid.New()

	mockVpcRepo.On("GetByID", ctx, vpcID).Return(&domain.VPC{ID: vpcID}, nil)
	mockRepo.On("CountGroupsByVPC", ctx, vpcID).Return(0, nil)
	mockRepo.On("CreateGroup", ctx, mock.AnythingOfType("*domain.ScalingGroup")).Return(nil)

	group, err := svc.CreateGroup(ctx, "my-asg", vpcID, "nginx", "80:80", 1, 5, 2, nil, "")

	assert.NoError(t, err)
	assert.NotNil(t, group)
	assert.Equal(t, "my-asg", group.Name)
	assert.Equal(t, 1, group.MinInstances)
	assert.Equal(t, 5, group.MaxInstances)
	assert.Equal(t, 2, group.DesiredCount)
	mockRepo.AssertExpectations(t)
}

func TestCreateGroup_Idempotency(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockVpcRepo := new(MockVpcRepo)
	svc := services.NewAutoScalingService(mockRepo, mockVpcRepo)
	ctx := context.Background()
	vpcID := uuid.New()

	existingGroup := &domain.ScalingGroup{ID: uuid.New(), Name: "existing", IdempotencyKey: "key123"}
	mockRepo.On("GetGroupByIdempotencyKey", ctx, "key123").Return(existingGroup, nil)

	group, err := svc.CreateGroup(ctx, "new-name", vpcID, "nginx", "80:80", 1, 5, 2, nil, "key123")

	assert.NoError(t, err)
	assert.Equal(t, existingGroup.ID, group.ID)
	mockRepo.AssertNotCalled(t, "CreateGroup", mock.Anything, mock.Anything)
}

func TestCreateGroup_ValidationErrors(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockVpcRepo := new(MockVpcRepo)
	svc := services.NewAutoScalingService(mockRepo, mockVpcRepo)
	ctx := context.Background()
	vpcID := uuid.New()

	t.Run("NegativeMin", func(t *testing.T) {
		_, err := svc.CreateGroup(ctx, "test", vpcID, "img", "", -1, 5, 1, nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be negative")
	})

	t.Run("MinGreaterThanMax", func(t *testing.T) {
		_, err := svc.CreateGroup(ctx, "test", vpcID, "img", "", 5, 2, 3, nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be greater than max")
	})

	t.Run("DesiredOutOfRange", func(t *testing.T) {
		_, err := svc.CreateGroup(ctx, "test", vpcID, "img", "", 2, 5, 10, nil, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "between min and max")
	})
}

func TestDeleteGroup_Success(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockVpcRepo := new(MockVpcRepo)
	svc := services.NewAutoScalingService(mockRepo, mockVpcRepo)
	ctx := context.Background()
	groupID := uuid.New()

	group := &domain.ScalingGroup{ID: groupID, Status: domain.ScalingGroupStatusActive, MinInstances: 1, DesiredCount: 2}
	mockRepo.On("GetGroupByID", ctx, groupID).Return(group, nil)
	mockRepo.On("UpdateGroup", ctx, mock.MatchedBy(func(g *domain.ScalingGroup) bool {
		return g.Status == domain.ScalingGroupStatusDeleting && g.DesiredCount == 0 && g.MinInstances == 0
	})).Return(nil)

	err := svc.DeleteGroup(ctx, groupID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestSetDesiredCapacity_Success(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockVpcRepo := new(MockVpcRepo)
	svc := services.NewAutoScalingService(mockRepo, mockVpcRepo)
	ctx := context.Background()
	groupID := uuid.New()

	group := &domain.ScalingGroup{ID: groupID, MinInstances: 1, MaxInstances: 10, DesiredCount: 2}
	mockRepo.On("GetGroupByID", ctx, groupID).Return(group, nil)
	mockRepo.On("UpdateGroup", ctx, mock.MatchedBy(func(g *domain.ScalingGroup) bool {
		return g.DesiredCount == 5
	})).Return(nil)

	err := svc.SetDesiredCapacity(ctx, groupID, 5)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestSetDesiredCapacity_OutOfRange(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockVpcRepo := new(MockVpcRepo)
	svc := services.NewAutoScalingService(mockRepo, mockVpcRepo)
	ctx := context.Background()
	groupID := uuid.New()

	group := &domain.ScalingGroup{ID: groupID, MinInstances: 2, MaxInstances: 5, DesiredCount: 3}
	mockRepo.On("GetGroupByID", ctx, groupID).Return(group, nil)

	err := svc.SetDesiredCapacity(ctx, groupID, 100)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be between")
}

func TestCreatePolicy_Success(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockVpcRepo := new(MockVpcRepo)
	svc := services.NewAutoScalingService(mockRepo, mockVpcRepo)
	ctx := context.Background()
	groupID := uuid.New()

	mockRepo.On("GetGroupByID", ctx, groupID).Return(&domain.ScalingGroup{ID: groupID}, nil)
	mockRepo.On("CreatePolicy", ctx, mock.AnythingOfType("*domain.ScalingPolicy")).Return(nil)

	policy, err := svc.CreatePolicy(ctx, groupID, "cpu-high", "cpu", 70.0, 1, 1, 300)

	assert.NoError(t, err)
	assert.NotNil(t, policy)
	assert.Equal(t, "cpu-high", policy.Name)
	assert.Equal(t, 70.0, policy.TargetValue)
	mockRepo.AssertExpectations(t)
}

func TestCreatePolicy_CooldownTooLow(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockVpcRepo := new(MockVpcRepo)
	svc := services.NewAutoScalingService(mockRepo, mockVpcRepo)
	ctx := context.Background()
	groupID := uuid.New()

	mockRepo.On("GetGroupByID", ctx, groupID).Return(&domain.ScalingGroup{ID: groupID}, nil)

	_, err := svc.CreatePolicy(ctx, groupID, "cpu-high", "cpu", 70.0, 1, 1, 10) // Too low cooldown

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cooldown must be at least")
}

func TestListGroups(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockVpcRepo := new(MockVpcRepo)
	svc := services.NewAutoScalingService(mockRepo, mockVpcRepo)
	ctx := context.Background()

	groups := []*domain.ScalingGroup{{Name: "asg1"}, {Name: "asg2"}}
	mockRepo.On("ListGroups", ctx).Return(groups, nil)

	result, err := svc.ListGroups(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	mockRepo.AssertExpectations(t)
}
