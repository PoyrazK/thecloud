package httputil

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/errors"
)

type Meta struct {
	RequestID string `json:"request_id,omitempty"`
	Timestamp string `json:"timestamp"`
}

type Response struct {
	Data  interface{} `json:"data,omitempty"`
	Error interface{} `json:"error,omitempty"`
	Meta  *Meta       `json:"meta,omitempty"`
}

func Success(c *gin.Context, code int, data interface{}) {
	requestID, _ := c.Get("requestID")
	reqIDStr, _ := requestID.(string)

	c.JSON(code, Response{
		Data: data,
		Meta: &Meta{
			RequestID: reqIDStr,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func Error(c *gin.Context, err error) {
	var e errors.Error
	if apiErr, ok := err.(errors.Error); ok {
		e = apiErr
	} else {
		// Log unknown errors for debugging
		// Use a global logger if available or just fmt (standard lib logging) as fallback
		// ideally c.Error(err) should be used so middleware can log it
		_ = c.Error(err)
		fmt.Printf("DEBUG ERROR: %v\n", err)
		e = errors.Error{
			Type:    errors.Internal,
			Message: "An unexpected error occurred",
			Code:    string(errors.Internal),
			Cause:   err,
		}
	}

	statusCode := http.StatusInternalServerError
	switch e.Type {
	case errors.NotFound:
		statusCode = http.StatusNotFound
	case errors.InvalidInput:
		statusCode = http.StatusBadRequest
	case errors.Unauthorized:
		statusCode = http.StatusUnauthorized
	case errors.Forbidden:
		statusCode = http.StatusForbidden
	case errors.Conflict:
		statusCode = http.StatusConflict
	case errors.BucketNotFound, errors.ObjectNotFound:
		statusCode = http.StatusNotFound
	case errors.ObjectTooLarge:
		statusCode = http.StatusRequestEntityTooLarge
	case errors.InstanceNotRunning, errors.PortConflict, errors.TooManyPorts:
		statusCode = http.StatusConflict
	case errors.ResourceLimitExceeded:
		statusCode = http.StatusTooManyRequests // 429
	}

	requestID, _ := c.Get("requestID")
	reqIDStr, _ := requestID.(string)

	c.JSON(statusCode, Response{
		Error: e,
		Meta: &Meta{
			RequestID: reqIDStr,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}
