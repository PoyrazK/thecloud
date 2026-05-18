package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

// AdminHandler provides internal admin operations.
type AdminHandler struct {
	compute ports.ComputeBackend
}

// NewAdminHandler creates an AdminHandler.
func NewAdminHandler(compute ports.ComputeBackend) *AdminHandler {
	return &AdminHandler{compute: compute}
}

// ResetCircuitBreakers resets the compute circuit breaker state.
// This is used by E2E tests to ensure clean state between test runs.
func (h *AdminHandler) ResetCircuitBreakers(c *gin.Context) {
	if cb, ok := h.compute.(interface{ ResetCircuitBreaker() }); ok {
		cb.ResetCircuitBreaker()
	}
	c.JSON(http.StatusOK, gin.H{"reset": true})
}
