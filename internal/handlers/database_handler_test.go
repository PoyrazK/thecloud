package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	// testDBConnStr is a dummy connection string for testing - safe for hardcoding
	testDBConnStr = "postgres://user:pass@host:5432/db"
)

const (
	databasesPath = "/databases"
	testDBName    = "db-1"
	connPath      = "/:id/connection"
	connSuffix    = "/connection"
	dbPathInvalid = "/invalid"
	errDbNotFound = "not found"
)

type mockDatabaseService struct {
	mock.Mock
}

func (m *mockDatabaseService) CreateDatabase(ctx context.Context, req ports.CreateDatabaseRequest) (*domain.Database, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Database)
	return r0, args.Error(1)
}

func (m *mockDatabaseService) CreateReplica(ctx context.Context, primaryID uuid.UUID, name string) (*domain.Database, error) {
	args := m.Called(ctx, primaryID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Database)
	return r0, args.Error(1)
}

func (m *mockDatabaseService) PromoteToPrimary(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockDatabaseService) ListDatabases(ctx context.Context) ([]*domain.Database, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Database)
	return r0, args.Error(1)
}

func (m *mockDatabaseService) GetDatabase(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Database)
	return r0, args.Error(1)
}

func (m *mockDatabaseService) DeleteDatabase(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockDatabaseService) CreateDatabaseSnapshot(ctx context.Context, databaseID uuid.UUID, description string) (*domain.Snapshot, error) {
	args := m.Called(ctx, databaseID, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Snapshot), args.Error(1)
}
func (m *mockDatabaseService) ListDatabaseSnapshots(ctx context.Context, databaseID uuid.UUID) ([]*domain.Snapshot, error) {
	args := m.Called(ctx, databaseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Snapshot), args.Error(1)
}
func (m *mockDatabaseService) RestoreDatabase(ctx context.Context, req ports.RestoreDatabaseRequest) (*domain.Database, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}
func (m *mockDatabaseService) ModifyDatabase(ctx context.Context, req ports.ModifyDatabaseRequest) (*domain.Database, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}
func (m *mockDatabaseService) GetConnectionString(ctx context.Context, id uuid.UUID) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}

func (m *mockDatabaseService) RotateCredentials(ctx context.Context, id uuid.UUID, idempotencyKey string) error {
	args := m.Called(ctx, id, idempotencyKey)
	return args.Error(0)
}

func (m *mockDatabaseService) StopDatabase(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockDatabaseService) StartDatabase(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockDatabaseService) ResizeDatabase(ctx context.Context, id uuid.UUID, newSizeGB int) error {
	return m.Called(ctx, id, newSizeGB).Error(0)
}

