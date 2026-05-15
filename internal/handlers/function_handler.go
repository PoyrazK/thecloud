// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

const (
	invalidFunctionIDMsg = "invalid function id"
	// maxFunctionCodeSize prevents memory exhaustion when reading function code uploads.
	maxFunctionCodeSize = 10 * 1024 * 1024 // 10 MB
	// maxInvokePayloadSize prevents memory exhaustion on function invocation payloads.
	maxInvokePayloadSize = 1 * 1024 * 1024 // 1 MB
)

// FunctionHandler handles serverless function HTTP endpoints.
type FunctionHandler struct {
	svc ports.FunctionService
}

// NewFunctionHandler constructs a FunctionHandler.
func NewFunctionHandler(svc ports.FunctionService) *FunctionHandler {
	return &FunctionHandler{svc: svc}
}

// CreateFunctionRequest is the payload for function creation.
type CreateFunctionRequest struct {
	Name    string `form:"name" binding:"required"`
	Runtime string `form:"runtime" binding:"required"`
	Handler string `form:"handler" binding:"required"`
}

// UpdateFunctionRequest is the payload for function update.
type UpdateFunctionRequest struct {
	Handler                  *string          `json:"handler,omitempty"`
	Timeout                  *int             `json:"timeout,omitempty"`
	MemoryMB                 *int             `json:"memory_mb,omitempty"`
	CPUs                     *float64         `json:"cpus,omitempty"`
	Status                   string           `json:"status,omitempty"`
	EnvVars                  []*domain.EnvVar `json:"env_vars,omitempty"`
	MaxConcurrentInvocations *int             `json:"max_concurrent_invocations,omitempty"`
	MaxQueueDepth            *int             `json:"max_queue_depth,omitempty"`
	MaxRetries               *int             `json:"max_retries,omitempty"`
}

func (h *FunctionHandler) Create(c *gin.Context) {
	var req CreateFunctionRequest
	if err := c.ShouldBind(&req); err != nil {
		httputil.Error(c, errors.Wrap(errors.InvalidInput, "invalid request", err))
		return
	}

	file, err := c.FormFile("code")
	if err != nil {
		httputil.Error(c, errors.Wrap(errors.InvalidInput, "code file is required", err))
		return
	}

	f, err := file.Open()
	if err != nil {
		httputil.Error(c, errors.Wrap(errors.Internal, "failed to open code file", err))
		return
	}
	defer func() { _ = f.Close() }()

	code, err := io.ReadAll(io.LimitReader(f, maxFunctionCodeSize))
	if err != nil {
		httputil.Error(c, errors.Wrap(errors.Internal, "failed to read code file", err))
		return
	}

	function, err := h.svc.CreateFunction(c.Request.Context(), req.Name, req.Runtime, req.Handler, code)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, function)
}

func (h *FunctionHandler) List(c *gin.Context) {
	functions, err := h.svc.ListFunctions(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, functions)
}

func (h *FunctionHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, invalidFunctionIDMsg))
		return
	}

	function, err := h.svc.GetFunction(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, function)
}

func (h *FunctionHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, invalidFunctionIDMsg))
		return
	}

	var req UpdateFunctionRequest
	if err := c.ShouldBind(&req); err != nil {
		httputil.Error(c, errors.Wrap(errors.InvalidInput, "invalid request", err))
		return
	}

	fn, err := h.svc.UpdateFunction(c.Request.Context(), id, &domain.FunctionUpdate{
		Handler:                  req.Handler,
		Timeout:                  req.Timeout,
		MemoryMB:                 req.MemoryMB,
		CPUs:                     req.CPUs,
		Status:                   req.Status,
		EnvVars:                  req.EnvVars,
		MaxConcurrentInvocations: req.MaxConcurrentInvocations,
		MaxQueueDepth:            req.MaxQueueDepth,
		MaxRetries:               req.MaxRetries,
	})
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, fn)
}

func (h *FunctionHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, invalidFunctionIDMsg))
		return
	}

	if err := h.svc.DeleteFunction(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "function deleted"})
}

func (h *FunctionHandler) Invoke(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, invalidFunctionIDMsg))
		return
	}

	payload, err := io.ReadAll(io.LimitReader(c.Request.Body, maxInvokePayloadSize))
	if err != nil {
		httputil.Error(c, errors.Wrap(errors.InvalidInput, "failed to read payload", err))
		return
	}

	async := c.Query("async") == "true"

	invocation, err := h.svc.InvokeFunction(c.Request.Context(), id, payload, async)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	status := http.StatusOK
	if async {
		status = http.StatusAccepted
	}
	httputil.Success(c, status, invocation)
}

func (h *FunctionHandler) GetLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, invalidFunctionIDMsg))
		return
	}

	logs, err := h.svc.GetFunctionLogs(c.Request.Context(), id, 100)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, logs)
}

func (h *FunctionHandler) GetDLQ(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, invalidFunctionIDMsg))
		return
	}

	invocations, err := h.svc.GetDLQInvocations(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, invocations)
}

func (h *FunctionHandler) RetryDLQ(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, invalidFunctionIDMsg))
		return
	}

	invocation, err := h.svc.RetryDLQInvocation(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, invocation)
}
