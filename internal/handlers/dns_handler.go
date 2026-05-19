package httphandlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	errs "github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

// DNSHandler handles DNS operations via HTTP.
type DNSHandler struct {
	svc ports.DNSService
}

// NewDNSHandler creates a new DNSHandler.
func NewDNSHandler(svc ports.DNSService) *DNSHandler {
	return &DNSHandler{svc: svc}
}

// CreateZoneRequest defines the payload for creating a DNS zone.
type CreateZoneRequest struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	VpcID       uuid.UUID `json:"vpc_id" binding:"required"`
}

// CreateZone creates a new DNS zone.
// @Summary Create a new DNS zone
// @Tags dns
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param request body CreateZoneRequest true "Create Zone Request"
// @Success 201 {object} domain.DNSZone
// @Failure 400,401,500 {object} httputil.Response
// @Router /dns/zones [post]
func (h *DNSHandler) CreateZone(c *gin.Context) {
	var raw struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		VpcID       string `json:"vpc_id"`
	}

	if err := c.ShouldBindJSON(&raw); err != nil {
		httputil.Error(c, errs.New(errs.InvalidInput, fmt.Sprintf("invalid request body: %s", err.Error())))
		return
	}

	if raw.Name == "" {
		httputil.Error(c, errs.New(errs.InvalidInput, "field 'name' is required"))
		return
	}

	if raw.VpcID == "" {
		httputil.Error(c, errs.New(errs.InvalidInput, "field 'vpc_id' is required"))
		return
	}

	vpcID, err := uuid.Parse(raw.VpcID)
	if err != nil {
		httputil.Error(c, errs.New(errs.InvalidInput, "field 'vpc_id' must be a valid UUID"))
		return
	}

	zone, err := h.svc.CreateZone(c.Request.Context(), vpcID, raw.Name, raw.Description)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, zone)
}

// ListZones lists all DNS zones.
// @Summary List all DNS zones
// @Tags dns
// @Security APIKeyAuth
// @Produce json
// @Success 200 {array} domain.DNSZone
// @Failure 401,500 {object} httputil.Response
// @Router /dns/zones [get]
func (h *DNSHandler) ListZones(c *gin.Context) {
	zones, err := h.svc.ListZones(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, zones)
}

// GetZone retrieves a DNS zone.
// @Summary Get a DNS zone
// @Tags dns
// @Security APIKeyAuth
// @Produce json
// @Param id path string true "Zone ID or Name"
// @Success 200 {object} domain.DNSZone
// @Failure 404,500 {object} httputil.Response
// @Router /dns/zones/{id} [get]
func (h *DNSHandler) GetZone(c *gin.Context) {
	idOrName := c.Param("id")
	zone, err := h.svc.GetZone(c.Request.Context(), idOrName)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, zone)
}

// DeleteZone deletes a DNS zone.
// @Summary Delete a DNS zone
// @Tags dns
// @Security APIKeyAuth
// @Param id path string true "Zone ID or Name"
// @Success 204 "No Content"
// @Failure 404,500 {object} httputil.Response
// @Router /dns/zones/{id} [delete]
func (h *DNSHandler) DeleteZone(c *gin.Context) {
	idOrName := c.Param("id")
	if err := h.svc.DeleteZone(c.Request.Context(), idOrName); err != nil {
		httputil.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// CreateRecordRequest defines the payload for creating a DNS record.
type CreateRecordRequest struct {
	Name     string            `json:"name" binding:"required"`
	Type     domain.RecordType `json:"type" binding:"required"`
	Content  string            `json:"content" binding:"required"`
	TTL      int               `json:"ttl"`
	Priority *int              `json:"priority"`
}

// CreateRecord creates a new DNS record.
// @Summary Create a new DNS record
// @Tags dns
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Zone ID"
// @Param request body CreateRecordRequest true "Create Record Request"
// @Success 201 {object} domain.DNSRecord
// @Failure 400,401,500 {object} httputil.Response
// @Router /dns/zones/{id}/records [post]
func (h *DNSHandler) CreateRecord(c *gin.Context) {
	zoneID, ok := parseUUID(c)
	if !ok {
		return
	}

	var req CreateRecordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errs.New(errs.InvalidInput, errInvalidRequestBody))
		return
	}

	record, err := h.svc.CreateRecord(c.Request.Context(), *zoneID, req.Name, req.Type, req.Content, req.TTL, req.Priority)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, record)
}

// ListRecords lists records in a zone.
// @Summary List records in a zone
// @Tags dns
// @Security APIKeyAuth
// @Produce json
// @Param id path string true "Zone ID"
// @Success 200 {array} domain.DNSRecord
// @Failure 404,500 {object} httputil.Response
// @Router /dns/zones/{id}/records [get]
func (h *DNSHandler) ListRecords(c *gin.Context) {
	zoneID, ok := parseUUID(c)
	if !ok {
		return
	}

	records, err := h.svc.ListRecords(c.Request.Context(), *zoneID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, records)
}

// GetRecord retrieves a DNS record.
// @Summary Get a DNS record
// @Tags dns
// @Security APIKeyAuth
// @Produce json
// @Param id path string true "Record ID"
// @Success 200 {object} domain.DNSRecord
// @Failure 404,500 {object} httputil.Response
// @Router /dns/records/{id} [get]
func (h *DNSHandler) GetRecord(c *gin.Context) {
	id, ok := parseUUID(c)
	if !ok {
		return
	}

	record, err := h.svc.GetRecord(c.Request.Context(), *id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, record)
}

// UpdateRecordRequest defines the payload for updating a DNS record.
type UpdateRecordRequest struct {
	Content  string `json:"content" binding:"required"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority"`
}

// UpdateRecord updates a DNS record.
// @Summary Update a DNS record
// @Tags dns
// @Security APIKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Record ID"
// @Param request body UpdateRecordRequest true "Update Record Request"
// @Success 200 {object} domain.DNSRecord
// @Failure 400,404,500 {object} httputil.Response
// @Router /dns/records/{id} [put]
func (h *DNSHandler) UpdateRecord(c *gin.Context) {
	id, ok := parseUUID(c)
	if !ok {
		return
	}

	var req UpdateRecordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errs.New(errs.InvalidInput, errInvalidRequestBody))
		return
	}

	record, err := h.svc.UpdateRecord(c.Request.Context(), *id, req.Content, req.TTL, req.Priority)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, record)
}

// DeleteRecord deletes a DNS record.
// @Summary Delete a DNS record
// @Tags dns
// @Security APIKeyAuth
// @Param id path string true "Record ID"
// @Success 204 "No Content"
// @Failure 404,500 {object} httputil.Response
// @Router /dns/records/{id} [delete]
func (h *DNSHandler) DeleteRecord(c *gin.Context) {
	id, ok := parseUUID(c)
	if !ok {
		return
	}

	if err := h.svc.DeleteRecord(c.Request.Context(), *id); err != nil {
		httputil.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