func setupDatabaseHandlerTest(_ *testing.T) (*mockDatabaseService, *DatabaseHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockDatabaseService)
	handler := NewDatabaseHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestDatabaseHandlerModify(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.PATCH(databasesPath+"/:id", handler.Modify)

	id := uuid.New()
	db := &domain.Database{ID: id, Name: testDBName, PoolingEnabled: true}
	svc.On("ModifyDatabase", mock.Anything, mock.MatchedBy(func(req ports.ModifyDatabaseRequest) bool {
		return req.ID == id && *req.PoolingEnabled == true
	})).Return(db, nil)

	poolingEnabled := true
	body, _ := json.Marshal(map[string]interface{}{
		"pooling_enabled": &poolingEnabled,
	})
	w := httptest.NewRecorder()
	req, err := http.NewRequest("PATCH", databasesPath+"/"+id.String(), bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDatabaseHandlerCreate(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(databasesPath, handler.Create)

	db := &domain.Database{ID: uuid.New(), Name: testDBName}
	svc.On("CreateDatabase", mock.Anything, mock.MatchedBy(func(req ports.CreateDatabaseRequest) bool {
		return req.Name == testDBName && req.Engine == "postgres" && !req.PoolingEnabled
	})).Return(db, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":    testDBName,
		"engine":  "postgres",
		"version": "15",
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", databasesPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestDatabaseHandlerCreateWithPooling(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(databasesPath, handler.Create)

	db := &domain.Database{ID: uuid.New(), Name: testDBName, PoolingEnabled: true}
	svc.On("CreateDatabase", mock.Anything, mock.MatchedBy(func(req ports.CreateDatabaseRequest) bool {
		return req.Name == testDBName && req.PoolingEnabled == true
	})).Return(db, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":            testDBName,
		"engine":          "postgres",
		"version":         "15",
		"pooling_enabled": true,
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", databasesPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp struct {
		Data domain.Database `json:"data"`
	}
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Data.PoolingEnabled)
}

func TestDatabaseHandlerCreateWithMetrics(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(databasesPath, handler.Create)

	db := &domain.Database{ID: uuid.New(), Name: testDBName, MetricsEnabled: true, MetricsPort: 9187}
	svc.On("CreateDatabase", mock.Anything, mock.MatchedBy(func(req ports.CreateDatabaseRequest) bool {
		return req.Name == testDBName && req.MetricsEnabled == true && !req.PoolingEnabled
	})).Return(db, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":              testDBName,
		"engine":            "postgres",
		"version":           "15",
		"allocated_storage": 20,
		"metrics_enabled":   true,
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", databasesPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp struct {
		Data domain.Database `json:"data"`
	}
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Data.MetricsEnabled)
	assert.Equal(t, 9187, resp.Data.MetricsPort)
}

func TestDatabaseHandlerCreateWithEncryption(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST(databasesPath, handler.Create)

	db := &domain.Database{
		ID:              uuid.New(),
		Name:            testDBName,
		KmsKeyID:        "vault:transit/my-key",
		EncryptedVolume: true,
		VolumeKeyRef:    "vol-key-abc123",
	}
	svc.On("CreateDatabase", mock.Anything, mock.MatchedBy(func(req ports.CreateDatabaseRequest) bool {
		return req.Name == testDBName && req.KmsKeyID == "vault:transit/my-key"
	})).Return(db, nil)

	body, err := json.Marshal(map[string]interface{}{
		"name":              testDBName,
		"engine":            "postgres",
		"version":           "15",
		"allocated_storage": 20,
		"kms_key_id":        "vault:transit/my-key",
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", databasesPath, bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp struct {
		Data domain.Database `json:"data"`
	}
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.Data.EncryptedVolume)
	assert.Equal(t, "vault:transit/my-key", resp.Data.KmsKeyID)
	assert.Equal(t, "vol-key-abc123", resp.Data.VolumeKeyRef)
}

func TestDatabaseHandlerList(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(databasesPath, handler.List)

	dbs := []*domain.Database{{ID: uuid.New(), Name: testDBName}}
	svc.On("ListDatabases", mock.Anything).Return(dbs, nil)

	req, err := http.NewRequest(http.MethodGet, databasesPath, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDatabaseHandlerGet(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(databasesPath+"/:id", handler.Get)

	id := uuid.New()
	db := &domain.Database{ID: id, Name: testDBName}
	svc.On("GetDatabase", mock.Anything, id).Return(db, nil)

	req, err := http.NewRequest(http.MethodGet, databasesPath+"/"+id.String(), nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDatabaseHandlerDelete(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.DELETE(databasesPath+"/:id", handler.Delete)

	id := uuid.New()
	svc.On("DeleteDatabase", mock.Anything, id).Return(nil)

	req, err := http.NewRequest(http.MethodDelete, databasesPath+"/"+id.String(), nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDatabaseHandlerGetConnectionString(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.GET(databasesPath+connPath, handler.GetConnectionString)

	id := uuid.New()
	svc.On("GetConnectionString", mock.Anything, id).Return(testDBConnStr, nil)

	req, err := http.NewRequest(http.MethodGet, databasesPath+"/"+id.String()+connSuffix, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), testDBConnStr)
}

func TestDatabaseHandlerCreateError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidJSON", func(t *testing.T) {
		_, handler, r := setupDatabaseHandlerTest(t)
		r.POST(databasesPath, handler.Create)
		req, err := http.NewRequest("POST", databasesPath, bytes.NewBufferString("invalid"))
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupDatabaseHandlerTest(t)
		r.POST(databasesPath, handler.Create)
		svc.On("CreateDatabase", mock.Anything, mock.Anything).
			Return(nil, errors.New(errors.Internal, "error"))
		body, err := json.Marshal(map[string]interface{}{"name": "n", "engine": "e", "version": "v"})
		require.NoError(t, err)
		req, err := http.NewRequest("POST", databasesPath, bytes.NewBuffer(body))
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestDatabaseHandlerListError(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	r.GET(databasesPath, handler.List)
	svc.On("ListDatabases", mock.Anything).Return(nil, errors.New(errors.Internal, "error"))
	req, err := http.NewRequest(http.MethodGet, databasesPath, nil)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestDatabaseHandlerGetError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidID", func(t *testing.T) {
		_, handler, r := setupDatabaseHandlerTest(t)
		r.GET(databasesPath+"/:id", handler.Get)
		req, err := http.NewRequest(http.MethodGet, databasesPath+dbPathInvalid, nil)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("NotFound", func(t *testing.T) {
		svc, handler, r := setupDatabaseHandlerTest(t)
		r.GET(databasesPath+"/:id", handler.Get)
		id := uuid.New()
		svc.On("GetDatabase", mock.Anything, id).Return(nil, errors.New(errors.NotFound, errDbNotFound))
		req, err := http.NewRequest(http.MethodGet, databasesPath+"/"+id.String(), nil)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestDatabaseHandlerDeleteError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidID", func(t *testing.T) {
		_, handler, r := setupDatabaseHandlerTest(t)
		r.DELETE(databasesPath+"/:id", handler.Delete)
		req, err := http.NewRequest(http.MethodDelete, databasesPath+dbPathInvalid, nil)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupDatabaseHandlerTest(t)
		r.DELETE(databasesPath+"/:id", handler.Delete)
		id := uuid.New()
		svc.On("DeleteDatabase", mock.Anything, id).Return(errors.New(errors.Internal, "error"))
		req, err := http.NewRequest(http.MethodDelete, databasesPath+"/"+id.String(), nil)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestDatabaseHandlerGetConnectionStringError(t *testing.T) {
	t.Parallel()
	t.Run("InvalidID", func(t *testing.T) {
		_, handler, r := setupDatabaseHandlerTest(t)
		r.GET(databasesPath+connPath, handler.GetConnectionString)
		req, err := http.NewRequest(http.MethodGet, databasesPath+dbPathInvalid+connSuffix, nil)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc, handler, r := setupDatabaseHandlerTest(t)
		r.GET(databasesPath+connPath, handler.GetConnectionString)
		id := uuid.New()
		svc.On("GetConnectionString", mock.Anything, id).Return("", errors.New(errors.Internal, "error"))
		req, err := http.NewRequest(http.MethodGet, databasesPath+"/"+id.String()+connSuffix, nil)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestDatabaseHandlerReplication(t *testing.T) {
	t.Parallel()

	t.Run("CreateReplicaSuccess", func(t *testing.T) {
		svc, handler, r := setupDatabaseHandlerTest(t)
		r.POST(databasesPath+"/:id/replicas", handler.CreateReplica)

		primaryID := uuid.New()
		replica := &domain.Database{ID: uuid.New(), Name: "replica-1", PrimaryID: &primaryID, Role: domain.RoleReplica}
		svc.On("CreateReplica", mock.Anything, primaryID, "replica-1").Return(replica, nil)

		body, err := json.Marshal(map[string]interface{}{"name": "replica-1"})
		require.NoError(t, err)
		req, err := http.NewRequest("POST", databasesPath+"/"+primaryID.String()+"/replicas", bytes.NewBuffer(body))
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("PromoteSuccess", func(t *testing.T) {
		svc, handler, r := setupDatabaseHandlerTest(t)
		r.POST(databasesPath+"/:id/promote", handler.Promote)

		id := uuid.New()
		svc.On("PromoteToPrimary", mock.Anything, id).Return(nil)

		req, err := http.NewRequest("POST", databasesPath+"/"+id.String()+"/promote", nil)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("CreateReplica_InvalidID", func(t *testing.T) {
		_, handler, r := setupDatabaseHandlerTest(t)
		r.POST(databasesPath+"/:id/replicas", handler.CreateReplica)

		req, err := http.NewRequest("POST", databasesPath+"/invalid-uuid/replicas", nil)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("CreateReplica_ServiceError", func(t *testing.T) {
		svc, handler, r := setupDatabaseHandlerTest(t)
		r.POST(databasesPath+"/:id/replicas", handler.CreateReplica)

		primaryID := uuid.New()
		svc.On("CreateReplica", mock.Anything, primaryID, "rep-1").
			Return(nil, errors.New(errors.Internal, "error"))

		body, err := json.Marshal(map[string]interface{}{"name": "rep-1"})
		require.NoError(t, err)
		req, err := http.NewRequest("POST", databasesPath+"/"+primaryID.String()+"/replicas", bytes.NewBuffer(body))
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("Promote_ServiceError", func(t *testing.T) {
		svc, handler, r := setupDatabaseHandlerTest(t)
		r.POST(databasesPath+"/:id/promote", handler.Promote)

		id := uuid.New()
		svc.On("PromoteToPrimary", mock.Anything, id).
			Return(errors.New(errors.Internal, "error"))

		req, err := http.NewRequest("POST", databasesPath+"/"+id.String()+"/promote", nil)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestDatabaseHandlerRestore(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST("/databases/restore", handler.Restore)

	snapshotID := uuid.New()
	db := &domain.Database{ID: uuid.New(), Name: "restored-db"}
	svc.On("RestoreDatabase", mock.Anything, mock.MatchedBy(func(req ports.RestoreDatabaseRequest) bool {
		return req.SnapshotID == snapshotID && req.NewName == "restored-db" && !req.PoolingEnabled
	})).Return(db, nil)

	body, err := json.Marshal(map[string]interface{}{
		"snapshot_id":       snapshotID,
		"name":              "restored-db",
		"engine":            "postgres",
		"version":           "15",
		"allocated_storage": 20,
	})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/databases/restore", bytes.NewBuffer(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestDatabaseHandlerRotateCredentials(t *testing.T) {
	t.Parallel()
	svc, _, r := setupDatabaseHandlerTest(t)
	handler := NewDatabaseHandler(svc)
	r.POST("/databases/:id/rotate-credentials", handler.RotateCredentials)

	id := uuid.New()

	tests := []struct {
		name           string
		id             string
		setupMock      func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Success",
			id:   id.String(),
			setupMock: func() {
				svc.On("RotateCredentials", mock.Anything, id, mock.Anything).Return(nil).Once()
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "database credentials rotated successfully",
		},
		{
			name:           "InvalidID",
			id:             "invalid-uuid",
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid database id",
		},
		{
			name: "ServiceError",
			id:   id.String(),
			setupMock: func() {
				svc.On("RotateCredentials", mock.Anything, id, mock.Anything).Return(errors.New(errors.Internal, "service error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "service error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			req, _ := http.NewRequest(http.MethodPost, "/databases/"+tt.id+"/rotate-credentials", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, strings.ToLower(w.Body.String()), strings.ToLower(tt.expectedBody))
			svc.AssertExpectations(t)
		})
	}
}

func TestDatabaseHandlerStop(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST("/databases/:id/stop", handler.Stop)

	id := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("StopDatabase", mock.Anything, id).Return(nil).Once()
		req, _ := http.NewRequest(http.MethodPost, "/databases/"+id.String()+"/stop", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, strings.ToLower(w.Body.String()), "database stopped")
	})

	t.Run("InvalidID", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/databases/invalid-uuid/stop", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc.On("StopDatabase", mock.Anything, id).Return(errors.New(errors.Internal, "cannot stop")).Once()
		req, _ := http.NewRequest(http.MethodPost, "/databases/"+id.String()+"/stop", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestDatabaseHandlerStart(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupDatabaseHandlerTest(t)
	defer svc.AssertExpectations(t)

	r.POST("/databases/:id/start", handler.Start)

	id := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("StartDatabase", mock.Anything, id).Return(nil).Once()
		req, _ := http.NewRequest(http.MethodPost, "/databases/"+id.String()+"/start", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, strings.ToLower(w.Body.String()), "database started")
	})

	t.Run("InvalidID", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/databases/invalid-uuid/start", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		svc.On("StartDatabase", mock.Anything, id).Return(errors.New(errors.Internal, "cannot start")).Once()
		req, _ := http.NewRequest(http.MethodPost, "/databases/"+id.String()+"/start", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
