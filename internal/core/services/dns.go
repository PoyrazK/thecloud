package services

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

const (
	defaultTTL     = 300
	minTTL         = 60
	maxTTL         = 86400
	internalSuffix = ".internal"
)

var validZoneNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

// DNSService manages private DNS zones and records.
type DNSService struct {
	repo     ports.DNSRepository
	rbacSvc  ports.RBACService
	backend  ports.DNSBackend
	vpcRepo  ports.VpcRepository
	auditSvc ports.AuditService
	eventSvc ports.EventService
	logger   *slog.Logger
}

// DNSServiceParams holds dependencies for DNSService.
type DNSServiceParams struct {
	Repo     ports.DNSRepository
	RBAC     ports.RBACService
	Backend  ports.DNSBackend
	VpcRepo  ports.VpcRepository
	AuditSvc ports.AuditService
	EventSvc ports.EventService
	Logger   *slog.Logger
}

// NewDNSService constructs a DNSService with its dependencies.
func NewDNSService(params DNSServiceParams) *DNSService {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &DNSService{
		repo:     params.Repo,
		rbacSvc:  params.RBAC,
		backend:  params.Backend,
		vpcRepo:  params.VpcRepo,
		auditSvc: params.AuditSvc,
		eventSvc: params.EventSvc,
		logger:   logger,
	}
}

// CreateZone creates a new private DNS zone linked to a VPC.
func (s *DNSService) CreateZone(ctx context.Context, vpcID uuid.UUID, name, description string) (*domain.DNSZone, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDNSCreate, "*"); err != nil {
		return nil, err
	}

	// 1. Validate zone name
	if !validZoneNameRegex.MatchString(name) {
		return nil, errors.New(errors.InvalidInput, "invalid zone name format")
	}

	// 2. Verify VPC exists and belongs to user
	vpc, err := s.vpcRepo.GetByID(ctx, vpcID)
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "VPC not found", err)
	}

	// 3. Check if zone already exists for this VPC
	existing, _ := s.repo.GetZoneByVPC(ctx, vpcID)
	if existing != nil {
		return nil, errors.New(errors.Conflict, "zone already exists for this VPC")
	}

	// 4. Create zone in PowerDNS
	powerdnsZone := name + "."
	nameservers := []string{"ns1." + name + ".", "ns2." + name + "."}
	if err := s.backend.CreateZone(ctx, powerdnsZone, nameservers); err != nil {
		s.logger.Error("failed to create zone in backend", "zone", powerdnsZone, "error", err)
		return nil, errors.Wrap(errors.Internal, "failed to create zone in DNS backend", err)
	}

	// 5. Save to database
	zone := &domain.DNSZone{
		ID:          uuid.New(),
		UserID:      userID,
		TenantID:    tenantID,
		VpcID:       vpcID,
		Name:        name,
		Description: description,
		Status:      domain.ZoneStatusActive,
		DefaultTTL:  defaultTTL,
		PowerDNSID:  powerdnsZone,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateZone(ctx, zone); err != nil {
		// Rollback PowerDNS
		_ = s.backend.DeleteZone(ctx, powerdnsZone)
		return nil, errors.Wrap(errors.Internal, "failed to save zone", err)
	}

	// 6. Audit log
	if err := s.auditSvc.Log(ctx, userID, "dns.zone.create", "dns_zone", zone.ID.String(), map[string]interface{}{
		"name":   name,
		"vpc_id": vpcID.String(),
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "dns.zone.create", "zone_id", zone.ID, "error", err)
	}

	s.logger.Info("created DNS zone", "zone", name, "vpc", vpc.Name)
	return zone, nil
}

func (s *DNSService) GetZone(ctx context.Context, idOrName string) (*domain.DNSZone, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDNSRead, idOrName); err != nil {
		return nil, err
	}

	if id, err := uuid.Parse(idOrName); err == nil {
		return s.repo.GetZoneByID(ctx, id)
	}
	return s.repo.GetZoneByName(ctx, idOrName)
}

func (s *DNSService) GetZoneByVPC(ctx context.Context, vpcID uuid.UUID) (*domain.DNSZone, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDNSRead, vpcID.String()); err != nil {
		return nil, err
	}

	return s.repo.GetZoneByVPC(ctx, vpcID)
}

func (s *DNSService) ListZones(ctx context.Context) ([]*domain.DNSZone, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDNSRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListZones(ctx)
}

