package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type SnapshotHandler struct {
	svc ports.SnapshotService
}

func NewSnapshotHandler(svc ports.SnapshotService) *SnapshotHandler {
	return &SnapshotHandler{svc: svc}
}

type CreateSnapshotRequest struct {
	VolumeID    uuid.UUID `json:"volume_id" binding:"required"`
	Description string    `json:"description"`
}

type RestoreSnapshotRequest struct {
	NewVolumeName string `json:"new_volume_name" binding:"required,min=3,max=64"`
}

// Create creates a new snapshot from a volume
// @Summary Create a new snapshot
// @Description Creates a point-in-time snapshot of an existing volume
// @Tags snapshots
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body CreateSnapshotRequest true "Snapshot creation request"
// @Success 201 {object} domain.Snapshot
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /snapshots [post]
func (h *SnapshotHandler) Create(c *gin.Context) {
	var req CreateSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	snapshot, err := h.svc.CreateSnapshot(c.Request.Context(), req.VolumeID, req.Description)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, snapshot)
}

// List returns all snapshots
// @Summary List all snapshots
// @Description Gets a list of all existing snapshots for the user
// @Tags snapshots
// @Produce json
// @Security APIKeyAuth
// @Success 200 {array} domain.Snapshot
// @Failure 500 {object} httputil.Response
// @Router /snapshots [get]
func (h *SnapshotHandler) List(c *gin.Context) {
	snapshots, err := h.svc.ListSnapshots(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, snapshots)
}

// Get returns snapshot details
// @Summary Get snapshot details
// @Description Gets detailed information about a specific snapshot
// @Tags snapshots
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Snapshot ID"
// @Success 200 {object} domain.Snapshot
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /snapshots/{id} [get]
func (h *SnapshotHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	snapshot, err := h.svc.GetSnapshot(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, snapshot)
}

// Delete deletes a snapshot
// @Summary Delete a snapshot
// @Description Removes a snapshot and its stored data
// @Tags snapshots
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Snapshot ID"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /snapshots/{id} [delete]
func (h *SnapshotHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.svc.DeleteSnapshot(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "snapshot deleted"})
}

// Restore creates a new volume from a snapshot
// @Summary Restore snapshot to volume
// @Description Creates a new volume using data from a snapshot
// @Tags snapshots
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Snapshot ID"
// @Param request body RestoreSnapshotRequest true "Restore request"
// @Success 201 {object} domain.Volume
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /snapshots/{id}/restore [post]
func (h *SnapshotHandler) Restore(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	var req RestoreSnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	vol, err := h.svc.RestoreSnapshot(c.Request.Context(), id, req.NewVolumeName)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, vol)
}
