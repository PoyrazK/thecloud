package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type AuthHandler struct {
	authSvc ports.AuthService
	resp    *httputil.Responder
}

func NewAuthHandler(authSvc ports.AuthService) *AuthHandler {
	return &AuthHandler{
		authSvc: authSvc,
		resp:    httputil.NewResponder(),
	}
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	User   interface{} `json:"user"`
	ApiKey string      `json:"api_key"`
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
		h.resp.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	user, err := h.authSvc.Register(c.Request.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		h.resp.Error(c, err)
		return
	}

	h.resp.Success(c, http.StatusCreated, user)
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
		h.resp.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	user, apiKey, err := h.authSvc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.resp.Error(c, err)
		return
	}

	h.resp.Success(c, http.StatusOK, LoginResponse{
		User:   user,
		ApiKey: apiKey,
	})
}
