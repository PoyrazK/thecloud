package services_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEventRepo
type MockEventRepo struct {
	mock.Mock
}

func (m *MockEventRepo) Create(ctx context.Context, event *domain.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}
func (m *MockEventRepo) List(ctx context.Context, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Event), args.Error(1)
}

func TestEventService_RecordEvent_Success(t *testing.T) {
	repo := new(MockEventRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewEventService(repo, logger)
	ctx := appcontext.WithUserID(context.Background(), uuid.New())

	repo.On("Create", ctx, mock.AnythingOfType("*domain.Event")).Return(nil)

	err := svc.RecordEvent(ctx, "TEST_ACTION", "res-123", "TEST", map[string]interface{}{"key": "value"})

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestEventService_RecordEvent_Failure(t *testing.T) {
	repo := new(MockEventRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewEventService(repo, logger)
	ctx := context.Background()

	repo.On("Create", ctx, mock.Anything).Return(assert.AnError)

	err := svc.RecordEvent(ctx, "FAIL_ACTION", "res-456", "TEST", nil)

	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestEventService_ListEvents_Success(t *testing.T) {
	repo := new(MockEventRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewEventService(repo, logger)
	ctx := context.Background()

	events := []*domain.Event{{Action: "A1"}, {Action: "A2"}}
	repo.On("List", ctx, 10).Return(events, nil)

	result, err := svc.ListEvents(ctx, 10)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	repo.AssertExpectations(t)
}

func TestEventService_ListEvents_DefaultLimit(t *testing.T) {
	repo := new(MockEventRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := services.NewEventService(repo, logger)
	ctx := context.Background()

	events := []*domain.Event{}
	repo.On("List", ctx, 50).Return(events, nil) // Default limit

	_, err := svc.ListEvents(ctx, 0) // Pass 0 to trigger default

	assert.NoError(t, err)
	repo.AssertCalled(t, "List", ctx, 50)
}
