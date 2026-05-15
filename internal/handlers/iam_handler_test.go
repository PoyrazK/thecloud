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
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/ports/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

func TestIAMHandler_Simulate(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupIAMHandlerTest()
	r.POST("/iam/simulate", handler.Simulate)

	userID := uuid.New()
	svc.On("SimulatePolicy", mock.Anything, ports.Principal{UserID: &userID}, []string{"compute:instance:launch"}, []string{"arn:thecloud:compute:us-east-1:*:instance/*"}, mock.Anything).
		Return(&ports.SimulateResult{Decision: domain.EffectAllow, Evaluated: 1, Matched: &ports.StatementMatch{Effect: domain.EffectAllow, Reason: "allow statement matched"}}, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":   userID.String(),
		"actions":   []string{"compute:instance:launch"},
		"resources": []string{"arn:thecloud:compute:us-east-1:*:instance/*"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/iam/simulate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "Allow", data["decision"])
	assert.Equal(t, float64(1), data["evaluated"])
	svc.AssertExpectations(t)
}

func TestIAMHandler_SimulateDeny(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupIAMHandlerTest()
	r.POST("/iam/simulate", handler.Simulate)

	userID := uuid.New()
	svc.On("SimulatePolicy", mock.Anything, ports.Principal{UserID: &userID}, []string{"compute:instance:delete"}, []string{"arn:thecloud:compute:us-east-1:*:instance/*"}, mock.Anything).
		Return(&ports.SimulateResult{Decision: domain.EffectDeny, Evaluated: 2, Matched: &ports.StatementMatch{Effect: domain.EffectDeny, Reason: "deny statement matched"}}, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":   userID.String(),
		"actions":   []string{"compute:instance:delete"},
		"resources": []string{"arn:thecloud:compute:us-east-1:*:instance/*"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/iam/simulate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "Deny", data["decision"])
	svc.AssertExpectations(t)
}

func TestIAMHandler_SimulateMissingPrincipal(t *testing.T) {
	t.Parallel()
	_, handler, r := setupIAMHandlerTest()
	r.POST("/iam/simulate", handler.Simulate)

	body, _ := json.Marshal(map[string]interface{}{
		"actions":   []string{"compute:instance:launch"},
		"resources": []string{"arn:thecloud:compute:us-east-1:*:instance/*"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/iam/simulate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestIAMHandler_SimulateInvalidJSON(t *testing.T) {
	t.Parallel()
	_, handler, r := setupIAMHandlerTest()
	r.POST("/iam/simulate", handler.Simulate)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/iam/simulate", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
