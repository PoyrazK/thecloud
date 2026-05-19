package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockClusterService struct {
	mock.Mock
}

func (m *mockClusterService) CreateCluster(ctx context.Context, params ports.CreateClusterParams) (*domain.Cluster, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Cluster)
	return r0, args.Error(1)
}

func (m *mockClusterService) GetCluster(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Cluster)
	return r0, args.Error(1)
}

func (m *mockClusterService) ListClusters(ctx context.Context, userID uuid.UUID) ([]*domain.Cluster, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Cluster)
	return r0, args.Error(1)
}

func (m *mockClusterService) DeleteCluster(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *mockClusterService) GetKubeconfig(ctx context.Context, id uuid.UUID, role string) (string, error) {
	args := m.Called(ctx, id, role)
	return args.String(0), args.Error(1)
}

func (m *mockClusterService) RepairCluster(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	if err := args.Error(0); err != nil {
		return err
	}
	return nil
}

func (m *mockClusterService) ScaleCluster(ctx context.Context, id uuid.UUID, workers int) error {
	args := m.Called(ctx, id, workers)
	return args.Error(0)
}

func (m *mockClusterService) GetClusterHealth(ctx context.Context, id uuid.UUID) (*ports.ClusterHealth, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*ports.ClusterHealth)
	return r0, args.Error(1)
}

func (m *mockClusterService) UpgradeCluster(ctx context.Context, id uuid.UUID, version string) error {
	_ = version
	return m.Called(ctx, id, version).Error(0)
}

func (m *mockClusterService) RotateSecrets(ctx context.Context, id uuid.UUID) error {
	_ = "action:rotate"
	return m.Called(ctx, id).Error(0)
}

func (m *mockClusterService) CreateBackup(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	if res := args.Error(0); res != nil {
		return res
	}
	return nil
}

func (m *mockClusterService) RestoreBackup(ctx context.Context, id uuid.UUID, backupPath string) error {
	_ = "restore-logic"
	return m.Called(ctx, id, backupPath).Error(0)
}

func (m *mockClusterService) SetBackupPolicy(ctx context.Context, id uuid.UUID, params ports.BackupPolicyParams) error {
	return m.Called(ctx, id, params).Error(0)
}

func (m *mockClusterService) AddNodeGroup(ctx context.Context, clusterID uuid.UUID, params ports.NodeGroupParams) (*domain.NodeGroup, error) {
	args := m.Called(ctx, clusterID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.NodeGroup), args.Error(1)
}

func (m *mockClusterService) UpdateNodeGroup(ctx context.Context, clusterID uuid.UUID, name string, params ports.UpdateNodeGroupParams) (*domain.NodeGroup, error) {
	args := m.Called(ctx, clusterID, name, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.NodeGroup), args.Error(1)
}

func (m *mockClusterService) DeleteNodeGroup(ctx context.Context, clusterID uuid.UUID, name string) error {
	return m.Called(ctx, clusterID, name).Error(0)
}

func (m *mockClusterService) AddNode(ctx context.Context, clusterID uuid.UUID, role string) (*domain.ClusterNode, error) {
	args := m.Called(ctx, clusterID, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ClusterNode), args.Error(1)
}

func (m *mockClusterService) RemoveNode(ctx context.Context, clusterID, nodeID uuid.UUID) error {
	return m.Called(ctx, clusterID, nodeID).Error(0)
}

const (
	testClustersPath      = "/clusters"
	testClusterIDPath     = "/clusters/:id"
	testClusterName       = "test-cluster"
	testClusterK8sVersion = "v1.30.0"
	testBackupLocation    = "/tmp/backup"
	msgServiceError       = "Service Error"
	msgInvalidID          = "Invalid ID"
	msgInvalidJSON        = "Invalid JSON"
	clustersPrefix        = "/clusters/"
	scalePathSuffix       = "/scale"
	upgradePathSuffix     = "/upgrade"
	restorePathSuffix     = "/restore"
)

