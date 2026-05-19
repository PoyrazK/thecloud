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

type mockRouteTableService struct {
	mock.Mock
}

func (m *mockRouteTableService) CreateRouteTable(ctx context.Context, vpcID uuid.UUID, name string, isMain bool) (*domain.RouteTable, error) {
	args := m.Called(ctx, vpcID, name, isMain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.RouteTable)
	return r0, args.Error(1)
}

func (m *mockRouteTableService) GetRouteTable(ctx context.Context, id uuid.UUID) (*domain.RouteTable, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.RouteTable)
	return r0, args.Error(1)
}

func (m *mockRouteTableService) ListRouteTables(ctx context.Context, vpcID uuid.UUID) ([]*domain.RouteTable, error) {
	args := m.Called(ctx, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.RouteTable)
	return r0, args.Error(1)
}

func (m *mockRouteTableService) DeleteRouteTable(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockRouteTableService) AddRoute(ctx context.Context, rtID uuid.UUID, destinationCIDR string, targetType domain.RouteTargetType, targetID *uuid.UUID) (*domain.Route, error) {
	args := m.Called(ctx, rtID, destinationCIDR, targetType, targetID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Route)
	return r0, args.Error(1)
}

func (m *mockRouteTableService) RemoveRoute(ctx context.Context, rtID, routeID uuid.UUID) error {
	args := m.Called(ctx, rtID, routeID)
	return args.Error(0)
}

func (m *mockRouteTableService) AssociateSubnet(ctx context.Context, rtID, subnetID uuid.UUID) error {
	args := m.Called(ctx, rtID, subnetID)
	return args.Error(0)
}

func (m *mockRouteTableService) DisassociateSubnet(ctx context.Context, rtID, subnetID uuid.UUID) error {
	args := m.Called(ctx, rtID, subnetID)
	return args.Error(0)
}

func (m *mockRouteTableService) ReplaceRoute(ctx context.Context, rtID, routeID uuid.UUID, newTargetID *uuid.UUID) error {
	args := m.Called(ctx, rtID, routeID, newTargetID)
	return args.Error(0)
}

const routeTablePath = "/route-tables"

