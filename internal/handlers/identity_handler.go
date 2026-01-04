package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type IdentityHandler struct {
	svc ports.IdentityService
}

func NewIdentityHandler(svc ports.IdentityService) *IdentityHandler {
	return &IdentityHandler{svc: svc}
}

type CreateKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateKey generates a new API key
// @Summary Create a new API key
// @Description Bootstraps access by generating an API key for a given name
// @Tags identity
// @Accept json
// @Produce json
// @Param request body CreateKeyRequest true "Key creation request"
// @Success 201 {object} domain.APIKey
// @Failure 400 {object} httputil.Response
// @Router /auth/keys [post]
func (h *IdentityHandler) CreateKey(c *gin.Context) {
	var req CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	// For bootstrapping, we use a nil UUID or a system-level UUID
	// In the future, this endpoint should be protected or removed in favor of Login
	key, err := h.svc.CreateKey(c.Request.Context(), uuid.Nil, req.Name)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, key)
}
