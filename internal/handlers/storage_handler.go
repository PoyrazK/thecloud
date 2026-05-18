// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/pkg/crypto"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

// StorageHandler handles object storage HTTP endpoints.
type StorageHandler struct {
	svc ports.StorageService
	cfg *platform.Config
}

// NewStorageHandler constructs a StorageHandler.
func NewStorageHandler(svc ports.StorageService, cfg *platform.Config) *StorageHandler {
	return &StorageHandler{
		svc: svc,
		cfg: cfg,
	}
}

const (
	errInvalidUploadID  = "invalid upload id"
	headerContentSha256 = "X-Content-Sha256"
	maxUploadSize       = 5 * 1024 * 1024 * 1024 // 5 GB
)

// contentDispositionAttachment builds a safe `Content-Disposition: attachment`
// header for a stored object.
//
// Object keys can contain path segments, non-ASCII characters, control
// characters, quotes, backslashes, or CRLF that — if interpolated naively —
// would either corrupt the header (HTTP response splitting) or let an attacker
// inject additional headers. The output therefore emits two parameters per
// RFC 6266:
//
//   - `filename="..."`     ASCII-only fallback for legacy clients. All bytes
//     outside the safe printable range and the two
//     characters that are special inside a quoted-string
//     (`"` and `\`) are replaced with `_`.
//   - `filename*=UTF-8”…` RFC 5987 percent-encoded form preserving the
//     original Unicode basename for modern clients.
//
// `path.Base` is used to discard any path segments embedded in the key. If the
// resulting name is empty we fall back to "download".
func contentDispositionAttachment(key string) string {
	name := path.Base(key)
	if name == "." || name == "/" || name == "" {
		name = "download"
	}

	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		asciiFilenameFallback(name), rfc5987Encode(name))
}

// asciiFilenameFallback returns an ASCII-only sanitized copy of name suitable
// for the legacy `filename=` parameter. Any byte that is a control character
// (<0x20 or 0x7f), non-ASCII (>=0x80), or special inside a quoted-string is
// replaced with `_` so the value can be safely wrapped in double quotes.
func asciiFilenameFallback(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c < 0x20, c == 0x7f, c >= 0x80, c == '"', c == '\\':
			out = append(out, '_')
		default:
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "download"
	}
	return string(out)
}

// rfc5987Encode percent-encodes a value per RFC 5987 attr-char rules so it can
// be safely placed in a `filename*` parameter.
func rfc5987Encode(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z',
			c >= 'a' && c <= 'z',
			c >= '0' && c <= '9',
			c == '!', c == '#', c == '$', c == '&', c == '+',
			c == '-', c == '.', c == '^', c == '_', c == '`',
			c == '|', c == '~':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
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
// @Param X-Content-Sha256 header string false "SHA-256 checksum of the content"
// @Success 201 {object} domain.Object
// @Failure 400 {object} httputil.Response
// @Router /storage/{bucket}/{key} [put]
func (h *StorageHandler) Upload(c *gin.Context) {
	bucket, key, ok := getBucketAndKeyRequired(c)
	if !ok {
		return
	}

	// Use only the filename (last path segment) as the object key, not the full path.
	// This mirrors contentDispositionAttachment which already uses path.Base for the same reason.
	objectKey := path.Base(key)

	providedChecksum := c.GetHeader(headerContentSha256)

	// Read from request body (stream)
	obj, err := h.svc.Upload(c.Request.Context(), bucket, objectKey, io.LimitReader(c.Request.Body, maxUploadSize), providedChecksum)
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
	bucket, key, ok := getBucketAndKeyRequired(c)
	if !ok {
		return
	}
	// Normalize key to basename for consistency with Upload
	key = path.Base(key)
	versionID := c.Query("versionId")

	var reader io.ReadCloser
	var obj *domain.Object
	var err error

	if versionID != "" {
		reader, obj, err = h.svc.DownloadVersion(c.Request.Context(), bucket, key, versionID)
	} else {
		reader, obj, err = h.svc.Download(c.Request.Context(), bucket, key)
	}

	if err != nil {
		httputil.Error(c, err)
		return
	}
	defer func() { _ = reader.Close() }()

	// Set headers
	c.Header("Content-Disposition", contentDispositionAttachment(key))
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
	bucket, ok := getBucket(c)
	if !ok {
		return
	}

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
	bucket, key, ok := getBucketAndKeyRequired(c)
	if !ok {
		return
	}
	// Normalize key to basename for consistency with Upload
	key = path.Base(key)
	versionID := c.Query("versionId")

	var err error
	if versionID != "" {
		err = h.svc.DeleteVersion(c.Request.Context(), bucket, key, versionID)
	} else {
		err = h.svc.DeleteObject(c.Request.Context(), bucket, key)
	}

	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusNoContent, nil)
}

