package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type VpcHandler struct {
	svc ports.VpcService
}

func NewVpcHandler(svc ports.VpcService) *VpcHandler {
	return &VpcHandler{svc: svc}
}

// Create creates a new VPC
// @Summary Create a new VPC
// @Description Creates a new virtual private cloud network
// @Tags vpcs
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body object{name=string} true "VPC creation request"
// @Success 201 {object} domain.VPC
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /vpcs [post]
func (h *VpcHandler) Create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	vpc, err := h.svc.CreateVPC(c.Request.Context(), req.Name)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, vpc)
}

// List returns all VPCs
// @Summary List all VPCs
// @Description Gets a list of all existing VPCs
// @Tags vpcs
// @Produce json
// @Security APIKeyAuth
// @Success 200 {array} domain.VPC
// @Failure 500 {object} httputil.Response
// @Router /vpcs [get]
func (h *VpcHandler) List(c *gin.Context) {
	vpcs, err := h.svc.ListVPCs(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, vpcs)
}

// Get returns VPC details
// @Summary Get VPC details
// @Description Gets detailed information about a specific VPC
// @Tags vpcs
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "VPC ID or Name"
// @Success 200 {object} domain.VPC
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /vpcs/{id} [get]
func (h *VpcHandler) Get(c *gin.Context) {
	idOrName := c.Param("id")
	vpc, err := h.svc.GetVPC(c.Request.Context(), idOrName)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, vpc)
}

// Delete deletes a VPC
// @Summary Delete a VPC
// @Description Removes a VPC network (must be empty of instances)
// @Tags vpcs
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "VPC ID or Name"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /vpcs/{id} [delete]
func (h *VpcHandler) Delete(c *gin.Context) {
	idOrName := c.Param("id")
	if err := h.svc.DeleteVPC(c.Request.Context(), idOrName); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "vpc deleted"})
}
