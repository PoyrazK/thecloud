// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

// SecretHandler handles secret HTTP endpoints.
type SecretHandler struct {
	svc ports.SecretService
}

// NewSecretHandler constructs a SecretHandler.
func NewSecretHandler(svc ports.SecretService) *SecretHandler {
	return &SecretHandler{svc: svc}
}

// CreateSecretRequest is the payload for secret creation.
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

func (h *SecretHandler) resolveShortID(ctx context.Context, idStr string) (uuid.UUID, error) {
	// First try as full UUID
	if id, err := uuid.Parse(idStr); err == nil {
		return id, nil
	}

	// Then try by name
	if secret, err := h.svc.GetSecretByName(ctx, idStr); err == nil {
		return secret.ID, nil
	}

	// Then try short ID prefix match
	secrets, err := h.svc.ListSecrets(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	idStrLower := strings.ToLower(idStr)
	var candidates []*domain.Secret
	for _, s := range secrets {
		if strings.HasPrefix(s.ID.String(), idStr) || strings.HasPrefix(strings.ToLower(s.Name), idStrLower) {
			candidates = append(candidates, s)
		}
	}

	if len(candidates) == 0 {
		return uuid.Nil, errors.New(errors.NotFound, "secret not found")
	}
	if len(candidates) > 1 {
		return uuid.Nil, errors.New(errors.InvalidInput, "ambiguous secret identifier")
	}

	return candidates[0].ID, nil
}

func (h *SecretHandler) Get(c *gin.Context) {
	idStr := c.Param("id")

	id, err := h.resolveShortID(c.Request.Context(), idStr)
	if err != nil {
		httputil.Error(c, err)
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

	id, err := h.resolveShortID(c.Request.Context(), idStr)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	if err := h.svc.DeleteSecret(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "secret deleted"})
}
