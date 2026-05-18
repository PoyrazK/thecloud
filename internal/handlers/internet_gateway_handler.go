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

// InternetGatewayHandler handles HTTP requests for internet gateways.
type InternetGatewayHandler struct {
	svc ports.InternetGatewayService
}

// NewInternetGatewayHandler creates a new InternetGatewayHandler.
func NewInternetGatewayHandler(svc ports.InternetGatewayService) *InternetGatewayHandler {
	return &InternetGatewayHandler{svc: svc}
}

// Create creates a new internet gateway.
// @Summary Create Internet Gateway
// @Tags internet-gateways
// @Security APIKeyAuth
// @Produce json
// @Success 201 {object} domain.InternetGateway
// @Failure 500 {object} httputil.Response
// @Router /internet-gateways [post]
func (h *InternetGatewayHandler) Create(c *gin.Context) {
	igw, err := h.svc.CreateIGW(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusCreated, igw)
}

// List returns all internet gateways.
// @Summary List Internet Gateways
// @Tags internet-gateways
// @Security APIKeyAuth
// @Produce json
// @Success 200 {array} domain.InternetGateway
// @Failure 500 {object} httputil.Response
// @Router /internet-gateways [get]
func (h *InternetGatewayHandler) List(c *gin.Context) {
	igws, err := h.svc.ListIGWs(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, igws)
}

// Get retrieves a specific internet gateway.
// @Summary Get Internet Gateway
// @Tags internet-gateways
// @Security APIKeyAuth
// @Produce json
// @Param id path string true "Internet Gateway ID"
// @Success 200 {object} domain.InternetGateway
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /internet-gateways/{id} [get]
func (h *InternetGatewayHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid internet gateway id"))
		return
	}

	igw, err := h.svc.GetIGW(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, igw)
}

// IGWAttachRequest represents the body for attaching an IGW to a VPC.
type IGWAttachRequest struct {
	VPCID string `json:"vpc_id" binding:"required,uuid"`
}

// Attach attaches an internet gateway to a VPC.
// @Summary Attach Internet Gateway
// @Tags internet-gateways
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Internet Gateway ID"
// @Param request body IGWAttachRequest true "Attach Request"
// @Success 200
// @Failure 400 {object} httputil.Response
// @Failure 409 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /internet-gateways/{id}/attach [post]
func (h *InternetGatewayHandler) Attach(c *gin.Context) {
	idStr := c.Param("id")
	igwID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid internet gateway id"))
		return
	}

	var req IGWAttachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	vpcID, _ := uuid.Parse(req.VPCID)
	if err := h.svc.AttachIGW(c.Request.Context(), igwID, vpcID); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, nil)
}

// Detach detaches an internet gateway from its VPC.
// @Summary Detach Internet Gateway
// @Tags internet-gateways
// @Security APIKeyAuth
// @Param id path string true "Internet Gateway ID"
// @Success 200
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /internet-gateways/{id}/detach [post]
func (h *InternetGatewayHandler) Detach(c *gin.Context) {
	idStr := c.Param("id")
	igwID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid internet gateway id"))
		return
	}

	if err := h.svc.DetachIGW(c.Request.Context(), igwID); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, nil)
}

// Delete removes an internet gateway (must be detached first).
// @Summary Delete Internet Gateway
// @Tags internet-gateways
// @Security APIKeyAuth
// @Param id path string true "Internet Gateway ID"
// @Success 204
// @Failure 400 {object} httputil.Response
// @Failure 409 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /internet-gateways/{id} [delete]
func (h *InternetGatewayHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	igwID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid internet gateway id"))
		return
	}

	if err := h.svc.DeleteIGW(c.Request.Context(), igwID); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusNoContent, nil)
}
