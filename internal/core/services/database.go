// Package services implements core business workflows.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/pkg/util"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerNameDatabase = "database-service"

const (
	// Default ports for database engines
	DefaultPostgresPort = "5432"
	DefaultMySQLPort    = "3306"

	// Connection Pooling (PgBouncer) defaults
	PoolerImage          = "edoburu/pgbouncer:latest"
	PoolerInternalPort   = "5432"
	DefaultPoolMode      = "transaction"
	DefaultMaxClientConn = "1000"
	DefaultPoolSize      = "20"

	// Exporter defaults
	PostgresExporterImage = "prometheuscommunity/postgres-exporter"
	PostgresExporterPort  = "9187"
	MySQLExporterImage    = "prom/mysqld-exporter"
	MySQLExporterPort     = "9104"
)

// DatabaseService manages database instances and lifecycle.
type DatabaseService struct {
	repo             ports.DatabaseRepository
	rbacSvc          ports.RBACService
	compute          ports.ComputeBackend
	vpcRepo          ports.VpcRepository
	volumeSvc        ports.VolumeService
	snapshotSvc      ports.SnapshotService
	snapshotRepo     ports.SnapshotRepository
	eventSvc         ports.EventService
	auditSvc         ports.AuditService
	secrets          ports.SecretsManager
	volumeEncryption ports.VolumeEncryptionService
	logger           *slog.Logger
	vaultMountPath   string
	// In-memory idempotency cache for rotation. Stores timestamp of last rotation attempt.
	// Expired entries are deleted on lookup to prevent unbounded growth, but this does
	// not guarantee all expired entries are reaped.
	rotationCache    map[string]time.Time
	rotationCacheTTL time.Duration
	rotationMu       sync.Mutex

	// In-flight rotation state for idempotency cache
	rotationInFlight map[string]*rotationInFlightEntry
}

// rotationInFlightEntry holds the state of an in-progress rotation
type rotationInFlightEntry struct {
	done chan struct{}
	err  error
}

// defaultRotationCacheTTL is the default TTL for rotation idempotency entries
const defaultRotationCacheTTL = 24 * time.Hour

// Ensure DatabaseService implements ports.DatabaseService
var _ ports.DatabaseService = (*DatabaseService)(nil)

// DatabaseServiceParams holds dependencies for DatabaseService creation.
type DatabaseServiceParams struct {
	Repo             ports.DatabaseRepository
	RBAC             ports.RBACService
	Compute          ports.ComputeBackend
	VpcRepo          ports.VpcRepository
	VolumeSvc        ports.VolumeService
	SnapshotSvc      ports.SnapshotService
	SnapshotRepo     ports.SnapshotRepository
	EventSvc         ports.EventService
	AuditSvc         ports.AuditService
	Secrets          ports.SecretsManager
	VolumeEncryption ports.VolumeEncryptionService
	Logger           *slog.Logger
	VaultMountPath   string
}

// NewDatabaseService constructs a DatabaseService with its dependencies.
func NewDatabaseService(params DatabaseServiceParams) *DatabaseService {
	return &DatabaseService{
		repo:             params.Repo,
		rbacSvc:          params.RBAC,
		compute:          params.Compute,
		vpcRepo:          params.VpcRepo,
		volumeSvc:        params.VolumeSvc,
		snapshotSvc:      params.SnapshotSvc,
		snapshotRepo:     params.SnapshotRepo,
		eventSvc:         params.EventSvc,
		auditSvc:         params.AuditSvc,
		secrets:          params.Secrets,
		volumeEncryption: params.VolumeEncryption,
		logger:           params.Logger,
		vaultMountPath:   params.VaultMountPath,
		rotationCache:    make(map[string]time.Time),
		rotationCacheTTL: defaultRotationCacheTTL,
		rotationInFlight: make(map[string]*rotationInFlightEntry),
	}
}

func (s *DatabaseService) CreateDatabase(ctx context.Context, req ports.CreateDatabaseRequest) (*domain.Database, error) {
	tracer := otel.Tracer(tracerNameDatabase)
	_, span := tracer.Start(ctx, "DatabaseService.CreateDatabase",
		trace.WithAttributes(
			attribute.String("db.name", req.Name),
			attribute.String("db.engine", req.Engine),
			attribute.String("db.version", req.Version),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBCreate, "*"); err != nil {
		span.RecordError(err)
		return nil, err
	}

	if s.compute.Type() == "libvirt" {
		return nil, errors.New(errors.InvalidInput, "managed databases require docker compute backend (COMPUTE_BACKEND=libvirt is not supported)")
	}

	dbEngine := domain.DatabaseEngine(req.Engine)

	if err := s.validateCreationRequest(req, dbEngine); err != nil {
		return nil, err
	}

	password, err := util.GenerateRandomPassword(16)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to generate password", err)
	}

	username := s.getDefaultUsername(dbEngine)
	db := s.initialDatabaseRecord(userID, req.Name, dbEngine, req.Version, username, password, req.VpcID)
	db.TenantID = tenantID
	db.Role = domain.RolePrimary
	db.AllocatedStorage = req.AllocatedStorage
	db.Parameters = req.Parameters
	db.MetricsEnabled = req.MetricsEnabled
	db.PoolingEnabled = req.PoolingEnabled
	if req.KmsKeyID != "" {
		db.KmsKeyID = req.KmsKeyID
		db.EncryptedVolume = true
	}

	return s.provisionDatabase(ctx, db, password, req.Parameters, "", "DATABASE_CREATE")
}

func (s *DatabaseService) CreateReplica(ctx context.Context, primaryID uuid.UUID, name string) (*domain.Database, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBCreate, "*"); err != nil {
		return nil, err
	}

	if s.compute.Type() == "libvirt" {
		return nil, errors.New(errors.InvalidInput, "database replicas require docker compute backend (COMPUTE_BACKEND=libvirt is not supported)")
	}

	primary, err := s.repo.GetByID(ctx, primaryID)
	if err != nil {
		return nil, err
	}

	// Ensure the primary database belongs to the same tenant without leaking cross-tenant existence
	if primary.TenantID != tenantID {
		return nil, errors.New(errors.NotFound, "database not found")
	}

	primaryIP, err := s.compute.GetInstanceIP(ctx, primary.ContainerID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to get primary IP", err)
	}

	password := primary.Password
	if primary.CredentialPath != "" {
		secret, err := s.secrets.GetSecret(ctx, primary.CredentialPath)
		if err == nil && secret != nil {
			if p, ok := secret["password"].(string); ok {
				password = p
			}
		}
	}

	db := s.initialDatabaseRecord(userID, name, primary.Engine, primary.Version, primary.Username, password, primary.VpcID)
	db.TenantID = tenantID
	db.Role = domain.RoleReplica
	db.PrimaryID = &primaryID
	db.AllocatedStorage = primary.AllocatedStorage
	db.MetricsEnabled = primary.MetricsEnabled
	db.PoolingEnabled = primary.PoolingEnabled

	return s.provisionDatabase(ctx, db, password, primary.Parameters, primaryIP, "DATABASE_REPLICA_CREATE")
}

