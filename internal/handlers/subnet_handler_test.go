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

type mockSubnetService struct {
	mock.Mock
}

func (m *mockSubnetService) CreateSubnet(ctx context.Context, vpcID uuid.UUID, name, cidrBlock, az string) (*domain.Subnet, error) {
	args := m.Called(ctx, vpcID, name, cidrBlock, az)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subnet), args.Error(1)
}

func (m *mockSubnetService) ListSubnets(ctx context.Context, vpcID uuid.UUID) ([]*domain.Subnet, error) {
	args := m.Called(ctx, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Subnet), args.Error(1)
}

func (m *mockSubnetService) GetSubnet(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.Subnet, error) {
	args := m.Called(ctx, idOrName, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subnet), args.Error(1)
}

func (m *mockSubnetService) DeleteSubnet(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestSubnetHandler_Create_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSubnetService)
	handler := NewSubnetHandler(svc)

	vpcID := uuid.New()
	subnetID := uuid.New()

	expectedSubnet := &domain.Subnet{
		ID:        subnetID,
		VPCID:     vpcID,
		Name:      "test-subnet",
		CIDRBlock: "10.0.1.0/24",
	}

	svc.On("CreateSubnet", mock.Anything, vpcID, "test-subnet", "10.0.1.0/24", "us-east-1a").Return(expectedSubnet, nil)

	reqBody := map[string]string{
		"name":              "test-subnet",
		"cidr_block":        "10.0.1.0/24",
		"availability_zone": "us-east-1a",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/vpcs/"+vpcID.String()+"/subnets", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "vpc_id", Value: vpcID.String()}}

	handler.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestSubnetHandler_Create_InvalidVpcID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSubnetService)
	handler := NewSubnetHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/vpcs/invalid-uuid/subnets", nil)
	c.Params = gin.Params{{Key: "vpc_id", Value: "invalid-uuid"}}

	handler.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestSubnetHandler_Create_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSubnetService)
	handler := NewSubnetHandler(svc)

	vpcID := uuid.New()

	reqBody := map[string]string{
		"name": "missing-cidr",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/vpcs/"+vpcID.String()+"/subnets", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "vpc_id", Value: vpcID.String()}}

	handler.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestSubnetHandler_List_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSubnetService)
	handler := NewSubnetHandler(svc)

	vpcID := uuid.New()

	expectedSubnets := []*domain.Subnet{
		{ID: uuid.New(), Name: "subnet-1"},
		{ID: uuid.New(), Name: "subnet-2"},
	}

	svc.On("ListSubnets", mock.Anything, vpcID).Return(expectedSubnets, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/vpcs/"+vpcID.String()+"/subnets", nil)
	c.Params = gin.Params{{Key: "vpc_id", Value: vpcID.String()}}

	handler.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSubnetHandler_List_InvalidVpcID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSubnetService)
	handler := NewSubnetHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/vpcs/invalid/subnets", nil)
	c.Params = gin.Params{{Key: "vpc_id", Value: "invalid"}}

	handler.List(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestSubnetHandler_Get_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSubnetService)
	handler := NewSubnetHandler(svc)

	subnetID := uuid.New()

	expectedSubnet := &domain.Subnet{
		ID:   subnetID,
		Name: "test-subnet",
	}

	svc.On("GetSubnet", mock.Anything, subnetID.String(), uuid.Nil).Return(expectedSubnet, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/subnets/"+subnetID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: subnetID.String()}}

	handler.Get(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSubnetHandler_Get_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSubnetService)
	handler := NewSubnetHandler(svc)

	subnetID := uuid.New()

	svc.On("GetSubnet", mock.Anything, subnetID.String(), uuid.Nil).Return(nil, assert.AnError)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/subnets/"+subnetID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: subnetID.String()}}

	handler.Get(c)

	assert.NotEqual(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSubnetHandler_Delete_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSubnetService)
	handler := NewSubnetHandler(svc)

	subnetID := uuid.New()

	svc.On("DeleteSubnet", mock.Anything, subnetID).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/subnets/"+subnetID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: subnetID.String()}}

	handler.Delete(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSubnetHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSubnetService)
	handler := NewSubnetHandler(svc)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/subnets/invalid", nil)
	c.Params = gin.Params{{Key: "id", Value: "invalid"}}

	handler.Delete(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestSubnetHandler_Delete_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSubnetService)
	handler := NewSubnetHandler(svc)

	subnetID := uuid.New()

	svc.On("DeleteSubnet", mock.Anything, subnetID).Return(assert.AnError)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/subnets/"+subnetID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: subnetID.String()}}

	handler.Delete(c)

	assert.NotEqual(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}
