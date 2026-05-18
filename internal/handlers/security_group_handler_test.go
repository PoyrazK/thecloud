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
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testSgName       = "web-sg"
	testContentType  = "Content-Type"
	testAppJSON      = "application/json"
	testSgPath       = "/security-groups"
	testSgDetailPath = "/security-groups/"
	vpcIDQuery       = "?vpc_id="
)

type mockSecurityGroupService struct {
	mock.Mock
}

func (m *mockSecurityGroupService) CreateGroup(ctx context.Context, vpcID uuid.UUID, name, description string) (*domain.SecurityGroup, error) {
	args := m.Called(ctx, vpcID, name, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.SecurityGroup)
	return r0, args.Error(1)
}

func (m *mockSecurityGroupService) ListGroups(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error) {
	args := m.Called(ctx, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.SecurityGroup)
	return r0, args.Error(1)
}

func (m *mockSecurityGroupService) GetGroup(ctx context.Context, id string, vpcID uuid.UUID) (*domain.SecurityGroup, error) {
	args := m.Called(ctx, id, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.SecurityGroup)
	return r0, args.Error(1)
}

func (m *mockSecurityGroupService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockSecurityGroupService) AddRule(ctx context.Context, idOrName string, rule domain.SecurityRule) (*domain.SecurityRule, error) {
	args := m.Called(ctx, idOrName, rule)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.SecurityRule)
	return r0, args.Error(1)
}

func (m *mockSecurityGroupService) AttachToInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	args := m.Called(ctx, instanceID, groupID)
	return args.Error(0)
}

func (m *mockSecurityGroupService) DetachFromInstance(c context.Context, i uuid.UUID, g uuid.UUID) error {
	args := m.Called(c, i, g)
	return args.Error(0)
}

func (m *mockSecurityGroupService) RemoveRule(ctx context.Context, ruleID uuid.UUID) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}

