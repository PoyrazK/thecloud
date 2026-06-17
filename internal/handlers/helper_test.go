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

	t.Run("path traversal rejected", func(t *testing.T) {
		traversalKeys := []string{
			"../etc/passwd",
			"foo/../bar",
			"foo\\..\\bar",
			"foo/../../etc/passwd",
		}
		for _, key := range traversalKeys {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = []gin.Param{
				{Key: "bucket", Value: testBucket},
				{Key: "key", Value: key},
			}

			bucket, key, ok := getBucketAndKeyRequired(c)

			assert.False(t, ok, "key %q should be rejected", key)
			assert.Empty(t, bucket)
			assert.Empty(t, key)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	})
}