func (s *DatabaseService) RestoreDatabase(ctx context.Context, req ports.RestoreDatabaseRequest) (*domain.Database, error) {
	tracer := otel.Tracer(tracerNameDatabase)
	_, span := tracer.Start(ctx, "DatabaseService.RestoreDatabase",
		trace.WithAttributes(
			attribute.String("db.new_name", req.NewName),
			attribute.String("db.engine", req.Engine),
			attribute.String("db.snapshot_id", req.SnapshotID.String()),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBCreate, "*"); err != nil {
		span.RecordError(err)
		return nil, err
	}

	if s.compute.Type() == "libvirt" {
		return nil, errors.New(errors.InvalidInput, "database restore requires docker compute backend (COMPUTE_BACKEND=libvirt is not supported)")
	}

	snap, err := s.snapshotSvc.GetSnapshot(ctx, req.SnapshotID)
	if err != nil {
		return nil, err
	}
	dbEngine := domain.DatabaseEngine(req.Engine)
	password, err := util.GenerateRandomPassword(16)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to generate password for restore", err)
	}
	username := s.getDefaultUsername(dbEngine)
	db := s.initialDatabaseRecord(userID, req.NewName, dbEngine, req.Version, username, password, req.VpcID)
	db.TenantID = tenantID

	db.AllocatedStorage = req.AllocatedStorage
	if snap.SizeGB > db.AllocatedStorage {
		db.AllocatedStorage = snap.SizeGB
	}
	db.MetricsEnabled = req.MetricsEnabled
	db.PoolingEnabled = req.PoolingEnabled

	// Note: We store credentials in Vault BEFORE restoring the snapshot to ensure
	// secret availability during potential multi-step provisioning. If restore fails,
	// we explicitly clean up the secret.
	vaultPath := s.getVaultPath(db.ID)
	if err := s.secrets.StoreSecret(ctx, vaultPath, map[string]interface{}{"password": password}); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to store restored database credentials in vault", err)
	}
	db.CredentialPath = vaultPath

	vol, err := s.snapshotSvc.RestoreSnapshot(ctx, req.SnapshotID, fmt.Sprintf("db-vol-%s", db.ID.String()[:8]))
	if err != nil {
		if delErr := s.secrets.DeleteSecret(ctx, db.CredentialPath); delErr != nil {
			s.logger.Warn("failed to cleanup vault secret after snapshot restore failure", "path", db.CredentialPath, "error", delErr)
		}
		return nil, err
	}

	return s.finalizeProvisioning(ctx, db, vol, password, req.Parameters, "", "DATABASE_RESTORE")
}

func (s *DatabaseService) provisionDatabase(ctx context.Context, db *domain.Database, password string, parameters map[string]string, primaryIP string, action string) (*domain.Database, error) {
	vaultPath := s.getVaultPath(db.ID)
	if err := s.secrets.StoreSecret(ctx, vaultPath, map[string]interface{}{"password": password}); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to store database credentials in vault", err)
	}
	db.CredentialPath = vaultPath

	volumeName := fmt.Sprintf("db-vol-%s", db.ID.String()[:8])
	if db.Role == domain.RoleReplica {
		volumeName = fmt.Sprintf("db-replica-vol-%s", db.ID.String()[:8])
	}
	vol, err := s.volumeSvc.CreateVolume(ctx, volumeName, db.AllocatedStorage)
	if err != nil {
		if delErr := s.secrets.DeleteSecret(ctx, db.CredentialPath); delErr != nil {
			s.logger.Warn("failed to cleanup vault secret after volume creation failure", "path", db.CredentialPath, "error", delErr)
		}
		return nil, errors.Wrap(errors.Internal, "failed to create persistent volume", err)
	}

	// Create encryption key for volume if encryption is enabled
	if db.EncryptedVolume && db.KmsKeyID != "" {
		if s.volumeEncryption == nil {
			// Rollback: delete the created volume and secret
			if delErr := s.volumeSvc.DeleteVolume(ctx, vol.ID.String()); delErr != nil {
				s.logger.Warn("failed to rollback volume after encryption setup failure", "volume_id", vol.ID, "error", delErr)
			}
			if delErr := s.secrets.DeleteSecret(ctx, db.CredentialPath); delErr != nil {
				s.logger.Warn("failed to cleanup vault secret after volume creation failure", "path", db.CredentialPath, "error", delErr)
			}
			return nil, errors.New(errors.Internal, "volume encryption service not configured")
		}
		if err := s.volumeEncryption.CreateVolumeKey(ctx, vol.ID, db.KmsKeyID); err != nil {
			// Rollback: delete the created volume and secret
			if delErr := s.volumeSvc.DeleteVolume(ctx, vol.ID.String()); delErr != nil {
				s.logger.Warn("failed to rollback volume after encryption key creation failure", "volume_id", vol.ID, "error", delErr)
			}
			if delErr := s.secrets.DeleteSecret(ctx, db.CredentialPath); delErr != nil {
				s.logger.Warn("failed to cleanup vault secret after encryption key creation failure", "path", db.CredentialPath, "error", delErr)
			}
			return nil, errors.Wrap(errors.Internal, "failed to create volume encryption key", err)
		}
		db.VolumeKeyRef = fmt.Sprintf("vol-key-%s", vol.ID.String()[:8])
	}

	return s.finalizeProvisioning(ctx, db, vol, password, parameters, primaryIP, action)
}

