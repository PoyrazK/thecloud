package httphandlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockAuditService struct {
	mock.Mock
}

func (m *mockAuditService) Log(ctx context.Context, userID uuid.UUID, action, resourceType, resourceID string, details map[string]interface{}) error {
	args := m.Called(ctx, userID, action, resourceType, resourceID, details)
	return args.Error(0)
}

func (m *mockAuditService) ListLogs(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(ctx, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func TestAuditHandler_ListLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Success", func(t *testing.T) {
		svc := new(mockAuditService)
		h := NewAuditHandler(svc)

		userID := uuid.New()
		logs := []*domain.AuditLog{
			{ID: uuid.New(), UserID: userID, Action: "login"},
		}

		svc.On("ListLogs", mock.Anything, userID, 50).Return(logs, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/audit", nil)
		c.Set("userID", userID)

		h.ListLogs(c)

		assert.Equal(t, http.StatusOK, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("CustomLimit", func(t *testing.T) {
		svc := new(mockAuditService)
		h := NewAuditHandler(svc)

		userID := uuid.New()
		svc.On("ListLogs", mock.Anything, userID, 10).Return([]*domain.AuditLog{}, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/audit?limit=10", nil)
		c.Set("userID", userID)

		h.ListLogs(c)

		assert.Equal(t, http.StatusOK, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		svc := new(mockAuditService)
		h := NewAuditHandler(svc)

		userID := uuid.New()
		svc.On("ListLogs", mock.Anything, userID, 50).Return(nil, assert.AnError)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/audit", nil)
		c.Set("userID", userID)

		h.ListLogs(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}
