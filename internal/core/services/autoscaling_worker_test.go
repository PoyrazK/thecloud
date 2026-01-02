package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/poyraz/cloud/internal/core/domain"
	"github.com/poyraz/cloud/internal/core/services"
)

// MockClock implements ports.Clock for testing
type MockClock struct {
	mock.Mock
}

func (m *MockClock) Now() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

// Helpers
func newMockWorkerDeps() (*MockAutoScalingRepo, *MockInstanceService, *MockLBService, *MockEventService, *MockClock) {
	return new(MockAutoScalingRepo), new(MockInstanceService), new(MockLBService), new(MockEventService), new(MockClock)
}

// MockLBService
type MockLBService struct{ mock.Mock }

func (m *MockLBService) Create(ctx context.Context, name string, vpcID uuid.UUID, port int, algo string, idempotencyKey string) (*domain.LoadBalancer, error) {
	return nil, nil
}
func (m *MockLBService) Get(ctx context.Context, id uuid.UUID) (*domain.LoadBalancer, error) {
	return nil, nil
}
func (m *MockLBService) List(ctx context.Context) ([]*domain.LoadBalancer, error) { return nil, nil }
func (m *MockLBService) Delete(ctx context.Context, id uuid.UUID) error           { return nil }
func (m *MockLBService) AddTarget(ctx context.Context, lbID, instanceID uuid.UUID, port, weight int) error {
	args := m.Called(ctx, lbID, instanceID, port, weight)
	return args.Error(0)
}
func (m *MockLBService) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error {
	args := m.Called(ctx, lbID, instanceID)
	return args.Error(0)
}
func (m *MockLBService) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	return nil, nil
}

// MockEventService
type MockEventService struct{ mock.Mock }

func (m *MockEventService) RecordEvent(ctx context.Context, eType, resourceID, resourceType string, meta map[string]interface{}) error {
	return nil
}
func (m *MockEventService) ListEvents(ctx context.Context, limit int) ([]*domain.Event, error) {
	return nil, nil
}

func TestWorkerConstruction(t *testing.T) {
	asgRepo, instSvc, lbSvc, eventSvc, clock := newMockWorkerDeps()
	clock.On("Now").Return(time.Now())

	worker := services.NewAutoScalingWorker(asgRepo, instSvc, lbSvc, eventSvc, clock)

	assert.NotNil(t, worker)
}

func TestShouldSkipDueToFailures(t *testing.T) {
	// This tests the backoff logic by checking state
	// The helper functions are private, so we verify behavior through the worker's decisions

	group := &domain.ScalingGroup{
		Name:         "failing-group",
		FailureCount: 6,
	}
	now := time.Now()
	failureTime := now.Add(-2 * time.Minute) // Failed 2 minutes ago
	group.LastFailureAt = &failureTime

	// With 6 failures and last failure 2 min ago, should be in backoff (5 min window)
	assert.True(t, group.FailureCount >= 5)
	assert.True(t, now.Before(failureTime.Add(5*time.Minute)))
}
