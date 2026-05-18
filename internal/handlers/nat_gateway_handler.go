// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

// NATGatewayHandler handles HTTP requests for NAT gateways.
type NATGatewayHandler struct {
	svc ports.NATGatewayService
}

// NewNATGatewayHandler creates a new NATGatewayHandler.
func NewNATGatewayHandler(svc ports.NATGatewayService) *NATGatewayHandler {
	return &NATGatewayHandler{svc: svc}
}

// CreateNATGatewayRequest represents the body for creating a NAT gateway.
type CreateNATGatewayRequest struct {
	SubnetID string `json:"subnet_id" binding:"required,uuid"`
	EIPID    string `json:"eip_id" binding:"required,uuid"`
}

// Create creates a new NAT gateway.
// @Summary Create NAT Gateway
// @Tags nat-gateways
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param request body CreateNATGatewayRequest true "NAT Gateway Request"
// @Success 201 {object} domain.NATGateway
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /nat-gateways [post]
func (h *NATGatewayHandler) Create(c *gin.Context) {
	var req CreateNATGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	subnetID, _ := uuid.Parse(req.SubnetID)
	eipID, _ := uuid.Parse(req.EIPID)

	nat, err := h.svc.CreateNATGateway(c.Request.Context(), subnetID, eipID)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusCreated, nat)
}

// List returns all NAT gateways for a VPC.
// @Summary List NAT Gateways
// @Tags nat-gateways
// @Security APIKeyAuth
// @Produce json
// @Param vpc_id query string false "VPC ID to filter by"
// @Success 200 {array} domain.NATGateway
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /nat-gateways [get]
func (h *NATGatewayHandler) List(c *gin.Context) {
	vpcIDStr := c.Query("vpc_id")
	if vpcIDStr == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "vpc_id is required"))
		return
	}

	vpcID, _ := uuid.Parse(vpcIDStr)
	nats, err := h.svc.ListNATGateways(c.Request.Context(), vpcID)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, nats)
}

// Get retrieves a specific NAT gateway.
// @Summary Get NAT Gateway
// @Tags nat-gateways
// @Security APIKeyAuth
// @Produce json
// @Param id path string true "NAT Gateway ID"
// @Success 200 {object} domain.NATGateway
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /nat-gateways/{id} [get]
func (h *NATGatewayHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid NAT gateway id"))
		return
	}

	nat, err := h.svc.GetNATGateway(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, nat)
}

// Delete removes a NAT gateway.
// @Summary Delete NAT Gateway
// @Tags nat-gateways
// @Security APIKeyAuth
// @Param id path string true "NAT Gateway ID"
// @Success 204
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /nat-gateways/{id} [delete]
func (h *NATGatewayHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid NAT gateway id"))
		return
	}

	if err := h.svc.DeleteNATGateway(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusNoContent, nil)
}