func (s *DatabaseService) finalizeProvisioning(ctx context.Context, db *domain.Database, vol *domain.Volume, password string, parameters map[string]string, primaryIP string, action string) (*domain.Database, error) {
	networkID, err := s.resolveVpcNetwork(ctx, db.VpcID)
	if err != nil {
		return s.performProvisioningRollback(ctx, db, vol.ID.String(), err)
	}

	imageName, env, defaultPort := s.getEngineConfig(db.Engine, db.Version, db.Username, password, db.Name, db.Role, primaryIP)

	containerID, allocatedPorts, err := s.compute.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{
		Name:        fmt.Sprintf("cloud-db-%s-%s", db.Name, db.ID.String()[:8]),
		ImageName:   imageName,
		Ports:       []string{"0:" + defaultPort},
		NetworkID:   networkID,
		VolumeBinds: []string{fmt.Sprintf("%s:%s", s.getBackendVolName(vol), s.getMountPath(db.Engine))},
		Env:         env,
		Cmd:         s.buildEngineCmd(db.Engine, parameters),
	})

	if err != nil {
		return s.performProvisioningRollback(ctx, db, vol.ID.String(), errors.Wrap(errors.Internal, "failed to launch database container", err))
	}

	db.ContainerID = containerID
	if err := s.resolveDatabasePort(ctx, db, allocatedPorts, defaultPort); err != nil {
		return s.performProvisioningRollback(ctx, db, vol.ID.String(), errors.Wrap(errors.Internal, "failed to resolve database port", err))
	}
	db.Status = domain.DatabaseStatusRunning

	if db.MetricsEnabled || db.PoolingEnabled {
		dbIP, err := s.compute.GetInstanceIP(ctx, containerID)
		if err != nil {
			return s.performProvisioningRollback(ctx, db, vol.ID.String(), errors.Wrap(errors.Internal, "failed to get database IP", err))
		}

		if err := s.provisionSidecars(ctx, db, db.Engine, dbIP, db.Username, password, networkID); err != nil {
			return s.performProvisioningRollback(ctx, db, vol.ID.String(), err)
		}
	}

	if err := s.repo.Create(ctx, db); err != nil {
		return s.performProvisioningRollback(ctx, db, vol.ID.String(), err)
	}

	s.recordDatabaseCreation(ctx, db.UserID, db, action)
	return db, nil
}

func (s *DatabaseService) ModifyDatabase(ctx context.Context, req ports.ModifyDatabaseRequest) (*domain.Database, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBUpdate, req.ID.String()); err != nil {
		return nil, err
	}
	db, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	if req.Parameters != nil {
		db.Parameters = req.Parameters
	}

	if s.compute.Type() == "libvirt" && (req.MetricsEnabled != nil && *req.MetricsEnabled) {
		return nil, errors.New(errors.InvalidInput, "database metrics require docker compute backend (COMPUTE_BACKEND=libvirt is not supported)")
	}

	if req.AllocatedStorage != nil {
		if *req.AllocatedStorage <= 0 {
			return nil, errors.New(errors.InvalidInput, "allocated storage must be a positive integer (GB)")
		}
		if *req.AllocatedStorage < db.AllocatedStorage {
			return nil, errors.New(errors.InvalidInput, "cannot decrease allocated storage")
		}
		vol, err := s.getVolumeForDatabase(ctx, db)
		if err != nil {
			return nil, err
		}
		if err := s.volumeSvc.ResizeVolume(ctx, vol.ID.String(), *req.AllocatedStorage); err != nil {
			return nil, err
		}
		db.AllocatedStorage = *req.AllocatedStorage
	}

	networkID, _ := s.resolveVpcNetwork(ctx, db.VpcID)
	dbIP, _ := s.compute.GetInstanceIP(ctx, db.ContainerID)

	// Fetch current password from Vault for sidecar provisioning if needed
	password := db.Password
	if db.CredentialPath != "" {
		secret, err := s.secrets.GetSecret(ctx, db.CredentialPath)
		if err == nil && secret != nil {
			if p, ok := secret["password"].(string); ok {
				password = p
			}
		}
	}

	if req.MetricsEnabled != nil && *req.MetricsEnabled != db.MetricsEnabled {
		if *req.MetricsEnabled {
			if err := s.provisionMetricsSidecar(ctx, db, db.Engine, dbIP, db.Username, password, networkID); err != nil {
				return nil, err
			}
		} else if db.ExporterContainerID != "" {
			if err := s.compute.DeleteInstance(ctx, db.ExporterContainerID); err != nil {
				s.logger.Warn("failed to delete metrics sidecar during modification", "container_id", db.ExporterContainerID, "error", err)
			}
			db.ExporterContainerID = ""
			db.MetricsPort = 0
		}
		db.MetricsEnabled = *req.MetricsEnabled
	}

	if req.PoolingEnabled != nil && *req.PoolingEnabled != db.PoolingEnabled {
		if *req.PoolingEnabled {
			if db.Engine != domain.EnginePostgres {
				return nil, errors.New(errors.InvalidInput, "connection pooling is currently only supported for PostgreSQL")
			}
			if err := s.provisionPoolerSidecar(ctx, db, db.Engine, dbIP, db.Username, password, networkID); err != nil {
				return nil, err
			}
		} else if db.PoolerContainerID != "" {
			if err := s.compute.DeleteInstance(ctx, db.PoolerContainerID); err != nil {
				s.logger.Warn("failed to delete pooler sidecar during modification", "container_id", db.PoolerContainerID, "error", err)
			}
			db.PoolerContainerID = ""
			db.PoolingPort = 0
		}
		db.PoolingEnabled = *req.PoolingEnabled
	}

	if err := s.repo.Update(ctx, db); err != nil {
		return nil, err
	}

	_ = s.eventSvc.RecordEvent(ctx, "DATABASE_MODIFY", db.ID.String(), "DATABASE", nil)
	_ = s.auditSvc.Log(ctx, db.UserID, "database.modify", "database", db.ID.String(), map[string]interface{}{"name": db.Name})

	return db, nil
}

