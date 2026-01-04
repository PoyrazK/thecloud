package httphandlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type FunctionHandler struct {
	svc ports.FunctionService
}

func NewFunctionHandler(svc ports.FunctionService) *FunctionHandler {
	return &FunctionHandler{svc: svc}
}

type CreateFunctionRequest struct {
	Name    string `form:"name" binding:"required"`
	Runtime string `form:"runtime" binding:"required"`
	Handler string `form:"handler" binding:"required"`
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

	code, err := io.ReadAll(f)
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
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid function id"))
		return
	}

	function, err := h.svc.GetFunction(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, function)
}

func (h *FunctionHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid function id"))
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
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid function id"))
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
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
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid function id"))
		return
	}

	logs, err := h.svc.GetFunctionLogs(c.Request.Context(), id, 100)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, logs)
}
