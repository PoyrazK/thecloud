// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

// AccountingHandler handles billing and usage HTTP endpoints.
type AccountingHandler struct {
	svc ports.AccountingService
}

// NewAccountingHandler constructs an AccountingHandler.
func NewAccountingHandler(svc ports.AccountingService) *AccountingHandler {
	return &AccountingHandler{svc: svc}
}

// GetSummary returns a billing summary for the user
// @Summary Get billing summary
// @Tags billing
// @Produce json
// @Param start query string false "Start time (RFC3339)"
// @Param end query string false "End time (RFC3339)"
// @Success 200 {object} domain.BillSummary
// @Failure 401 {object} httputil.Response
// @Router /billing/summary [get]
func (h *AccountingHandler) GetSummary(c *gin.Context) {
	userID := httputil.GetUserID(c)
	startStr := c.Query("start")
	endStr := c.Query("end")

	now := time.Now()
	start := now.AddDate(0, -1, 0)
	end := now

	if startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			httputil.Error(c, errors.New(errors.InvalidInput, fmt.Sprintf("invalid start time %q (expected RFC3339)", startStr)))
			return
		}
		start = t
	}
	if endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			httputil.Error(c, errors.New(errors.InvalidInput, fmt.Sprintf("invalid end time %q (expected RFC3339)", endStr)))
			return
		}
		end = t
	}

	summary, err := h.svc.GetSummary(c.Request.Context(), userID, start, end)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, summary)
}

// ListUsage returns detailed usage records
// @Summary List usage records
// @Tags billing
// @Produce json
// @Param start query string false "Start time (RFC3339)"
// @Param end query string false "End time (RFC3339)"
// @Success 200 {array} domain.UsageRecord
// @Failure 401 {object} httputil.Response
// @Router /billing/usage [get]
func (h *AccountingHandler) ListUsage(c *gin.Context) {
	userID := httputil.GetUserID(c)
	startStr := c.Query("start")
	endStr := c.Query("end")

	now := time.Now()
	start := now.AddDate(0, -1, 0)
	end := now

	if startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			httputil.Error(c, errors.New(errors.InvalidInput, fmt.Sprintf("invalid start time %q (expected RFC3339)", startStr)))
			return
		}
		start = t
	}
	if endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			httputil.Error(c, errors.New(errors.InvalidInput, fmt.Sprintf("invalid end time %q (expected RFC3339)", endStr)))
			return
		}
		end = t
	}

	records, err := h.svc.ListUsage(c.Request.Context(), userID, start, end)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, records)
}