func (s *DatabaseService) GetDatabase(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBRead, id.String()); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *DatabaseService) ListDatabases(ctx context.Context) ([]*domain.Database, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBRead, "*"); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *DatabaseService) DeleteDatabase(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBDelete, id.String()); err != nil {
		return err
	}
	db, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if db.ContainerID != "" {
		if err := s.compute.DeleteInstance(ctx, db.ContainerID); err != nil {
			s.logger.Warn("failed to delete database container", "container_id", db.ContainerID, "error", err)
		}
	}
	if db.ExporterContainerID != "" {
		if err := s.compute.DeleteInstance(ctx, db.ExporterContainerID); err != nil {
			s.logger.Warn("failed to delete exporter container", "container_id", db.ExporterContainerID, "error", err)
		}
	}
	if db.PoolerContainerID != "" {
		if err := s.compute.DeleteInstance(ctx, db.PoolerContainerID); err != nil {
			s.logger.Warn("failed to delete pooler container", "container_id", db.PoolerContainerID, "error", err)
		}
	}

	// Delete from Vault
	if db.CredentialPath != "" {
		if err := s.secrets.DeleteSecret(ctx, db.CredentialPath); err != nil {
			s.logger.Warn("failed to delete database credentials from vault", "path", db.CredentialPath, "error", err)
		}
	}

	var volID uuid.UUID
	vols, err := s.volumeSvc.ListVolumes(ctx)
	if err == nil {
		expectedPrefix := fmt.Sprintf("db-vol-%s", db.ID.String()[:8])
		if db.Role == domain.RoleReplica {
			expectedPrefix = fmt.Sprintf("db-replica-vol-%s", db.ID.String()[:8])
		}
		for _, v := range vols {
			if strings.HasPrefix(v.Name, expectedPrefix) {
				if err := s.volumeSvc.DeleteVolume(ctx, v.ID.String()); err != nil {
					s.logger.Warn("failed to delete volume", "volume_id", v.ID, "error", err)
				} else {
					volID = v.ID
				}
				break
			}
		}
	}

	// Delete encryption key for volume if encryption was enabled
	if db.EncryptedVolume && volID != uuid.Nil {
		if err := s.volumeEncryption.DeleteVolumeKey(ctx, volID); err != nil {
			s.logger.Warn("failed to delete volume encryption key", "volume_id", volID, "error", err)
		}
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "DATABASE_DELETE", id.String(), "DATABASE", nil)
	_ = s.auditSvc.Log(ctx, db.UserID, "database.delete", "database", db.ID.String(), map[string]interface{}{"name": db.Name})
	platform.RDSInstancesTotal.WithLabelValues(string(db.Engine), "running").Dec()

	return nil
}

func (s *DatabaseService) PromoteToPrimary(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBUpdate, id.String()); err != nil {
		return err
	}

	if s.compute.Type() == "libvirt" {
		return errors.New(errors.InvalidInput, "database promotion requires docker compute backend (COMPUTE_BACKEND=libvirt is not supported)")
	}

	db, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if db.Role == domain.RolePrimary {
		return errors.New(errors.InvalidInput, "database is already a primary")
	}

	// Execute database engine promotion command in container
	var cmd []string
	switch db.Engine {
	case domain.EnginePostgres:
		// Create promotion trigger file - PostgreSQL monitors this file
		// and exits recovery mode when it appears
		cmd = []string{"touch", "/var/lib/postgresql/data/promote"}
	case domain.EngineMySQL:
		// Fetch current password from Vault (db.Password may be stale after rotation)
		password := db.Password
		if db.CredentialPath != "" {
			secret, err := s.secrets.GetSecret(ctx, db.CredentialPath)
			if err == nil && secret != nil {
				if p, ok := secret["password"].(string); ok {
					password = p
				}
			}
		}
		// Use MYSQL_PWD env var to avoid password appearing in process list
		cmd = []string{"sh", "-c", fmt.Sprintf("MYSQL_PWD='%s' mysql -u root --execute='STOP REPLICA; RESET REPLICA ALL;'", sqlStringLiteral(password))}
	default:
		return errors.New(errors.Internal, "unsupported engine for promotion")
	}

	if _, err := s.compute.Exec(ctx, db.ContainerID, cmd); err != nil {
		return errors.Wrap(errors.Internal, "failed to promote database engine", err)
	}

	db.Role = domain.RolePrimary
	db.PrimaryID = nil
	if err := s.repo.Update(ctx, db); err != nil {
		return err
	}
	_ = s.eventSvc.RecordEvent(ctx, "DATABASE_PROMOTED", db.ID.String(), "DATABASE", nil)
	return nil
}

func (s *DatabaseService) GetConnectionString(ctx context.Context, id uuid.UUID) (string, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBRead, id.String()); err != nil {
		return "", err
	}
	db, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	password := db.Password
	if db.CredentialPath != "" {
		secret, err := s.secrets.GetSecret(ctx, db.CredentialPath)
		if err != nil {
			s.logger.Warn("failed to fetch database password from vault, using fallback", "path", db.CredentialPath, "error", err)
		} else if secret != nil {
			if p, ok := secret["password"].(string); ok {
				password = p
			}
		}
	}

	port := db.Port
	if db.PoolingEnabled && db.PoolingPort != 0 {
		port = db.PoolingPort
	}
	switch db.Engine {
	case domain.EnginePostgres:
		return fmt.Sprintf("postgres://%s:%s@127.0.0.1:%d/%s", db.Username, password, port, db.Name), nil
	case domain.EngineMySQL:
		return fmt.Sprintf("%s:%s@tcp(127.0.0.1:%d)/%s", db.Username, password, port, db.Name), nil
	default:
		return "", errors.New(errors.Internal, "unknown engine")
	}
}

func (s *DatabaseService) CreateDatabaseSnapshot(ctx context.Context, databaseID uuid.UUID, description string) (*domain.Snapshot, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSnapshotCreate, "*"); err != nil {
		return nil, err
	}
	db, err := s.repo.GetByID(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	vol, err := s.getVolumeForDatabase(ctx, db)
	if err != nil {
		return nil, err
	}
	snapshotName := fmt.Sprintf("db-snap-%s-%s", db.Name, time.Now().Format("20060102150405"))
	snap, err := s.snapshotSvc.CreateSnapshot(ctx, vol.ID, snapshotName)
	if err != nil {
		return nil, err
	}
	return snap, nil
}

func (s *DatabaseService) ListDatabaseSnapshots(ctx context.Context, databaseID uuid.UUID) ([]*domain.Snapshot, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionSnapshotRead, "*"); err != nil {
		return nil, err
	}
	db, err := s.repo.GetByID(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	vol, err := s.getVolumeForDatabase(ctx, db)
	if err != nil {
		return nil, err
	}
	return s.snapshotRepo.ListByVolumeID(ctx, vol.ID)
}

