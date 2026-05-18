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
	"github.com/poyrazk/thecloud/internal/errors"
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
	assert.Equal(t, "allow", data["decision"])
	assert.InDelta(t, 1, data["evaluated"], 0)
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
	assert.Equal(t, "deny", data["decision"])
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

func TestIAMHandler_SimulateServiceAccount(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupIAMHandlerTest()
	r.POST("/iam/simulate", handler.Simulate)

	saID := uuid.New()
	svc.On("SimulatePolicy", mock.Anything, ports.Principal{ServiceAccountID: &saID}, []string{"compute:instance:launch"}, []string{"arn:thecloud:compute:us-east-1:*:instance/*"}, mock.Anything).
		Return(&ports.SimulateResult{Decision: domain.EffectAllow, Evaluated: 1, Matched: &ports.StatementMatch{Effect: domain.EffectAllow, Reason: "allow statement matched"}}, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"service_account_id": saID.String(),
		"actions":            []string{"compute:instance:launch"},
		"resources":          []string{"arn:thecloud:compute:us-east-1:*:instance/*"},
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
	assert.Equal(t, "allow", data["decision"])
	assert.InDelta(t, 1, data["evaluated"], 0)
	svc.AssertExpectations(t)
}

func TestIAMHandler_SimulateDenyWithCondition(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupIAMHandlerTest()
	r.POST("/iam/simulate", handler.Simulate)

	userID := uuid.New()
	svc.On("SimulatePolicy", mock.Anything, ports.Principal{UserID: &userID}, []string{"compute:instance:delete"}, []string{"arn:thecloud:compute:us-east-1:*:instance/*"}, mock.Anything).
		Return(&ports.SimulateResult{
			Decision: domain.EffectDeny, Evaluated: 1,
			Matched: &ports.StatementMatch{
				Action: "compute:instance:delete", Resource: "arn:thecloud:compute:us-east-1:*:instance/*",
				PolicyID: uuid.New(), PolicyName: "DenyDeletePolicy",
				StatementSid: "deny-delete", Effect: domain.EffectDeny,
				Reason: "deny statement matched",
			},
		}, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":   userID.String(),
		"actions":   []string{"compute:instance:delete"},
		"resources": []string{"arn:thecloud:compute:us-east-1:*:instance/*"},
		"context": map[string]interface{}{
			"aws:SourceIp": "10.0.0.1",
		},
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
	matched := data["matched"].(map[string]interface{})
	assert.Equal(t, "deny", data["decision"])
	assert.Equal(t, "compute:instance:delete", matched["action"])
	assert.Equal(t, "arn:thecloud:compute:us-east-1:*:instance/*", matched["resource"])
	assert.Equal(t, "DenyDeletePolicy", matched["policy_name"])
	assert.Equal(t, "deny-delete", matched["statement_sid"])
	assert.InDelta(t, 1, data["evaluated"], 0)
	svc.AssertExpectations(t)
}

func TestIAMHandler_SimulateTooManyPairs(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupIAMHandlerTest()
	r.POST("/iam/simulate", handler.Simulate)

	userID := uuid.New()
	actions := make([]string, 11)
	resources := make([]string, 10)
	for i := range actions {
		actions[i] = "compute:instance:launch"
	}
	for i := range resources {
		resources[i] = "arn:thecloud:compute:us-east-1:*:instance/*"
	}
	// Mock returns INVALID_INPUT (as the service would when pair count > 100)
	svc.On("SimulatePolicy", mock.Anything, ports.Principal{UserID: &userID}, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New(errors.InvalidInput, "too many action-resource pairs (max 100)"))

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":   userID.String(),
		"actions":   actions,
		"resources": resources,
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/iam/simulate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	errData := resp["error"].(map[string]interface{})
	assert.Equal(t, "INVALID_INPUT", errData["code"])
	svc.AssertExpectations(t)
}

func TestIAMHandler_SimulateContextOverride(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupIAMHandlerTest()
	r.POST("/iam/simulate", handler.Simulate)

	userID := uuid.New()
	overrideTime := "2025-01-01T00:00:00Z"
	svc.On("SimulatePolicy", mock.Anything, ports.Principal{UserID: &userID}, []string{"compute:instance:launch"}, []string{"arn:thecloud:compute:us-east-1:*:instance/*"}, mock.MatchedBy(func(evalCtx map[string]interface{}) bool {
		return evalCtx["aws:CurrentTime"] == overrideTime
	})).
		Return(&ports.SimulateResult{Decision: domain.EffectAllow, Evaluated: 1, Matched: &ports.StatementMatch{Effect: domain.EffectAllow, Reason: "allow statement matched"}}, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id":   userID.String(),
		"actions":   []string{"compute:instance:launch"},
		"resources": []string{"arn:thecloud:compute:us-east-1:*:instance/*"},
		"context": map[string]interface{}{
			"aws:CurrentTime": overrideTime,
		},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/iam/simulate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}
