package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

const errInvalidTransitGatewayID = "invalid transit gateway id"

// TransitGatewayHandler handles HTTP requests for Transit Gateway.
type TransitGatewayHandler struct {
	svc ports.TransitGatewayService
}

// CreateTransitGatewayRequest represents the body for creating a Transit Gateway.
type CreateTransitGatewayRequest struct {
	Name string `json:"name" binding:"required"`
}

// AttachVPCRequest represents the body for attaching a VPC to a Transit Gateway.
type AttachVPCRequest struct {
	VPCID string `json:"vpc_id" binding:"required,uuid"`
}

// NewTransitGatewayHandler creates a new TransitGatewayHandler.
func NewTransitGatewayHandler(svc ports.TransitGatewayService) *TransitGatewayHandler {
	return &TransitGatewayHandler{svc: svc}
}

// Create creates a new Transit Gateway.
// @Summary Create Transit Gateway
// @Tags transit-gateways
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param request body CreateTransitGatewayRequest true "Create Transit Gateway"
// @Success 201 {object} domain.TransitGateway
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /transit-gateways [post]
func (h *TransitGatewayHandler) Create(c *gin.Context) {
	var req CreateTransitGatewayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	tg, err := h.svc.CreateTransitGateway(c.Request.Context(), req.Name)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusCreated, tg)
}

// List returns all Transit Gateways for the tenant.
// @Summary List Transit Gateways
// @Tags transit-gateways
// @Security APIKeyAuth
// @Produce json
// @Success 200 {array} domain.TransitGateway
// @Failure 500 {object} httputil.Response
// @Router /transit-gateways [get]
func (h *TransitGatewayHandler) List(c *gin.Context) {
	tgs, err := h.svc.ListTransitGateways(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, tgs)
}

// Get retrieves a Transit Gateway by ID.
// @Summary Get Transit Gateway
// @Tags transit-gateways
// @Security APIKeyAuth
// @Param id path string true "Transit Gateway ID"
// @Produce json
// @Success 200 {object} domain.TransitGateway
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /transit-gateways/{id} [get]
func (h *TransitGatewayHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidTransitGatewayID))
		return
	}

	tg, err := h.svc.GetTransitGateway(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, tg)
}

// Delete removes a Transit Gateway.
// @Summary Delete Transit Gateway
// @Tags transit-gateways
// @Security APIKeyAuth
// @Param id path string true "Transit Gateway ID"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /transit-gateways/{id} [delete]
func (h *TransitGatewayHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidTransitGatewayID))
		return
	}

	if err := h.svc.DeleteTransitGateway(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "transit gateway deleted"})
}

// AttachVPC attaches a VPC to a Transit Gateway.
// @Summary Attach VPC to Transit Gateway
// @Tags transit-gateways
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Transit Gateway ID"
// @Param request body AttachVPCRequest true "Attach VPC Request"
// @Success 201 {object} domain.TransitGatewayAttachment
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 409 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /transit-gateways/{id}/attach [post]
func (h *TransitGatewayHandler) AttachVPC(c *gin.Context) {
	tgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidTransitGatewayID))
		return
	}

	var req AttachVPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	vpcID, err := uuid.Parse(req.VPCID)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid VPC ID"))
		return
	}
	att, err := h.svc.AttachVPC(c.Request.Context(), tgID, vpcID)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusCreated, att)
}

// DetachVPC detaches a VPC from a Transit Gateway.
// @Summary Detach VPC from Transit Gateway
// @Tags transit-gateways
// @Security APIKeyAuth
// @Param id path string true "Attachment ID"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /transit-gateways/attachments/{id}/detach [post]
func (h *TransitGatewayHandler) DetachVPC(c *gin.Context) {
	attID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid attachment id"))
		return
	}

	if err := h.svc.DetachVPC(c.Request.Context(), attID); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "VPC detached"})
}

// AssociateRouteTable associates an attachment with a TGW route table.
// @Summary Associate Route Table
// @Tags transit-gateways
// @Security APIKeyAuth
// @Param id path string true "Route Table ID"
// @Param request body AssociateRTRequest true "Associate Request"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /transit-gateways/route-tables/{id}/associate [post]
func (h *TransitGatewayHandler) AssociateRouteTable(c *gin.Context) {
	rtID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid route table id"))
		return
	}

	var req AssociateRTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	attID, err := uuid.Parse(req.AttachmentID)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid attachment ID"))
		return
	}
	if err := h.svc.AssociateRouteTable(c.Request.Context(), rtID, attID); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "route table associated"})
}

// EnableRoutePropagation enables route propagation on a TGW route table.
// @Summary Enable Route Propagation
// @Tags transit-gateways
// @Security APIKeyAuth
// @Param id path string true "Route Table ID"
// @Param request body AssociateRTRequest true "Enable Propagation Request"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /transit-gateways/route-tables/{id}/propagation [post]
func (h *TransitGatewayHandler) EnableRoutePropagation(c *gin.Context) {
	rtID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid route table id"))
		return
	}

	var req AssociateRTRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	attID, err := uuid.Parse(req.AttachmentID)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid attachment ID"))
		return
	}
	if err := h.svc.EnableRoutePropagation(c.Request.Context(), rtID, attID); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "route propagation enabled"})
}

// AssociateRTRequest is used for associating attachments to route tables.
type AssociateRTRequest struct {
	AttachmentID string `json:"attachment_id" binding:"required,uuid"`
}
