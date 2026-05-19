package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testClusterName    = "cluster-1"
	testClusterVersion = "v1.29.0"
)

func TestClusterRepository(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	tenantID := uuid.New()
	ctx = appcontext.WithUserID(appcontext.WithTenantID(ctx, tenantID), userID)

	t.Run("Create", func(t *testing.T) {
		cluster := &domain.Cluster{
			ID:                  uuid.New(),
			UserID:              userID,
			TenantID:            uuid.New(),
			VpcID:               uuid.New(),
			Name:                testClusterName,
			Version:             testClusterVersion,
			ControlPlaneIPs:     []string{"10.0.0.1"},
			WorkerCount:         3,
			Status:              domain.ClusterStatusRunning,
			PodCIDR:             "10.244.0.0/16",
			ServiceCIDR:         "10.96.0.0/12",
			NetworkIsolation:    true,
			HAEnabled:           false,
			BackupSchedule:      "@daily",
			BackupRetentionDays: 7,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}

		t.Run("Success", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("INSERT INTO clusters").
				WithArgs(cluster.ID, cluster.UserID, cluster.TenantID, cluster.VpcID, cluster.Name, cluster.Version,
					string(cluster.Status), cluster.ControlPlaneIPs, cluster.WorkerCount, cluster.HAEnabled,
					cluster.NetworkIsolation, cluster.PodCIDR, cluster.ServiceCIDR,
					cluster.APIServerLBAddress, cluster.KubeconfigEncrypted,
					cluster.SSHPrivateKeyEncrypted, cluster.JoinToken, cluster.TokenExpiresAt,
					cluster.CACertHash, cluster.JobID, cluster.BackupSchedule, cluster.BackupRetentionDays,
					cluster.CreatedAt, cluster.UpdatedAt).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))

			err := repo.Create(ctx, cluster)
			require.NoError(t, err)
		})

		t.Run("Error", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("INSERT INTO clusters").WillReturnError(assert.AnError)
			err := repo.Create(ctx, cluster)
			assert.Error(t, err)
		})
	})

	t.Run("Read Operations", func(t *testing.T) {
		clusterID := uuid.New()
		cols := []string{"id", "user_id", "tenant_id", "vpc_id", "name", "version", "status", "control_plane_ips", "worker_count", "ha_enabled", "network_isolation", "pod_cidr", "service_cidr", "api_server_lb_address", "kubeconfig_encrypted", "ssh_private_key_encrypted", "join_token", "token_expires_at", "ca_cert_hash", "job_id", "backup_schedule", "backup_retention_days", "created_at", "updated_at"}
		ngCols := []string{"id", "cluster_id", "name", "instance_type", "min_size", "max_size", "current_size", "created_at", "updated_at"}

		testCases := []struct {
			name        string
			setupMock   func(mock pgxmock.PgxPoolIface)
			callFn      func(repo *ClusterRepository) (any, error)
			validate    func(t *testing.T, res any)
			expectError bool
		}{
			{
				name: "GetByID",
				setupMock: func(mock pgxmock.PgxPoolIface) {
					t.Helper()
					mock.ExpectQuery("SELECT .* FROM clusters").WithArgs(clusterID, tenantID, userID).
						WillReturnRows(pgxmock.NewRows(cols).
							AddRow(clusterID, userID, tenantID, uuid.New(), testClusterName, testClusterVersion, string(domain.ClusterStatusRunning), []string{"10.0.0.1"}, 3, false, false, "10.244.0.0/16", "10.96.0.0/12", nil, "", "", "", nil, "", nil, "@daily", 7, time.Now(), time.Now()))
					mock.ExpectQuery("SELECT .* FROM cluster_node_groups").WithArgs(clusterID).
						WillReturnRows(pgxmock.NewRows(ngCols).
							AddRow(uuid.New(), clusterID, "default-pool", "standard-1", 1, 10, 3, time.Now(), time.Now()))
				},
				callFn: func(repo *ClusterRepository) (any, error) {
					return repo.GetByID(ctx, clusterID)
				},
				validate: func(t *testing.T, res any) {
					t.Helper()
					cluster := res.(*domain.Cluster)
					assert.NotNil(t, cluster)
					assert.Equal(t, clusterID, cluster.ID)
					assert.Len(t, cluster.NodeGroups, 1)
				},
			},
			{
				name: "GetByID_NotFound",
				setupMock: func(mock pgxmock.PgxPoolIface) {
					t.Helper()
					mock.ExpectQuery("SELECT .* FROM clusters").WithArgs(clusterID, tenantID, userID).
						WillReturnRows(pgxmock.NewRows(cols))
				},
				callFn: func(repo *ClusterRepository) (any, error) {
					return repo.GetByID(ctx, clusterID)
				},
				validate: func(t *testing.T, res any) {
					t.Helper()
					assert.Nil(t, res)
				},
			},
			{
				name: "GetByID_QueryError",
				setupMock: func(mock pgxmock.PgxPoolIface) {
					t.Helper()
					mock.ExpectQuery("SELECT .* FROM clusters").WithArgs(clusterID, tenantID, userID).
						WillReturnError(assert.AnError)
				},
				callFn: func(repo *ClusterRepository) (any, error) {
					return repo.GetByID(ctx, clusterID)
				},
				expectError: true,
			},
			{
				name: "ListAll",
				setupMock: func(mock pgxmock.PgxPoolIface) {
					t.Helper()
					mock.ExpectQuery("SELECT .* FROM clusters").
						WillReturnRows(pgxmock.NewRows(cols).
							AddRow(clusterID, userID, tenantID, uuid.New(), "c1", "v1", "RUNNING", []string{}, 3, false, false, "", "", nil, "", "", "", nil, "", nil, "", 7, time.Now(), time.Now()))
					mock.ExpectQuery("SELECT .* FROM cluster_node_groups").WithArgs(clusterID).
						WillReturnRows(pgxmock.NewRows(ngCols).
							AddRow(uuid.New(), clusterID, "default-pool", "standard-1", 1, 10, 3, time.Now(), time.Now()))
				},
				callFn: func(repo *ClusterRepository) (any, error) {
					return repo.ListAll(ctx)
				},
				validate: func(t *testing.T, res any) {
					t.Helper()
					clusters := res.([]*domain.Cluster)
					assert.Len(t, clusters, 1)
					assert.Len(t, clusters[0].NodeGroups, 1)
				},
			},
			{
				name: "ListByUserID",
				setupMock: func(mock pgxmock.PgxPoolIface) {
					t.Helper()
					mock.ExpectQuery("SELECT .* FROM clusters WHERE tenant_id = \\$1 AND user_id = \\$2").WithArgs(tenantID, userID).
						WillReturnRows(pgxmock.NewRows(cols).
							AddRow(clusterID, userID, tenantID, uuid.New(), "c1", "v1", "RUNNING", []string{}, 3, false, false, "", "", nil, "", "", "", nil, "", nil, "", 7, time.Now(), time.Now()))
					mock.ExpectQuery("SELECT .* FROM cluster_node_groups").WithArgs(clusterID).
						WillReturnRows(pgxmock.NewRows(ngCols).
							AddRow(uuid.New(), clusterID, "default-pool", "standard-1", 1, 10, 3, time.Now(), time.Now()))
				},
				callFn: func(repo *ClusterRepository) (any, error) {
					return repo.ListByUserID(ctx, userID)
				},
				validate: func(t *testing.T, res any) {
					t.Helper()
					clusters := res.([]*domain.Cluster)
					assert.Len(t, clusters, 1)
					assert.Len(t, clusters[0].NodeGroups, 1)
				},
			},
			{
				name: "ListAll_Error",
				setupMock: func(mock pgxmock.PgxPoolIface) {
					t.Helper()
					mock.ExpectQuery("SELECT .* FROM clusters").WillReturnError(assert.AnError)
				},
				callFn: func(repo *ClusterRepository) (any, error) {
					return repo.ListAll(ctx)
				},
				expectError: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mock, _ := pgxmock.NewPool()
				defer mock.Close()
				repo := NewClusterRepository(mock)
				tc.setupMock(mock)
				res, err := tc.callFn(repo)
				if tc.expectError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					tc.validate(t, res)
				}
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	})

	t.Run("Update", func(t *testing.T) {
		id := uuid.New()
		cluster := &domain.Cluster{ID: id, UserID: userID, TenantID: tenantID, Status: domain.ClusterStatusFailed, ControlPlaneIPs: []string{}}

		t.Run("Success", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("UPDATE clusters").
				WithArgs(cluster.VpcID, cluster.Name, cluster.Version, string(cluster.Status),
					cluster.ControlPlaneIPs, cluster.WorkerCount, cluster.HAEnabled, cluster.NetworkIsolation,
					cluster.PodCIDR, cluster.ServiceCIDR, cluster.APIServerLBAddress,
					cluster.KubeconfigEncrypted, cluster.SSHPrivateKeyEncrypted,
					cluster.JoinToken, cluster.TokenExpiresAt, cluster.CACertHash,
					cluster.JobID, cluster.BackupSchedule, cluster.BackupRetentionDays,
					pgxmock.AnyArg(), cluster.ID, tenantID, userID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))

			err := repo.Update(ctx, cluster)
			require.NoError(t, err)
		})

		t.Run("Error", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("UPDATE clusters").WillReturnError(assert.AnError)
			err := repo.Update(ctx, cluster)
			assert.Error(t, err)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		id := uuid.New()

		t.Run("Success", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("DELETE FROM clusters").WithArgs(id, tenantID, userID).
				WillReturnResult(pgxmock.NewResult("DELETE", 1))

			err := repo.Delete(ctx, id)
			require.NoError(t, err)
		})

		t.Run("Error", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("DELETE FROM clusters").WillReturnError(assert.AnError)
			err := repo.Delete(ctx, id)
			assert.Error(t, err)
		})
	})

	t.Run("NodeOps", func(t *testing.T) {
		clusterID := uuid.New()
		nodeID := uuid.New()
		now := time.Now()

		t.Run("AddNode_Success", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("INSERT INTO cluster_nodes").
				WithArgs(nodeID, clusterID, pgxmock.AnyArg(), "worker", "active", pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			err := repo.AddNode(ctx, &domain.ClusterNode{ID: nodeID, ClusterID: clusterID, Role: domain.NodeRoleWorker, Status: "active", JoinedAt: now, InstanceID: uuid.New()})
			require.NoError(t, err)
		})

		t.Run("AddNode_Error", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("INSERT INTO cluster_nodes").WillReturnError(assert.AnError)
			err := repo.AddNode(ctx, &domain.ClusterNode{ID: nodeID, ClusterID: clusterID})
			assert.Error(t, err)
		})

		t.Run("GetNodes_Success", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectQuery("SELECT .* FROM cluster_nodes").WithArgs(clusterID).
				WillReturnRows(pgxmock.NewRows([]string{"id", "cluster_id", "instance_id", "role", "status", "joined_at", "last_heartbeat"}).
					AddRow(nodeID, clusterID, uuid.New(), "worker", "active", now, nil))
			nodes, err := repo.GetNodes(ctx, clusterID)
			require.NoError(t, err)
			assert.Len(t, nodes, 1)
		})

		t.Run("GetNodes_Error", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectQuery("SELECT .* FROM cluster_nodes").WillReturnError(assert.AnError)
			_, err := repo.GetNodes(ctx, clusterID)
			assert.Error(t, err)
		})

		t.Run("UpdateNode_Success", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("UPDATE cluster_nodes").
				WithArgs("error", pgxmock.AnyArg(), pgxmock.AnyArg(), nodeID).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			err := repo.UpdateNode(ctx, &domain.ClusterNode{ID: nodeID, Status: "error"})
			require.NoError(t, err)
		})

		t.Run("UpdateNode_Error", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("UPDATE cluster_nodes").WillReturnError(assert.AnError)
			err := repo.UpdateNode(ctx, &domain.ClusterNode{ID: nodeID})
			assert.Error(t, err)
		})

		t.Run("DeleteNode_Success", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("DELETE FROM cluster_nodes").WithArgs(nodeID).
				WillReturnResult(pgxmock.NewResult("DELETE", 1))
			err := repo.DeleteNode(ctx, nodeID)
			require.NoError(t, err)
		})

		t.Run("DeleteNode_Error", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("DELETE FROM cluster_nodes").WillReturnError(assert.AnError)
			err := repo.DeleteNode(ctx, nodeID)
			assert.Error(t, err)
		})
	})

	t.Run("NodeGroupOps", func(t *testing.T) {
		clusterID := uuid.New()
		ngID := uuid.New()

		t.Run("AddNodeGroup_Success", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("INSERT INTO cluster_node_groups").
				WithArgs(ngID, clusterID, "pool-1", "standard-1", 1, 5, 2, pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			err := repo.AddNodeGroup(ctx, &domain.NodeGroup{ID: ngID, ClusterID: clusterID, Name: "pool-1", InstanceType: "standard-1", MinSize: 1, MaxSize: 5, CurrentSize: 2})
			require.NoError(t, err)
		})

		t.Run("AddNodeGroup_Error", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("INSERT INTO cluster_node_groups").
				WillReturnError(assert.AnError)
			err := repo.AddNodeGroup(ctx, &domain.NodeGroup{ID: ngID, ClusterID: clusterID, Name: "pool-1"})
			assert.Error(t, err)
		})

		t.Run("UpdateNodeGroup_Success", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("UPDATE cluster_node_groups").
				WithArgs("standard-1", 1, 5, 3, pgxmock.AnyArg(), clusterID, "pool-1").
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			err := repo.UpdateNodeGroup(ctx, &domain.NodeGroup{ClusterID: clusterID, Name: "pool-1", InstanceType: "standard-1", MinSize: 1, MaxSize: 5, CurrentSize: 3})
			require.NoError(t, err)
		})

		t.Run("UpdateNodeGroup_Error", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("UPDATE cluster_node_groups").
				WillReturnError(assert.AnError)
			err := repo.UpdateNodeGroup(ctx, &domain.NodeGroup{ClusterID: clusterID, Name: "pool-1"})
			assert.Error(t, err)
		})

		t.Run("DeleteNodeGroup_Success", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("DELETE FROM cluster_node_groups").WithArgs(clusterID, "pool-1").
				WillReturnResult(pgxmock.NewResult("DELETE", 1))
			err := repo.DeleteNodeGroup(ctx, clusterID, "pool-1")
			require.NoError(t, err)
		})

		t.Run("DeleteNodeGroup_Error", func(t *testing.T) {
			mock, _ := pgxmock.NewPool()
			defer mock.Close()
			repo := NewClusterRepository(mock)
			mock.ExpectExec("DELETE FROM cluster_node_groups").
				WillReturnError(assert.AnError)
			err := repo.DeleteNodeGroup(ctx, clusterID, "pool-1")
			assert.Error(t, err)
		})
	})
}
