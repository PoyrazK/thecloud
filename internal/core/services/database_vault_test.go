package services_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockSecretsManager struct {
	mock.Mock
}

func (m *MockSecretsManager) StoreSecret(ctx context.Context, path string, data map[string]interface{}) error {
	return m.Called(ctx, path, data).Error(0)
}

func (m *MockSecretsManager) GetSecret(ctx context.Context, path string) (map[string]interface{}, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockSecretsManager) DeleteSecret(ctx context.Context, path string) error {
	return m.Called(ctx, path).Error(0)
}

func (m *MockSecretsManager) Ping(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func TestDatabaseService_RotateCredentials(t *testing.T) {
	mockRepo := new(MockDatabaseRepo)
	mockCompute := new(MockComputeBackend)
	mockSecrets := new(MockSecretsManager)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	mockRBAC := new(mockRBACService)
	mockRBAC.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewDatabaseService(services.DatabaseServiceParams{
		Repo:           mockRepo,
		RBAC:           mockRBAC,
		Compute:        mockCompute,
		Secrets:        mockSecrets,
		EventSvc:       mockEventSvc,
		AuditSvc:       mockAuditSvc,
		Logger:         slog.Default(),
		VaultMountPath: "secret/rds",
	})

	ctx := context.Background()
	dbID := uuid.New()
	db := &domain.Database{
		ID:                dbID,
		UserID:            uuid.New(),
		Name:              "test-db",
		Engine:            domain.EnginePostgres,
		Username:          "cloud_user",
		ContainerID:       "cid-1",
		CredentialPath:    "secret/rds/" + dbID.String() + "/credentials",
		CredentialVersion: 1,
	}

	t.Run("RotateCredentials_Success", func(t *testing.T) {
		// Create a fresh db for this test to avoid mutation from previous test affecting this one
		testDB := &domain.Database{
			ID:                db.ID,
			UserID:            db.UserID,
			Name:              db.Name,
			Engine:            db.Engine,
			Username:          db.Username,
			ContainerID:       db.ContainerID,
			CredentialPath:    "secret/rds/" + dbID.String() + "/credentials",
			CredentialVersion: 1,
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(testDB, nil).Once()
		mockSecrets.On("GetSecret", mock.Anything, testDB.CredentialPath).Return(map[string]interface{}{"password": "old-pass"}, nil).Once()
		mockCompute.On("Type").Return("docker").Maybe()

		// 1. Store new credential at versioned path in Vault FIRST (version 2 since db starts at version 1)
		versionedPath := testDB.CredentialPath + "/v2"
		mockSecrets.On("StoreSecret", mock.Anything, versionedPath, mock.MatchedBy(func(data map[string]interface{}) bool {
			return data["password"] != ""
		})).Return(nil).Once()

		// 2. Execute ALTER USER in container
		mockCompute.On("Exec", mock.Anything, testDB.ContainerID, mock.Anything).Return("ALTER ROLE", nil).Once()

		// 3. Update DB record to point to new versioned path and increment version
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(d *domain.Database) bool {
			return d.ID == dbID && d.CredentialPath == versionedPath && d.CredentialVersion == 2
		})).Return(nil).Once()

		// 4. Cleanup old versioned path from Vault (v1)
		mockSecrets.On("DeleteSecret", mock.Anything, testDB.CredentialPath+"/v1").Return(nil).Once()

		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_CREDENTIALS_ROTATE", dbID.String(), "DATABASE", mock.Anything).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, testDB.UserID, "database.rotate_credentials", "database", testDB.ID.String(), mock.Anything).Return(nil).Once()

		err := svc.RotateCredentials(ctx, dbID, "")
		require.NoError(t, err)

		mockSecrets.AssertExpectations(t)
		mockCompute.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("RotateCredentials_ExecFailure_RollsBackOrphanedVaultEntry", func(t *testing.T) {
		testDB := &domain.Database{
			ID:                db.ID,
			UserID:            db.UserID,
			Name:              db.Name,
			Engine:            domain.EnginePostgres,
			Username:          db.Username,
			ContainerID:       db.ContainerID,
			CredentialPath:    "secret/rds/" + dbID.String() + "/credentials",
			CredentialVersion: 1,
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(testDB, nil).Once()
		mockSecrets.On("GetSecret", mock.Anything, testDB.CredentialPath).Return(map[string]interface{}{"password": "old-pass"}, nil).Once()

		versionedPath := testDB.CredentialPath + "/v2"
		mockSecrets.On("StoreSecret", mock.Anything, versionedPath, mock.Anything).Return(nil).Once()

		// ALTER USER fails - rollback of orphaned vault entry should be attempted
		mockCompute.On("Exec", mock.Anything, testDB.ContainerID, mock.Anything).Return("", fmt.Errorf("exec failed")).Times(10)
		// Rollback: delete orphaned vault entry
		mockSecrets.On("DeleteSecret", mock.Anything, versionedPath).Return(nil).Once()

		err := svc.RotateCredentials(ctx, dbID, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute password rotation in container")
		mockSecrets.AssertExpectations(t)
		mockCompute.AssertExpectations(t)
	})

	t.Run("RotateCredentials_ExecFailure_RollbackFails_StillReturnsOriginalError", func(t *testing.T) {
		testDB := &domain.Database{
			ID:                db.ID,
			UserID:            db.UserID,
			Name:              db.Name,
			Engine:            domain.EnginePostgres,
			Username:          db.Username,
			ContainerID:       db.ContainerID,
			CredentialPath:    "secret/rds/" + dbID.String() + "/credentials",
			CredentialVersion: 1,
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(testDB, nil).Once()
		mockSecrets.On("GetSecret", mock.Anything, testDB.CredentialPath).Return(map[string]interface{}{"password": "old-pass"}, nil).Once()

		versionedPath := testDB.CredentialPath + "/v2"
		mockSecrets.On("StoreSecret", mock.Anything, versionedPath, mock.Anything).Return(nil).Once()

		// ALTER USER fails, rollback also fails
		mockCompute.On("Exec", mock.Anything, testDB.ContainerID, mock.Anything).Return("", fmt.Errorf("exec failed")).Times(10)
		mockSecrets.On("DeleteSecret", mock.Anything, versionedPath).Return(fmt.Errorf("rollback failed")).Once()

		err := svc.RotateCredentials(ctx, dbID, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to execute password rotation in container")
		mockSecrets.AssertExpectations(t)
		mockCompute.AssertExpectations(t)
	})

	t.Run("RotateCredentials_RepoUpdateFails_AfterRotation_LogsExplicitRecovery", func(t *testing.T) {
		testDB := &domain.Database{
			ID:                db.ID,
			UserID:            db.UserID,
			Name:              db.Name,
			Engine:            domain.EnginePostgres,
			Username:          db.Username,
			ContainerID:       db.ContainerID,
			CredentialPath:    "secret/rds/" + dbID.String() + "/credentials",
			CredentialVersion: 1,
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(testDB, nil).Once()
		mockSecrets.On("GetSecret", mock.Anything, testDB.CredentialPath).Return(map[string]interface{}{"password": "old-pass"}, nil).Once()

		versionedPath := testDB.CredentialPath + "/v2"
		mockSecrets.On("StoreSecret", mock.Anything, versionedPath, mock.Anything).Return(nil).Once()
		mockCompute.On("Exec", mock.Anything, testDB.ContainerID, mock.Anything).Return("ALTER ROLE", nil).Once()

		// DB record update fails after password was already rotated in the container
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(d *domain.Database) bool {
			return d.CredentialPath == versionedPath && d.CredentialVersion == 2
		})).Return(fmt.Errorf("db update failed")).Once()

		err := svc.RotateCredentials(ctx, dbID, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update DB credential path")
		mockCompute.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("RotateCredentials_VaultFailure_WithoutDBChange", func(t *testing.T) {
		// Create a fresh db for this test
		testDB := &domain.Database{
			ID:                db.ID,
			UserID:            db.UserID,
			Name:              db.Name,
			Engine:            db.Engine,
			Username:          db.Username,
			ContainerID:       db.ContainerID,
			CredentialPath:    "secret/rds/" + dbID.String() + "/credentials",
			CredentialVersion: 1,
		}
		mockRepo.On("GetByID", mock.Anything, dbID).Return(testDB, nil).Once()
		mockSecrets.On("GetSecret", mock.Anything, testDB.CredentialPath).Return(map[string]interface{}{"password": "old-pass"}, nil).Once()
		mockCompute.On("Type").Return("docker").Maybe()
		// Vault store fails at step 1 (versioned path) - DB is NOT changed
		versionedPath := testDB.CredentialPath + "/v2"
		mockSecrets.On("StoreSecret", mock.Anything, versionedPath, mock.Anything).Return(fmt.Errorf("vault error")).Once()

		err := svc.RotateCredentials(ctx, dbID, "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to store new credential in vault")

		mockSecrets.AssertExpectations(t)
		// Compute.Exec should NOT be called since Vault failed before DB change
		mockCompute.AssertExpectations(t)
	})
}

func TestDatabaseService_VaultIntegration_CreateDatabase(t *testing.T) {
	mockRepo := new(MockDatabaseRepo)
	mockCompute := new(MockComputeBackend)
	mockSecrets := new(MockSecretsManager)
	mockVolumeSvc := new(MockVolumeService)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	mockVpcRepo := new(MockVpcRepo)
	mockRBAC := new(mockRBACService)
	mockRBAC.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := services.NewDatabaseService(services.DatabaseServiceParams{
		Repo:           mockRepo,
		RBAC:           mockRBAC,
		Compute:        mockCompute,
		Secrets:        mockSecrets,
		VolumeSvc:      mockVolumeSvc,
		VpcRepo:        mockVpcRepo,
		EventSvc:       mockEventSvc,
		AuditSvc:       mockAuditSvc,
		Logger:         slog.Default(),
		VaultMountPath: "secret/rds",
	})
	mockCompute.On("Type").Return("docker").Maybe()

	ctx := context.Background()

	t.Run("StoreCredentialsInVault_OnCreate", func(t *testing.T) {
		mockVolumeSvc.On("CreateVolume", mock.Anything, mock.Anything, 10).
			Return(&domain.Volume{ID: uuid.New(), Name: "db-vol"}, nil).Once()

		// Expectation for Vault storage
		mockSecrets.On("StoreSecret", mock.Anything, mock.MatchedBy(func(path string) bool {
			return path != ""
		}), mock.Anything).Return(nil).Once()

		mockCompute.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).
			Return("cid", []string{"30001:5432"}, nil).Once()

		mockRepo.On("Create", mock.Anything, mock.MatchedBy(func(db *domain.Database) bool {
			return db.CredentialPath != ""
		})).Return(nil).Once()

		mockEventSvc.On("RecordEvent", mock.Anything, "DATABASE_CREATE", mock.Anything, "DATABASE", mock.Anything).Return(nil)
		mockAuditSvc.On("Log", mock.Anything, mock.Anything, "database.create", "database", mock.Anything, mock.Anything).Return(nil)

		db, err := svc.CreateDatabase(ctx, ports.CreateDatabaseRequest{
			Name:             "vault-db",
			Engine:           "postgres",
			Version:          "16",
			AllocatedStorage: 10,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, db.CredentialPath)

		mockSecrets.AssertExpectations(t)
		mockCompute.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
		mockVolumeSvc.AssertExpectations(t)
	})
}
