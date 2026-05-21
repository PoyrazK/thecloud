// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

const (
	errInvalidRequestBody = "invalid request body"
	errInvalidID          = "invalid id"
)

// parseUUID extracts a UUID from a path parameter and handles error reporting.
func parseUUID(c *gin.Context) (*uuid.UUID, bool) {
	val := c.Param("id")
	if val == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "id is required"))
		return nil, false
	}

	id, err := uuid.Parse(val)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid id format"))
		return nil, false
	}

	return &id, true
}

// getBucket extracts bucket from path params.
func getBucket(c *gin.Context) (string, bool) {
	bucket := c.Param("bucket")
	if bucket == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "bucket is required"))
		return "", false
	}
	return bucket, true
}

// getBucketAndKeyRequired extracts bucket and key from path params, requiring both.
func getBucketAndKeyRequired(c *gin.Context) (bucket, key string, ok bool) {
	bucket, ok = getBucket(c)
	if !ok {
		return "", "", false
	}

	key = c.Param("key")
	if key == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "key is required"))
		return "", "", false
	}

	// Prevent path traversal attacks (e.g., "../../../etc/passwd")
	if strings.Contains(key, "..") || strings.Contains(key, "/") || strings.Contains(key, "\\") {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid key"))
		return "", "", false
	}

	return bucket, key, true
}