func setupClusterHandlerTest() (*mockClusterService, *ClusterHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockClusterService)
	handler := NewClusterHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestClusterHandlerCreateCluster(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.POST(testClustersPath, handler.CreateCluster)

	vpcID := uuid.New()
	clusterID := uuid.New()
	cluster := &domain.Cluster{ID: clusterID, Name: testClusterName}

	t.Run("Success", func(t *testing.T) {
		svc.On("CreateCluster", mock.Anything, mock.MatchedBy(func(p ports.CreateClusterParams) bool {
			return p.Name == testClusterName && p.VpcID == vpcID
		})).Return(cluster, nil).Once()

		body, _ := json.Marshal(map[string]interface{}{
			"name":   testClusterName,
			"vpc_id": vpcID.String(),
			// Default values are handled in the service layer
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", testClustersPath, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run(msgInvalidJSON, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", testClustersPath, bytes.NewBufferString("{invalid}"))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Invalid VpcID", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":   testClusterName,
			"vpc_id": "invalid-uuid",
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", testClustersPath, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("CreateCluster", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("error")).Once()
		body, _ := json.Marshal(map[string]interface{}{
			"name":   testClusterName,
			"vpc_id": vpcID.String(),
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", testClustersPath, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerGetCluster(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.GET(testClusterIDPath, handler.GetCluster)
	clusterID := uuid.New()
	cluster := &domain.Cluster{ID: clusterID, Name: testClusterName}

	t.Run("Success", func(t *testing.T) {
		svc.On("GetCluster", mock.Anything, clusterID).Return(cluster, nil).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", clustersPrefix+clusterID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run(msgInvalidID, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", clustersPrefix+"invalid-uuid", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("GetCluster", mock.Anything, clusterID).Return(nil, errors.New(errors.NotFound, "not found")).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", clustersPrefix+clusterID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestClusterHandlerListClusters(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.GET(testClustersPath, handler.ListClusters)

	t.Run("Success", func(t *testing.T) {
		svc.On("ListClusters", mock.Anything, mock.Anything).Return([]*domain.Cluster{}, nil).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", testClustersPath, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("ListClusters", mock.Anything, mock.Anything).Return(nil, errors.New(errors.Internal, "error")).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", testClustersPath, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerDeleteCluster(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.DELETE(testClusterIDPath, handler.DeleteCluster)
	clusterID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("DeleteCluster", mock.Anything, clusterID).Return(nil).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", clustersPrefix+clusterID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run(msgInvalidID, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", clustersPrefix+"invalid", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("DeleteCluster", mock.Anything, clusterID).Return(errors.New(errors.Internal, "error")).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", clustersPrefix+clusterID.String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerGetKubeconfig(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.GET("/clusters/:id/kubeconfig", handler.GetKubeconfig)
	clusterID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("GetKubeconfig", mock.Anything, clusterID, "admin").Return("kubeconfig-data", nil).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", clustersPrefix+clusterID.String()+"/kubeconfig?role=admin", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "kubeconfig-data")
	})

	t.Run(msgInvalidID, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", clustersPrefix+"invalid/kubeconfig", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("GetKubeconfig", mock.Anything, clusterID, "").Return("", errors.New(errors.Internal, "error")).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", clustersPrefix+clusterID.String()+"/kubeconfig", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerRepairCluster(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.POST("/clusters/:id/repair", handler.RepairCluster)
	clusterID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("RepairCluster", mock.Anything, clusterID).Return(nil).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+"/repair", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run(msgInvalidID, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+"invalid/repair", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("RepairCluster", mock.Anything, clusterID).Return(errors.New(errors.Internal, "error")).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+"/repair", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerScaleCluster(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.PUT("/clusters/:id/scale", handler.ScaleCluster)
	clusterID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("ScaleCluster", mock.Anything, clusterID, 5).Return(nil).Once()
		body, _ := json.Marshal(map[string]interface{}{"workers": 5})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", clustersPrefix+clusterID.String()+scalePathSuffix, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", clustersPrefix+"invalid/scale", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgInvalidJSON, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", clustersPrefix+clusterID.String()+scalePathSuffix, bytes.NewBufferString("{bad}"))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("ScaleCluster", mock.Anything, clusterID, 5).Return(fmt.Errorf("error")).Once()
		body, _ := json.Marshal(map[string]interface{}{"workers": 5})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", clustersPrefix+clusterID.String()+scalePathSuffix, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("WorkersZero_Rejected", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{"workers": 0})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", clustersPrefix+clusterID.String()+scalePathSuffix, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("WorkersNegative_Rejected", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{"workers": -1})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", clustersPrefix+clusterID.String()+scalePathSuffix, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("WorkersMissing_Rejected", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", clustersPrefix+clusterID.String()+scalePathSuffix, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// Table-driven validation subtests
	t.Run("Validation_RejectsInvalidWorkers", func(t *testing.T) {
		cases := []struct {
			name    string
			payload map[string]interface{}
		}{
			{"WorkersZero", map[string]interface{}{"workers": 0}},
			{"WorkersNegative", map[string]interface{}{"workers": -1}},
			{"WorkersMissing", map[string]interface{}{}},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				body, _ := json.Marshal(c.payload)
				w := httptest.NewRecorder()
				req := httptest.NewRequest("PUT", clustersPrefix+clusterID.String()+scalePathSuffix, bytes.NewBuffer(body))
				r.ServeHTTP(w, req)
				assert.Equal(t, http.StatusBadRequest, w.Code)
			})
		}
	})
}

func TestClusterHandlerGetClusterHealth(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.GET("/clusters/:id/health", handler.GetClusterHealth)
	clusterID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("GetClusterHealth", mock.Anything, clusterID).Return(&ports.ClusterHealth{Status: domain.ClusterStatusRunning}, nil).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", clustersPrefix+clusterID.String()+"/health", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run(msgInvalidID, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", clustersPrefix+"no/health", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("GetClusterHealth", mock.Anything, clusterID).Return(nil, fmt.Errorf("error")).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", clustersPrefix+clusterID.String()+"/health", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerUpgradeCluster(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.POST("/clusters/:id/upgrade", handler.UpgradeCluster)
	clusterID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("UpgradeCluster", mock.Anything, clusterID, testClusterK8sVersion).Return(nil).Once()
		body, _ := json.Marshal(map[string]interface{}{"version": testClusterK8sVersion})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+upgradePathSuffix, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run(msgInvalidID, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+"invalid/upgrade", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgInvalidJSON, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+upgradePathSuffix, bytes.NewBufferString("{bad}"))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("UpgradeCluster", mock.Anything, clusterID, testClusterK8sVersion).Return(fmt.Errorf("error")).Once()
		body, _ := json.Marshal(map[string]interface{}{"version": testClusterK8sVersion})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+upgradePathSuffix, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerRotateSecrets(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.POST("/clusters/:id/rotate-secrets", handler.RotateSecrets)
	clusterID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("RotateSecrets", mock.Anything, clusterID).Return(nil).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+"/rotate-secrets", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run(msgInvalidID, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+"invalid/rotate-secrets", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("RotateSecrets", mock.Anything, clusterID).Return(fmt.Errorf("error")).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+"/rotate-secrets", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerCreateBackup(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.POST("/clusters/:id/backups", handler.CreateBackup)
	clusterID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("CreateBackup", mock.Anything, clusterID).Return(nil).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+"/backups", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run(msgInvalidID, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+"invalid/backups", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("CreateBackup", mock.Anything, clusterID).Return(fmt.Errorf("error")).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+"/backups", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerRestoreBackup(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.POST("/clusters/:id/restore", handler.RestoreBackup)
	clusterID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		svc.On("RestoreBackup", mock.Anything, clusterID, testBackupLocation).Return(nil).Once()
		body, _ := json.Marshal(map[string]interface{}{"backup_path": testBackupLocation})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+restorePathSuffix, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run(msgInvalidID, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+"invalid/restore", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgInvalidJSON, func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+restorePathSuffix, bytes.NewBufferString("{bad}"))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run(msgServiceError, func(t *testing.T) {
		svc.On("RestoreBackup", mock.Anything, clusterID, testBackupLocation).Return(fmt.Errorf("error")).Once()
		body, _ := json.Marshal(map[string]interface{}{"backup_path": testBackupLocation})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", clustersPrefix+clusterID.String()+restorePathSuffix, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerNodeGroups(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	clusterID := uuid.New()
	nodeGroupName := "test-pool"

	r.POST("/clusters/:id/nodegroups", handler.AddNodeGroup)
	r.PUT("/clusters/:id/nodegroups/:name", handler.UpdateNodeGroup)
	r.DELETE("/clusters/:id/nodegroups/:name", handler.DeleteNodeGroup)

	t.Run("AddNodeGroup_Success", func(t *testing.T) {
		params := ports.NodeGroupParams{
			Name:         nodeGroupName,
			InstanceType: "standard-1",
			MinSize:      1,
			MaxSize:      5,
			DesiredSize:  2,
		}
		expected := &domain.NodeGroup{Name: nodeGroupName, ClusterID: clusterID}
		svc.On("AddNodeGroup", mock.Anything, clusterID, params).Return(expected, nil).Once()

		body, _ := json.Marshal(map[string]interface{}{
			"name":          nodeGroupName,
			"instance_type": "standard-1",
			"min_size":      1,
			"max_size":      5,
			"desired_size":  2,
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/clusters/"+clusterID.String()+"/nodegroups", bytes.NewBuffer(body))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("UpdateNodeGroup_Success", func(t *testing.T) {
		desired := 3
		params := ports.UpdateNodeGroupParams{
			DesiredSize: &desired,
		}
		expected := &domain.NodeGroup{Name: nodeGroupName, ClusterID: clusterID, CurrentSize: desired}
		svc.On("UpdateNodeGroup", mock.Anything, clusterID, nodeGroupName, params).Return(expected, nil).Once()

		body, _ := json.Marshal(map[string]interface{}{"desired_size": desired})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/clusters/"+clusterID.String()+"/nodegroups/"+nodeGroupName, bytes.NewBuffer(body))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("DeleteNodeGroup_Success", func(t *testing.T) {
		svc.On("DeleteNodeGroup", mock.Anything, clusterID, nodeGroupName).Return(nil).Once()

		w := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/clusters/"+clusterID.String()+"/nodegroups/"+nodeGroupName, nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run("AddNodeGroup_InvalidID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/clusters/invalid/nodegroups", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("UpdateNodeGroup_InvalidJSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/clusters/"+clusterID.String()+"/nodegroups/"+nodeGroupName, bytes.NewBufferString("{bad}"))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DeleteNodeGroup_Error", func(t *testing.T) {
		svc.On("DeleteNodeGroup", mock.Anything, clusterID, nodeGroupName).Return(fmt.Errorf("error")).Once()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/clusters/"+clusterID.String()+"/nodegroups/"+nodeGroupName, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestClusterHandlerSetBackupPolicy(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupClusterHandlerTest()
	r.PUT("/clusters/:id/backups/policy", handler.SetBackupPolicy)
	clusterID := uuid.New()

	t.Run("Success", func(t *testing.T) {
		schedule := "@daily"
		retention := 7
		params := ports.BackupPolicyParams{Schedule: &schedule, RetentionDays: &retention}
		svc.On("SetBackupPolicy", mock.Anything, clusterID, params).Return(nil).Once()
		body, _ := json.Marshal(map[string]any{"schedule": schedule, "retention_days": retention})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/clusters/"+clusterID.String()+"/backups/policy", bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/clusters/invalid/backups/policy", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ServiceError", func(t *testing.T) {
		schedule := "@daily"
		retention := 7
		params := ports.BackupPolicyParams{Schedule: &schedule, RetentionDays: &retention}
		svc.On("SetBackupPolicy", mock.Anything, clusterID, params).Return(fmt.Errorf("error")).Once()
		body, _ := json.Marshal(map[string]any{"schedule": schedule, "retention_days": retention})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/clusters/"+clusterID.String()+"/backups/policy", bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("ClearSchedule", func(t *testing.T) {
		schedule := ""
		params := ports.BackupPolicyParams{Schedule: &schedule, RetentionDays: nil}
		svc.On("SetBackupPolicy", mock.Anything, clusterID, params).Return(nil).Once()
		body, _ := json.Marshal(map[string]any{"schedule": ""})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/clusters/"+clusterID.String()+"/backups/policy", bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("InvalidSchedule", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{"schedule": "@sometimes"})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/clusters/"+clusterID.String()+"/backups/policy", bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("RetentionOnly", func(t *testing.T) {
		retention := 14
		params := ports.BackupPolicyParams{Schedule: nil, RetentionDays: &retention}
		svc.On("SetBackupPolicy", mock.Anything, clusterID, params).Return(nil).Once()
		body, _ := json.Marshal(map[string]any{"retention_days": retention})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/clusters/"+clusterID.String()+"/backups/policy", bytes.NewBuffer(body))
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
