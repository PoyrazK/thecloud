// Package httphandlers provides HTTP handlers for the API.
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

// InstanceHandler handles instance lifecycle HTTP endpoints.
type InstanceHandler struct {
	svc ports.InstanceService
}

// NewInstanceHandler constructs an InstanceHandler.
func NewInstanceHandler(svc ports.InstanceService) *InstanceHandler {
	return &InstanceHandler{svc: svc}
}

// VolumeAttachmentRequest is the payload for attaching a volume.
type VolumeAttachmentRequest struct {
	VolumeID  string `json:"volume_id"`
	MountPath string `json:"mount_path"`
}

// LaunchRequest is the payload for launching a new instance.
type LaunchRequest struct {
	Name         string                    `json:"name" binding:"required"`
	Image        string                    `json:"image" binding:"required"`
	InstanceType string                    `json:"instance_type,omitempty"`
	Ports        string                    `json:"ports"`
	VpcID        string                    `json:"vpc_id"`
	SubnetID     string                    `json:"subnet_id"`
	Volumes      []VolumeAttachmentRequest `json:"volumes"`
	VolumeBinds  []string                  `json:"volume_binds"`
	Env          []string                  `json:"env"`
	Cmd          []string                  `json:"cmd"`
	CPULimit     int64                     `json:"cpu_limit"`
	MemoryLimit  int64                     `json:"memory_limit"`
	DiskLimit    int64                     `json:"disk_limit"`
	SSHKeyID     string                    `json:"ssh_key_id,omitempty"`
	Metadata     map[string]string         `json:"metadata,omitempty"`
	Labels       map[string]string         `json:"labels,omitempty"`
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
// @Description Creates and starts a new compute instance with optional volumes, VPC, environment variables, commands, and resource limits
// @Tags instances
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body LaunchRequest true "Launch request"
// @Success 202 {object} domain.Instance
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances [post]
func (h *InstanceHandler) Launch(c *gin.Context) {
	var req LaunchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	if err := validateLaunchRequest(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	vpcUUID, subnetUUID, volumes, err := h.mapLaunchRequest(req)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}
	var sshKeyID *uuid.UUID
	if req.SSHKeyID != "" {
		id, err := uuid.Parse(req.SSHKeyID)
		if err != nil {
			httputil.Error(c, errors.New(errors.InvalidInput, "invalid ssh_key_id format"))
			return
		}
		sshKeyID = &id
	}

	inst, err := h.svc.LaunchInstance(c.Request.Context(), ports.LaunchParams{
		Name:         req.Name,
		Image:        req.Image,
		Ports:        req.Ports,
		InstanceType: req.InstanceType,
		VpcID:        vpcUUID,
		SubnetID:     subnetUUID,
		Volumes:      volumes,
		VolumeBinds:  req.VolumeBinds,
		Env:          req.Env,
		Cmd:          req.Cmd,
		CPULimit:     req.CPULimit,
		MemoryLimit:  req.MemoryLimit,

		DiskLimit: req.DiskLimit,
		SSHKeyID:  sshKeyID,
		Metadata:  req.Metadata,
		Labels:    req.Labels,
	})
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusAccepted, inst)
}

func (h *InstanceHandler) mapLaunchRequest(req LaunchRequest) (*uuid.UUID, *uuid.UUID, []domain.VolumeAttachment, error) {
	var vpcUUID, subnetUUID *uuid.UUID

	if req.VpcID != "" {
		id, err := uuid.Parse(req.VpcID)
		if err != nil {
			return nil, nil, nil, errors.New(errors.InvalidInput, "invalid vpc_id format")
		}
		vpcUUID = &id
	}

	if req.SubnetID != "" {
		id, err := uuid.Parse(req.SubnetID)
		if err == nil {
			subnetUUID = &id
		}
	}

	volumes := make([]domain.VolumeAttachment, 0, len(req.Volumes))
	for _, v := range req.Volumes {
		volumes = append(volumes, domain.VolumeAttachment{
			VolumeIDOrName: v.VolumeID,
			MountPath:      v.MountPath,
		})
	}

	return vpcUUID, subnetUUID, volumes, nil
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
	tags := c.QueryArray("tag")
	instances, err := h.svc.ListInstances(c.Request.Context(), tags)
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
	id := c.Param("id")
	if id == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "id is required"))
		return
	}

	if err := h.svc.StopInstance(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "instance stop initiated"})
}

// Pause pauses a running instance
// @Summary Pause an instance
// @Description Freezes a running instance (CPU halted, memory/network retained)
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 409 {object} httputil.Response "Instance not in RUNNING state"
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/pause [post]
func (h *InstanceHandler) Pause(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "id is required"))
		return
	}

	if err := h.svc.PauseInstance(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "instance paused"})
}

