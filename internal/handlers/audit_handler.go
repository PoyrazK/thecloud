package httphandlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type AuditHandler struct {
	svc ports.AuditService
}

func NewAuditHandler(svc ports.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// ListLogs lists audit logs for the current user
// @Summary List audit logs
// @Tags audit
// @Produce json
// @Param limit query int false "Limit"
// @Success 200 {array} domain.AuditLog
// @Failure 401 {object} httputil.Response
// @Router /audit [get]
func (h *AuditHandler) ListLogs(c *gin.Context) {
	userID := httputil.GetUserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	logs, err := h.svc.ListLogs(c.Request.Context(), userID, limit)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, logs)
}