// CreateBucket creates a new bucket
// @Summary Create a bucket
// @Description Creates a new storage bucket
// @Tags storage
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body domain.Bucket true "Bucket creation request"
// @Success 201 {object} domain.Bucket
// @Failure 400 {object} httputil.Response
// @Router /storage/buckets [post]
func (h *StorageHandler) CreateBucket(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		IsPublic bool   `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidRequestBody))
		return
	}

	bucket, err := h.svc.CreateBucket(c.Request.Context(), req.Name, req.IsPublic)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, bucket)
}

// ListBuckets lists all buckets for the user
// @Summary List buckets
// @Description Lists all storage buckets owned by the user
// @Tags storage
// @Produce json
// @Security APIKeyAuth
// @Success 200 {array} domain.Bucket
// @Failure 500 {object} httputil.Response
// @Router /storage/buckets [get]
func (h *StorageHandler) ListBuckets(c *gin.Context) {
	buckets, err := h.svc.ListBuckets(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, buckets)
}

// DeleteBucket deletes a bucket
// @Summary Delete a bucket
// @Description Deletes a storage bucket. Use force=true to delete a non-empty bucket.
// @Tags storage
// @Produce json
// @Security APIKeyAuth
// @Param bucket path string true "Bucket name"
// @Param force query bool false "Force delete even if non-empty"
// @Success 204
// @Failure 400 {object} httputil.Response
// @Failure 401 {object} httputil.Response
// @Router /storage/buckets/{bucket} [delete]
func (h *StorageHandler) DeleteBucket(c *gin.Context) {
	bucket := c.Param("bucket")
	if bucket == "" {
		httputil.Error(c, errors.New(errors.InvalidInput, "bucket name required"))
		return
	}

	force := c.Query("force") == "true"

	if err := h.svc.DeleteBucket(c.Request.Context(), bucket, force); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusNoContent, nil)
}

// GetClusterStatus returns the current state of the storage cluster
// @Summary Get storage cluster status
// @Description Returns the status of all nodes in the distributed storage cluster
// @Tags storage
// @Produce json
// @Security APIKeyAuth
// @Success 200 {object} domain.StorageCluster
// @Router /storage/cluster/status [get]
func (h *StorageHandler) GetClusterStatus(c *gin.Context) {
	status, err := h.svc.GetClusterStatus(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, status)
}

// InitiateMultipartUpload initiates a new multipart upload
// @Summary Initiate multipart upload
// @Description Creates a new multipart upload session for a bucket and key
// @Tags storage
// @Produce json
// @Security APIKeyAuth
// @Param bucket path string true "Bucket name"
// @Param key path string true "Object key"
// @Success 201 {object} domain.MultipartUpload
// @Router /storage/{bucket}/{key}/multipart [post]
func (h *StorageHandler) InitiateMultipartUpload(c *gin.Context) {
	bucket, key, ok := getBucketAndKeyRequired(c)
	if !ok {
		return
	}

	upload, err := h.svc.CreateMultipartUpload(c.Request.Context(), bucket, key)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, upload)
}

// UploadPart uploads a single part of an ongoing multipart upload
// @Summary Upload a part
// @Description Uploads a data chunk (part) for the specified multipart upload
// @Tags storage
// @Accept octet-stream
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Upload ID"
// @Param part query int true "Part number"
// @Success 200 {object} domain.Part
// @Router /storage/multipart/:id/parts [put]
func (h *StorageHandler) UploadPart(c *gin.Context) {
	idStr := c.Param("id")
	uploadID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidUploadID))
		return
	}

	partNumStr := c.Query("part")
	partNumber, err := strconv.Atoi(partNumStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid part number"))
		return
	}

	providedChecksum := c.GetHeader(headerContentSha256)

	part, err := h.svc.UploadPart(c.Request.Context(), uploadID, partNumber, io.LimitReader(c.Request.Body, maxUploadSize), providedChecksum)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, part)
}

// CompleteMultipartUpload completes a multipart upload
// @Summary Complete multipart upload
// @Description Assembles all uploaded parts into the final object
// @Tags storage
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Upload ID"
// @Success 200 {object} domain.Object
// @Router /storage/multipart/:id/complete [post]
func (h *StorageHandler) CompleteMultipartUpload(c *gin.Context) {
	idStr := c.Param("id")
	uploadID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidUploadID))
		return
	}

	obj, err := h.svc.CompleteMultipartUpload(c.Request.Context(), uploadID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, obj)
}

// AbortMultipartUpload aborts a multipart upload
// @Summary Abort multipart upload
// @Description Cancels the upload and deletes all uploaded parts
// @Tags storage
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Upload ID"
// @Success 204
// @Router /storage/multipart/:id [delete]
func (h *StorageHandler) AbortMultipartUpload(c *gin.Context) {
	idStr := c.Param("id")
	uploadID, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidUploadID))
		return
	}

	if err := h.svc.AbortMultipartUpload(c.Request.Context(), uploadID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusNoContent, nil)
}

// GeneratePresignedURL creates a time-limited signed URL
func (h *StorageHandler) GeneratePresignedURL(c *gin.Context) {
	bucket, key, ok := getBucketAndKeyRequired(c)
	if !ok {
		return
	}

	var req struct {
		Method    string `json:"method"` // GET or PUT
		ExpirySec int    `json:"expiry_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, errInvalidRequestBody))
		return
	}

	method := req.Method
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodPut {
		httputil.Error(c, errors.New(errors.InvalidInput, "only GET and PUT methods supported"))
		return
	}

	expirySec := req.ExpirySec
	if expirySec <= 0 {
		expirySec = 900 // 15 minutes default
	}
	if expirySec > 7*24*3600 { // max 7 days
		httputil.Error(c, errors.New(errors.InvalidInput, "expiry_seconds exceeds maximum allowed value"))
		return
	}

	presigned, err := h.svc.GeneratePresignedURL(c.Request.Context(), bucket, key, method, time.Duration(expirySec)*time.Second)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, presigned)
}

