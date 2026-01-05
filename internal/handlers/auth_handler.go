// Package httphandlers implements the HTTP handlers for the application.
package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type AuthHandler struct {
	authSvc ports.AuthService
}

func NewAuthHandler(authSvc ports.AuthService) *AuthHandler {
	return &AuthHandler{
		authSvc: authSvc,
	}
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Name     string `json:"name" binding:"required,min=2,max=100"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,max=72"`
}

type LoginResponse struct {
	User   interface{} `json:"user"`
	APIKey string      `json:"api_key"`
}

// Register godoc
// @Summary Register a new user
// @Description Create a new account on The Cloud
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration Info"
// @Success 201 {object} httputil.Response{data=domain.User}
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	user, err := h.authSvc.Register(c.Request.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, user)
}

// Login godoc
// @Summary Login as a user
// @Description Authenticate and get an API key
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Credentials"
// @Success 200 {object} httputil.Response{data=LoginResponse}
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	user, apiKey, err := h.authSvc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, LoginResponse{
		User:   user,
		APIKey: apiKey,
	})
}
