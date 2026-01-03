package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type SecretHandler struct {
	svc ports.SecretService
}

func NewSecretHandler(svc ports.SecretService) *SecretHandler {
	return &SecretHandler{svc: svc}
}

type CreateSecretRequest struct {
	Name        string `json:"name" binding:"required"`
	Value       string `json:"value" binding:"required"`
	Description string `json:"description"`
}

func (h *SecretHandler) Create(c *gin.Context) {
	var req CreateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	secret, err := h.svc.CreateSecret(c.Request.Context(), req.Name, req.Value, req.Description)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	// In response, we return metadata, value might be sensitive but for 'Create' it's confirmed back.
	httputil.Success(c, http.StatusCreated, secret)
}

func (h *SecretHandler) List(c *gin.Context) {
	secrets, err := h.svc.ListSecrets(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, secrets)
}

func (h *SecretHandler) Get(c *gin.Context) {
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		// Try by name if not UUID
		secret, err := h.svc.GetSecretByName(c.Request.Context(), idStr)
		if err != nil {
			httputil.Error(c, err)
			return
		}
		httputil.Success(c, http.StatusOK, secret)
		return
	}

	secret, err := h.svc.GetSecret(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, secret)
}

func (h *SecretHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		// Try by name
		secret, err := h.svc.GetSecretByName(c.Request.Context(), idStr)
		if err != nil {
			httputil.Error(c, err)
			return
		}
		id = secret.ID
	}

	if err := h.svc.DeleteSecret(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "secret deleted"})
}
