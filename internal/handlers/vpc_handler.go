package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/poyraz/cloud/internal/core/ports"
	"github.com/poyraz/cloud/pkg/httputil"
)

type VpcHandler struct {
	svc ports.VpcService
}

func NewVpcHandler(svc ports.VpcService) *VpcHandler {
	return &VpcHandler{svc: svc}
}

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

func (h *VpcHandler) List(c *gin.Context) {
	vpcs, err := h.svc.ListVPCs(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, vpcs)
}

func (h *VpcHandler) Get(c *gin.Context) {
	idOrName := c.Param("id")
	vpc, err := h.svc.GetVPC(c.Request.Context(), idOrName)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, vpc)
}

func (h *VpcHandler) Delete(c *gin.Context) {
	idOrName := c.Param("id")
	if err := h.svc.DeleteVPC(c.Request.Context(), idOrName); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "vpc deleted"})
}
