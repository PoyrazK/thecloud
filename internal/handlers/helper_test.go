package httphandlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

const (
	testBucket = "my-bucket"
	testKey    = "my-key"
)

func TestParseUUID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("valid uuid", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		id := uuid.New()
		c.Params = []gin.Param{{Key: "id", Value: id.String()}}

		parsedID, ok := parseUUID(c)

		assert.True(t, ok)
		assert.Equal(t, id, *parsedID)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing parameter", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = []gin.Param{}

		parsedID, ok := parseUUID(c)

		assert.False(t, ok)
		assert.Nil(t, parsedID)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "id is required")
	})

	t.Run("invalid format", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = []gin.Param{{Key: "id", Value: "invalid-uuid"}}

		parsedID, ok := parseUUID(c)

		assert.False(t, ok)
		assert.Nil(t, parsedID)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid id format")
	})
}

func TestGetBucket(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("valid bucket", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = []gin.Param{{Key: "bucket", Value: testBucket}}

		bucket, ok := getBucket(c)

		assert.True(t, ok)
		assert.Equal(t, testBucket, bucket)
	})

	t.Run("missing bucket", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		bucket, ok := getBucket(c)

		assert.False(t, ok)
		assert.Empty(t, bucket)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestGetBucketAndKeyRequired(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	t.Run("valid bucket and key", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = []gin.Param{
			{Key: "bucket", Value: testBucket},
			{Key: "key", Value: testKey},
		}

		bucket, key, ok := getBucketAndKeyRequired(c)

		assert.True(t, ok)
		assert.Equal(t, testBucket, bucket)
		assert.Equal(t, testKey, key)
	})

	t.Run("missing key", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = []gin.Param{
			{Key: "bucket", Value: "my-bucket"},
		}

		bucket, key, ok := getBucketAndKeyRequired(c)

		assert.False(t, ok)
		assert.Empty(t, bucket)
		assert.Empty(t, key)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "key is required")
	})

	// Regression for #683 — keys containing path traversal segments or NUL
	// bytes must be rejected before reaching the storage backend.
	t.Run("rejects path traversal", func(t *testing.T) {
		badKeys := []string{"../foo", "a/../b", "../../etc/passwd", "..\x00", "/../escape"}
		for _, k := range badKeys {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = []gin.Param{
				{Key: "bucket", Value: testBucket},
				{Key: "key", Value: k},
			}
			_, _, ok := getBucketAndKeyRequired(c)
			assert.Falsef(t, ok, "expected key %q to be rejected", k)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	})

	t.Run("accepts nested keys", func(t *testing.T) {
		goodKeys := []string{"a.txt", "folder/file.png", "deep/nested/object"}
		for _, k := range goodKeys {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = []gin.Param{
				{Key: "bucket", Value: testBucket},
				{Key: "key", Value: k},
			}
			_, gotKey, ok := getBucketAndKeyRequired(c)
			assert.Truef(t, ok, "expected key %q to be accepted", k)
			assert.Equal(t, k, gotKey)
		}
	})
}
