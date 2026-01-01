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
	VpcID string `json:"vpc_id"`
}

func (h *InstanceHandler) Launch(c *gin.Context) {
	var req LaunchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err) // Will be mapped to InvalidInput
		return
	}

	var vpcUUID *uuid.UUID
	if req.VpcID != "" {
		id, err := uuid.Parse(req.VpcID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid vpc_id format"})
			return
		}
		vpcUUID = &id
	}

	inst, err := h.svc.LaunchInstance(c.Request.Context(), req.Name, req.Image, req.Ports, vpcUUID)
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

	if err := h.svc.StopInstance(c.Request.Context(), idStr); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "instance stop initiated"})
}

func (h *InstanceHandler) GetLogs(c *gin.Context) {
	idStr := c.Param("id")

	logs, err := h.svc.GetInstanceLogs(c.Request.Context(), idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	c.String(http.StatusOK, logs)
}

func (h *InstanceHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	inst, err := h.svc.GetInstance(c.Request.Context(), idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, inst)
}

func (h *InstanceHandler) Terminate(c *gin.Context) {
	idStr := c.Param("id")

	if err := h.svc.TerminateInstance(c.Request.Context(), idStr); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "instance terminated"})
}

func (h *InstanceHandler) GetStats(c *gin.Context) {
	idStr := c.Param("id")
	stats, err := h.svc.GetInstanceStats(c.Request.Context(), idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, stats)
}
