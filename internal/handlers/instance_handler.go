package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyraz/cloud/internal/core/ports"
	"github.com/poyraz/cloud/pkg/httputil"
)

type InstanceHandler struct {
	svc ports.InstanceService
}

func NewInstanceHandler(svc ports.InstanceService) *InstanceHandler {
	return &InstanceHandler{svc: svc}
}

type LaunchRequest struct {
	Name  string `json:"name" binding:"required"`
	Image string `json:"image" binding:"required"`
	Ports string `json:"ports"`
}

func (h *InstanceHandler) Launch(c *gin.Context) {
	var req LaunchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err) // Will be mapped to InvalidInput
		return
	}

	inst, err := h.svc.LaunchInstance(c.Request.Context(), req.Name, req.Image, req.Ports)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, inst)
}

func (h *InstanceHandler) List(c *gin.Context) {
	instances, err := h.svc.ListInstances(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, instances)
}

func (h *InstanceHandler) Stop(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, err) // Invalid UUID
		return
	}

	if err := h.svc.StopInstance(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "instance stop initiated"})
}
