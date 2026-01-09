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

type mockSecurityGroupService struct {
	mock.Mock
}

func (m *mockSecurityGroupService) CreateGroup(ctx context.Context, vpcID uuid.UUID, name, description string) (*domain.SecurityGroup, error) {
	args := m.Called(ctx, vpcID, name, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SecurityGroup), args.Error(1)
}

func (m *mockSecurityGroupService) ListGroups(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error) {
	args := m.Called(ctx, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SecurityGroup), args.Error(1)
}

func (m *mockSecurityGroupService) GetGroup(ctx context.Context, id string, vpcID uuid.UUID) (*domain.SecurityGroup, error) {
	args := m.Called(ctx, id, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SecurityGroup), args.Error(1)
}

func (m *mockSecurityGroupService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockSecurityGroupService) AddRule(ctx context.Context, groupID uuid.UUID, rule domain.SecurityRule) (*domain.SecurityRule, error) {
	args := m.Called(ctx, groupID, rule)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SecurityRule), args.Error(1)
}

func (m *mockSecurityGroupService) AttachToInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	args := m.Called(ctx, instanceID, groupID)
	return args.Error(0)
}

func (m *mockSecurityGroupService) DetachFromInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	args := m.Called(ctx, instanceID, groupID)
	return args.Error(0)
}

func (m *mockSecurityGroupService) RemoveRule(ctx context.Context, ruleID uuid.UUID) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}

func TestSecurityGroupHandler_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	vpcID := uuid.New()
	sg := &domain.SecurityGroup{ID: uuid.New(), VPCID: vpcID, Name: "web-sg"}

	svc.On("CreateGroup", mock.Anything, vpcID, "web-sg", "web servers").Return(sg, nil)

	reqBody := map[string]interface{}{
		"vpc_id":      vpcID,
		"name":        "web-sg",
		"description": "web servers",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/security-groups", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	vpcID := uuid.New()
	groups := []*domain.SecurityGroup{{ID: uuid.New()}}

	svc.On("ListGroups", mock.Anything, vpcID).Return(groups, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/security-groups?vpc_id="+vpcID.String(), nil)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	id := uuid.New().String()
	vpcID := uuid.New()
	sg := &domain.SecurityGroup{ID: uuid.New()}

	svc.On("GetGroup", mock.Anything, id, vpcID).Return(sg, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/security-groups/"+id+"?vpc_id="+vpcID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.Get(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandler_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	id := uuid.New()

	svc.On("DeleteGroup", mock.Anything, id).Return(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/security-groups/"+id.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: id.String()}}

	h.Delete(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandler_AddRule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	groupID := uuid.New()
	rule := domain.SecurityRule{Protocol: "tcp", PortMin: 80, PortMax: 80}

	svc.On("AddRule", mock.Anything, groupID, rule).Return(&rule, nil)

	body, _ := json.Marshal(rule)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/security-groups/"+groupID.String()+"/rules", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: groupID.String()}}

	h.AddRule(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandler_Attach(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	instanceID := uuid.New()
	groupID := uuid.New()

	svc.On("AttachToInstance", mock.Anything, instanceID, groupID).Return(nil)

	reqBody := map[string]interface{}{
		"instance_id": instanceID,
		"group_id":    groupID,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/security-groups/attach", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Attach(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}