func (s *DNSService) DeleteZone(ctx context.Context, idOrName string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDNSDelete, idOrName); err != nil {
		return err
	}

	zone, err := s.GetZone(ctx, idOrName)
	if err != nil {
		return err
	}

	// 1. Delete from PowerDNS
	if err := s.backend.DeleteZone(ctx, zone.PowerDNSID); err != nil {
		// Log but force delete from DB? Or fail?
		// Usually we want to ensure backend is clean.
		s.logger.Error("failed to delete zone from backend", "zone", zone.PowerDNSID, "error", err)
		return errors.Wrap(errors.Internal, "failed to delete zone from backend", err)
	}

	// 2. Delete from DB
	if err := s.repo.DeleteZone(ctx, zone.ID); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, zone.UserID, "dns.zone.delete", "dns_zone", zone.ID.String(), map[string]interface{}{
		"name": zone.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "dns.zone.delete", "zone_id", zone.ID, "error", err)
	}

	return nil
}

// --- Record Operations ---

func (s *DNSService) CreateRecord(ctx context.Context, zoneID uuid.UUID, name string, recordType domain.RecordType, content string, ttl int, priority *int) (*domain.DNSRecord, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDNSCreate, "*"); err != nil {
		return nil, err
	}

	// 1. Validate inputs
	if !domain.IsValidRecordType(recordType) {
		s.logger.Warn("invalid record type provided", "type", string(recordType))
		return nil, errors.New(errors.InvalidInput, "invalid record type")
	}
	if ttl < minTTL {
		ttl = minTTL
	}
	if ttl > maxTTL {
		ttl = maxTTL
	}

	// 2. Get Zone
	zone, err := s.repo.GetZoneByID(ctx, zoneID)
	if err != nil {
		return nil, err
	}

	// 3. Format FQDN
	// If name is "@", use zone name.
	fqdn := name + "." + zone.Name
	if name == "@" {
		fqdn = zone.Name
	}
	if !strings.HasSuffix(fqdn, ".") {
		fqdn += "."
	}

	// 4. Update PowerDNS
	// Handle Priority formatting for MX/SRV if needed
	formattedContent := content
	if priority != nil {
		if recordType == domain.RecordTypeMX {
			formattedContent = fmt.Sprintf("%d %s", *priority, content)
		}
		// SRV usually takes priority weight port target
		// But here we only have priority and content.
		// If user passes full content for SRV, we use it.
		// If implementation plan assumed separate fields, we might need to adjust.
		// For now, let's assume content contains what's needed for other parts or just priority.
	}

	recordSet := ports.RecordSet{
		Name:    fqdn,
		Type:    string(recordType),
		TTL:     ttl,
		Records: []string{formattedContent},
	}

	if err := s.backend.AddRecords(ctx, zone.PowerDNSID, []ports.RecordSet{recordSet}); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create record in backend", err)
	}

	// 5. Save to DB
	record := &domain.DNSRecord{
		ID:          uuid.New(),
		ZoneID:      zoneID,
		TenantID:    tenantID,
		Name:        name,
		Type:        recordType,
		Content:     content,
		TTL:         ttl,
		Priority:    priority,
		Disabled:    false,
		AutoManaged: false,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateRecord(ctx, record); err != nil {
		// Rollback backend? Difficult with records.
		s.logger.Error("failed to create record in DB", "error", err)
		return nil, errors.Wrap(errors.Internal, "failed to save record", err)
	}

	return record, nil
}

func (s *DNSService) GetRecord(ctx context.Context, id uuid.UUID) (*domain.DNSRecord, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDNSRead, id.String()); err != nil {
		return nil, err
	}

	return s.repo.GetRecordByID(ctx, id)
}

func (s *DNSService) ListRecords(ctx context.Context, zoneID uuid.UUID) ([]*domain.DNSRecord, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDNSRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListRecordsByZone(ctx, zoneID)
}

