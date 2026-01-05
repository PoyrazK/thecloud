package simpleaudit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestSimpleAuditLogger_Log(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	audit := NewSimpleAuditLogger(logger)

	userID := uuid.New()
	entry := &domain.AuditLog{
		ID:        uuid.New(),
		UserID:    &userID,
		Action:    "CREATE",
		Resource:  "instance:123",
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
		Details:   json.RawMessage(`{"foo":"bar"}`),
		CreatedAt: time.Now(),
	}

	err := audit.Log(context.Background(), entry)
	assert.NoError(t, err)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "AUDIT_LOG")
	assert.Contains(t, logOutput, "security_audit")
	assert.Contains(t, logOutput, "CREATE")
	assert.Contains(t, logOutput, userID.String())
	assert.Contains(t, logOutput, "instance:123")
	assert.Contains(t, logOutput, "127.0.0.1")
	assert.Contains(t, logOutput, "test-agent")
	assert.Contains(t, logOutput, `{\"foo\":\"bar\"}`)
}

func TestSimpleAuditLogger_Log_Anonymous(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	audit := NewSimpleAuditLogger(logger)

	entry := &domain.AuditLog{
		ID:        uuid.New(),
		UserID:    nil,
		Action:    "LOGIN",
		Resource:  "auth",
		CreatedAt: time.Now(),
	}

	err := audit.Log(context.Background(), entry)
	assert.NoError(t, err)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "anonymous")
}

func TestSimpleAuditLogger_Log_EmptyDetails(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	audit := NewSimpleAuditLogger(logger)

	entry := &domain.AuditLog{
		ID:       uuid.New(),
		Action:   "TEST",
		Resource: "test",
		Details:  json.RawMessage(""),
	}

	err := audit.Log(context.Background(), entry)
	assert.NoError(t, err)

	logOutput := buf.String()
	// Should fallback to "{}"
	// In JSON output it will be escaped as "details":"{}"
	assert.True(t, strings.Contains(logOutput, `"details":"{}"`), "expected details to be empty json object")
}
