package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type ContainerHandler struct {
	svc ports.ContainerService
}

func NewContainerHandler(svc ports.ContainerService) *ContainerHandler {
	return &ContainerHandler{svc: svc}
}

func (h *ContainerHandler) CreateDeployment(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		Image    string `json:"image" binding:"required"`
		Replicas int    `json:"replicas" binding:"required"`
		Ports    string `json:"ports"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "Invalid request body"))
		return
	}

	dep, err := h.svc.CreateDeployment(c.Request.Context(), req.Name, req.Image, req.Replicas, req.Ports)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, dep)
}

func (h *ContainerHandler) ListDeployments(c *gin.Context) {
	deps, err := h.svc.ListDeployments(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, deps)
}

func (h *ContainerHandler) GetDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "Invalid deployment ID"))
		return
	}

	dep, err := h.svc.GetDeployment(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, dep)
}

func (h *ContainerHandler) ScaleDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "Invalid deployment ID"))
		return
	}

	var req struct {
		Replicas int `json:"replicas" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "Invalid request body"))
		return
	}

	if err := h.svc.ScaleDeployment(c.Request.Context(), id, req.Replicas); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "Deployment scaling initiated"})
}

func (h *ContainerHandler) DeleteDeployment(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "Invalid deployment ID"))
		return
	}

	if err := h.svc.DeleteDeployment(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "Deployment deletion initiated"})
}