// ServePresignedDownload handles object download via signed URL (no auth needed)
func (h *StorageHandler) ServePresignedDownload(c *gin.Context) {
	bucket, key, ok := getBucketAndKeyRequired(c)
	if !ok {
		return
	}
	expires := c.Query("expires")
	signature := c.Query("signature")

	// Verify signature
	// Note: We need the secret key here to verify.
	// Ideally the service handles verification, but the domain model is cleaner if we verify here
	// OR delegate to a service method `VerifyPresignedAccess`.
	// For now, let's verify using the shared secret.
	secret := h.cfg.StorageSecret // Use configured secret
	if secret == "" {
		httputil.Error(c, errors.New(errors.Internal, "storage secret not configured"))
		return
	}
	path := fmt.Sprintf("/storage/presigned/%s/%s", bucket, strings.TrimPrefix(key, "/"))

	if err := crypto.VerifyURL(secret, http.MethodGet, path, expires, signature); err != nil {
		httputil.Error(c, errors.New(errors.Forbidden, "invalid or expired signature"))
		return
	}

	// Bypass normal Download auth checks?
	// The problem is `svc.Download` might assume checking permissions via Context's UserID.
	// But `Download` implementation currently only checks repository metadata.
	// We need to bypass the 'AuditLog' or 'RBAC' that might check UserID in context.
	// Since `svc.Download` is mostly pure logic, it should work if we pass background context
	// but we lose audit of *who* generated the link.

	reader, obj, err := h.svc.Download(c.Request.Context(), bucket, key)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	defer func() { _ = reader.Close() }()

	c.Header("Content-Disposition", contentDispositionAttachment(key))
	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", obj.SizeBytes))
	_, _ = io.Copy(c.Writer, reader)
}

