package httphandlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type GatewayHandler struct {
	svc *services.GatewayService
}

func NewGatewayHandler(svc *services.GatewayService) *GatewayHandler {
	return &GatewayHandler{svc: svc}
}

func (h *GatewayHandler) CreateRoute(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		PathPrefix  string `json:"path_prefix" binding:"required"`
		TargetURL   string `json:"target_url" binding:"required"`
		StripPrefix bool   `json:"strip_prefix"`
		RateLimit   int    `json:"rate_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "Invalid request body"))
		return
	}

	if req.RateLimit == 0 {
		req.RateLimit = 100
	}

	route, err := h.svc.CreateRoute(c.Request.Context(), req.Name, req.PathPrefix, req.TargetURL, req.StripPrefix, req.RateLimit)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, route)
}

func (h *GatewayHandler) ListRoutes(c *gin.Context) {
	routes, err := h.svc.ListRoutes(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, routes)
}

func (h *GatewayHandler) DeleteRoute(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "Invalid route ID"))
		return
	}

	if err := h.svc.DeleteRoute(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "Route deleted"})
}

func (h *GatewayHandler) Proxy(c *gin.Context) {
	path := c.Param("proxy") // Expecting routes like /gw/*proxy
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	proxy, ok := h.svc.GetProxy(path)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "No route found for " + path})
		return
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}
