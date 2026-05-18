package k8s

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	kubeMasterIP = "10.0.0.10"
	kubeShell    = "sh"
	kubectlBase  = "kubectl --kubeconfig /etc/kubernetes/admin.conf"
)

func newProvisioner(instSvc *mockInstanceService, repo *mockClusterRepo) *KubeadmProvisioner {
	return &KubeadmProvisioner{
		instSvc: instSvc,
		repo:    repo,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestKubeadmProvisionerGetHealthNoControlPlaneIPs(t *testing.T) {
	instSvc := new(mockInstanceService)
	repo := new(mockClusterRepo)
	p := newProvisioner(instSvc, repo)

	cluster := &domain.Cluster{ID: uuid.New()}

	health, err := p.GetHealth(context.Background(), cluster)

	require.Error(t, err)
	assert.Nil(t, health)
}

func TestKubeadmProvisionerGetHealthServiceExecutor(t *testing.T) {
	instSvc := new(mockInstanceService)
	repo := new(mockClusterRepo)
	p := newProvisioner(instSvc, repo)

	clusterID := uuid.New()
	instanceID := uuid.New()
	masterIP := kubeMasterIP
	cluster := &domain.Cluster{ID: clusterID, ControlPlaneIPs: []string{masterIP}, Status: domain.ClusterStatusRunning}

	repo.On("GetNodes", mock.Anything, clusterID).Return([]*domain.ClusterNode{{ID: uuid.New(), InstanceID: instanceID}}, nil)
	instSvc.On("GetInstance", mock.Anything, instanceID.String()).Return(&domain.Instance{ID: instanceID, PrivateIP: masterIP}, nil)
	instSvc.On("ListInstances", mock.Anything, mock.Anything).Return([]*domain.Instance{{ID: instanceID, PrivateIP: masterIP}}, nil)
	instSvc.On("Exec", mock.Anything, instanceID.String(), []string{kubeShell, "-c", kubectlBase + " get nodes"}).Return("", nil)
	instSvc.On("Exec", mock.Anything, instanceID.String(), []string{kubeShell, "-c", kubectlBase + " get nodes --no-headers"}).Return("node-a Ready \nnode-b NotReady", nil)

	health, err := p.GetHealth(context.Background(), cluster)

	require.NoError(t, err)
	assert.True(t, health.APIServer)
	assert.Equal(t, 2, health.NodesTotal)
	assert.Equal(t, 1, health.NodesReady)
	assert.Equal(t, "1/2 nodes are ready", health.Message)
}

func TestKubeadmProvisionerGetHealthAPIServerDown(t *testing.T) {
	instSvc := new(mockInstanceService)
	repo := new(mockClusterRepo)
	p := newProvisioner(instSvc, repo)

	clusterID := uuid.New()
	instanceID := uuid.New()
	masterIP := kubeMasterIP
	cluster := &domain.Cluster{ID: clusterID, ControlPlaneIPs: []string{masterIP}, Status: domain.ClusterStatusRunning}

	repo.On("GetNodes", mock.Anything, clusterID).Return([]*domain.ClusterNode{{ID: uuid.New(), InstanceID: instanceID}}, nil)
	instSvc.On("GetInstance", mock.Anything, instanceID.String()).Return(&domain.Instance{ID: instanceID, PrivateIP: masterIP}, nil)
	instSvc.On("ListInstances", mock.Anything, mock.Anything).Return([]*domain.Instance{{ID: instanceID, PrivateIP: masterIP}}, nil)
	instSvc.On("Exec", mock.Anything, instanceID.String(), []string{kubeShell, "-c", kubectlBase + " get nodes"}).Return("", errors.New("down"))
	instSvc.On("Exec", mock.Anything, instanceID.String(), []string{kubeShell, "-c", kubectlBase + " get nodes --no-headers"}).Return("", io.EOF)

	health, err := p.GetHealth(context.Background(), cluster)

	require.NoError(t, err)
	assert.False(t, health.APIServer)
	assert.Equal(t, "API server is unreachable", health.Message)
}

func TestKubeadmProvisionerGetKubeconfigNoControlPlaneIPs(t *testing.T) {
	instSvc := new(mockInstanceService)
	repo := new(mockClusterRepo)
	p := newProvisioner(instSvc, repo)

	cluster := &domain.Cluster{ID: uuid.New()}

	kubeconfig, err := p.GetKubeconfig(context.Background(), cluster, "admin")

	require.Error(t, err)
	assert.Empty(t, kubeconfig)
}

func TestKubeadmProvisionerGetKubeconfigViewerNotImplemented(t *testing.T) {
	instSvc := new(mockInstanceService)
	repo := new(mockClusterRepo)
	p := newProvisioner(instSvc, repo)

	clusterID := uuid.New()
	cluster := &domain.Cluster{ID: clusterID, ControlPlaneIPs: []string{kubeMasterIP}}

	repo.On("GetNodes", mock.Anything, clusterID).Return(nil, errors.New("no nodes"))

	kubeconfig, err := p.GetKubeconfig(context.Background(), cluster, "viewer")

	require.Error(t, err)
	assert.Empty(t, kubeconfig)
}

func TestKubeadmProvisionerGetKubeconfigAdmin(t *testing.T) {
	instSvc := new(mockInstanceService)
	repo := new(mockClusterRepo)
	p := newProvisioner(instSvc, repo)

	clusterID := uuid.New()
	masterIP := kubeMasterIP
	cluster := &domain.Cluster{ID: clusterID, ControlPlaneIPs: []string{masterIP}, KubeconfigEncrypted: "kubeconfig", UserID: uuid.New()}

	mockSecret := new(mocks.SecretService)
	p.secretSvc = mockSecret
	mockSecret.On("Decrypt", mock.Anything, cluster.UserID, "kubeconfig").Return("kubeconfig", nil)

	kubeconfig, err := p.GetKubeconfig(context.Background(), cluster, "admin")

	require.NoError(t, err)
	assert.Equal(t, "kubeconfig", kubeconfig)
}
