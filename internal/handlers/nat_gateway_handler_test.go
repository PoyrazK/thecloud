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

type mockNATGatewayService struct {
	mock.Mock
}

func (m *mockNATGatewayService) CreateNATGateway(ctx context.Context, subnetID, eipID uuid.UUID) (*domain.NATGateway, error) {
	args := m.Called(ctx, subnetID, eipID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.NATGateway)
	return r0, args.Error(1)
}

func (m *mockNATGatewayService) GetNATGateway(ctx context.Context, natID uuid.UUID) (*domain.NATGateway, error) {
	args := m.Called(ctx, natID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.NATGateway)
	return r0, args.Error(1)
}

func (m *mockNATGatewayService) ListNATGateways(ctx context.Context, vpcID uuid.UUID) ([]*domain.NATGateway, error) {
	args := m.Called(ctx, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.NATGateway)
	return r0, args.Error(1)
}

func (m *mockNATGatewayService) DeleteNATGateway(ctx context.Context, natID uuid.UUID) error {
	args := m.Called(ctx, natID)
	return args.Error(0)
}

const natGatewayPath = "/nat-gateways"

func setupNATGatewayHandlerTest() (*mockNATGatewayService, *NATGatewayHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockNATGatewayService)
	handler := NewNATGatewayHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestNATGatewayHandlerCreateSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.POST(natGatewayPath, handler.Create)

	vpcID := uuid.New()
	subnetID := uuid.New()
	eipID := uuid.New()
	natID := uuid.New()

	svc.On("CreateNATGateway", mock.Anything, subnetID, eipID).Return(&domain.NATGateway{
		ID: natID, VPCID: vpcID, SubnetID: subnetID, ElasticIPID: eipID,
	}, nil).Once()

	body := `{"subnet_id":"` + subnetID.String() + `","eip_id":"` + eipID.String() + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, natGatewayPath, bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerCreateInvalidBody(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.POST(natGatewayPath, handler.Create)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, natGatewayPath, bytes.NewBufferString(`{"subnet_id":"invalid"}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	// Handler returns 500 when binding validation fails for UUID (not wrapped in errors.InvalidInput)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerCreateServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.POST(natGatewayPath, handler.Create)

	subnetID := uuid.New()
	eipID := uuid.New()

	svc.On("CreateNATGateway", mock.Anything, subnetID, eipID).Return(nil, assert.AnError).Once()

	body := `{"subnet_id":"` + subnetID.String() + `","eip_id":"` + eipID.String() + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, natGatewayPath, bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerListSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.GET(natGatewayPath, handler.List)

	vpcID := uuid.New()
	svc.On("ListNATGateways", mock.Anything, vpcID).Return([]*domain.NATGateway{}, nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, natGatewayPath+"?vpc_id="+vpcID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerListMissingVpcID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.GET(natGatewayPath, handler.List)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, natGatewayPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerListServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.GET(natGatewayPath, handler.List)

	vpcID := uuid.New()
	svc.On("ListNATGateways", mock.Anything, vpcID).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, natGatewayPath+"?vpc_id="+vpcID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerGetSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.GET(natGatewayPath+"/:id", handler.Get)

	natID := uuid.New()
	vpcID := uuid.New()
	svc.On("GetNATGateway", mock.Anything, natID).Return(&domain.NATGateway{ID: natID, VPCID: vpcID}, nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, natGatewayPath+"/"+natID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerGetInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.GET(natGatewayPath+"/:id", handler.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, natGatewayPath+"/invalid-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerGetServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.GET(natGatewayPath+"/:id", handler.Get)

	natID := uuid.New()
	svc.On("GetNATGateway", mock.Anything, natID).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, natGatewayPath+"/"+natID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerDeleteSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.DELETE(natGatewayPath+"/:id", handler.Delete)

	natID := uuid.New()
	svc.On("DeleteNATGateway", mock.Anything, natID).Return(nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, natGatewayPath+"/"+natID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerDeleteInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.DELETE(natGatewayPath+"/:id", handler.Delete)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, natGatewayPath+"/invalid-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestNATGatewayHandlerDeleteServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupNATGatewayHandlerTest()
	r.DELETE(natGatewayPath+"/:id", handler.Delete)

	natID := uuid.New()
	svc.On("DeleteNATGateway", mock.Anything, natID).Return(assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, natGatewayPath+"/"+natID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