func TestSecurityGroupHandlerCreate(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	vpcID := uuid.New()
	sg := &domain.SecurityGroup{ID: uuid.New(), VPCID: vpcID, Name: testSgName}
	svc.On("CreateGroup", mock.Anything, vpcID, testSgName, "web servers").Return(sg, nil)

	reqBody := map[string]interface{}{
		"vpc_id":      vpcID,
		"name":        testSgName,
		"description": "web servers",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", testSgPath, bytes.NewBuffer(body))
	c.Request.Header.Set(testContentType, testAppJSON)

	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandlerList(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	vpcID := uuid.New()
	groups := []*domain.SecurityGroup{{ID: uuid.New()}}

	svc.On("ListGroups", mock.Anything, vpcID).Return(groups, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", testSgPath+vpcIDQuery+vpcID.String(), nil)

	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandlerGet(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	id := uuid.New().String()
	vpcID := uuid.New()
	sg := &domain.SecurityGroup{ID: uuid.New()}

	svc.On("GetGroup", mock.Anything, id, vpcID).Return(sg, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", testSgDetailPath+id+vpcIDQuery+vpcID.String(), nil)
	c.Params = gin.Params{{Key: "id", Value: id}}

	h.Get(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandlerDelete(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupSecurityGroupHandlerTest(t)
	r.DELETE(testSgDetailPath+":id", handler.Delete)
	id := uuid.New()
	svc.On("DeleteGroup", mock.Anything, id).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", testSgDetailPath+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandlerAddRule(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	groupID := uuid.New()
	rule := domain.SecurityRule{Protocol: "tcp", PortMin: 80, PortMax: 80}

	svc.On("AddRule", mock.Anything, groupID.String(), rule).Return(&rule, nil)

	body, _ := json.Marshal(rule)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", testSgDetailPath+groupID.String()+"/rules", bytes.NewBuffer(body))
	c.Request.Header.Set(testContentType, testAppJSON)
	c.Params = gin.Params{{Key: "id", Value: groupID.String()}}

	h.AddRule(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandlerAttach(t *testing.T) {
	t.Parallel()
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
	c.Request = httptest.NewRequest("POST", testSgPath+"/attach", bytes.NewBuffer(body))
	c.Request.Header.Set(testContentType, testAppJSON)

	h.Attach(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandlerRemoveRule(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupSecurityGroupHandlerTest(t)
	r.DELETE(testSgPath+"/rules/:rule_id", handler.RemoveRule)
	ruleID := uuid.New()
	svc.On("RemoveRule", mock.Anything, ruleID).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", testSgPath+"/rules/"+ruleID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}
func TestSecurityGroupHandlerDetach(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	instanceID := uuid.New()
	groupID := uuid.New()

	svc.On("DetachFromInstance", mock.Anything, instanceID, groupID).Return(nil)

	reqBody := map[string]interface{}{
		"instance_id": instanceID,
		"group_id":    groupID,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", testSgPath+"/detach", bytes.NewBuffer(body))
	c.Request.Header.Set(testContentType, testAppJSON)

	h.Detach(c)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandlerAttachError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	instanceID := uuid.New()
	groupID := uuid.New()

	svc.On("AttachToInstance", mock.Anything, instanceID, groupID).Return(context.DeadlineExceeded)

	reqBody := map[string]interface{}{
		"instance_id": instanceID,
		"group_id":    groupID,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", testSgPath+"/attach", bytes.NewBuffer(body))
	c.Request.Header.Set(testContentType, testAppJSON)

	h.Attach(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestSecurityGroupHandlerDetachError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svc := new(mockSecurityGroupService)
	h := NewSecurityGroupHandler(svc)

	instanceID := uuid.New()
	groupID := uuid.New()

	svc.On("DetachFromInstance", mock.Anything, instanceID, groupID).Return(context.DeadlineExceeded)

	reqBody := map[string]interface{}{
		"instance_id": instanceID,
		"group_id":    groupID,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", testSgPath+"/detach", bytes.NewBuffer(body))
	c.Request.Header.Set(testContentType, testAppJSON)

	h.Detach(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
func TestSecurityGroupHandlerErrorPaths(t *testing.T) {
	t.Parallel()
	t.Run("CreateInvalidJSON", func(t *testing.T) {
		_, handler, _ := setupSecurityGroupHandlerTest(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", testSgPath, bytes.NewBufferString("invalid"))
		handler.Create(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("CreateServiceError", func(t *testing.T) {
		svc, handler, _ := setupSecurityGroupHandlerTest(t)
		svc.On("CreateGroup", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded)
		body, _ := json.Marshal(map[string]interface{}{"vpc_id": uuid.New(), "name": "n"})
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", testSgPath, bytes.NewBuffer(body))
		handler.Create(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ListMissingVPC", func(t *testing.T) {
		_, handler, _ := setupSecurityGroupHandlerTest(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", testSgPath, nil)
		handler.List(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ListInvalidVPC", func(t *testing.T) {
		_, handler, _ := setupSecurityGroupHandlerTest(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", testSgPath+vpcIDQuery+"invalid", nil)
		handler.List(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ListServiceError", func(t *testing.T) {
		svc, handler, _ := setupSecurityGroupHandlerTest(t)
		vpcID := uuid.New()
		svc.On("ListGroups", mock.Anything, vpcID).Return(nil, context.DeadlineExceeded)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", testSgPath+vpcIDQuery+vpcID.String(), nil)
		handler.List(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("DeleteInvalidID", func(t *testing.T) {
		_, handler, _ := setupSecurityGroupHandlerTest(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}
		handler.Delete(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DeleteServiceError", func(t *testing.T) {
		svc, handler, _ := setupSecurityGroupHandlerTest(t)
		id := uuid.New()
		svc.On("DeleteGroup", mock.Anything, id).Return(context.DeadlineExceeded)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("DELETE", "/", nil)
		c.Params = gin.Params{{Key: "id", Value: id.String()}}
		handler.Delete(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("AddRuleInvalidID", func(t *testing.T) {
		svc, handler, _ := setupSecurityGroupHandlerTest(t)
		svc.On("AddRule", mock.Anything, "invalid", mock.Anything).Return(nil, errors.New(errors.NotFound, "not found"))
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "invalid"}}
		c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString("{}"))
		handler.AddRule(c)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("AddRuleInvalidJSON", func(t *testing.T) {
		_, handler, _ := setupSecurityGroupHandlerTest(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: uuid.New().String()}}
		c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString("invalid"))
		handler.AddRule(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("AddRuleServiceError", func(t *testing.T) {
		svc, handler, _ := setupSecurityGroupHandlerTest(t)
		groupID := uuid.New()
		svc.On("AddRule", mock.Anything, groupID.String(), mock.Anything).Return(nil, context.DeadlineExceeded)
		r := domain.SecurityRule{}
		body, _ := json.Marshal(r)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: groupID.String()}}
		c.Request = httptest.NewRequest("POST", "/", bytes.NewBuffer(body))
		handler.AddRule(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("RemoveRuleInvalidID", func(t *testing.T) {
		_, handler, _ := setupSecurityGroupHandlerTest(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "rule_id", Value: "invalid"}}
		handler.RemoveRule(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("RemoveRuleServiceError", func(t *testing.T) {
		svc, handler, _ := setupSecurityGroupHandlerTest(t)
		ruleID := uuid.New()
		svc.On("RemoveRule", mock.Anything, ruleID).Return(context.DeadlineExceeded)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("DELETE", "/", nil)
		c.Params = gin.Params{{Key: "rule_id", Value: ruleID.String()}}
		handler.RemoveRule(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("AttachInvalidJSON", func(t *testing.T) {
		_, handler, _ := setupSecurityGroupHandlerTest(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString("invalid"))
		handler.Attach(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DetachInvalidJSON", func(t *testing.T) {
		_, handler, _ := setupSecurityGroupHandlerTest(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/", bytes.NewBufferString("invalid"))
		handler.Detach(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetServiceError", func(t *testing.T) {
		svc, handler, _ := setupSecurityGroupHandlerTest(t)
		id := "some-id"
		svc.On("GetGroup", mock.Anything, id, mock.Anything).Return(nil, context.DeadlineExceeded)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Params = gin.Params{{Key: "id", Value: id}}
		handler.Get(c)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func setupSecurityGroupHandlerTest(_ *testing.T) (*mockSecurityGroupService, *SecurityGroupHandler, *gin.Engine) {
	svc := new(mockSecurityGroupService)
	handler := NewSecurityGroupHandler(svc)
	r := gin.New()
	return svc, handler, r
}
