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

const (
	vpcsPath    = "/vpcs"
	testVpcName = "test-vpc"
	testCidr    = "10.0.0.0/16"
)

type mockVpcService struct {
	mock.Mock
}

func (m *mockVpcService) CreateVPC(ctx context.Context, name, cidrBlock string) (*domain.VPC, error) {
	args := m.Called(ctx, name, cidrBlock)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPC), args.Error(1)
}

func (m *mockVpcService) ListVPCs(ctx context.Context) ([]*domain.VPC, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VPC), args.Error(1)
}

func (m *mockVpcService) GetVPC(ctx context.Context, idOrName string) (*domain.VPC, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPC), args.Error(1)
}

func (m *mockVpcService) DeleteVPC(ctx context.Context, idOrName string) error {
	args := m.Called(ctx, idOrName)
	return args.Error(0)
}

func setupVpcHandlerTest(t *testing.T) (*mockVpcService, *VpcHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockVpcService)
	handler := NewVpcHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestVpcHandlerCreate(t *testing.T) {
	svc, handler, r := setupVpcHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(vpcsPath, handler.Create)

	vpc := &domain.VPC{ID: uuid.New(), Name: testVpcName}
	svc.On("CreateVPC", mock.Anything, testVpcName, testCidr).Return(vpc, nil)

	body, err := json.Marshal(map[string]string{"name": testVpcName, "cidr_block": testCidr})
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", vpcsPath, bytes.NewBuffer(body))
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestVpcHandlerCreate_Errors(t *testing.T) {
	svc, handler, r := setupVpcHandlerTest(t)
	r.POST(vpcsPath, handler.Create)

	t.Run("InvalidJSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", vpcsPath, bytes.NewBufferString("{invalid}"))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc.On("CreateVPC", mock.Anything, "err-vpc", "").Return(nil, assert.AnError)
		body, _ := json.Marshal(map[string]string{"name": "err-vpc"})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", vpcsPath, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestVpcHandlerList(t *testing.T) {
	svc, handler, r := setupVpcHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(vpcsPath, handler.List)

	vpcs := []*domain.VPC{{ID: uuid.New(), Name: "vpc1"}}
	svc.On("ListVPCs", mock.Anything).Return(vpcs, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", vpcsPath, nil)
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVpcHandlerGet(t *testing.T) {
	svc, handler, r := setupVpcHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(vpcsPath+"/:id", handler.Get)

	vpcID := uuid.New().String()
	vpc := &domain.VPC{ID: uuid.New(), Name: "vpc1"}
	svc.On("GetVPC", mock.Anything, vpcID).Return(vpc, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", vpcsPath+"/"+vpcID, nil)
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVpcHandlerGet_Error(t *testing.T) {
	svc, handler, r := setupVpcHandlerTest(t)
	r.GET(vpcsPath+"/:id", handler.Get)

	svc.On("GetVPC", mock.Anything, "non-existent").Return(nil, assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", vpcsPath+"/non-existent", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestVpcHandlerDelete(t *testing.T) {
	svc, handler, r := setupVpcHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(vpcsPath+"/:id", handler.Delete)

	vpcID := uuid.New().String()
	svc.On("DeleteVPC", mock.Anything, vpcID).Return(nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", vpcsPath+"/"+vpcID, nil)
	assert.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestVpcHandlerList_Error(t *testing.T) {
	svc, handler, r := setupVpcHandlerTest(t)
	r.GET(vpcsPath, handler.List)

	svc.On("ListVPCs", mock.Anything).Return(nil, assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", vpcsPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestVpcHandlerDelete_Error(t *testing.T) {
	svc, handler, r := setupVpcHandlerTest(t)
	r.DELETE(vpcsPath+"/:id", handler.Delete)

	svc.On("DeleteVPC", mock.Anything, "error-id").Return(assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", vpcsPath+"/error-id", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
