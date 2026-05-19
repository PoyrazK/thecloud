package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockVPCPeeringService struct {
	mock.Mock
}

func (m *mockVPCPeeringService) CreatePeering(ctx context.Context, reqVPCID, accVPCID, accTenantID uuid.UUID) (*domain.VPCPeering, error) {
	args := m.Called(ctx, reqVPCID, accVPCID, accTenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPCPeering), args.Error(1)
}

func (m *mockVPCPeeringService) AcceptPeering(ctx context.Context, id uuid.UUID) (*domain.VPCPeering, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPCPeering), args.Error(1)
}

func (m *mockVPCPeeringService) RejectPeering(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockVPCPeeringService) DeletePeering(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockVPCPeeringService) GetPeering(ctx context.Context, id uuid.UUID) (*domain.VPCPeering, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPCPeering), args.Error(1)
}

func (m *mockVPCPeeringService) ListPeerings(ctx context.Context) ([]*domain.VPCPeering, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VPCPeering), args.Error(1)
}

func setupVPCPeeringHandlerTest() (*mockVPCPeeringService, *VPCPeeringHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockVPCPeeringService)
	handler := NewVPCPeeringHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestVPCPeeringHandler(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		svc, handler, r := setupVPCPeeringHandlerTest()
		r.POST("/vpc-peerings", handler.Create)

		reqVPC := uuid.New()
		accVPC := uuid.New()
		accTenant := uuid.New()
		svc.On("CreatePeering", mock.Anything, reqVPC, accVPC, accTenant).Return(&domain.VPCPeering{ID: uuid.New()}, nil)

		body, _ := json.Marshal(map[string]interface{}{
			"requester_vpc_id":   reqVPC,
			"accepter_vpc_id":    accVPC,
			"accepter_tenant_id": accTenant,
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/vpc-peerings", bytes.NewBuffer(body))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("List", func(t *testing.T) {
		svc, handler, r := setupVPCPeeringHandlerTest()
		r.GET("/vpc-peerings", handler.List)

		svc.On("ListPeerings", mock.Anything).Return([]*domain.VPCPeering{}, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/vpc-peerings", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Accept", func(t *testing.T) {
		svc, handler, r := setupVPCPeeringHandlerTest()
		r.POST("/vpc-peerings/:id/accept", handler.Accept)

		id := uuid.New()
		svc.On("AcceptPeering", mock.Anything, id).Return(&domain.VPCPeering{ID: id}, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/vpc-peerings/"+id.String()+"/accept", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
