package httphandlers

import (
	"bytes"
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

type mockInternetGatewayService struct {
	mock.Mock
}

func (m *mockInternetGatewayService) CreateIGW(ctx context.Context) (*domain.InternetGateway, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.InternetGateway)
	return r0, args.Error(1)
}

func (m *mockInternetGatewayService) GetIGW(ctx context.Context, igwID uuid.UUID) (*domain.InternetGateway, error) {
	args := m.Called(ctx, igwID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.InternetGateway)
	return r0, args.Error(1)
}

func (m *mockInternetGatewayService) ListIGWs(ctx context.Context) ([]*domain.InternetGateway, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.InternetGateway)
	return r0, args.Error(1)
}

func (m *mockInternetGatewayService) AttachIGW(ctx context.Context, igwID, vpcID uuid.UUID) error {
	args := m.Called(ctx, igwID, vpcID)
	return args.Error(0)
}

func (m *mockInternetGatewayService) DetachIGW(ctx context.Context, igwID uuid.UUID) error {
	args := m.Called(ctx, igwID)
	return args.Error(0)
}

func (m *mockInternetGatewayService) DeleteIGW(ctx context.Context, igwID uuid.UUID) error {
	args := m.Called(ctx, igwID)
	return args.Error(0)
}

const igwPath = "/internet-gateways"

func setupInternetGatewayHandlerTest() (*mockInternetGatewayService, *InternetGatewayHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockInternetGatewayService)
	handler := NewInternetGatewayHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestInternetGatewayHandlerCreateSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.POST(igwPath, handler.Create)

	igwID := uuid.New()
	svc.On("CreateIGW", mock.Anything).Return(&domain.InternetGateway{ID: igwID}, nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, igwPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerCreateServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.POST(igwPath, handler.Create)

	svc.On("CreateIGW", mock.Anything).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, igwPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerListSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.GET(igwPath, handler.List)

	svc.On("ListIGWs", mock.Anything).Return([]*domain.InternetGateway{}, nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, igwPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerListServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.GET(igwPath, handler.List)

	svc.On("ListIGWs", mock.Anything).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, igwPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerGetSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.GET(igwPath+"/:id", handler.Get)

	igwID := uuid.New()
	svc.On("GetIGW", mock.Anything, igwID).Return(&domain.InternetGateway{ID: igwID}, nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, igwPath+"/"+igwID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerGetInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.GET(igwPath+"/:id", handler.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, igwPath+"/invalid-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerGetServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.GET(igwPath+"/:id", handler.Get)

	igwID := uuid.New()
	svc.On("GetIGW", mock.Anything, igwID).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, igwPath+"/"+igwID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerAttachSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.POST(igwPath+"/:id/attach", handler.Attach)

	igwID := uuid.New()
	vpcID := uuid.New()
	svc.On("AttachIGW", mock.Anything, igwID, vpcID).Return(nil).Once()

	body := `{"vpc_id":"` + vpcID.String() + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, igwPath+"/"+igwID.String()+"/attach", bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerAttachInvalidIGWUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.POST(igwPath+"/:id/attach", handler.Attach)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, igwPath+"/invalid-uuid/attach", bytes.NewBufferString(`{}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerAttachInvalidBody(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.POST(igwPath+"/:id/attach", handler.Attach)

	igwID := uuid.New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, igwPath+"/"+igwID.String()+"/attach", bytes.NewBufferString(`{"vpc_id":"invalid"}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	// Handler parses IGW ID from path (OK), then ShouldBindJSON fails on body vpc_id — binding error not wrapped in InvalidInput, so 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerAttachServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.POST(igwPath+"/:id/attach", handler.Attach)

	igwID := uuid.New()
	vpcID := uuid.New()
	svc.On("AttachIGW", mock.Anything, igwID, vpcID).Return(assert.AnError).Once()

	body := `{"vpc_id":"` + vpcID.String() + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, igwPath+"/"+igwID.String()+"/attach", bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerDetachSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.POST(igwPath+"/:id/detach", handler.Detach)

	igwID := uuid.New()
	svc.On("DetachIGW", mock.Anything, igwID).Return(nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, igwPath+"/"+igwID.String()+"/detach", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerDetachInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.POST(igwPath+"/:id/detach", handler.Detach)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, igwPath+"/invalid-uuid/detach", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerDetachServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.POST(igwPath+"/:id/detach", handler.Detach)

	igwID := uuid.New()
	svc.On("DetachIGW", mock.Anything, igwID).Return(assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, igwPath+"/"+igwID.String()+"/detach", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerDeleteSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.DELETE(igwPath+"/:id", handler.Delete)

	igwID := uuid.New()
	svc.On("DeleteIGW", mock.Anything, igwID).Return(nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, igwPath+"/"+igwID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerDeleteInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.DELETE(igwPath+"/:id", handler.Delete)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, igwPath+"/invalid-uuid", nil)
	r.ServeHTTP(w, req)

	// Handler parses invalid UUID and returns 400
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestInternetGatewayHandlerDeleteServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupInternetGatewayHandlerTest()
	r.DELETE(igwPath+"/:id", handler.Delete)

	igwID := uuid.New()
	svc.On("DeleteIGW", mock.Anything, igwID).Return(assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, igwPath+"/"+igwID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
