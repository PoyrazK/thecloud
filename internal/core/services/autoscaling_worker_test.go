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

func TestAutoScalingWorker_Evaluate_ScaleOut(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockInstSvc := new(MockInstanceService)
	mockLBSvc := new(MockLBService)
	mockEventSvc := new(MockEventService)
	mockClock := new(MockClock)

	worker := services.NewAutoScalingWorker(mockRepo, mockInstSvc, mockLBSvc, mockEventSvc, mockClock)

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

	// 1. List groups
	mockRepo.On("ListAllGroups", ctx).Return([]*domain.ScalingGroup{group}, nil)

	// 2. Get instances for groups
	mockRepo.On("GetAllScalingGroupInstances", ctx, []uuid.UUID{groupID}).Return(map[uuid.UUID][]uuid.UUID{
		groupID: instances,
	}, nil)

	// 3. Get policies
	mockRepo.On("GetAllPolicies", ctx, []uuid.UUID{groupID}).Return(map[uuid.UUID][]*domain.ScalingPolicy{
		groupID: {}, // No policies for this test
	}, nil)

	// 4. Update group count (reconciliation check if needed, here counts match)
	// But Desired (2) > Current (1), so it will try to Scale Out

	// 5. Scale Out
	// Launch new instance
	newInst := &domain.Instance{ID: uuid.New()}
	mockInstSvc.On("LaunchInstance", mock.Anything, mock.MatchedBy(func(name string) bool {
		return true // validating name prefix if needed
	}), "nginx", mock.Anything, mock.Anything, []domain.VolumeAttachment(nil)).Return(newInst, nil)

	// Add to group
	mockRepo.On("AddInstanceToGroup", mock.Anything, groupID, newInst.ID).Return(nil)

	// Record Event
	mockEventSvc.On("RecordEvent", mock.Anything, "AUTOSCALING_SCALE_OUT", groupID.String(), "SCALING_GROUP", mock.Anything).Return(nil)

	// Reset failures (called on success)
	// Only if failure count > 0, which is 0 here. Wait, code calls resetFailures indiscriminately in loop?
	// reconcileInstances: if err == nil { w.resetFailures(ctx, group) }
	// resetFailures checks if FailureCount > 0. Since it's 0, it does nothing.

	// Evaluate
	worker.Evaluate(ctx)

	mockRepo.AssertExpectations(t)
	mockInstSvc.AssertExpectations(t)
	mockEventSvc.AssertExpectations(t)
}

func TestAutoScalingWorker_Evaluate_ScaleIn(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockInstSvc := new(MockInstanceService)
	mockLBSvc := new(MockLBService)
	mockEventSvc := new(MockEventService)
	mockClock := new(MockClock)

	worker := services.NewAutoScalingWorker(mockRepo, mockInstSvc, mockLBSvc, mockEventSvc, mockClock)

	ctx := context.Background()
	groupID := uuid.New()
	userID := uuid.New()

	now := time.Now()
	mockClock.On("Now").Return(now)

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
	mockRepo.On("GetAllScalingGroupInstances", ctx, []uuid.UUID{groupID}).Return(map[uuid.UUID][]uuid.UUID{groupID: instances}, nil)
	mockRepo.On("GetAllPolicies", ctx, []uuid.UUID{groupID}).Return(map[uuid.UUID][]*domain.ScalingPolicy{groupID: {}}, nil)

	// Scale In: Remove last instance (inst2ID)
	mockRepo.On("RemoveInstanceFromGroup", mock.Anything, groupID, inst2ID).Return(nil)
	mockInstSvc.On("TerminateInstance", mock.Anything, inst2ID.String()).Return(nil)
	mockEventSvc.On("RecordEvent", mock.Anything, "AUTOSCALING_SCALE_IN", groupID.String(), "SCALING_GROUP", mock.Anything).Return(nil)

	worker.Evaluate(ctx)

	mockRepo.AssertExpectations(t)
	mockInstSvc.AssertExpectations(t)
}

func TestAutoScalingWorker_Evaluate_PolicyTrigger(t *testing.T) {
	mockRepo := new(MockAutoScalingRepo)
	mockInstSvc := new(MockInstanceService)
	mockLBSvc := new(MockLBService)
	mockEventSvc := new(MockEventService)
	mockClock := new(MockClock)

	worker := services.NewAutoScalingWorker(mockRepo, mockInstSvc, mockLBSvc, mockEventSvc, mockClock)

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
	mockRepo.On("GetAllScalingGroupInstances", ctx, []uuid.UUID{groupID}).Return(map[uuid.UUID][]uuid.UUID{groupID: instances}, nil)
	mockRepo.On("GetAllPolicies", ctx, []uuid.UUID{groupID}).Return(map[uuid.UUID][]*domain.ScalingPolicy{groupID: {policy}}, nil)

	// Mock CPU usage > Target (80 > 70)
	mockRepo.On("GetAverageCPU", mock.Anything, instances, mock.Anything).Return(80.0, nil)

	// Expect Scale Out Trigger
	// 1. Update Group DesiredCount (1 -> 2)
	mockRepo.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(g *domain.ScalingGroup) bool {
		return g.DesiredCount == 2
	})).Return(nil)

	// 2. Update Policy LastScaledAt
	mockRepo.On("UpdatePolicyLastScaled", mock.Anything, policyID, now).Return(nil)

	// Note: scaleOut is NOT called immediately here because evaluatePolicies only updates DesiredCount.
	worker.Evaluate(ctx)

	mockRepo.AssertExpectations(t)
}
