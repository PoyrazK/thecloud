package httphandlers

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

const (
	maxNameLength  = 64
	minNameLength  = 1
	maxImageLength = 256
)

type InstanceHandler struct {
	svc ports.InstanceService
}

func NewInstanceHandler(svc ports.InstanceService) *InstanceHandler {
	return &InstanceHandler{svc: svc}
}

type VolumeAttachmentRequest struct {
	VolumeID  string `json:"volume_id"`
	MountPath string `json:"mount_path"`
}

type LaunchRequest struct {
	Name    string                    `json:"name" binding:"required"`
	Image   string                    `json:"image" binding:"required"`
	Ports   string                    `json:"ports"`
	VpcID   string                    `json:"vpc_id"`
	Volumes []VolumeAttachmentRequest `json:"volumes"`
}

// validateLaunchRequest performs custom validation beyond struct tags
func validateLaunchRequest(req *LaunchRequest) error {
	// Validate name
	req.Name = strings.TrimSpace(req.Name)
	if len(req.Name) < minNameLength || len(req.Name) > maxNameLength {
		return errors.New(errors.InvalidInput, "name must be between 1 and 64 characters")
	}
	if !isValidResourceName(req.Name) {
		return errors.New(errors.InvalidInput, "name must contain only alphanumeric characters, hyphens, and underscores")
	}

	// Validate image
	req.Image = strings.TrimSpace(req.Image)
	if req.Image == "" {
		return errors.New(errors.InvalidInput, "image is required")
	}
	if len(req.Image) > maxImageLength {
		return errors.New(errors.InvalidInput, "image name too long (max 256 characters)")
	}

	// Validate volume attachments
	for i, v := range req.Volumes {
		if strings.TrimSpace(v.VolumeID) == "" {
			return errors.New(errors.InvalidInput, "volume_id is required for volume attachment")
		}
		if strings.TrimSpace(v.MountPath) == "" {
			return errors.New(errors.InvalidInput, "mount_path is required for volume attachment")
		}
		if !strings.HasPrefix(v.MountPath, "/") {
			return errors.New(errors.InvalidInput, "mount_path must be an absolute path starting with /")
		}
		req.Volumes[i].VolumeID = strings.TrimSpace(v.VolumeID)
		req.Volumes[i].MountPath = strings.TrimSpace(v.MountPath)
	}

	return nil
}

// isValidResourceName checks if name contains only valid characters (alphanumeric, hyphen, underscore)
func isValidResourceName(name string) bool {
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// Launch launches a new instance
// @Summary Launch a new instance
// @Description Creates and starts a new compute instance with optional volumes and VPC
// @Tags instances
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body LaunchRequest true "Launch request"
// @Success 201 {object} domain.Instance
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances [post]
func (h *InstanceHandler) Launch(c *gin.Context) {
	var req LaunchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	// Custom validation
	if err := validateLaunchRequest(&req); err != nil {
		httputil.Error(c, err)
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

	// Convert volume attachments
	var volumes []domain.VolumeAttachment
	for _, v := range req.Volumes {
		volumes = append(volumes, domain.VolumeAttachment{
			VolumeIDOrName: v.VolumeID,
			MountPath:      v.MountPath,
		})
	}

	inst, err := h.svc.LaunchInstance(c.Request.Context(), req.Name, req.Image, req.Ports, vpcUUID, volumes)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, inst)
}

// List returns all instances
// @Summary List all instances
// @Description Gets a list of all compute instances
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Success 200 {array} domain.Instance
// @Failure 500 {object} httputil.Response
// @Router /instances [get]
func (h *InstanceHandler) List(c *gin.Context) {
	instances, err := h.svc.ListInstances(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, instances)
}

// Stop stops an instance
// @Summary Stop an instance
// @Description Initiates a graceful shutdown of a compute instance
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/stop [post]
func (h *InstanceHandler) Stop(c *gin.Context) {
	idStr := c.Param("id")

	if err := h.svc.StopInstance(c.Request.Context(), idStr); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "instance stop initiated"})
}

// GetLogs returns instance logs
// @Summary Get instance logs
// @Description Gets the console output logs for a compute instance
// @Tags instances
// @Produce plain
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {string} string "Logs content"
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/logs [get]
func (h *InstanceHandler) GetLogs(c *gin.Context) {
	idStr := c.Param("id")

	logs, err := h.svc.GetInstanceLogs(c.Request.Context(), idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	c.String(http.StatusOK, logs)
}

// Get returns instance details
// @Summary Get instance details
// @Description Gets detailed information about a specific compute instance
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} domain.Instance
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id} [get]
func (h *InstanceHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	inst, err := h.svc.GetInstance(c.Request.Context(), idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, inst)
}

// Terminate terminates an instance
// @Summary Terminate an instance
// @Description Deletes a compute instance and its associated resources
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id} [delete]
func (h *InstanceHandler) Terminate(c *gin.Context) {
	idStr := c.Param("id")

	if err := h.svc.TerminateInstance(c.Request.Context(), idStr); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "instance terminated"})
}

// GetStats returns instance metrics
// @Summary Get instance stats
// @Description Gets real-time CPU and Memory usage for a compute instance
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} domain.InstanceStats
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/stats [get]
func (h *InstanceHandler) GetStats(c *gin.Context) {
	idStr := c.Param("id")
	stats, err := h.svc.GetInstanceStats(c.Request.Context(), idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, stats)
}
