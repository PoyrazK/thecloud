package httphandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockAccountingService struct {
	mock.Mock
}

func (m *mockAccountingService) TrackUsage(ctx context.Context, record domain.UsageRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *mockAccountingService) GetSummary(ctx context.Context, userID uuid.UUID, start, end time.Time) (*domain.BillSummary, error) {
	args := m.Called(ctx, userID, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.BillSummary)
	return r0, args.Error(1)
}

func (m *mockAccountingService) ListUsage(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.UsageRecord, error) {
	args := m.Called(ctx, userID, start, end)
	r0, _ := args.Get(0).([]domain.UsageRecord)
	return r0, args.Error(1)
}

func (m *mockAccountingService) ProcessHourlyBilling(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestAccountingHandlerGetSummary(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := new(mockAccountingService)
		handler := NewAccountingHandler(svc)

		summary := &domain.BillSummary{
			TotalAmount: 10.5,
			Currency:    "USD",
		}

		svc.On("GetSummary", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(summary, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/billing/summary", nil)

		userID := uuid.New()
		c.Set("userID", userID.String())

		handler.GetSummary(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data domain.BillSummary `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.InDelta(t, 10.5, resp.Data.TotalAmount, 0.01)
	})

	t.Run("invalid_start_time", func(t *testing.T) {
		svc := new(mockAccountingService)
		handler := NewAccountingHandler(svc)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/billing/summary?start=not-a-time", nil)

		userID := uuid.New()
		c.Set("userID", userID.String())

		handler.GetSummary(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid start time")
		assert.Contains(t, w.Body.String(), "expected RFC3339")
	})

	t.Run("invalid_end_time", func(t *testing.T) {
		svc := new(mockAccountingService)
		handler := NewAccountingHandler(svc)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/billing/summary?end=2026-13-01T00:00:00Z", nil)

		userID := uuid.New()
		c.Set("userID", userID.String())

		handler.GetSummary(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid end time")
		assert.Contains(t, w.Body.String(), "expected RFC3339")
	})
}

func TestAccountingHandlerListUsage(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		svc := new(mockAccountingService)
		handler := NewAccountingHandler(svc)

		records := []domain.UsageRecord{
			{ID: uuid.New(), ResourceID: uuid.New(), Quantity: 1.5},
		}

		svc.On("ListUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(records, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/billing/usage", nil)

		userID := uuid.New()
		c.Set("userID", userID.String())

		handler.ListUsage(c)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("invalid_start_time", func(t *testing.T) {
		svc := new(mockAccountingService)
		handler := NewAccountingHandler(svc)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/billing/usage?start=invalid", nil)

		userID := uuid.New()
		c.Set("userID", userID.String())

		handler.ListUsage(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid start time")
		assert.Contains(t, w.Body.String(), "expected RFC3339")
	})

	t.Run("invalid_end_time", func(t *testing.T) {
		svc := new(mockAccountingService)
		handler := NewAccountingHandler(svc)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/billing/usage?end=2026-13-01T00:00:00Z", nil)

		userID := uuid.New()
		c.Set("userID", userID.String())

		handler.ListUsage(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid end time")
		assert.Contains(t, w.Body.String(), "expected RFC3339")
	})
}
