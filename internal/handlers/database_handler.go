package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type DatabaseHandler struct {
	svc ports.DatabaseService
}

func NewDatabaseHandler(svc ports.DatabaseService) *DatabaseHandler {
	return &DatabaseHandler{svc: svc}
}

type CreateDatabaseRequest struct {
	Name    string     `json:"name" binding:"required"`
	Engine  string     `json:"engine" binding:"required"`
	Version string     `json:"version" binding:"required"`
	VpcID   *uuid.UUID `json:"vpc_id"`
}

func (h *DatabaseHandler) Create(c *gin.Context) {
	var req CreateDatabaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	db, err := h.svc.CreateDatabase(c.Request.Context(), req.Name, req.Engine, req.Version, req.VpcID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, db)
}

func (h *DatabaseHandler) List(c *gin.Context) {
	dbs, err := h.svc.ListDatabases(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, dbs)
}

func (h *DatabaseHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid database id"))
		return
	}

	db, err := h.svc.GetDatabase(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, db)
}

func (h *DatabaseHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid database id"))
		return
	}

	if err := h.svc.DeleteDatabase(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "database deleted"})
}

func (h *DatabaseHandler) GetConnectionString(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid database id"))
		return
	}

	connStr, err := h.svc.GetConnectionString(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"connection_string": connStr})
}

// GetLogs returns database logs
// @Summary Get database logs
// @Description Gets the console output logs for a database instance
// @Tags databases
// @Produce plain
// @Security ApiKeyAuth
// @Param id path string true "Database ID"
// @Success 200 {string} string "Logs content"
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /databases/{id}/logs [get]
func (h *DatabaseHandler) GetLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid database id"))
		return
	}

	logs, err := h.svc.GetDatabaseLogs(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	c.String(http.StatusOK, logs)
}
