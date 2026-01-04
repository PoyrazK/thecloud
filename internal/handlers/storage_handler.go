package httphandlers

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type StorageHandler struct {
	svc ports.StorageService
}

func NewStorageHandler(svc ports.StorageService) *StorageHandler {
	return &StorageHandler{
		svc: svc,
	}
}

// Upload uploads an object to a bucket
// @Summary Upload an object
// @Description Uploads a file/object to the specified bucket and key
// @Tags storage
// @Accept octet-stream
// @Produce json
// @Security APIKeyAuth
// @Param bucket path string true "Bucket name"
// @Param key path string true "Object key"
// @Param file formData file true "File to upload"
// @Success 201 {object} domain.Object
// @Failure 400 {object} httputil.Response
// @Router /storage/{bucket}/{key} [put]
func (h *StorageHandler) Upload(c *gin.Context) {
	bucket := c.Param("bucket")
	key := c.Param("key")

	if bucket == "" || key == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "bucket and key are required"))
		return
	}

	// Read from request body (stream)
	obj, err := h.svc.Upload(c.Request.Context(), bucket, key, c.Request.Body)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, obj)
}

// Download downloads an object from a bucket
// @Summary Download an object
// @Description Streams the specified object as an attachment
// @Tags storage
// @Produce octet-stream
// @Security APIKeyAuth
// @Param bucket path string true "Bucket name"
// @Param key path string true "Object key"
// @Success 200 {file} file "Object content"
// @Failure 404 {object} httputil.Response
// @Router /storage/{bucket}/{key} [get]
func (h *StorageHandler) Download(c *gin.Context) {
	bucket := c.Param("bucket")
	key := c.Param("key")

	reader, obj, err := h.svc.Download(c.Request.Context(), bucket, key)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	defer func() { _ = reader.Close() }()

	// Set headers
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", key))
	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", obj.SizeBytes))

	// Stream file to client
	_, _ = io.Copy(c.Writer, reader)
}

// List returns objects in a bucket
// @Summary List objects in a bucket
// @Description Gets a list of all objects within a specific bucket
// @Tags storage
// @Produce json
// @Security APIKeyAuth
// @Param bucket path string true "Bucket name"
// @Success 200 {array} domain.Object
// @Failure 404 {object} httputil.Response
// @Router /storage/{bucket} [get]
func (h *StorageHandler) List(c *gin.Context) {
	bucket := c.Param("bucket")

	objects, err := h.svc.ListObjects(c.Request.Context(), bucket)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, objects)
}

// Delete deletes an object from a bucket
// @Summary Delete an object
// @Description Removes an object from the specified bucket
// @Tags storage
// @Produce json
// @Security APIKeyAuth
// @Param bucket path string true "Bucket name"
// @Param key path string true "Object key"
// @Success 204
// @Failure 404 {object} httputil.Response
// @Router /storage/{bucket}/{key} [delete]
func (h *StorageHandler) Delete(c *gin.Context) {
	bucket := c.Param("bucket")
	key := c.Param("key")

	if err := h.svc.DeleteObject(c.Request.Context(), bucket, key); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusNoContent, nil)
}