// ServePresignedUpload handles object upload via signed URL (no auth needed)
func (h *StorageHandler) ServePresignedUpload(c *gin.Context) {
	bucket, key, ok := getBucketAndKeyRequired(c)
	if !ok {
		return
	}
	expires := c.Query("expires")
	signature := c.Query("signature")

	secret := h.cfg.StorageSecret
	if secret == "" {
		httputil.Error(c, errors.New(errors.Internal, "storage secret not configured"))
		return
	}
	path := fmt.Sprintf("/storage/presigned/%s/%s", bucket, strings.TrimPrefix(key, "/"))

	if err := crypto.VerifyURL(secret, http.MethodPut, path, expires, signature); err != nil {
		httputil.Error(c, errors.New(errors.Forbidden, "invalid or expired signature"))
		return
	}

	// For anonymous uploads, we might ideally need a dummy user ID or the owner's ID embedded in the token.
	// Current Upload implementation grabs UserID from context.
	// We need to support "system" or "anonymous" upload.

	// FIX: The current `svc.Upload` derives UserID from context.
	// We should probably allow the context to carry a "PresignedUser" or similar.
	// or modify Upload to accept nil UserID.

	// For now, let's proceed and see:
	// `Upload` -> `appcontext.UserIDFromContext(ctx)` which returns uuid.Nil if not found.
	// The Repository `SaveMeta` saves this UserID. It's valid to have Nil (0000...) for system/anon uploads?
	// It's acceptable for this feature.

	obj, err := h.svc.Upload(c.Request.Context(), bucket, key, io.LimitReader(c.Request.Body, maxUploadSize), "")
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, obj)
}

// SetBucketVersioning toggles versioning for a bucket
// @Summary Set bucket versioning
// @Description Enables or disables object versioning for the specified bucket
// @Tags storage
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param bucket path string true "Bucket name"
// @Param request body object true "Versioning request"
// @Success 200
// @Router /storage/buckets/{bucket}/versioning [patch]
func (h *StorageHandler) SetBucketVersioning(c *gin.Context) {
	bucket, ok := getBucket(c)
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	if err := h.svc.SetBucketVersioning(c.Request.Context(), bucket, req.Enabled); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"status": "updated"})
}

// ListVersions returns all versions of an object
// @Summary List object versions
// @Description Gets a list of all versions of a specific object
// @Tags storage
// @Produce json
// @Security APIKeyAuth
// @Param bucket path string true "Bucket name"
// @Param key path string true "Object key"
// @Success 200 {array} domain.Object
// @Router /storage/versions/{bucket}/{key} [get]
func (h *StorageHandler) ListVersions(c *gin.Context) {
	bucket, key, ok := getBucketAndKeyRequired(c)
	if !ok {
		return
	}
	// Normalize key to basename for consistency with Upload
	key = path.Base(key)

	versions, err := h.svc.ListVersions(c.Request.Context(), bucket, key)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, versions)
}
