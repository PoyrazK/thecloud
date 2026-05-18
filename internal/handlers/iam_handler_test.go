package httphandlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupIAMHandlerTest() (*mocks.IAMService, *IAMHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	iamSvc := new(mocks.IAMService)
	identitySvc := new(mockIdentityService)
	handler := NewIAMHandler(iamSvc, identitySvc)
	r := gin.New()
	return iamSvc, handler, r
}

func TestIAMHandler_CreatePolicy(t *testing.T) {
	svc, handler, r := setupIAMHandlerTest()
	r.POST("/iam/policies", handler.CreatePolicy)

	policy := domain.Policy{
		Name: "TestPolicy",
		Statements: []domain.Statement{
			{Effect: domain.EffectAllow, Action: []string{"*"}, Resource: []string{"*"}},
		},
	}
	body, _ := json.Marshal(policy)

	svc.On("CreatePolicy", mock.Anything, mock.AnythingOfType("*domain.Policy")).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/iam/policies", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestIAMHandler_ListPolicies(t *testing.T) {
	svc, handler, r := setupIAMHandlerTest()
	r.GET("/iam/policies", handler.ListPolicies)

	svc.On("ListPolicies", mock.Anything).Return([]*domain.Policy{{ID: uuid.New(), Name: "P1"}}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/iam/policies", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestIAMHandler_AttachPolicy(t *testing.T) {
	svc, handler, r := setupIAMHandlerTest()
	r.POST("/iam/users/:userId/policies/:policyId", handler.AttachPolicyToUser)

	uID := uuid.New()
	pID := uuid.New()

	svc.On("AttachPolicyToUser", mock.Anything, uID, pID).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/iam/users/"+uID.String()+"/policies/"+pID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}