func setupRouteTableHandlerTest() (*mockRouteTableService, *RouteTableHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockRouteTableService)
	handler := NewRouteTableHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestRouteTableHandlerCreateSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath, handler.Create)

	vpcID := uuid.New()
	rtID := uuid.New()

	svc.On("CreateRouteTable", mock.Anything, vpcID, "test-rt", false).Return(&domain.RouteTable{ID: rtID, VPCID: vpcID, Name: "test-rt"}, nil).Once()

	body := `{"vpc_id":"` + vpcID.String() + `","name":"test-rt"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath, bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerCreateInvalidBody(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath, handler.Create)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath, bytes.NewBufferString(`{"name":""}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerCreateInvalidVpcID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath, handler.Create)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath, bytes.NewBufferString(`{"vpc_id":"invalid","name":"test-rt"}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerCreateServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath, handler.Create)

	vpcID := uuid.New()
	svc.On("CreateRouteTable", mock.Anything, vpcID, "test-rt", false).Return(nil, assert.AnError).Once()

	body := `{"vpc_id":"` + vpcID.String() + `","name":"test-rt"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath, bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerListSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.GET(routeTablePath, handler.List)

	vpcID := uuid.New()
	svc.On("ListRouteTables", mock.Anything, vpcID).Return([]*domain.RouteTable{}, nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, routeTablePath+"?vpc_id="+vpcID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerListMissingVpcID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.GET(routeTablePath, handler.List)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, routeTablePath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerListServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.GET(routeTablePath, handler.List)

	vpcID := uuid.New()
	svc.On("ListRouteTables", mock.Anything, vpcID).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, routeTablePath+"?vpc_id="+vpcID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerGetSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.GET(routeTablePath+"/:id", handler.Get)

	rtID := uuid.New()
	svc.On("GetRouteTable", mock.Anything, rtID).Return(&domain.RouteTable{ID: rtID, Name: "test-rt"}, nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, routeTablePath+"/"+rtID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerGetInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.GET(routeTablePath+"/:id", handler.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, routeTablePath+"/invalid-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerGetServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.GET(routeTablePath+"/:id", handler.Get)

	rtID := uuid.New()
	svc.On("GetRouteTable", mock.Anything, rtID).Return(nil, assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, routeTablePath+"/"+rtID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerDeleteSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.DELETE(routeTablePath+"/:id", handler.Delete)

	rtID := uuid.New()
	svc.On("DeleteRouteTable", mock.Anything, rtID).Return(nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, routeTablePath+"/"+rtID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerDeleteInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.DELETE(routeTablePath+"/:id", handler.Delete)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, routeTablePath+"/invalid-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerDeleteServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.DELETE(routeTablePath+"/:id", handler.Delete)

	rtID := uuid.New()
	svc.On("DeleteRouteTable", mock.Anything, rtID).Return(assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, routeTablePath+"/"+rtID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerAddRouteSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/routes", handler.AddRoute)

	rtID := uuid.New()
	routeID := uuid.New()
	targetID := uuid.New()

	svc.On("AddRoute", mock.Anything, rtID, "10.0.0.0/16", domain.RouteTargetType("nat"), &targetID).Return(&domain.Route{ID: routeID}, nil).Once()

	body := `{"destination_cidr":"10.0.0.0/16","target_type":"nat","target_id":"` + targetID.String() + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/"+rtID.String()+"/routes", bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerAddRouteInvalidRTUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/routes", handler.AddRoute)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/invalid-uuid/routes", bytes.NewBufferString(`{}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerAddRouteInvalidBody(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/routes", handler.AddRoute)

	rtID := uuid.New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/"+rtID.String()+"/routes", bytes.NewBufferString(`{"destination_cidr":""}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerAddRouteServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/routes", handler.AddRoute)

	rtID := uuid.New()
	targetID := uuid.New()
	svc.On("AddRoute", mock.Anything, rtID, "10.0.0.0/16", domain.RouteTargetType("nat"), &targetID).Return(nil, assert.AnError).Once()

	body := `{"destination_cidr":"10.0.0.0/16","target_type":"nat","target_id":"` + targetID.String() + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/"+rtID.String()+"/routes", bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerRemoveRouteSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.DELETE(routeTablePath+"/:id/routes/:route_id", handler.RemoveRoute)

	rtID := uuid.New()
	routeID := uuid.New()
	svc.On("RemoveRoute", mock.Anything, rtID, routeID).Return(nil).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, routeTablePath+"/"+rtID.String()+"/routes/"+routeID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerRemoveRouteInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.DELETE(routeTablePath+"/:id/routes/:route_id", handler.RemoveRoute)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, routeTablePath+"/invalid-uuid/routes/route-id", nil)
	r.ServeHTTP(w, req)

	// route_id parse error is ignored by the handler (uuid.Nil used instead)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerRemoveRouteInvalidRouteID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.DELETE(routeTablePath+"/:id/routes/:route_id", handler.RemoveRoute)

	rtID := uuid.New()
	svc.On("RemoveRoute", mock.Anything, rtID, uuid.Nil).Return(nil).Once()

	// Valid route table ID, invalid route_id (parse error discarded, uses uuid.Nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, routeTablePath+"/"+rtID.String()+"/routes/invalid-route-id", nil)
	r.ServeHTTP(w, req)

	// Handler ignores route_id parse error and calls service with uuid.Nil
	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerRemoveRouteServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.DELETE(routeTablePath+"/:id/routes/:route_id", handler.RemoveRoute)

	rtID := uuid.New()
	routeID := uuid.New()
	svc.On("RemoveRoute", mock.Anything, rtID, routeID).Return(assert.AnError).Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, routeTablePath+"/"+rtID.String()+"/routes/"+routeID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerAssociateSubnetSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/associate", handler.AssociateSubnet)

	rtID := uuid.New()
	subnetID := uuid.New()
	svc.On("AssociateSubnet", mock.Anything, rtID, subnetID).Return(nil).Once()

	body := `{"subnet_id":"` + subnetID.String() + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/"+rtID.String()+"/associate", bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerAssociateSubnetInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/associate", handler.AssociateSubnet)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/invalid-uuid/associate", bytes.NewBufferString(`{}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerAssociateSubnetInvalidBody(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/associate", handler.AssociateSubnet)

	rtID := uuid.New()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/"+rtID.String()+"/associate", bytes.NewBufferString(`{"subnet_id":"invalid"}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerAssociateSubnetServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/associate", handler.AssociateSubnet)

	rtID := uuid.New()
	subnetID := uuid.New()
	svc.On("AssociateSubnet", mock.Anything, rtID, subnetID).Return(assert.AnError).Once()

	body := `{"subnet_id":"` + subnetID.String() + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/"+rtID.String()+"/associate", bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerDisassociateSubnetSuccess(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/disassociate", handler.DisassociateSubnet)

	rtID := uuid.New()
	subnetID := uuid.New()
	svc.On("DisassociateSubnet", mock.Anything, rtID, subnetID).Return(nil).Once()

	body := `{"subnet_id":"` + subnetID.String() + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/"+rtID.String()+"/disassociate", bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerDisassociateSubnetInvalidUUID(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/disassociate", handler.DisassociateSubnet)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/invalid-uuid/disassociate", bytes.NewBufferString(`{}`))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestRouteTableHandlerDisassociateSubnetServiceError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupRouteTableHandlerTest()
	r.POST(routeTablePath+"/:id/disassociate", handler.DisassociateSubnet)

	rtID := uuid.New()
	subnetID := uuid.New()
	svc.On("DisassociateSubnet", mock.Anything, rtID, subnetID).Return(assert.AnError).Once()

	body := `{"subnet_id":"` + subnetID.String() + `"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, routeTablePath+"/"+rtID.String()+"/disassociate", bytes.NewBufferString(body))
	req.Header.Set(contentType, applicationJSON)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}
