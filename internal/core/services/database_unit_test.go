package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type DatabaseUnitMockRepo struct {
	mock.Mock
}

func (m *DatabaseUnitMockRepo) Create(ctx context.Context, db *domain.Database) error {
	return m.Called(ctx, db).Error(0)
}
func (m *DatabaseUnitMockRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Database)
	return r0, args.Error(1)
}
func (m *DatabaseUnitMockRepo) List(ctx context.Context) ([]*domain.Database, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Database)
	return r0, args.Error(1)
}
func (m *DatabaseUnitMockRepo) ListReplicas(ctx context.Context, primaryID uuid.UUID) ([]*domain.Database, error) {
	args := m.Called(ctx, primaryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Database)
	return r0, args.Error(1)
}
func (m *DatabaseUnitMockRepo) Update(ctx context.Context, db *domain.Database) error {
	return m.Called(ctx, db).Error(0)
}
func (m *DatabaseUnitMockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func TestDatabaseService_Unit(t *testing.T) {
	t.Run("Extended", testDatabaseServiceUnitExtended)
	t.Run("RBACErrors", testDatabaseServiceUnitRbacErrors)
	t.Run("RepoErrors", testDatabaseServiceUnitRepoErrors)
	t.Run("ValidationErrors", testDatabaseServiceUnitValidationErrors)
}

func testDatabaseServiceUnitExtended(t *testing.T) {
	mockRepo := new(DatabaseUnitMockRepo)
	mockCompute := new(MockComputeBackend)
	mockVpcRepo := new(MockVpcRepo)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	mockVolumeSvc := new(MockVolumeService)
	mockSecrets := new(MockSecretsManager)
	mockRBAC := new(mockRBACService)
	snapSvc := new(mockSnapshotService)
	snapRepo := new(mockSnapshotRepository)
	mockRBAC.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewDatabaseService(services.DatabaseServiceParams{
		Repo:         mockRepo,
		RBAC:         mockRBAC,
		Compute:      mockCompute,
		VpcRepo:      mockVpcRepo,
		VolumeSvc:    mockVolumeSvc,
		SnapshotSvc:  snapSvc,
		SnapshotRepo: snapRepo,
		EventSvc:     mockEventSvc,
		AuditSvc:     mockAuditSvc,
		Secrets:      mockSecrets,
		Logger:       slog.Default(),
	})

	userID := uuid.New()
	ctx := context.Background()

	t.Run("CreateDatabase_Success", func(t *testing.T) {
		mockCompute.On("Type").Return("docker").Maybe()
		mockVolumeSvc.On("CreateVolume", mock.Anything, mock.Anything, 20).
			Return(&domain.Volume{ID: uuid.New(), Name: "db-vol"}, nil).Once()

		mockSecrets.On("StoreSecret", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).
			Return("cid", []string{"30001:5432"}, nil).Once()
		mockCompute.On("GetInstanceIP", mock.Anything, "cid").Return("10.0.0.5", nil).Once()
		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_CREATE", mock.Anything, "DATABASE", mock.Anything).
			Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, mock.Anything, "database.create", "database", mock.Anything, mock.Anything).
			Return(nil).Once()

		db, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{
			Name:             "test-db",
			Engine:           "postgres",
			Version:          "16",
			AllocatedStorage: 20,
			Parameters:       map[string]string{"max_connections": "100"},
		})
		require.NoError(t, err)
		assert.NotNil(t, db)
		assert.Equal(t, 30001, db.Port)
		assert.Equal(t, 20, db.AllocatedStorage)
		assert.Equal(t, "100", db.Parameters["max_connections"])
	})

	t.Run("CreateReplica", func(t *testing.T) {
		primaryID := uuid.New()
		primary := &domain.Database{ID: primaryID, Engine: "postgres", Version: "16", Port: 5432, ContainerID: "primary-cid", AllocatedStorage: 20, Username: "cloud_user", Password: "pass"}
		mockRepo.On("GetByID", mock.Anything, primaryID).Return(primary, nil).Once()
		mockCompute.On("GetInstanceIP", mock.Anything, "primary-cid").Return("10.0.0.5", nil).Once()
		mockVolumeSvc.On("CreateVolume", mock.Anything, mock.Anything, 20).
			Return(&domain.Volume{ID: uuid.New(), Name: "db-replica-vol"}, nil).Once()

		mockSecrets.On("StoreSecret", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).
			Return("cid-rep", []string{"30002:5432"}, nil).Once()
		mockCompute.On("GetInstanceIP", mock.Anything, "cid-rep").Return("10.0.0.6", nil).Once()
		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_REPLICA_CREATE", mock.Anything, "DATABASE", mock.Anything).
			Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, mock.Anything, "database.replica_create", "database", mock.Anything, mock.Anything).
			Return(nil).Once()

		replica, err := svc.CreateReplica(ctx, primaryID, "test-rep")
		require.NoError(t, err)
		assert.NotNil(t, replica)
		assert.Equal(t, domain.RoleReplica, replica.Role)
		assert.Equal(t, 20, replica.AllocatedStorage)
	})

	t.Run("PromoteToPrimary", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, Role: domain.RoleReplica, Engine: domain.EnginePostgres, Username: "user", Password: "pass", ContainerID: "test-container"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		// Assert the promotion command uses trigger file method
		mockCompute.On("Exec", mock.Anything, "test-container", []string{"touch", "/var/lib/postgresql/data/promote"}).Return("", nil).Once()
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(d *domain.Database) bool {
			return d.Role == domain.RolePrimary
		})).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_PROMOTED", dbID.String(), "DATABASE", mock.Anything).
			Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, mock.Anything, "database.promote", "database", dbID.String(), mock.Anything).
			Return(nil).Once()

		err := svc.PromoteToPrimary(ctx, dbID)
		require.NoError(t, err)
	})

	t.Run("GetConnectionString", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{
			ID:             dbID,
			Engine:         domain.EnginePostgres,
			Username:       "user",
			Password:       "pass",
			Port:           5432,
			Name:           "mydb",
			CredentialPath: "secret/rds/db1",
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockSecrets.On("GetSecret", mock.Anything, "secret/rds/db1").Return(map[string]interface{}{"password": "pass"}, nil).Once()

		conn, err := svc.GetConnectionString(ctx, dbID)
		require.NoError(t, err)
		assert.Contains(t, conn, "postgres://user:pass@127.0.0.1:5432/mydb")
	})

	t.Run("DeleteDatabase", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, ContainerID: "cid", Role: domain.RolePrimary}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockCompute.On("DeleteInstance", mock.Anything, "cid").Return(nil).Once()

		volID := uuid.New()
		mockVolumeSvc.On("ListVolumes", mock.Anything).Return([]*domain.Volume{
			{ID: volID, Name: fmt.Sprintf("db-vol-%s", dbID.String()[:8])},
		}, nil).Once()
		mockVolumeSvc.On("DeleteVolume", mock.Anything, volID.String()).Return(nil).Once()

		mockRepo.On("Delete", mock.Anything, dbID).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_DELETE", dbID.String(), "DATABASE", mock.Anything).
			Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, mock.Anything, "database.delete", "database", dbID.String(), mock.Anything).
			Return(nil).Once()

		err := svc.DeleteDatabase(ctx, dbID)
		require.NoError(t, err)
	})

	t.Run("GetDatabase", func(t *testing.T) {
		dbID := uuid.New()
		expected := &domain.Database{ID: dbID, Name: "test-db"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(expected, nil).Once()

		result, err := svc.GetDatabase(ctx, dbID)
		require.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("ListDatabases", func(t *testing.T) {
		expected := []*domain.Database{{ID: uuid.New()}}
		mockRepo.On("List", mock.Anything).Return(expected, nil).Once()

		result, err := svc.ListDatabases(ctx)
		require.NoError(t, err)
		assert.Equal(t, expected, result)
	})

	t.Run("CreateDatabase_RollbackOnLaunchFailure", func(t *testing.T) {
		volID := uuid.New()
		mockVolumeSvc.On("CreateVolume", mock.Anything, mock.Anything, 10).
			Return(&domain.Volume{ID: volID, Name: "db-vol"}, nil).Once()

		mockSecrets.On("StoreSecret", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).
			Return("", nil, fmt.Errorf("launch failed")).Once()

		mockSecrets.On("DeleteSecret", mock.Anything, mock.Anything).Return(nil).Once()
		mockVolumeSvc.On("DeleteVolume", mock.Anything, volID.String()).Return(nil).Once()

		db, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{
			Name:             "fail-db",
			Engine:           "postgres",
			Version:          "16",
			AllocatedStorage: 10,
		})
		require.Error(t, err)
		assert.Nil(t, db)
		assert.Contains(t, err.Error(), "launch failed")
	})

	t.Run("CreateDatabase_RollbackOnRepoFailure", func(t *testing.T) {
		volID := uuid.New()
		mockVolumeSvc.On("CreateVolume", mock.Anything, mock.Anything, 10).
			Return(&domain.Volume{ID: volID, Name: "db-vol"}, nil).Once()

		mockSecrets.On("StoreSecret", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).
			Return("cid-123", []string{"30001:5432"}, nil).Once()

		mockCompute.On("GetInstanceIP", mock.Anything, "cid-123").Return("10.0.0.5", nil).Once()

		mockRepo.On("Create", mock.Anything, mock.Anything).
			Return(fmt.Errorf("repo failed")).Once()

		mockCompute.On("DeleteInstance", mock.Anything, "cid-123").Return(nil).Once()
		mockSecrets.On("DeleteSecret", mock.Anything, mock.Anything).Return(nil).Once()
		mockVolumeSvc.On("DeleteVolume", mock.Anything, volID.String()).Return(nil).Once()

		db, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{
			Name:             "repo-fail-db",
			Engine:           "postgres",
			Version:          "16",
			AllocatedStorage: 10,
		})
		require.Error(t, err)
		assert.Nil(t, db)
		assert.Contains(t, err.Error(), "repo failed")
	})

	t.Run("CreateReplica failure cases", func(t *testing.T) {
		cases := []struct {
			name            string
			primaryID       uuid.UUID
			mockReturn      *domain.Database
			mockErr         error
			callersTenantID uuid.UUID
			otherTenantID   uuid.UUID
			expectErrSubstr string
		}{
			{
				name:            "primary not found",
				primaryID:       uuid.New(),
				mockReturn:      nil,
				mockErr:         fmt.Errorf("not found"),
				expectErrSubstr: "not found",
			},
			{
				name:            "cross-tenant",
				primaryID:       uuid.New(),
				callersTenantID: uuid.New(),
				otherTenantID:   uuid.New(),
				mockReturn:      nil,
				mockErr:         fmt.Errorf("not found"),
				expectErrSubstr: "not found",
			},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				testCtx := ctx
				if c.callersTenantID != uuid.Nil {
					primary := &domain.Database{ID: c.primaryID, TenantID: c.otherTenantID, Engine: "postgres", Version: "16", Port: 5432, ContainerID: "primary-cid", AllocatedStorage: 20, Username: "cloud_user", Password: "pass"}
					mockRepo.On("GetByID", mock.Anything, c.primaryID).Return(primary, nil).Once()
					testCtx = appcontext.WithTenantID(ctx, c.callersTenantID)
				} else {
					mockRepo.On("GetByID", mock.Anything, c.primaryID).Return(c.mockReturn, c.mockErr).Once()
				}

				replica, err := svc.CreateReplica(testCtx, c.primaryID, "fail-rep")
				require.Error(t, err)
				assert.Nil(t, replica)
				assert.Contains(t, err.Error(), c.expectErrSubstr)
			})
		}
	})

	t.Run("CreateDatabaseSnapshot_Success", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, Name: "test-db", Status: domain.DatabaseStatusRunning, Role: domain.RolePrimary}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		mockVolumeSvc.On("ListVolumes", mock.Anything).Return([]*domain.Volume{
			{ID: uuid.New(), Name: fmt.Sprintf("db-vol-%s", dbID.String()[:8])},
		}, nil).Once()

		snapSvc.On("CreateSnapshot", mock.Anything, mock.Anything, mock.Anything).
			Return(&domain.Snapshot{ID: uuid.New()}, nil).Once()

		mockAuditSvc.On("Log", mock.Anything, mock.Anything, "database.snapshot.create", "database", dbID.String(), mock.Anything).
			Return(nil).Once()

		snap, err := svc.CreateDatabaseSnapshot(ctx, dbID, "manual backup")
		require.NoError(t, err)
		assert.NotNil(t, snap)
	})

	t.Run("RestoreDatabase_Success", func(t *testing.T) {
		snapID := uuid.New()
		snapSvc.On("GetSnapshot", mock.Anything, snapID).
			Return(&domain.Snapshot{ID: snapID, SizeGB: 10}, nil).Once()

		snapSvc.On("RestoreSnapshot", mock.Anything, snapID, mock.Anything).
			Return(&domain.Volume{ID: uuid.New(), SizeGB: 10}, nil).Once()

		mockSecrets.On("StoreSecret", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.MatchedBy(func(opts ports.CreateInstanceOptions) bool {
			return strings.Contains(opts.Name, "cloud-db-")
		})).Return("new-cid", []string{"30005:5432"}, nil).Once()
		mockCompute.On("GetInstanceIP", mock.Anything, "new-cid").Return("10.0.0.10", nil).Once()

		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_RESTORE", mock.Anything, "DATABASE", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, mock.Anything, "database.restore", "database", mock.Anything, mock.Anything).Return(nil).Once()

		db, err := svc.RestoreDatabase(ctx, ports.RestoreDatabaseRequest{
			SnapshotID:       snapID,
			NewName:          "restored-db",
			Engine:           "postgres",
			Version:          "16",
			AllocatedStorage: 10,
		})
		require.NoError(t, err)
		assert.NotNil(t, db)
		assert.Equal(t, "new-cid", db.ContainerID)
	})

	t.Run("ListDatabaseSnapshots_Success", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, Role: domain.RolePrimary}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		mockVolumeSvc.On("ListVolumes", mock.Anything).Return([]*domain.Volume{
			{ID: uuid.New(), Name: fmt.Sprintf("db-vol-%s", dbID.String()[:8])},
		}, nil).Once()

		snapRepo.On("ListByVolumeID", mock.Anything, mock.Anything).
			Return([]*domain.Snapshot{{ID: uuid.New()}}, nil).Once()

		snaps, err := svc.ListDatabaseSnapshots(ctx, dbID)
		require.NoError(t, err)
		assert.Len(t, snaps, 1)
	})

	t.Run("CreateDatabase_WithMetrics", func(t *testing.T) {
		mockVolumeSvc.On("CreateVolume", mock.Anything, mock.Anything, 10).
			Return(&domain.Volume{ID: uuid.New(), Name: "db-vol"}, nil).Once()

		mockSecrets.On("StoreSecret", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		// DB container
		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.MatchedBy(func(opts ports.CreateInstanceOptions) bool {
			return strings.Contains(opts.Name, "cloud-db-") && !strings.Contains(opts.Name, "exporter")
		})).Return("db-cid", []string{"30001:5432"}, nil).Once()

		mockCompute.On("GetInstanceIP", mock.Anything, "db-cid").Return("10.0.0.5", nil).Once()

		// Exporter sidecar container
		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.MatchedBy(func(opts ports.CreateInstanceOptions) bool {
			return strings.Contains(opts.Name, "exporter")
		})).Return("exp-cid", []string{"30002:9187"}, nil).Once()

		// Mock GetInstancePort because parseAllocatedPort might fail if ports are empty or formatted differently
		mockCompute.On("GetInstancePort", mock.Anything, "exp-cid", "9187").Return(30002, nil).Maybe()

		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_CREATE", mock.Anything, "DATABASE", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, mock.Anything, "database.create", "database", mock.Anything, mock.Anything).Return(nil).Once()

		db, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{
			Name:             "metrics-db",
			Engine:           "postgres",
			Version:          "16",
			AllocatedStorage: 10,
			MetricsEnabled:   true,
		})
		require.NoError(t, err)
		assert.NotNil(t, db)
		assert.Equal(t, "exp-cid", db.ExporterContainerID)
		assert.Equal(t, 30002, db.MetricsPort)
	})

	t.Run("CreateDatabase_WithPooling", func(t *testing.T) {
		mockVolumeSvc.On("CreateVolume", mock.Anything, mock.Anything, 10).
			Return(&domain.Volume{ID: uuid.New(), Name: "db-vol"}, nil).Once()

		mockSecrets.On("StoreSecret", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		// DB container
		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.MatchedBy(func(opts ports.CreateInstanceOptions) bool {
			return strings.Contains(opts.Name, "cloud-db-") && !strings.Contains(opts.Name, "pooler")
		})).Return("db-cid", []string{"30001:5432"}, nil).Once()

		mockCompute.On("GetInstanceIP", mock.Anything, "db-cid").Return("10.0.0.5", nil).Once()

		// Pooler sidecar container - internal port is now 5432
		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.MatchedBy(func(opts ports.CreateInstanceOptions) bool {
			return strings.Contains(opts.Name, "pooler")
		})).Return("pooler-cid", []string{"30003:5432"}, nil).Once()

		// Mock GetInstancePort for the pooler sidecar
		mockCompute.On("GetInstancePort", mock.Anything, "pooler-cid", "5432").Return(30003, nil).Maybe()

		mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_CREATE", mock.Anything, "DATABASE", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, mock.Anything, "database.create", "database", mock.Anything, mock.Anything).Return(nil).Once()

		db, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{
			Name:             "test-db-pooling",
			Engine:           "postgres",
			Version:          "16",
			AllocatedStorage: 10,
			PoolingEnabled:   true,
		})
		require.NoError(t, err)
		assert.NotNil(t, db)
		assert.Equal(t, "pooler-cid", db.PoolerContainerID)
		assert.Equal(t, 30003, db.PoolingPort)
		assert.True(t, db.PoolingEnabled)
	})

	t.Run("CreateDatabase_PoolerFailure", func(t *testing.T) {
		volID := uuid.New()
		mockVolumeSvc.On("CreateVolume", mock.Anything, mock.Anything, 10).
			Return(&domain.Volume{ID: volID, Name: "db-vol"}, nil).Once()

		mockSecrets.On("StoreSecret", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		// DB container succeeds
		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.MatchedBy(func(opts ports.CreateInstanceOptions) bool {
			return strings.Contains(opts.Name, "cloud-db-") && !strings.Contains(opts.Name, "pooler")
		})).Return("db-cid", []string{"30001:5432"}, nil).Once()

		mockCompute.On("GetInstanceIP", mock.Anything, "db-cid").Return("10.0.0.5", nil).Once()

		// Pooler fails to launch
		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.MatchedBy(func(opts ports.CreateInstanceOptions) bool {
			return strings.Contains(opts.Name, "pooler")
		})).Return("", nil, fmt.Errorf("pooler launch failed")).Once()

		// Rollback expectations
		mockCompute.On("DeleteInstance", mock.Anything, "db-cid").Return(nil).Once()
		mockSecrets.On("DeleteSecret", mock.Anything, mock.Anything).Return(nil).Once()
		mockVolumeSvc.On("DeleteVolume", mock.Anything, volID.String()).Return(nil).Once()

		db, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{
			Name:             "pooler-fail",
			Engine:           "postgres",
			Version:          "16",
			AllocatedStorage: 10,
			PoolingEnabled:   true,
		})

		require.Error(t, err)
		assert.Nil(t, db)
		assert.Contains(t, err.Error(), "pooler launch failed")
	})

	t.Run("ModifyDatabase_VolumeResize", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{
			ID:               dbID,
			UserID:           userID,
			Name:             "test-db",
			AllocatedStorage: 10,
			ContainerID:      "cid",
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		volID := uuid.New()
		mockVolumeSvc.On("ListVolumes", mock.Anything).Return([]*domain.Volume{
			{ID: volID, Name: fmt.Sprintf("db-vol-%s", dbID.String()[:8])},
		}, nil).Once()

		newSize := 20
		mockVolumeSvc.On("ResizeVolume", mock.Anything, volID.String(), newSize).Return(nil).Once()
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(d *domain.Database) bool {
			return d.AllocatedStorage == newSize
		})).Return(nil).Once()

		mockCompute.On("GetInstanceIP", mock.Anything, "cid").Return("10.0.0.5", nil).Once()

		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_MODIFY", dbID.String(), "DATABASE", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "database.modify", "database", dbID.String(), mock.Anything).Return(nil).Once()

		result, err := svc.ModifyDatabase(ctx, ports.ModifyDatabaseRequest{
			ID:               dbID,
			AllocatedStorage: &newSize,
		})

		require.NoError(t, err)
		assert.Equal(t, newSize, result.AllocatedStorage)
	})

	t.Run("StopDatabase_Success", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{
			ID:                  dbID,
			UserID:              userID,
			Status:              domain.DatabaseStatusRunning,
			Role:                domain.RolePrimary,
			Engine:              domain.EnginePostgres,
			Name:                "test-stop-db",
			ContainerID:         "db-cid",
			ExporterContainerID: "exp-cid",
			PoolingEnabled:      true,
			PoolerContainerID:   "pooler-cid",
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockCompute.On("StopInstance", mock.Anything, "exp-cid").Return(nil).Once()
		mockCompute.On("StopInstance", mock.Anything, "pooler-cid").Return(nil).Once()
		mockCompute.On("StopInstance", mock.Anything, "db-cid").Return(nil).Once()
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(d *domain.Database) bool {
			return d.Status == domain.DatabaseStatusStopped
		})).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_STOP", dbID.String(), "DATABASE", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "database.stop", "database", dbID.String(), mock.Anything).Return(nil).Once()

		err := svc.StopDatabase(ctx, dbID)
		require.NoError(t, err)
	})

	t.Run("StopDatabase_NotRunning", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, Status: domain.DatabaseStatusStopped, Role: domain.RolePrimary}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		err := svc.StopDatabase(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})

	t.Run("StopDatabase_Replica", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, Status: domain.DatabaseStatusRunning, Role: domain.RoleReplica}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		err := svc.StopDatabase(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "replica")
	})

	t.Run("StopDatabase_ComputeError", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{
			ID:          dbID,
			UserID:      userID,
			Status:      domain.DatabaseStatusRunning,
			Role:        domain.RolePrimary,
			ContainerID: "db-cid",
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockCompute.On("StopInstance", mock.Anything, "db-cid").Return(fmt.Errorf("stop failed")).Once()

		err := svc.StopDatabase(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stop failed")
	})

	t.Run("StartDatabase_Success", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{
			ID:                dbID,
			UserID:            userID,
			Status:            domain.DatabaseStatusStopped,
			Role:              domain.RolePrimary,
			Engine:            domain.EnginePostgres,
			Name:              "test-start-db",
			ContainerID:       "db-cid",
			PoolingEnabled:    true,
			PoolerContainerID: "pooler-cid",
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockCompute.On("StartInstance", mock.Anything, "db-cid").Return(nil).Once()
		mockCompute.On("GetInstanceIP", mock.Anything, "db-cid").Return("10.0.0.5", nil).Once()
		mockCompute.On("StartInstance", mock.Anything, "pooler-cid").Return(nil).Once()
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(d *domain.Database) bool {
			return d.Status == domain.DatabaseStatusRunning
		})).Return(nil).Once()
		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_START", dbID.String(), "DATABASE", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "database.start", "database", dbID.String(), mock.Anything).Return(nil).Once()

		err := svc.StartDatabase(ctx, dbID)
		require.NoError(t, err)
	})

	t.Run("StartDatabase_NotStopped", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, Status: domain.DatabaseStatusRunning, Role: domain.RolePrimary}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		err := svc.StartDatabase(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not stopped")
	})

	t.Run("StartDatabase_MissingContainerID", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, Status: domain.DatabaseStatusStopped, Role: domain.RolePrimary, ContainerID: ""}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		err := svc.StartDatabase(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "container ID is missing")
	})

	t.Run("StartDatabase_ComputeError", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{
			ID:          dbID,
			UserID:      userID,
			Status:      domain.DatabaseStatusStopped,
			Role:        domain.RolePrimary,
			ContainerID: "db-cid",
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockCompute.On("StartInstance", mock.Anything, "db-cid").Return(fmt.Errorf("start failed")).Once()

		err := svc.StartDatabase(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start failed")
	})

	t.Run("StartDatabase_ReadinessTimeout", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{
			ID:          dbID,
			UserID:      userID,
			Status:      domain.DatabaseStatusStopped,
			Role:        domain.RolePrimary,
			ContainerID: "cid-timeout",
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockCompute.On("StartInstance", mock.Anything, "cid-timeout").Return(nil).Maybe()
		mockCompute.On("GetInstanceIP", mock.Anything, "cid-timeout").Return("", nil).Maybe()
		mockRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
		mockEventSvc.On("RecordEvent", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		mockAuditSvc.On("Log", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

		err := svc.StartDatabase(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "did not become ready")
	})
}

func testDatabaseServiceUnitRbacErrors(t *testing.T) {
	mockRepo := new(DatabaseUnitMockRepo)
	mockCompute := new(MockComputeBackend)
	mockVpcRepo := new(MockVpcRepo)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	mockVolumeSvc := new(MockVolumeService)
	mockSecrets := new(MockSecretsManager)
	mockRBAC := new(mockRBACService)
	snapSvc := new(mockSnapshotService)
	snapRepo := new(mockSnapshotRepository)
	defer mock.AssertExpectationsForObjects(t, mockRepo, mockRBAC, mockCompute, mockVpcRepo, mockVolumeSvc, snapSvc, snapRepo, mockEventSvc, mockAuditSvc, mockSecrets)

	svc := services.NewDatabaseService(services.DatabaseServiceParams{
		Repo:         mockRepo,
		RBAC:         mockRBAC,
		Compute:      mockCompute,
		VpcRepo:      mockVpcRepo,
		VolumeSvc:    mockVolumeSvc,
		SnapshotSvc:  snapSvc,
		SnapshotRepo: snapRepo,
		EventSvc:     mockEventSvc,
		AuditSvc:     mockAuditSvc,
		Secrets:      mockSecrets,
		Logger:       slog.Default(),
	})

	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	ctx = appcontext.WithTenantID(ctx, tenantID)
	ctx = appcontext.WithUserID(ctx, userID)

	dbID := uuid.New()

	type rbacCase struct {
		name       string
		permission domain.Permission
		resourceID string
		invoke     func() error
	}

	cases := []rbacCase{
		{
			name:       "CreateDatabase_Unauthorized",
			permission: domain.PermissionDBCreate,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{Name: "db", Engine: "postgres", Version: "16"})
				return err
			},
		},
		{
			name:       "CreateReplica_Unauthorized",
			permission: domain.PermissionDBCreate,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.CreateReplica(ctx, dbID, "rep")
				return err
			},
		},
		{
			name:       "RestoreDatabase_Unauthorized",
			permission: domain.PermissionDBCreate,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.RestoreDatabase(ctx, ports.RestoreDatabaseRequest{SnapshotID: uuid.New(), NewName: "db"})
				return err
			},
		},
		{
			name:       "GetDatabase_Unauthorized",
			permission: domain.PermissionDBRead,
			resourceID: dbID.String(),
			invoke: func() error {
				_, err := svc.GetDatabase(ctx, dbID)
				return err
			},
		},
		{
			name:       "ListDatabases_Unauthorized",
			permission: domain.PermissionDBRead,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.ListDatabases(ctx)
				return err
			},
		},
		{
			name:       "DeleteDatabase_Unauthorized",
			permission: domain.PermissionDBDelete,
			resourceID: dbID.String(),
			invoke: func() error {
				return svc.DeleteDatabase(ctx, dbID)
			},
		},
		{
			name:       "PromoteToPrimary_Unauthorized",
			permission: domain.PermissionDBUpdate,
			resourceID: dbID.String(),
			invoke: func() error {
				return svc.PromoteToPrimary(ctx, dbID)
			},
		},
		{
			name:       "GetConnectionString_Unauthorized",
			permission: domain.PermissionDBRead,
			resourceID: dbID.String(),
			invoke: func() error {
				_, err := svc.GetConnectionString(ctx, dbID)
				return err
			},
		},
		{
			name:       "CreateDatabaseSnapshot_Unauthorized",
			permission: domain.PermissionSnapshotCreate,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.CreateDatabaseSnapshot(ctx, dbID, "snap")
				return err
			},
		},
		{
			name:       "ListDatabaseSnapshots_Unauthorized",
			permission: domain.PermissionSnapshotRead,
			resourceID: "*",
			invoke: func() error {
				_, err := svc.ListDatabaseSnapshots(ctx, dbID)
				return err
			},
		},
		{
			name:       "ModifyDatabase_Unauthorized",
			permission: domain.PermissionDBUpdate,
			resourceID: dbID.String(),
			invoke: func() error {
				newSize := 20
				_, err := svc.ModifyDatabase(ctx, ports.ModifyDatabaseRequest{ID: dbID, AllocatedStorage: &newSize})
				return err
			},
		},
		{
			name:       "StopDatabase_Unauthorized",
			permission: domain.PermissionDBUpdate,
			resourceID: dbID.String(),
			invoke: func() error {
				return svc.StopDatabase(ctx, dbID)
			},
		},
		{
			name:       "StartDatabase_Unauthorized",
			permission: domain.PermissionDBUpdate,
			resourceID: dbID.String(),
			invoke: func() error {
				return svc.StartDatabase(ctx, dbID)
			},
		},
	}

	authErr := errors.New(errors.Forbidden, "permission denied")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mockRBAC.On("Authorize", mock.Anything, userID, tenantID, c.permission, c.resourceID).Return(authErr).Once()
			err := c.invoke()
			require.Error(t, err)
			assert.True(t, errors.Is(err, errors.Forbidden))
		})
	}
}

