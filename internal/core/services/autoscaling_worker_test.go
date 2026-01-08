package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/mock"
)

func setupAutoScalingWorkerTest(t *testing.T) (*MockAutoScalingRepo, *MockInstanceService, *MockLBService, *MockEventService, *MockClock, *services.AutoScalingWorker) {
	mockRepo := new(MockAutoScalingRepo)
	mockInstSvc := new(MockInstanceService)
	mockLBSvc := new(MockLBService)
	mockEventSvc := new(MockEventService)
	mockClock := new(MockClock)

	worker := services.NewAutoScalingWorker(mockRepo, mockInstSvc, mockLBSvc, mockEventSvc, mockClock)
	return mockRepo, mockInstSvc, mockLBSvc, mockEventSvc, mockClock, worker
}

func TestAutoScalingWorker_Evaluate_ScaleOut(t *testing.T) {
	mockRepo, mockInstSvc, _, mockEventSvc, mockClock, worker := setupAutoScalingWorkerTest(t)
	defer mockRepo.AssertExpectations(t)
	defer mockInstSvc.AssertExpectations(t)
	defer mockEventSvc.AssertExpectations(t)
	defer mockClock.AssertExpectations(t)

	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()

	now := time.Now()
	mockClock.On("Now").Return(now)

	// Setup Group
	group := &domain.ScalingGroup{
		ID:           groupID,
		UserID:       userID,
		Name:         "test-asg",
		MinInstances: 1,
		MaxInstances: 5,
		DesiredCount: 2,
		CurrentCount: 1,
		Image:        "nginx",
		Ports:        "80:80",
		Status:       domain.ScalingGroupStatusActive,
	}

	// Current State: 1 instance running
	inst1ID := uuid.New()
	instances := []uuid.UUID{inst1ID}

	// Expectations
	mockRepo.On("ListAllGroups", ctx).Return([]*domain.ScalingGroup{group}, nil)
	mockRepo.On("GetAllScalingGroupInstances", ctx, mock.Anything).Return(map[uuid.UUID][]uuid.UUID{groupID: instances}, nil)
	mockRepo.On("GetAllPolicies", ctx, mock.Anything).Return(map[uuid.UUID][]*domain.ScalingPolicy{groupID: {}}, nil)

	newInst := &domain.Instance{ID: uuid.New()}
	mockInstSvc.On("LaunchInstance", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(newInst, nil)
	mockRepo.On("AddInstanceToGroup", mock.Anything, groupID, newInst.ID).Return(nil)
	mockEventSvc.On("RecordEvent", mock.Anything, "AUTOSCALING_SCALE_OUT", groupID.String(), "SCALING_GROUP", mock.Anything).Return(nil)

	worker.Evaluate(ctx)
}

func TestAutoScalingWorker_Evaluate_ScaleIn(t *testing.T) {
	mockRepo, mockInstSvc, _, mockEventSvc, mockClock, worker := setupAutoScalingWorkerTest(t)
	defer mockRepo.AssertExpectations(t)
	defer mockInstSvc.AssertExpectations(t)
	defer mockClock.AssertExpectations(t)

	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()

	// Setup Group
	group := &domain.ScalingGroup{
		ID:           groupID,
		UserID:       userID,
		Name:         "test-asg-in",
		MinInstances: 1,
		MaxInstances: 5,
		DesiredCount: 1,
		CurrentCount: 2, // Excess
		Status:       domain.ScalingGroupStatusActive,
	}

	inst1ID := uuid.New()
	inst2ID := uuid.New()
	instances := []uuid.UUID{inst1ID, inst2ID}

	mockRepo.On("ListAllGroups", ctx).Return([]*domain.ScalingGroup{group}, nil)
	mockRepo.On("GetAllScalingGroupInstances", ctx, mock.Anything).Return(map[uuid.UUID][]uuid.UUID{groupID: instances}, nil)
	mockRepo.On("GetAllPolicies", ctx, mock.Anything).Return(map[uuid.UUID][]*domain.ScalingPolicy{groupID: {}}, nil)

	// Scale In: Remove last instance (inst2ID)
	// Using mock.Anything for everything to avoid matching issues
	mockRepo.On("RemoveInstanceFromGroup", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockInstSvc.On("TerminateInstance", mock.Anything, mock.Anything).Return(nil)
	mockEventSvc.On("RecordEvent", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	worker.Evaluate(ctx)
}

func TestAutoScalingWorker_Evaluate_PolicyTrigger(t *testing.T) {
	mockRepo, _, _, _, mockClock, worker := setupAutoScalingWorkerTest(t)
	defer mockRepo.AssertExpectations(t)
	defer mockClock.AssertExpectations(t)

	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()
	policyID := uuid.New()

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	mockClock.On("Now").Return(now)

	group := &domain.ScalingGroup{
		ID:           groupID,
		UserID:       userID,
		Name:         "test-asg-policy",
		MinInstances: 1,
		MaxInstances: 5,
		DesiredCount: 1,
		CurrentCount: 1,
		Status:       domain.ScalingGroupStatusActive,
		Image:        "nginx",
		Ports:        "80:80",
	}

	inst1ID := uuid.New()
	instances := []uuid.UUID{inst1ID}

	policy := &domain.ScalingPolicy{
		ID:           policyID,
		Name:         "cpu-high",
		MetricType:   "cpu",
		TargetValue:  70.0,
		ScaleOutStep: 1,
		ScaleInStep:  1,
		CooldownSec:  300,
		LastScaledAt: nil, // Ready to scale
	}

	mockRepo.On("ListAllGroups", ctx).Return([]*domain.ScalingGroup{group}, nil)
	mockRepo.On("GetAllScalingGroupInstances", ctx, mock.Anything).Return(map[uuid.UUID][]uuid.UUID{groupID: instances}, nil)
	mockRepo.On("GetAllPolicies", ctx, mock.Anything).Return(map[uuid.UUID][]*domain.ScalingPolicy{groupID: {policy}}, nil)

	mockRepo.On("GetAverageCPU", mock.Anything, mock.Anything, mock.Anything).Return(80.0, nil)
	mockRepo.On("UpdateGroup", mock.Anything, mock.Anything).Return(nil)
	mockRepo.On("UpdatePolicyLastScaled", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	worker.Evaluate(ctx)
}