func (s *DatabaseService) StopDatabase(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBUpdate, id.String()); err != nil {
		return err
	}

	db, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if db.Status != domain.DatabaseStatusRunning {
		return errors.New(errors.InvalidInput, "database is not running")
	}
	if db.Role == domain.RoleReplica {
		return errors.New(errors.InvalidInput, "cannot stop a replica database")
	}

	// Stop sidecars first
	if db.ExporterContainerID != "" {
		if err := s.compute.StopInstance(ctx, db.ExporterContainerID); err != nil {
			s.logger.Warn("failed to stop exporter container", "container_id", db.ExporterContainerID, "error", err)
		}
	}
	if db.PoolerContainerID != "" {
		if err := s.compute.StopInstance(ctx, db.PoolerContainerID); err != nil {
			s.logger.Warn("failed to stop pooler container", "container_id", db.PoolerContainerID, "error", err)
		}
	}

	// Stop database container; must succeed before marking as stopped.
	if db.ContainerID != "" {
		if err := s.compute.StopInstance(ctx, db.ContainerID); err != nil {
			return errors.Wrap(errors.Internal, "failed to stop database container", err)
		}
	}

	db.Status = domain.DatabaseStatusStopped
	if err := s.repo.Update(ctx, db); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "DATABASE_STOP", db.ID.String(), "DATABASE", nil)
	_ = s.auditSvc.Log(ctx, db.UserID, "database.stop", "database", db.ID.String(), map[string]interface{}{"name": db.Name})
	platform.RDSInstancesTotal.WithLabelValues(string(db.Engine), "stopped").Inc()
	platform.RDSInstancesTotal.WithLabelValues(string(db.Engine), "running").Dec()

	return nil
}

func (s *DatabaseService) StartDatabase(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBUpdate, id.String()); err != nil {
		return err
	}

	db, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if db.Status != domain.DatabaseStatusStopped {
		return errors.New(errors.InvalidInput, "database is not stopped")
	}
	if db.ContainerID == "" {
		return errors.New(errors.Internal, "database container ID is missing")
	}

	// Start database container
	if err := s.compute.StartInstance(ctx, db.ContainerID); err != nil {
		return errors.Wrap(errors.Internal, "failed to start database container", err)
	}

	// Wait for database to be ready; return error if it fails.
	if err := s.waitForDatabaseReady(ctx, db); err != nil {
		return errors.Wrap(errors.Internal, "database failed to become ready", err)
	}

	// Start sidecars if enabled
	if db.ExporterContainerID != "" {
		if err := s.compute.StartInstance(ctx, db.ExporterContainerID); err != nil {
			s.logger.Warn("failed to start exporter container", "container_id", db.ExporterContainerID, "error", err)
		}
	}
	if db.PoolingEnabled && db.PoolerContainerID != "" {
		if err := s.compute.StartInstance(ctx, db.PoolerContainerID); err != nil {
			s.logger.Warn("failed to start pooler container", "container_id", db.PoolerContainerID, "error", err)
		}
	}

	db.Status = domain.DatabaseStatusRunning
	if err := s.repo.Update(ctx, db); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "DATABASE_START", db.ID.String(), "DATABASE", nil)
	_ = s.auditSvc.Log(ctx, db.UserID, "database.start", "database", db.ID.String(), map[string]interface{}{"name": db.Name})
	platform.RDSInstancesTotal.WithLabelValues(string(db.Engine), "running").Inc()
	platform.RDSInstancesTotal.WithLabelValues(string(db.Engine), "stopped").Dec()

	return nil
}

func (s *DatabaseService) ResizeDatabase(ctx context.Context, id uuid.UUID, newSizeGB int) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionDBUpdate, id.String()); err != nil {
		return err
	}

	db, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if newSizeGB <= db.AllocatedStorage {
		return errors.New(errors.InvalidInput, "new size must be larger than current allocated storage")
	}

	vol, err := s.getVolumeForDatabase(ctx, db)
	if err != nil {
		return err
	}

	if err := s.volumeSvc.ResizeVolume(ctx, vol.ID.String(), newSizeGB); err != nil {
		return err
	}

	db.AllocatedStorage = newSizeGB
	db.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, db); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, db.UserID, "database.resize", "database", db.ID.String(), map[string]interface{}{
		"name":        db.Name,
		"old_size_gb": vol.SizeGB,
		"new_size_gb": newSizeGB,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "database_id", db.ID, "error", err)
	}

	return nil
}

func (s *DatabaseService) waitForDatabaseReady(ctx context.Context, db *domain.Database) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		dbIP, err := s.compute.GetInstanceIP(ctx, db.ContainerID)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		if dbIP != "" {
			return nil
		}
	}
	return errors.New(errors.Internal, "database did not become ready in time")
}

func (s *DatabaseService) RotateCredentials(ctx context.Context, id uuid.UUID, idempotencyKey string) error {
	// Handle idempotency with in-flight support
	if idempotencyKey != "" {
		s.rotationMu.Lock()
		// Check for existing completed entry
		if cachedAt, ok := s.rotationCache[idempotencyKey]; ok {
			if time.Since(cachedAt) < s.rotationCacheTTL {
				s.rotationMu.Unlock()
				return nil // Already rotated
			}
			// Expired entry - remove it
			delete(s.rotationCache, idempotencyKey)
		}
		// Check for in-flight rotation
		if entry, exists := s.rotationInFlight[idempotencyKey]; exists {
			s.rotationMu.Unlock()
			// Wait for the in-flight rotation to complete
			<-entry.done
			if entry.err != nil {
				return entry.err
			}
			return nil // Rotation completed successfully
		}
		// Create in-flight entry
		entry := &rotationInFlightEntry{done: make(chan struct{})}
		s.rotationInFlight[idempotencyKey] = entry
		s.rotationMu.Unlock()

		// Perform rotation, then update in-flight state
		rotateErr := s.doRotateCredentials(ctx, id, idempotencyKey)

		s.rotationMu.Lock()
		if rotateErr == nil {
			s.rotationCache[idempotencyKey] = time.Now()
		}
		entry.err = rotateErr
		close(entry.done)
		delete(s.rotationInFlight, idempotencyKey)
		s.rotationMu.Unlock()

		return rotateErr
	}

	return s.doRotateCredentials(ctx, id, idempotencyKey)
}

