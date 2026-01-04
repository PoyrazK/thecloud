package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/pkg/httputil"
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

// Create creates a new volume
// @Summary Create a new volume
// @Description Creates a new block storage volume
// @Tags volumes
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body CreateVolumeRequest true "Volume creation request"
// @Success 201 {object} domain.Volume
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /volumes [post]
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

// List returns all volumes
// @Summary List all volumes
// @Description Gets a list of all existing block storage volumes
// @Tags volumes
// @Produce json
// @Security APIKeyAuth
// @Success 200 {array} domain.Volume
// @Failure 500 {object} httputil.Response
// @Router /volumes [get]
func (h *VolumeHandler) List(c *gin.Context) {
	volumes, err := h.svc.ListVolumes(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, volumes)
}

// Get returns volume details
// @Summary Get volume details
// @Description Gets detailed information about a specific block storage volume
// @Tags volumes
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Volume ID"
// @Success 200 {object} domain.Volume
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /volumes/{id} [get]
func (h *VolumeHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	vol, err := h.svc.GetVolume(c.Request.Context(), idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, vol)
}

// Delete deletes a volume
// @Summary Delete a volume
// @Description Removes a block storage volume
// @Tags volumes
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Volume ID"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /volumes/{id} [delete]
func (h *VolumeHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	if err := h.svc.DeleteVolume(c.Request.Context(), idStr); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "volume deleted"})
}
