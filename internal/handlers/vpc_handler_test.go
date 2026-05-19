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
	"github.com/poyrazk/thecloud/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	vpcsPath    = "/vpcs"
	testVpcName = "test-vpc"
)

type mockVpcService struct {
	mock.Mock
}

func (m *mockVpcService) CreateVPC(ctx context.Context, name, cidrBlock, idempotencyKey string) (*domain.VPC, error) {
	args := m.Called(ctx, name, cidrBlock, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.VPC)
	return r0, args.Error(1)
}

func (m *mockVpcService) ListVPCs(ctx context.Context) ([]*domain.VPC, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.VPC)
	return r0, args.Error(1)
}

func (m *mockVpcService) GetVPC(ctx context.Context, idOrName string) (*domain.VPC, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.VPC)
	return r0, args.Error(1)
}

func (m *mockVpcService) DeleteVPC(ctx context.Context, idOrName string, force bool) error {
	args := m.Called(ctx, idOrName, force)
	return args.Error(0)
}

func (m *mockVpcService) UpdateVPC(ctx context.Context, idOrName, name string) (*domain.VPC, error) {
	args := m.Called(ctx, idOrName, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.VPC)
	return r0, args.Error(1)
}

func setupVpcHandlerTest(_ *testing.T) (*mockVpcService, *VpcHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockVpcService)
	handler := NewVpcHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestVpcHandlerCreate(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupVpcHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(vpcsPath, handler.Create)

	vpc := &domain.VPC{ID: uuid.New(), Name: testVpcName}
	svc.On("CreateVPC", mock.Anything, testVpcName, testutil.TestCIDR, "").Return(vpc, nil)

	body, err := json.Marshal(map[string]string{"name": testVpcName, "cidr_block": testutil.TestCIDR})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", vpcsPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestVpcHandlerCreateErrors(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupVpcHandlerTest(t)
	r.POST(vpcsPath, handler.Create)

	t.Run("InvalidJSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", vpcsPath, bytes.NewBufferString("{invalid}"))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc.On("CreateVPC", mock.Anything, "err-vpc", "", "").Return(nil, assert.AnError)
		body, _ := json.Marshal(map[string]string{"name": "err-vpc"})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", vpcsPath, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestVpcHandlerList(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupVpcHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(vpcsPath, handler.List)

	vpcs := []*domain.VPC{{ID: uuid.New(), Name: "vpc1"}}
	svc.On("ListVPCs", mock.Anything).Return(vpcs, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", vpcsPath, nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVpcHandlerGet(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupVpcHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(vpcsPath+"/:id", handler.Get)

	vpcID := uuid.New().String()
	vpc := &domain.VPC{ID: uuid.New(), Name: "vpc1"}
	svc.On("GetVPC", mock.Anything, vpcID).Return(vpc, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", vpcsPath+"/"+vpcID, nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVpcHandlerGetError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupVpcHandlerTest(t)
	r.GET(vpcsPath+"/:id", handler.Get)

	svc.On("GetVPC", mock.Anything, "non-existent").Return(nil, assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", vpcsPath+"/non-existent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestVpcHandlerDelete(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupVpcHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(vpcsPath+"/:id", handler.Delete)

	vpcID := uuid.New().String()
	svc.On("DeleteVPC", mock.Anything, vpcID, false).Return(nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", vpcsPath+"/"+vpcID, nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVpcHandlerListError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupVpcHandlerTest(t)
	r.GET(vpcsPath, handler.List)

	svc.On("ListVPCs", mock.Anything).Return(nil, assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", vpcsPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestVpcHandlerDeleteError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupVpcHandlerTest(t)
	r.DELETE(vpcsPath+"/:id", handler.Delete)

	svc.On("DeleteVPC", mock.Anything, "error-id", false).Return(assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", vpcsPath+"/error-id", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
