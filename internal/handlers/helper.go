// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"path"
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

	if err := validateObjectKey(key); err != nil {
		httputil.Error(c, err)
		return "", "", false
	}

	return bucket, key, true
}

// validateObjectKey rejects object keys that could be used for path traversal,
// contain control characters, or would otherwise be unsafe when used as a
// backend object name. Any `..` segment in the raw key is rejected (we do
// not silently collapse it, because the user request explicitly tried to
// reference a parent directory). NUL bytes are rejected outright because
// they can truncate strings in some backends.
func validateObjectKey(key string) error {
	if strings.ContainsRune(key, 0x00) {
		return errors.New(errors.InvalidInput, "invalid characters in key")
	}

	for _, seg := range strings.Split(key, "/") {
		if seg == ".." {
			return errors.New(errors.InvalidInput, "path traversal in key")
		}
	}

	// Reject keys whose canonical form would be just the root or empty.
	cleaned := path.Clean("/" + strings.TrimPrefix(key, "/"))
	if cleaned == "/" {
		return errors.New(errors.InvalidInput, "invalid key")
	}
	return nil
}