func (s *DNSService) UpdateRecord(ctx context.Context, id uuid.UUID, content string, ttl int, priority *int) (*domain.DNSRecord, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDNSUpdate, id.String()); err != nil {
		return nil, err
	}

	record, err := s.repo.GetRecordByID(ctx, id)
	if err != nil {
		return nil, err
	}

	zone, err := s.repo.GetZoneByID(ctx, record.ZoneID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to find associated zone", err)
	}

	// Update fields
	record.Content = content
	if ttl > 0 {
		record.TTL = ttl
	}
	if priority != nil {
		record.Priority = priority
	}

	// FQDN
	fqdn := record.Name + "." + zone.Name
	if record.Name == "@" {
		fqdn = zone.Name
	}
	if !strings.HasSuffix(fqdn, ".") {
		fqdn += "."
	}

	formattedContent := content
	if record.Priority != nil {
		if record.Type == domain.RecordTypeMX {
			formattedContent = fmt.Sprintf("%d %s", *record.Priority, content)
		}
	}

	recordSet := ports.RecordSet{
		Name:    fqdn,
		Type:    string(record.Type),
		TTL:     record.TTL,
		Records: []string{formattedContent},
	}

	if err := s.backend.UpdateRecords(ctx, zone.PowerDNSID, []ports.RecordSet{recordSet}); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to update record in backend", err)
	}

	if err := s.repo.UpdateRecord(ctx, record); err != nil {
		return nil, err
	}

	return record, nil
}

func (s *DNSService) DeleteRecord(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDNSDelete, id.String()); err != nil {
		return err
	}

	record, err := s.repo.GetRecordByID(ctx, id)
	if err != nil {
		return err
	}

	zone, err := s.repo.GetZoneByID(ctx, record.ZoneID)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to find associated zone", err)
	}

	fqdn := record.Name + "." + zone.Name
	if record.Name == "@" {
		fqdn = zone.Name
	}
	if !strings.HasSuffix(fqdn, ".") {
		fqdn += "."
	}

	if err := s.backend.DeleteRecords(ctx, zone.PowerDNSID, fqdn, string(record.Type)); err != nil {
		return errors.Wrap(errors.Internal, "failed to delete record from backend", err)
	}

	return s.repo.DeleteRecord(ctx, id)
}

// RegisterInstance creates an A record for an instance in its VPC's private zone.
func (s *DNSService) RegisterInstance(ctx context.Context, instance *domain.Instance, ipAddress string) error {
	s.logger.Info("RegisterInstance called", "instance", instance.Name, "vpc_id", instance.VpcID, "ip", ipAddress)
	if instance.VpcID == nil {
		s.logger.Info("RegisterInstance skipped: no VPC ID")
		return nil // No VPC, no private DNS
	}

	// Find zone for VPC
	zone, err := s.repo.GetZoneByVPC(ctx, *instance.VpcID)
	if err != nil {
		s.logger.Warn("no private zone for VPC, skipping DNS registration", "vpc_id", instance.VpcID, "error", err)
		return nil // No zone configured, skip silently
	}

	fqdn := fmt.Sprintf("%s.%s.", instance.Name, zone.Name)

	// Add record to PowerDNS
	recordSet := ports.RecordSet{
		Name:    fqdn,
		Type:    "A",
		TTL:     zone.DefaultTTL,
		Records: []string{ipAddress},
	}

	if err := s.backend.AddRecords(ctx, zone.PowerDNSID, []ports.RecordSet{recordSet}); err != nil {
		return errors.Wrap(errors.Internal, "failed to register instance DNS in backend", err)
	}

	// Save to database
	record := &domain.DNSRecord{
		ID:          uuid.New(),
		ZoneID:      zone.ID,
		TenantID:    zone.TenantID,
		Name:        instance.Name,
		Type:        domain.RecordTypeA,
		Content:     ipAddress,
		TTL:         zone.DefaultTTL,
		AutoManaged: true,
		InstanceID:  &instance.ID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateRecord(ctx, record); err != nil {
		s.logger.Warn("failed to save DNS record to database", "error", err)
	}

	s.logger.Info("registered instance DNS", "instance", instance.Name, "fqdn", fqdn, "ip", ipAddress)
	return nil
}

// UnregisterInstance removes DNS records for an instance.
func (s *DNSService) UnregisterInstance(ctx context.Context, instanceID uuid.UUID) error {
	records, err := s.repo.GetRecordsByInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to get records for instance: %w", err)
	}
	if len(records) == 0 {
		return nil // No records to remove
	}

	for _, record := range records {
		zone, err := s.repo.GetZoneByID(ctx, record.ZoneID)
		if err != nil {
			continue
		}

		fqdn := fmt.Sprintf("%s.%s.", record.Name, zone.Name)
		_ = s.backend.DeleteRecords(ctx, zone.PowerDNSID, fqdn, string(record.Type))
	}

	return s.repo.DeleteRecordsByInstance(ctx, instanceID)
}