// doRotateCredentials performs the actual credential rotation
func (s *DatabaseService) doRotateCredentials(ctx context.Context, id uuid.UUID, _ string) error {
	tracer := otel.Tracer(tracerNameDatabase)
	_, span := tracer.Start(ctx, "DatabaseService.doRotateCredentials",
		trace.WithAttributes(
			attribute.String("db.id", id.String()),
		))
	defer span.End()

	db, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if s.compute.Type() == "libvirt" {
		return errors.New(errors.InvalidInput, "credential rotation requires docker compute backend (COMPUTE_BACKEND=libvirt is not supported)")
	}

	newPassword, err := util.GenerateRandomPassword(16)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to generate new password", err)
	}

	// Get current password for DB auth
	currentPassword := db.Password
	if db.CredentialPath != "" {
		secret, err := s.secrets.GetSecret(ctx, db.CredentialPath)
		if err == nil && secret != nil {
			if p, ok := secret["password"].(string); ok {
				currentPassword = p
			}
		}
	}

	// 1. Store new password in Vault at versioned path FIRST (before DB change)
	vaultPath := db.CredentialPath
	if vaultPath == "" {
		vaultPath = s.getVaultPath(db.ID)
	}
	// Increment version and compute new versioned path
	newVersion := db.CredentialVersion + 1
	versionedPath := vaultPath + "/v" + strconv.Itoa(newVersion)
	if err := s.secrets.StoreSecret(ctx, versionedPath, map[string]interface{}{"password": newPassword}); err != nil {
		return errors.Wrap(errors.Internal, "failed to store new credential in vault", err)
	}

	// 2. Execute ALTER USER in container using currentPassword for auth
	cmd := s.buildPasswordChangeCmd(db.Engine, db.Username, currentPassword, newPassword)
	if cmd == nil {
		return errors.New(errors.Internal, "unsupported engine for credential rotation")
	}

	var execErr error
	for i := 0; i < 10; i++ {
		if _, execErr = s.compute.Exec(ctx, db.ContainerID, cmd); execErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if execErr != nil {
		// Roll back the orphaned vault entry. Log rollback failures separately
		// so the operator can clean up by hand instead of conflating two unrelated errors.
		if rbErr := s.secrets.DeleteSecret(ctx, versionedPath); rbErr != nil {
			s.logger.Error("orphaned vault entry left after failed credential rotation",
				"db_id", db.ID, "vault_path", versionedPath, "rollback_error", rbErr)
			platform.CredentialCleanupFailures.Inc()
		}
		return errors.Wrap(errors.Internal, "failed to execute password rotation in container", execErr)
	}

	// 3. Update DB record to point to new versioned path (ONLY after DB confirmed updated)
	// The old path still has the old password for rollback if needed
	db.CredentialPath = versionedPath
	db.CredentialVersion = newVersion
	if err := s.repo.Update(ctx, db); err != nil {
		// DB password has rotated but our record is out of date. The new password is
		// still in vault at versionedPath, so the operator can recover; log clearly.
		s.logger.Error("password rotated on engine but failed to persist credential path; recovery requires manual update",
			"db_id", db.ID, "vault_path", versionedPath, "version", newVersion, "error", err)
		return errors.Wrap(errors.Internal, "failed to update DB credential path", err)
	}

	// 4. Cleanup old versioned path from Vault
	oldVersionedPath := vaultPath + "/v" + strconv.Itoa(db.CredentialVersion-1)
	if err := s.secrets.DeleteSecret(ctx, oldVersionedPath); err != nil {
		// Log but don't fail - old version may not exist if this is first rotation
		s.logger.Warn("failed to cleanup old credential version from vault", "path", oldVersionedPath, "error", err)
		platform.CredentialCleanupFailures.Inc()
	}

	// 4. If pooler is enabled, restart it to pick up new credentials
	if db.PoolerContainerID != "" {
		if err := s.compute.DeleteInstance(ctx, db.PoolerContainerID); err != nil {
			s.logger.Warn("failed to delete old pooler during rotation", "pooler_id", db.PoolerContainerID, "error", err)
		}
		db.PoolerContainerID = ""
		dbIP, err := s.compute.GetInstanceIP(ctx, db.ContainerID)
		if err != nil {
			return errors.Wrap(errors.Internal, "failed to get database IP for pooler restart", err)
		}
		networkID, err := s.resolveVpcNetwork(ctx, db.VpcID)
		if err != nil {
			return errors.Wrap(errors.Internal, "failed to resolve network for pooler restart", err)
		}
		if err := s.provisionPoolerSidecar(ctx, db, db.Engine, dbIP, db.Username, newPassword, networkID); err != nil {
			return errors.Wrap(errors.Internal, "failed to provision new pooler sidecar during rotation", err)
		}
		if err := s.repo.Update(ctx, db); err != nil {
			return err
		}
	}

	_ = s.eventSvc.RecordEvent(ctx, "DATABASE_CREDENTIALS_ROTATE", db.ID.String(), "DATABASE", nil)
	_ = s.auditSvc.Log(ctx, db.UserID, "database.rotate_credentials", "database", db.ID.String(), nil)

	return nil
}

// Internal helper methods

func (s *DatabaseService) validateCreationRequest(req ports.CreateDatabaseRequest, engine domain.DatabaseEngine) error {
	if !s.isValidEngine(engine) {
		return errors.New(errors.InvalidInput, "unsupported database engine")
	}
	if req.AllocatedStorage <= 0 {
		return errors.New(errors.InvalidInput, "allocated storage must be a positive integer (GB)")
	}
	if req.AllocatedStorage < 10 {
		return errors.New(errors.InvalidInput, "allocated storage must be at least 10GB")
	}
	if req.PoolingEnabled && engine != domain.EnginePostgres {
		return errors.New(errors.InvalidInput, "connection pooling is currently only supported for PostgreSQL")
	}
	return nil
}

func (s *DatabaseService) getVaultPath(dbID uuid.UUID) string {
	return fmt.Sprintf("%s/%s/credentials", s.vaultMountPath, dbID.String())
}

// sqlStringLiteral escapes a string for use in SQL string literals
func sqlStringLiteral(s string) string {
	// Escape single quotes for SQL
	return strings.ReplaceAll(s, "'", "''")
}

// postgresIdentifier escapes a PostgreSQL identifier
func postgresIdentifier(id string) string {
	// PostgreSQL uses double quotes for identifiers
	return "\"" + strings.ReplaceAll(id, "\"", "\"\"") + "\""
}

