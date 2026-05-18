// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

// AuthHandler handles authentication and password reset endpoints.
type AuthHandler struct {
	authSvc     ports.AuthService
	pwdSvc      ports.PasswordResetService
	identitySvc ports.IdentityService
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(authSvc ports.AuthService, pwdSvc ports.PasswordResetService, identitySvc ports.IdentityService) *AuthHandler {
	return &AuthHandler{
		authSvc:     authSvc,
		pwdSvc:      pwdSvc,
		identitySvc: identitySvc,
	}
}

// RegisterRequest is the payload for user registration.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Name     string `json:"name" binding:"required,min=2,max=100"`
}

// LoginRequest is the payload for user login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,max=72"`
}

// ForgotPasswordRequest is the payload for requesting a reset token.
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest is the payload for completing a password reset.
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// TokenRequest is the payload for OAuth2 token endpoint.
type TokenRequest struct {
	GrantType    string `form:"grant_type" binding:"required"`
	ClientID     string `form:"client_id" binding:"required"`
	ClientSecret string `form:"client_secret" binding:"required"`
}

// TokenResponse is the OAuth2 token response.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// LoginResponse contains the authenticated user and API key.
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

// ForgotPassword godoc
// @Summary Request password reset
// @Description Sends a password reset token (logged to console for now)
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body ForgotPasswordRequest true "Email"
// @Success 200 {object} httputil.Response
// @Router /auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	// We ignore errors to prevent user enumeration
	_ = h.pwdSvc.RequestReset(c.Request.Context(), req.Email)

	httputil.Success(c, http.StatusOK, gin.H{
		"message": "If the email exists, a reset token has been sent.",
	})
}

// ResetPassword godoc
// @SummaryReset password with token
// @Description Resets a user's password using a valid token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body ResetPasswordRequest true "Reset Info"
// @Success 200 {object} httputil.Response
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	if err := h.pwdSvc.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "password updated successfully"})
}

// Me godoc
// @Summary Get current user info
// @Description Returns the profile of the currently authenticated user
// @Tags Auth
// @Security APIKeyAuth
// @Produce json
// @Success 200 {object} httputil.Response{data=domain.User}
// @Failure 401 {object} httputil.Response
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID := httputil.GetUserID(c)
	user, err := h.authSvc.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, user)
}

// Token godoc
// @Summary Exchange client credentials for access token
// @Description OAuth2 Client Credentials flow - exchange client_id and client_secret for JWT. Response includes access_token (JWT), token_type ("Bearer"), and expires_in (seconds, e.g. 3600 = 1 hour).
// @Tags Auth
// @Accept x-www-form-urlencoded
// @Produce json
// @Param grant_type formData string true "grant_type (client_credentials)"
// @Param client_id formData string true "client_id (service account ID)"
// @Param client_secret formData string true "client_secret"
// @Success 200 {object} TokenResponse
// @Failure 400 {object} httputil.Response
// @Failure 401 {object} httputil.Response
// @Router /oauth2/token [post]
func (h *AuthHandler) Token(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBind(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request"))
		return
	}

	if req.GrantType != "client_credentials" {
		httputil.Error(c, errors.New(errors.InvalidInput, "unsupported grant type"))
		return
	}

	token, err := h.identitySvc.ValidateClientCredentials(c.Request.Context(), req.ClientID, req.ClientSecret)
	if err != nil {
		httputil.Error(c, errors.New(errors.Unauthorized, "invalid client credentials"))
		return
	}

	httputil.Success(c, http.StatusOK, TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int(h.identitySvc.TokenTTL().Seconds()),
	})
}
