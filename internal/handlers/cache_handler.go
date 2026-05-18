// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

// CacheHandler handles cache HTTP endpoints.
type CacheHandler struct {
	svc ports.CacheService
}

// NewCacheHandler constructs a CacheHandler.
func NewCacheHandler(svc ports.CacheService) *CacheHandler {
	return &CacheHandler{svc: svc}
}

// CreateCacheRequest is the payload for cache creation.
type CreateCacheRequest struct {
	Name     string     `json:"name" binding:"required"`
	Version  string     `json:"version" binding:"required"`
	MemoryMB int        `json:"memory_mb" binding:"required"`
	VpcID    *uuid.UUID `json:"vpc_id"`
}

func (h *CacheHandler) Create(c *gin.Context) {
	var req CreateCacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	cache, err := h.svc.CreateCache(c.Request.Context(), req.Name, req.Version, req.MemoryMB, req.VpcID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, cache)
}

func (h *CacheHandler) List(c *gin.Context) {
	caches, err := h.svc.ListCaches(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, caches)
}

func (h *CacheHandler) Get(c *gin.Context) {
	idOrName := c.Param("id")
	cache, err := h.svc.GetCache(c.Request.Context(), idOrName)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, cache)
}

func (h *CacheHandler) Delete(c *gin.Context) {
	idOrName := c.Param("id")
	if err := h.svc.DeleteCache(c.Request.Context(), idOrName); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "cache deleted"})
}

func (h *CacheHandler) GetConnectionString(c *gin.Context) {
	idOrName := c.Param("id")
	connStr, err := h.svc.GetConnectionString(c.Request.Context(), idOrName)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"connection_string": connStr})
}

func (h *CacheHandler) Flush(c *gin.Context) {
	idOrName := c.Param("id")
	if err := h.svc.FlushCache(c.Request.Context(), idOrName); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "cache flushed"})
}

func (h *CacheHandler) GetStats(c *gin.Context) {
	idOrName := c.Param("id")
	stats, err := h.svc.GetCacheStats(c.Request.Context(), idOrName)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, stats)
}

// ResizeCacheRequest is the payload for resizing cache memory.
type ResizeCacheRequest struct {
	MemoryMB int `json:"memory_mb" binding:"required,min=64"`
}

// Resize resizes a cache instance's memory allocation
// @Summary Resize cache memory
// @Description Changes the memory allocation of a running Redis cache instance (requires restart)
// @Tags caches
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Cache ID or name"
// @Param request body ResizeCacheRequest true "Resize request"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /caches/{id}/resize [post]
func (h *CacheHandler) Resize(c *gin.Context) {
	idOrName := c.Param("id")
	var req ResizeCacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}
	if err := h.svc.ResizeCache(c.Request.Context(), idOrName, req.MemoryMB); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "cache resized", "memory_mb": req.MemoryMB})
}