func (s *DatabaseService) buildPasswordChangeCmd(engine domain.DatabaseEngine, username, authPassword, targetPassword string) []string {
	switch engine {
	case domain.EnginePostgres:
		// Use PGPASSWORD env var for authentication (libpq standard).
		// Note: While env vars avoid cmdline exposure, the password is still visible to
		// root users via /proc/$pid/environ. This is a known tradeoff - the alternative
		// (stdin password) requires more complex interaction patterns with psql.
		// authPassword (current password) authenticates the connection; targetPassword is the new password being set.
		stmt := fmt.Sprintf("ALTER USER %s WITH PASSWORD '%s';", postgresIdentifier(username), sqlStringLiteral(targetPassword))
		return []string{"sh", "-c", "PGPASSWORD='" + sqlStringLiteral(authPassword) + "' psql -h 127.0.0.1 -U " + username + " -d postgres -c '" + stmt + "'"}
	case domain.EngineMySQL:
		// Use mysql with password via MYSQL_PWD env var to avoid cmdline exposure.
		// Same tradeoff as above - password visible in /proc environment to root users.
		stmt := fmt.Sprintf("ALTER USER '%s'@'%%' IDENTIFIED BY '%s';", sqlStringLiteral(username), sqlStringLiteral(targetPassword))
		return []string{"sh", "-c", "MYSQL_PWD='" + sqlStringLiteral(authPassword) + "' mysql -u " + sqlStringLiteral(username) + " --execute='" + stmt + "'"}
	}
	return nil
}

func (s *DatabaseService) resolveDatabasePort(ctx context.Context, db *domain.Database, allocatedPorts []string, defaultPort string) error {
	hostPort, err := s.parseAllocatedPort(allocatedPorts, defaultPort)
	if err != nil || hostPort == 0 {
		hostPort, err = s.compute.GetInstancePort(ctx, db.ContainerID, defaultPort)
		if err != nil {
			return err
		}
	}
	db.Port = hostPort
	return nil
}

func (s *DatabaseService) provisionSidecars(ctx context.Context, db *domain.Database, engine domain.DatabaseEngine, dbIP, username, password, networkID string) error {
	if db.MetricsEnabled {
		if err := s.provisionMetricsSidecar(ctx, db, engine, dbIP, username, password, networkID); err != nil {
			return err
		}
	}
	if db.PoolingEnabled {
		if err := s.provisionPoolerSidecar(ctx, db, engine, dbIP, username, password, networkID); err != nil {
			return err
		}
	}
	return nil
}

func (s *DatabaseService) provisionMetricsSidecar(ctx context.Context, db *domain.Database, engine domain.DatabaseEngine, dbIP, username, password, networkID string) error {
	image, env, internalPort := s.getExporterConfig(engine, dbIP, username, password, db.Name)
	cid, ports, err := s.compute.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{
		Name:      fmt.Sprintf("cloud-db-exporter-%s-%s", db.Name, db.ID.String()[:8]),
		ImageName: image,
		Ports:     []string{"0:" + internalPort},
		NetworkID: networkID,
		Env:       env,
	})
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to launch metrics exporter", err)
	}
	db.ExporterContainerID = cid
	hostPort, err := s.parseAllocatedPort(ports, internalPort)
	if err != nil || hostPort == 0 {
		hostPort, err = s.compute.GetInstancePort(ctx, cid, internalPort)
		if err != nil {
			// Cleanup the sidecar on port resolution failure
			_ = s.compute.DeleteInstance(ctx, cid)
			db.ExporterContainerID = ""
			return errors.Wrap(errors.Internal, "failed to resolve metrics exporter port", err)
		}
	}
	db.MetricsPort = hostPort
	return nil
}

func (s *DatabaseService) provisionPoolerSidecar(ctx context.Context, db *domain.Database, engine domain.DatabaseEngine, dbIP, username, password, networkID string) error {
	image, env, internalPort := s.getPoolerConfig(engine, dbIP, username, password, db.Name)
	cid, ports, err := s.compute.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{
		Name:      fmt.Sprintf("cloud-db-pooler-%s-%s", db.Name, db.ID.String()[:8]),
		ImageName: image,
		Ports:     []string{"0:" + internalPort},
		NetworkID: networkID,
		Env:       env,
	})
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to launch connection pooler", err)
	}
	db.PoolerContainerID = cid
	hostPort, err := s.parseAllocatedPort(ports, internalPort)
	if err != nil || hostPort == 0 {
		hostPort, err = s.compute.GetInstancePort(ctx, cid, internalPort)
		if err != nil {
			// Cleanup the sidecar on port resolution failure
			_ = s.compute.DeleteInstance(ctx, cid)
			db.PoolerContainerID = ""
			return errors.Wrap(errors.Internal, "failed to resolve connection pooler port", err)
		}
	}
	db.PoolingPort = hostPort
	return nil
}

func (s *DatabaseService) performProvisioningRollback(ctx context.Context, db *domain.Database, volID string, err error) (*domain.Database, error) {
	s.logger.Error("rolling back database provisioning due to failure", "error", err)
	if db.ContainerID != "" {
		if deleteErr := s.compute.DeleteInstance(ctx, db.ContainerID); deleteErr != nil {
			s.logger.Warn("failed to delete database container during rollback", "container_id", db.ContainerID, "error", deleteErr)
		}
	}
	if db.ExporterContainerID != "" {
		if deleteErr := s.compute.DeleteInstance(ctx, db.ExporterContainerID); deleteErr != nil {
			s.logger.Warn("failed to delete exporter container during rollback", "container_id", db.ExporterContainerID, "error", deleteErr)
		}
	}
	if db.PoolerContainerID != "" {
		if deleteErr := s.compute.DeleteInstance(ctx, db.PoolerContainerID); deleteErr != nil {
			s.logger.Warn("failed to delete pooler container during rollback", "container_id", db.PoolerContainerID, "error", deleteErr)
		}
	}
	if db.CredentialPath != "" {
		if delErr := s.secrets.DeleteSecret(ctx, db.CredentialPath); delErr != nil {
			s.logger.Warn("failed to delete database credentials from vault during rollback", "path", db.CredentialPath, "error", delErr)
		}
	}
	if deleteErr := s.volumeSvc.DeleteVolume(ctx, volID); deleteErr != nil {
		s.logger.Warn("failed to delete volume during rollback", "volume_id", volID, "error", deleteErr)
	}
	return nil, err
}

