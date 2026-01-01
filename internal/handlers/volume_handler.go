package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/poyraz/cloud/internal/core/ports"
	"github.com/poyraz/cloud/pkg/httputil"
)

type VolumeHandler struct {
	svc ports.VolumeService
}

func NewVolumeHandler(svc ports.VolumeService) *VolumeHandler {
	return &VolumeHandler{svc: svc}
}

type CreateVolumeRequest struct {
	Name   string `json:"name" binding:"required"`
	SizeGB int    `json:"size_gb"`
}

func (h *VolumeHandler) Create(c *gin.Context) {
	var req CreateVolumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	if req.SizeGB <= 0 {
		req.SizeGB = 1 // Default 1GB
	}

	vol, err := h.svc.CreateVolume(c.Request.Context(), req.Name, req.SizeGB)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, vol)
}

func (h *VolumeHandler) List(c *gin.Context) {
	volumes, err := h.svc.ListVolumes(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, volumes)
}

func (h *VolumeHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	vol, err := h.svc.GetVolume(c.Request.Context(), idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, vol)
}

func (h *VolumeHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	if err := h.svc.DeleteVolume(c.Request.Context(), idStr); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "volume deleted"})
}