func testDatabaseServiceUnitRepoErrors(t *testing.T) {
	mockRepo := new(DatabaseUnitMockRepo)
	mockCompute := new(MockComputeBackend)
	mockVpcRepo := new(MockVpcRepo)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	mockVolumeSvc := new(MockVolumeService)
	mockSecrets := new(MockSecretsManager)
	mockRBAC := new(mockRBACService)
	snapSvc := new(mockSnapshotService)
	snapRepo := new(mockSnapshotRepository)
	mockRBAC.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	svc := services.NewDatabaseService(services.DatabaseServiceParams{
		Repo:         mockRepo,
		RBAC:         mockRBAC,
		Compute:      mockCompute,
		VpcRepo:      mockVpcRepo,
		VolumeSvc:    mockVolumeSvc,
		SnapshotSvc:  snapSvc,
		SnapshotRepo: snapRepo,
		EventSvc:     mockEventSvc,
		AuditSvc:     mockAuditSvc,
		Secrets:      mockSecrets,
		Logger:       slog.Default(),
	})
	// Allow unexpected calls to snapshotSvc methods in case service internally calls them
	snapSvc.On("ListByVolumeID", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	snapSvc.On("CreateSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	ctx = appcontext.WithTenantID(ctx, tenantID)
	ctx = appcontext.WithUserID(ctx, userID)

	t.Run("GetDatabase_NotFound", func(t *testing.T) {
		dbID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, dbID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		_, err := svc.GetDatabase(ctx, dbID)
		require.Error(t, err)
	})

	t.Run("GetDatabase_RepoError", func(t *testing.T) {
		dbID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, dbID).Return(nil, fmt.Errorf("db error")).Once()

		_, err := svc.GetDatabase(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("ListDatabases_RepoError", func(t *testing.T) {
		mockRepo.On("List", mock.Anything).Return(nil, fmt.Errorf("db error")).Once()

		_, err := svc.ListDatabases(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("DeleteDatabase_NotFound", func(t *testing.T) {
		dbID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, dbID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		err := svc.DeleteDatabase(ctx, dbID)
		require.Error(t, err)
	})

	t.Run("DeleteDatabase_RepoDeleteError", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, ContainerID: ""}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockVolumeSvc.On("ListVolumes", mock.Anything).Return([]*domain.Volume{}, nil).Once()
		mockRepo.On("Delete", mock.Anything, dbID).Return(fmt.Errorf("db error")).Once()

		err := svc.DeleteDatabase(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("PromoteToPrimary_NotFound", func(t *testing.T) {
		dbID := uuid.New()
		mockCompute.On("Type").Return("docker").Maybe()
		mockRepo.On("GetByID", mock.Anything, dbID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		err := svc.PromoteToPrimary(ctx, dbID)
		require.Error(t, err)
	})

	t.Run("PromoteToPrimary_AlreadyPrimary", func(t *testing.T) {
		dbID := uuid.New()
		mockCompute.On("Type").Return("docker").Maybe()
		db := &domain.Database{ID: dbID, Role: domain.RolePrimary, Engine: domain.EnginePostgres, ContainerID: "cid"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		err := svc.PromoteToPrimary(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already a primary")
	})

	t.Run("PromoteToPrimary_UnsupportedEngine", func(t *testing.T) {
		dbID := uuid.New()
		mockCompute.On("Type").Return("docker").Maybe()
		db := &domain.Database{ID: dbID, Role: domain.RoleReplica, Engine: domain.DatabaseEngine("oracle"), ContainerID: "cid"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		err := svc.PromoteToPrimary(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported engine")
	})

	t.Run("PromoteToPrimary_ExecError", func(t *testing.T) {
		dbID := uuid.New()
		mockCompute.On("Type").Return("docker").Maybe()
		db := &domain.Database{ID: dbID, Role: domain.RoleReplica, Engine: domain.EnginePostgres, ContainerID: "cid", Username: "user"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockCompute.On("Exec", mock.Anything, "cid", []string{"touch", "/var/lib/postgresql/data/promote"}).Return("", fmt.Errorf("exec error")).Once()

		err := svc.PromoteToPrimary(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exec error")
	})

	t.Run("GetConnectionString_NotFound", func(t *testing.T) {
		dbID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, dbID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		_, err := svc.GetConnectionString(ctx, dbID)
		require.Error(t, err)
	})

	t.Run("GetConnectionString_UnknownEngine", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, Engine: domain.DatabaseEngine("unknown"), Username: "u", Password: "p", Name: "n"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		_, err := svc.GetConnectionString(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown engine")
	})

	t.Run("CreateDatabaseSnapshot_NotFound", func(t *testing.T) {
		dbID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, dbID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		_, err := svc.CreateDatabaseSnapshot(ctx, dbID, "snap")
		require.Error(t, err)
	})

	t.Run("CreateDatabaseSnapshot_GetVolumeError", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, Role: domain.RolePrimary, ContainerID: "cid"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockVolumeSvc.On("ListVolumes", mock.Anything).Return(nil, fmt.Errorf("volume error")).Once()

		_, err := svc.CreateDatabaseSnapshot(ctx, dbID, "snap")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "volume error")
	})

	t.Run("ListDatabaseSnapshots_NotFound", func(t *testing.T) {
		dbID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, dbID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		_, err := svc.ListDatabaseSnapshots(ctx, dbID)
		require.Error(t, err)
	})

	t.Run("ListDatabaseSnapshots_GetVolumeError", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, Role: domain.RolePrimary, ContainerID: "cid"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockVolumeSvc.On("ListVolumes", mock.Anything).Return(nil, fmt.Errorf("volume error")).Once()

		_, err := svc.ListDatabaseSnapshots(ctx, dbID)
		require.Error(t, err)
	})

	t.Run("ListDatabaseSnapshots_RepoError", func(t *testing.T) {
		dbID := uuid.New()
		volID := uuid.New()
		db := &domain.Database{ID: dbID, Role: domain.RolePrimary, ContainerID: "cid"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockVolumeSvc.On("ListVolumes", mock.Anything).Return([]*domain.Volume{{ID: volID, Name: fmt.Sprintf("db-vol-%s", dbID.String()[:8])}}, nil).Once()
		snapRepo.On("ListByVolumeID", mock.Anything, volID).Return(nil, fmt.Errorf("db error")).Once()

		_, err := svc.ListDatabaseSnapshots(ctx, dbID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("StopDatabase_NotFound", func(t *testing.T) {
		dbID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, dbID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		err := svc.StopDatabase(ctx, dbID)
		require.Error(t, err)
	})

	t.Run("StartDatabase_NotFound", func(t *testing.T) {
		dbID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, dbID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		err := svc.StartDatabase(ctx, dbID)
		require.Error(t, err)
	})

	t.Run("ModifyDatabase_NotFound", func(t *testing.T) {
		dbID := uuid.New()
		mockRepo.On("GetByID", mock.Anything, dbID).Return(nil, errors.New(errors.NotFound, "not found")).Once()

		newSize := 20
		_, err := svc.ModifyDatabase(ctx, ports.ModifyDatabaseRequest{ID: dbID, AllocatedStorage: &newSize})
		require.Error(t, err)
	})

	t.Run("ModifyDatabase_DecreaseStorage", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, UserID: userID, AllocatedStorage: 20, ContainerID: "cid"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()

		decreaseSize := 10
		_, err := svc.ModifyDatabase(ctx, ports.ModifyDatabaseRequest{ID: dbID, AllocatedStorage: &decreaseSize})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot decrease")
	})

	t.Run("ModifyDatabase_VolumeResizeError", func(t *testing.T) {
		dbID := uuid.New()
		volID := uuid.New()
		db := &domain.Database{ID: dbID, UserID: userID, AllocatedStorage: 10, ContainerID: "cid"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockVolumeSvc.On("ListVolumes", mock.Anything).Return([]*domain.Volume{{ID: volID, Name: fmt.Sprintf("db-vol-%s", dbID.String()[:8])}}, nil).Once()

		increaseSize := 20
		mockVolumeSvc.On("ResizeVolume", mock.Anything, volID.String(), increaseSize).Return(fmt.Errorf("resize error")).Once()
		mockCompute.On("GetInstanceIP", mock.Anything, "cid").Return("", nil).Maybe()

		_, err := svc.ModifyDatabase(ctx, ports.ModifyDatabaseRequest{ID: dbID, AllocatedStorage: &increaseSize})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resize error")
	})

	t.Run("ModifyDatabase_UnsupportedPoolingEngine", func(t *testing.T) {
		dbID := uuid.New()
		db := &domain.Database{ID: dbID, UserID: userID, Engine: domain.EngineMySQL, AllocatedStorage: 10, ContainerID: "cid"}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(db, nil).Once()
		mockCompute.On("GetInstanceIP", mock.Anything, "cid").Return("", nil).Once()

		enablePooling := true
		_, err := svc.ModifyDatabase(ctx, ports.ModifyDatabaseRequest{ID: dbID, PoolingEnabled: &enablePooling})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pooling")
	})

	t.Run("RestoreDatabase_SnapshotNotFound", func(t *testing.T) {
		mockRepo.On("GetByID", mock.Anything, mock.Anything).Return(nil, errors.New(errors.NotFound, "not found")).Once()
		snapSvc.On("GetSnapshot", mock.Anything, mock.Anything).Return(nil, errors.New(errors.NotFound, "snapshot not found")).Once()

		_, err := svc.RestoreDatabase(ctx, ports.RestoreDatabaseRequest{SnapshotID: uuid.New(), NewName: "db"})
		require.Error(t, err)
	})

	t.Run("RestoreDatabase_VaultStoreError", func(t *testing.T) {
		snapSvc.On("GetSnapshot", mock.Anything, mock.Anything).Return(&domain.Snapshot{ID: uuid.New(), SizeGB: 10}, nil).Once()
		mockSecrets.On("StoreSecret", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("vault error")).Once()

		_, err := svc.RestoreDatabase(ctx, ports.RestoreDatabaseRequest{SnapshotID: uuid.New(), NewName: "db", Engine: "postgres", Version: "16"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vault")
	})

	t.Run("RestoreDatabase_VpcNotFound", func(t *testing.T) {
		snapID := uuid.New()
		vpcID := uuid.New()
		snapSvc.On("GetSnapshot", mock.Anything, snapID).Return(&domain.Snapshot{ID: snapID, SizeGB: 10}, nil).Once()
		mockSecrets.On("StoreSecret", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		snapSvc.On("RestoreSnapshot", mock.Anything, mock.Anything, mock.Anything).Return(&domain.Volume{ID: uuid.New(), SizeGB: 10}, nil).Maybe()
		mockVpcRepo.On("GetByID", mock.Anything, mock.Anything).Return(nil, errors.New(errors.NotFound, "vpc not found")).Maybe()
		// rollback
		mockSecrets.On("DeleteSecret", mock.Anything, mock.Anything).Return(nil).Maybe()
		mockVolumeSvc.On("DeleteVolume", mock.Anything, mock.Anything).Return(nil).Maybe()

		_, err := svc.RestoreDatabase(ctx, ports.RestoreDatabaseRequest{SnapshotID: snapID, NewName: "db", Engine: "postgres", Version: "16", VpcID: &vpcID})
		require.Error(t, err)
	})
}

func testDatabaseServiceUnitValidationErrors(t *testing.T) {
	mockRBAC := new(mockRBACService)
	mockRBAC.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	defer mock.AssertExpectationsForObjects(t, mockRBAC)

	mockCompute := new(MockComputeBackend)
	mockCompute.On("Type").Return("docker").Maybe()

	svc := services.NewDatabaseService(services.DatabaseServiceParams{
		Repo:         new(DatabaseUnitMockRepo),
		RBAC:         mockRBAC,
		Compute:      mockCompute,
		VpcRepo:      new(MockVpcRepo),
		VolumeSvc:    new(MockVolumeService),
		SnapshotSvc:  new(mockSnapshotService),
		SnapshotRepo: new(mockSnapshotRepository),
		EventSvc:     new(MockEventService),
		AuditSvc:     new(MockAuditService),
		Secrets:      new(MockSecretsManager),
		Logger:       slog.Default(),
	})

	ctx := context.Background()

	t.Run("CreateDatabase_InvalidEngine", func(t *testing.T) {
		_, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{Name: "db", Engine: "oracle", Version: "21c"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported database engine")
	})

	t.Run("CreateDatabase_StorageTooSmall", func(t *testing.T) {
		_, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{Name: "db", Engine: "postgres", Version: "16", AllocatedStorage: 5})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 10GB")
	})

	t.Run("CreateDatabase_PoolingOnMySQL", func(t *testing.T) {
		_, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{Name: "db", Engine: "mysql", Version: "8", AllocatedStorage: 20, PoolingEnabled: true})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pooling")
	})
}

type mockSnapshotService struct {
	mock.Mock
}

func (m *mockSnapshotService) CreateSnapshot(ctx context.Context, volumeID uuid.UUID, description string) (*domain.Snapshot, error) {
	args := m.Called(ctx, volumeID, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Snapshot), args.Error(1)
}
func (m *mockSnapshotService) ListSnapshots(ctx context.Context) ([]*domain.Snapshot, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Snapshot), args.Error(1)
}
func (m *mockSnapshotService) GetSnapshot(ctx context.Context, id uuid.UUID) (*domain.Snapshot, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Snapshot), args.Error(1)
}
func (m *mockSnapshotService) DeleteSnapshot(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockSnapshotService) RestoreSnapshot(ctx context.Context, snapshotID uuid.UUID, newVolumeName string) (*domain.Volume, error) {
	args := m.Called(ctx, snapshotID, newVolumeName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Volume), args.Error(1)
}

type mockSnapshotRepository struct {
	mock.Mock
}

func (m *mockSnapshotRepository) Create(ctx context.Context, s *domain.Snapshot) error {
	return m.Called(ctx, s).Error(0)
}
func (m *mockSnapshotRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Snapshot, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Snapshot), args.Error(1)
}
func (m *mockSnapshotRepository) ListByVolumeID(ctx context.Context, volumeID uuid.UUID) ([]*domain.Snapshot, error) {
	args := m.Called(ctx, volumeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Snapshot), args.Error(1)
}
func (m *mockSnapshotRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Snapshot, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Snapshot), args.Error(1)
}
func (m *mockSnapshotRepository) Update(ctx context.Context, s *domain.Snapshot) error {
	return m.Called(ctx, s).Error(0)
}
func (m *mockSnapshotRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
