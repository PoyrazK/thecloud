package simpleaudit

import (
	"context"
	"log/slog"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

// SimpleAuditLogger logs audit events to standard structured logger
type SimpleAuditLogger struct {
	logger *slog.Logger
}

func NewSimpleAuditLogger(logger *slog.Logger) ports.AuditLogger {
	return &SimpleAuditLogger{logger: logger}
}

func (l *SimpleAuditLogger) Log(ctx context.Context, entry *domain.AuditLog) error {
	details := string(entry.Details)
	if details == "" {
		details = "{}"
	}

	userID := "anonymous"
	if entry.UserID != nil {
		userID = entry.UserID.String()
	}

	// We log at INFO level but marked as AUDIT for easy filtering
	l.logger.Info("AUDIT_LOG",
		slog.String("type", "security_audit"),
		slog.String("id", entry.ID.String()),
		slog.String("action", string(entry.Action)),
		slog.String("user_id", userID),
		slog.String("resource", entry.Resource),
		slog.String("ip", entry.IPAddress),
		slog.String("user_agent", entry.UserAgent),
		slog.String("details", details),
	)
	return nil
}