// Resume resumes a paused instance
// @Summary Resume an instance
// @Description Resumes a paused instance back to running state
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 401 {object} httputil.Response
// @Failure 403 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 409 {object} httputil.Response "Instance not in PAUSED state"
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/resume [post]
func (h *InstanceHandler) Resume(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "id is required"))
		return
	}

	if err := h.svc.ResumeInstance(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "instance resumed"})
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
	id := c.Param("id")
	if id == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "id is required"))
		return
	}

	logs, err := h.svc.GetInstanceLogs(c.Request.Context(), id)
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
	id := c.Param("id")
	if id == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "id is required"))
		return
	}

	inst, err := h.svc.GetInstance(c.Request.Context(), id)
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
	id := c.Param("id")
	if id == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "id is required"))
		return
	}

	if err := h.svc.TerminateInstance(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "instance terminated"})
}

// GetStats returns instance metrics
// @Summary Get instance stats
// @Description Gets real-time CPU, memory, network I/O, disk I/O, and CPU time (nanoseconds) for a compute instance
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} domain.InstanceStats
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/stats [get]
func (h *InstanceHandler) GetStats(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "id is required"))
		return
	}
	stats, err := h.svc.GetInstanceStats(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, stats)
}

// GetConsole returns a console URL for the instance
// @Summary Get instance console
// @Description Gets a VNC URL for the instance console
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/console [get]
func (h *InstanceHandler) GetConsole(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "id is required"))
		return
	}
	url, err := h.svc.GetConsoleURL(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"url": url})
}

// UpdateMetadata updates instance metadata
// @Summary Update instance metadata
// @Description Updates the metadata and labels for a compute instance
// @Tags instances
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Param request body map[string]interface{} true "Metadata and Labels"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/metadata [put]
func (h *InstanceHandler) UpdateMetadata(c *gin.Context) {
	id, ok := parseUUID(c)
	if !ok {
		return
	}

	var req struct {
		Metadata map[string]string `json:"metadata"`
		Labels   map[string]string `json:"labels"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	if err := h.svc.UpdateInstanceMetadata(c.Request.Context(), *id, req.Metadata, req.Labels); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "metadata updated"})
}

// GetTags returns the labels for an instance.
// @Summary Get instance tags
// @Description Returns the label set (tags) assigned to an instance.
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/tags [get]
func (h *InstanceHandler) GetTags(c *gin.Context) {
	id, ok := parseUUID(c)
	if !ok {
		return
	}

	labels, err := h.svc.GetTags(c.Request.Context(), *id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, labels)
}

// SetTagsRequest is the payload for setting instance tags.
type SetTagsRequest struct {
	Tags map[string]string `json:"tags"`
}

// SetTags replaces an instance's labels with the provided map (add/update).
// @Summary Set instance tags
// @Description Merges the provided tags into the instance's label set. Existing keys are overwritten.
// @Tags instances
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/tags [post]
func (h *InstanceHandler) SetTags(c *gin.Context) {
	id, ok := parseUUID(c)
	if !ok {
		return
	}

	var req SetTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	if err := h.svc.SetTags(c.Request.Context(), *id, req.Tags); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "tags updated"})
}

// RemoveTag deletes a single label key from an instance.
// @Summary Remove instance tag
// @Description Deletes the label with the given key from an instance.
// @Tags instances
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Param key path string true "Tag key"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/tags/{key} [delete]
func (h *InstanceHandler) RemoveTag(c *gin.Context) {
	id, ok := parseUUID(c)
	if !ok {
		return
	}
	key := c.Param("key")
	if key == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "key is required"))
		return
	}

	if err := h.svc.RemoveTag(c.Request.Context(), *id, key); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "tag removed"})
}

// ResizeInstanceRequest is the payload for resizing an instance.
type ResizeInstanceRequest struct {
	InstanceType string `json:"instance_type" binding:"required"`
}

// ResizeInstanceResponse is the response for a successful resize operation.
type ResizeInstanceResponse struct {
	Message      string `json:"message"`
	InstanceType string `json:"instance_type"`
	Status       string `json:"status"`
}

// ResizeInstance godoc
// @Summary Resize an instance
// @Description Change the instance type (CPU/memory) of an existing instance. Note: Libvirt-backed instances require a brief restart (cold resize); Docker-backed instances support live resize without downtime.
// @Tags instances
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Instance ID"
// @Param request body ResizeInstanceRequest true "Resize request"
// @Success 200 {object} httphandlers.ResizeInstanceResponse
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 429 {object} httputil.Response "Too Many Requests"
// @Failure 500 {object} httputil.Response
// @Router /instances/{id}/resize [post]
func (h *InstanceHandler) ResizeInstance(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "id is required"))
		return
	}

	var req ResizeInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	inst, err := h.svc.ResizeInstance(c.Request.Context(), idStr, req.InstanceType)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, ResizeInstanceResponse{
		Message:      "instance resized",
		InstanceType: inst.InstanceType,
		Status:       string(inst.Status),
	})
}
