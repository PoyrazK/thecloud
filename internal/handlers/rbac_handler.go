package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type RBACHandler struct {
	authSvc ports.AuthService
}

func NewRBACHandler(authSvc ports.AuthService) *RBACHandler {
	return &RBACHandler{authSvc: authSvc}
}

type RoleResponse struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"`
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// ListRoles returns the available roles.
func (h *RBACHandler) ListRoles(c *gin.Context) {
	roles := []string{
		domain.RoleOwner,
		domain.RoleAdmin,
		domain.RoleDeveloper,
		domain.RoleViewer,
	}
	httputil.Success(c, http.StatusOK, gin.H{"roles": roles})
}

// GetMyRole returns the role for the authenticated user.
func (h *RBACHandler) GetMyRole(c *gin.Context) {
	userID := appcontext.UserIDFromContext(c.Request.Context())
	if userID == uuid.Nil {
		httputil.Error(c, errors.New(errors.Unauthorized, "missing user context"))
		return
	}

	user, err := h.authSvc.ValidateUser(c.Request.Context(), userID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, RoleResponse{UserID: user.ID, Role: user.Role})
}

// GetUserRole returns the role for a specific user ID.
func (h *RBACHandler) GetUserRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid user id"))
		return
	}

	user, err := h.authSvc.ValidateUser(c.Request.Context(), userID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, RoleResponse{UserID: user.ID, Role: user.Role})
}

// UpdateUserRole updates the role for a specific user ID.
func (h *RBACHandler) UpdateUserRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid user id"))
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, err)
		return
	}

	user, err := h.authSvc.UpdateUserRole(c.Request.Context(), userID, req.Role)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, RoleResponse{UserID: user.ID, Role: user.Role})
}
