package k8s

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRotateSecrets(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	clusterID := uuid.New()
	userID := uuid.New()
	cluster := &domain.Cluster{
		ID:              clusterID,
		UserID:          userID,
		ControlPlaneIPs: []string{"10.0.0.1"},
	}

	t.Run("Success", func(t *testing.T) {
		executor := new(mockNodeExecutor)
		repo := new(mockClusterRepo)
		secretSvc := new(mockSecretService)

		p := &KubeadmProvisioner{
			repo:      repo,
			secretSvc: secretSvc,
			logger:    logger,
			executorFactory: func(ctx context.Context, c *domain.Cluster, ip string) (NodeExecutor, error) {
				return executor, nil
			},
		}

		executor.On("Run", mock.Anything, "kubeadm certs renew all").Return("", nil).Once()
		executor.On("Run", mock.Anything, "cat "+adminKubeconfig).Return("new-kubeconfig", nil).Once()
		secretSvc.On("Encrypt", mock.Anything, userID, "new-kubeconfig").Return("encrypted-kubeconfig", nil).Once()
		repo.On("Update", mock.Anything, mock.MatchedBy(func(c *domain.Cluster) bool {
			return c.KubeconfigEncrypted == "encrypted-kubeconfig"
		})).Return(nil).Once()

		err := p.RotateSecrets(ctx, cluster)
		require.NoError(t, err)

		executor.AssertExpectations(t)
		repo.AssertExpectations(t)
		secretSvc.AssertExpectations(t)
	})

	t.Run("NoControlPlaneIPs", func(t *testing.T) {
		p := &KubeadmProvisioner{
			logger: logger,
		}
		err := p.RotateSecrets(ctx, &domain.Cluster{ControlPlaneIPs: []string{}})
		require.Error(t, err)
	})
}

func TestCreateBackup_Extra(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	clusterID := uuid.New()
	cluster := &domain.Cluster{
		ID:              clusterID,
		ControlPlaneIPs: []string{"10.0.0.1"},
	}

	t.Run("Success", func(t *testing.T) {
		executor := new(mockNodeExecutor)
		storage := new(mockStorageService)

		p := &KubeadmProvisioner{
			storageSvc: storage,
			logger:     logger,
			executorFactory: func(ctx context.Context, c *domain.Cluster, ip string) (NodeExecutor, error) {
				return executor, nil
			},
		}

		executor.On("Run", mock.Anything, mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, "snapshot save /tmp/snapshot.db")
		})).Return("", nil).Once()

		executor.On("Run", mock.Anything, "base64 /tmp/snapshot.db").Return("YmFja3VwLWRhdGE=", nil).Once()

		storage.On("Upload", mock.Anything, "k8s-backups", mock.MatchedBy(func(key string) bool {
			return strings.HasPrefix(key, "k8s-backups/"+clusterID.String()+"/")
		}), mock.Anything, mock.Anything).Return(&domain.Object{}, nil).Once()

		err := p.CreateBackup(ctx, cluster)
		require.NoError(t, err)

		executor.AssertExpectations(t)
		storage.AssertExpectations(t)
	})
}

func TestFailCluster(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("UpdateSucceeds", func(t *testing.T) {
		repo := new(mockClusterRepo)
		cluster := &domain.Cluster{ID: uuid.New()}
		p := &KubeadmProvisioner{repo: repo, logger: logger}

		repo.On("Update", ctx, mock.MatchedBy(func(c *domain.Cluster) bool {
			return c.Status == domain.ClusterStatusFailed
		})).Return(nil).Once()

		err := p.failCluster(ctx, cluster, "test error", fmt.Errorf("underlying"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test error")
		repo.AssertExpectations(t)
	})

	t.Run("UpdateFails_StillReturnsOriginalError", func(t *testing.T) {
		repo := new(mockClusterRepo)
		cluster := &domain.Cluster{ID: uuid.New()}
		p := &KubeadmProvisioner{repo: repo, logger: logger}

		repo.On("Update", ctx, mock.Anything).Return(fmt.Errorf("db down")).Once()

		err := p.failCluster(ctx, cluster, "test error", fmt.Errorf("underlying"))
		require.Error(t, err)
		// Original failure must surface, not the persistence error.
		assert.Contains(t, err.Error(), "test error")
		assert.Contains(t, err.Error(), "underlying")
		assert.NotContains(t, err.Error(), "db down")
		// In-memory status still flipped to Failed even if persistence failed.
		assert.Equal(t, domain.ClusterStatusFailed, cluster.Status)
		repo.AssertExpectations(t)
	})
}

func TestDeprovision(t *testing.T) {
	ctx := context.Background()
	clusterID := uuid.New()
	cluster := &domain.Cluster{ID: clusterID, Name: "test-cluster"}

	t.Run("Success_With_HA", func(t *testing.T) {
		instSvc := new(mockInstanceService)
		repo := new(mockClusterRepo)
		lbSvc := new(mockLBService)
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))

		p := &KubeadmProvisioner{
			instSvc: instSvc,
			repo:    repo,
			lbSvc:   lbSvc,
			logger:  logger,
		}

		cluster.HAEnabled = true
		cluster.VpcID = uuid.New()

		node1 := &domain.ClusterNode{ID: uuid.New(), InstanceID: uuid.New()}
		repo.On("GetNodes", ctx, clusterID).Return([]*domain.ClusterNode{node1}, nil).Once()
		instSvc.On("TerminateInstance", ctx, node1.InstanceID.String()).Return(nil).Once()
		repo.On("DeleteNode", ctx, node1.ID).Return(nil).Once()

		lbName := fmt.Sprintf("lb-k8s-%s", cluster.Name)
		lb1 := &domain.LoadBalancer{ID: uuid.New(), Name: lbName, VpcID: cluster.VpcID}
		lbSvc.On("List", ctx).Return([]*domain.LoadBalancer{lb1}, nil).Once()
		lbSvc.On("Delete", ctx, lb1.ID.String()).Return(nil).Once()

		err := p.Deprovision(ctx, cluster)
		require.NoError(t, err)
	})
}

func TestGetExecutor_FailPaths(t *testing.T) {
	ctx := context.Background()
	cluster := &domain.Cluster{UserID: uuid.New(), SSHPrivateKeyEncrypted: ""}

	t.Run("No_SSH_Key", func(t *testing.T) {
		instSvc := new(mockInstanceService)
		p := &KubeadmProvisioner{instSvc: instSvc}

		instSvc.On("ListInstances", ctx, mock.Anything).Return([]*domain.Instance{}, nil).Once()

		_, err := p.getExecutor(ctx, cluster, "10.0.0.1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no SSH key found")
	})
}
