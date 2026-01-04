package ws

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development; restrict in production
		return true
	},
}

// Handler handles WebSocket connections.
type Handler struct {
	hub         *Hub
	identitySvc ports.IdentityService
	logger      *slog.Logger
}

// NewHandler creates a new WebSocket handler.
func NewHandler(hub *Hub, identitySvc ports.IdentityService, logger *slog.Logger) *Handler {
	return &Handler{
		hub:         hub,
		identitySvc: identitySvc,
		logger:      logger,
	}
}

// ServeWS upgrades HTTP to WebSocket and registers the client.
func (h *Handler) ServeWS(c *gin.Context) {
	// Validate API key from query param
	apiKey := c.Query("api_key")
	if apiKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "api_key required"})
		return
	}

	_, err := h.identitySvc.ValidateAPIKey(c.Request.Context(), apiKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid api_key"})
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", slog.String("error", err.Error()))
		return
	}

	client := NewClient(h.hub, conn, apiKey, h.logger)
	h.hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
