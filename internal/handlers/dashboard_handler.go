package httphandlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/poyraz/cloud/internal/core/ports"
	"github.com/poyraz/cloud/pkg/httputil"
)

// DashboardHandler handles dashboard API endpoints.
type DashboardHandler struct {
	svc ports.DashboardService
}

// NewDashboardHandler creates a new dashboard handler.
func NewDashboardHandler(svc ports.DashboardService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

// GetSummary returns resource counts and overview metrics.
// GET /api/dashboard/summary
func (h *DashboardHandler) GetSummary(c *gin.Context) {
	summary, err := h.svc.GetSummary(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, summary)
}

// GetRecentEvents returns the most recent audit events.
// GET /api/dashboard/events?limit=10
func (h *DashboardHandler) GetRecentEvents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100 // Cap at 100 events
	}

	events, err := h.svc.GetRecentEvents(c.Request.Context(), limit)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, events)
}

// GetStats returns full dashboard statistics.
// GET /api/dashboard/stats
func (h *DashboardHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, stats)
}

// StreamEvents sends real-time dashboard updates via SSE.
// GET /api/dashboard/stream
func (h *DashboardHandler) StreamEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Send initial summary
	summary, err := h.svc.GetSummary(c.Request.Context())
	if err == nil {
		c.SSEvent("summary", summary)
		c.Writer.Flush()
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			summary, err := h.svc.GetSummary(c.Request.Context())
			if err != nil {
				continue
			}
			c.SSEvent("summary", summary)
			c.Writer.Flush()
		}
	}
}