func (s *DatabaseService) getBackendVolName(vol *domain.Volume) string {
	if vol.BackendPath != "" {
		return vol.BackendPath
	}
	return "thecloud-vol-" + vol.ID.String()[:8]
}

func (s *DatabaseService) getMountPath(engine domain.DatabaseEngine) string {
	if engine == domain.EngineMySQL {
		return "/var/lib/mysql"
	}
	return "/var/lib/postgresql/data"
}

func (s *DatabaseService) isValidEngine(engine domain.DatabaseEngine) bool {
	return engine == domain.EnginePostgres || engine == domain.EngineMySQL
}

func (s *DatabaseService) getDefaultUsername(engine domain.DatabaseEngine) string {
	if engine == domain.EngineMySQL {
		return "root"
	}
	return "cloud_user"
}

func (s *DatabaseService) getExporterConfig(engine domain.DatabaseEngine, dbIP, username, password, dbName string) (string, []string, string) {
	switch engine {
	case domain.EnginePostgres:
		dsn := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable", username, password, dbIP, DefaultPostgresPort, dbName)
		return PostgresExporterImage, []string{"DATA_SOURCE_NAME=" + dsn}, PostgresExporterPort
	case domain.EngineMySQL:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", username, password, dbIP, DefaultMySQLPort, dbName)
		return MySQLExporterImage, []string{"DATA_SOURCE_NAME=" + dsn}, MySQLExporterPort
	}
	return "", nil, ""
}

func (s *DatabaseService) getPoolerConfig(engine domain.DatabaseEngine, dbIP, username, password, dbName string) (string, []string, string) {
	if engine == domain.EnginePostgres {
		env := []string{
			"DB_HOST=" + dbIP,
			"DB_PORT=" + DefaultPostgresPort,
			"DB_USER=" + username,
			"DB_PASSWORD=" + password,
			"DB_NAME=" + dbName,
			"POOL_MODE=" + DefaultPoolMode,
			"MAX_CLIENT_CONN=" + DefaultMaxClientConn,
			"DEFAULT_POOL_SIZE=" + DefaultPoolSize,
		}
		return PoolerImage, env, PoolerInternalPort
	}
	return "", nil, ""
}

func (s *DatabaseService) buildEngineCmd(engine domain.DatabaseEngine, parameters map[string]string) []string {
	if len(parameters) == 0 {
		return nil
	}
	var cmd []string
	switch engine {
	case domain.EnginePostgres:
		cmd = append(cmd, "postgres")
		for k, v := range parameters {
			cmd = append(cmd, "-c", fmt.Sprintf("%s=%s", k, v))
		}
	case domain.EngineMySQL:
		cmd = append(cmd, "mysqld")
		for k, v := range parameters {
			cmd = append(cmd, fmt.Sprintf("--%s=%s", k, v))
		}
	}
	return cmd
}

func (s *DatabaseService) getEngineConfig(engine domain.DatabaseEngine, version, username, password, name string, role domain.DatabaseRole, primaryIP string) (string, []string, string) {
	switch engine {
	case domain.EnginePostgres:
		env := []string{"POSTGRES_USER=" + username, "POSTGRES_PASSWORD=" + password, "POSTGRES_DB=" + name}
		if role == domain.RoleReplica {
			env = append(env, "PRIMARY_HOST="+primaryIP)
		}
		return fmt.Sprintf("postgres:%s-alpine", version), env, DefaultPostgresPort
	case domain.EngineMySQL:
		env := []string{"MYSQL_ROOT_PASSWORD=" + password, "MYSQL_DATABASE=" + name}
		if role == domain.RoleReplica {
			env = append(env, "PRIMARY_HOST="+primaryIP)
		}
		return fmt.Sprintf("mysql:%s", version), env, DefaultMySQLPort
	}
	return "", nil, ""
}

func (s *DatabaseService) resolveVpcNetwork(ctx context.Context, vpcID *uuid.UUID) (string, error) {
	if vpcID == nil {
		return "", nil
	}
	vpc, err := s.vpcRepo.GetByID(ctx, *vpcID)
	if err != nil {
		return "", err
	}
	return vpc.NetworkID, nil
}

func (s *DatabaseService) initialDatabaseRecord(userID uuid.UUID, name string, engine domain.DatabaseEngine, version, username, password string, vpcID *uuid.UUID) *domain.Database {
	return &domain.Database{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		Engine:    engine,
		Version:   version,
		Status:    domain.DatabaseStatusCreating,
		VpcID:     vpcID,
		Username:  username,
		Password:  password,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (s *DatabaseService) recordDatabaseCreation(ctx context.Context, userID uuid.UUID, db *domain.Database, action string) {
	_ = s.eventSvc.RecordEvent(ctx, action, db.ID.String(), "DATABASE", map[string]interface{}{"name": db.Name, "engine": db.Engine})

	auditAction := "database.create"
	switch action {
	case "DATABASE_REPLICA_CREATE":
		auditAction = "database.replica_create"
	case "DATABASE_RESTORE":
		auditAction = "database.restore"
	}

	_ = s.auditSvc.Log(ctx, userID, auditAction, "database", db.ID.String(), map[string]interface{}{"name": db.Name, "engine": string(db.Engine)})
	platform.RDSInstancesTotal.WithLabelValues(string(db.Engine), "running").Inc()
}

func (s *DatabaseService) getVolumeForDatabase(ctx context.Context, db *domain.Database) (*domain.Volume, error) {
	vols, err := s.volumeSvc.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}
	expectedPrefix := fmt.Sprintf("db-vol-%s", db.ID.String()[:8])
	if db.Role == domain.RoleReplica {
		expectedPrefix = fmt.Sprintf("db-replica-vol-%s", db.ID.String()[:8])
	}
	for _, v := range vols {
		if strings.HasPrefix(v.Name, expectedPrefix) {
			return v, nil
		}
	}
	return nil, errors.New(errors.NotFound, "volume not found")
}

func (s *DatabaseService) parseAllocatedPort(allocatedPorts []string, targetPort string) (int, error) {
	for _, p := range allocatedPorts {
		parts := strings.Split(p, ":")
		if len(parts) == 2 && parts[1] == targetPort {
			hp, err := strconv.Atoi(parts[0])
			if err != nil {
				return 0, err
			}
			return hp, nil
		}
	}
	return 0, nil
}
