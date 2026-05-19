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

const errInvalidPeeringID = "invalid vpc peering id"

// VPCPeeringHandler handles HTTP requests for VPC peering connections.
type VPCPeeringHandler struct {
	svc ports.VPCPeeringService
}

// CreatePeeringRequest represents the body for creating a new peering connection.
type CreatePeeringRequest struct {
	RequesterVPCID   string `json:"requester_vpc_id" binding:"required,uuid"`
	AccepterVPCID    string `json:"accepter_vpc_id" binding:"required,uuid"`
	AccepterTenantID string `json:"accepter_tenant_id" binding:"required,uuid"`
}

// NewVPCPeeringHandler creates a new VPCPeeringHandler.
func NewVPCPeeringHandler(svc ports.VPCPeeringService) *VPCPeeringHandler {
	return &VPCPeeringHandler{svc: svc}
}

// Create initiates a new VPC peering connection request.
// @Summary Create VPC Peering
// @Tags vpc-peerings
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param request body CreatePeeringRequest true "Peering Request"
// @Success 201 {object} domain.VPCPeering
// @Failure 400 {object} httputil.Response
// @Failure 409 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /vpc-peerings [post]
func (h *VPCPeeringHandler) Create(c *gin.Context) {
	var req CreatePeeringRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	requesterID, _ := uuid.Parse(req.RequesterVPCID)
	accepterID, _ := uuid.Parse(req.AccepterVPCID)
	accepterTenantID, _ := uuid.Parse(req.AccepterTenantID)

	peering, err := h.svc.CreatePeering(c.Request.Context(), requesterID, accepterID, accepterTenantID)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusCreated, peering)
}

// List returns all VPC peering connections for the tenant.
// @Summary List VPC Peerings
// @Tags vpc-peerings
// @Security APIKeyAuth
// @Produce json
// @Success 200 {array} domain.VPCPeering
// @Failure 500 {object} httputil.Response
// @Router /vpc-peerings [get]
func (h *VPCPeeringHandler) List(c *gin.Context) {
	peerings, err := h.svc.ListPeerings(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, peerings)
}

// Get retrieves a specific VPC peering connection.
// @Summary Get VPC Peering
// @Tags vpc-peerings
// @Security APIKeyAuth
// @Param id path string true "Peering ID"
// @Produce json
// @Success 200 {object} domain.VPCPeering
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /vpc-peerings/{id} [get]
func (h *VPCPeeringHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidPeeringID))
		return
	}

	peering, err := h.svc.GetPeering(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, peering)
}

// Accept accepts a pending VPC peering connection.
// @Summary Accept VPC Peering
// @Tags vpc-peerings
// @Security APIKeyAuth
// @Param id path string true "Peering ID"
// @Produce json
// @Success 200 {object} domain.VPCPeering
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /vpc-peerings/{id}/accept [post]
func (h *VPCPeeringHandler) Accept(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidPeeringID))
		return
	}

	peering, err := h.svc.AcceptPeering(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, peering)
}

// Reject rejects a pending VPC peering connection.
// @Summary Reject VPC Peering
// @Tags vpc-peerings
// @Security APIKeyAuth
// @Param id path string true "Peering ID"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /vpc-peerings/{id}/reject [post]
func (h *VPCPeeringHandler) Reject(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidPeeringID))
		return
	}

	if err := h.svc.RejectPeering(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "peering rejected"})
}

// Delete tears down and removes a VPC peering connection.
// @Summary Delete VPC Peering
// @Tags vpc-peerings
// @Security APIKeyAuth
// @Param id path string true "Peering ID"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /vpc-peerings/{id} [delete]
func (h *VPCPeeringHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidPeeringID))
		return
	}

	if err := h.svc.DeletePeering(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "peering deleted"})
}
