package k8s

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/mock"
)

func TestEnsureAPIServerLB_AlreadyExists(t *testing.T) {
	t.Parallel()
	cluster := testCluster()

	existingLB := &domain.LoadBalancer{
		ID:     uuid.New(),
		Name:   "lb-k8s-" + cluster.Name,
		VpcID:  cluster.VpcID,
		IP:     "10.0.0.99",
		Status: "active",
	}

	mockedLB := new(mockLBService)
	mockedLB.On("List", mock.Anything).Return([]*domain.LoadBalancer{existingLB}, nil)

	prov := newTestProvisioner(withLBService(mockedLB))

	err := prov.ensureAPIServerLB(context.Background(), cluster)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.99", *cluster.APIServerLBAddress)
}

func TestEnsureAPIServerLB_CreateFails(t *testing.T) {
	t.Parallel()
	cluster := testCluster()

	mockedLB := new(mockLBService)
	mockedLB.On("List", mock.Anything).Return([]*domain.LoadBalancer{}, nil)
	mockedLB.On("Create", mock.Anything, "lb-k8s-"+cluster.Name, cluster.VpcID, 6443, "round-robin", cluster.ID.String()).
		Return(nil, errors.New("failed to create load balancer"))

	prov := newTestProvisioner(withLBService(mockedLB))

	err := prov.ensureAPIServerLB(context.Background(), cluster)
	assert.ErrorContains(t, err, "failed to create load balancer")
}

func TestEnsureAPIServerLB_IPNeverAssigned(t *testing.T) {
	t.Parallel()
	cluster := testCluster()

	lbID := uuid.New()
	mockedLB := new(mockLBService)
	mockedLB.On("List", mock.Anything).Return([]*domain.LoadBalancer{}, nil)
	mockedLB.On("Create", mock.Anything, "lb-k8s-"+cluster.Name, cluster.VpcID, 6443, "round-robin", cluster.ID.String()).
		Return(&domain.LoadBalancer{ID: lbID, Name: "lb-k8s-" + cluster.Name, VpcID: cluster.VpcID, IP: "", Status: "provisioning"}, nil)
	mockedLB.On("Get", mock.Anything, lbID.String()).Return(&domain.LoadBalancer{ID: lbID, IP: "", Status: "provisioning"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	prov := newTestProvisioner(withLBService(mockedLB))

	err := prov.ensureAPIServerLB(ctx, cluster)
	assert.ErrorContains(t, err, "load balancer IP never assigned")
}

func TestEnsureAPIServerLB_IPAssignedImmediately(t *testing.T) {
	t.Parallel()
	cluster := testCluster()

	lbID := uuid.New()
	mockedLB := new(mockLBService)
	mockedLB.On("List", mock.Anything).Return([]*domain.LoadBalancer{}, nil)
	mockedLB.On("Create", mock.Anything, "lb-k8s-"+cluster.Name, cluster.VpcID, 6443, "round-robin", cluster.ID.String()).
		Return(&domain.LoadBalancer{ID: lbID, Name: "lb-k8s-" + cluster.Name, VpcID: cluster.VpcID, IP: "10.0.0.1", Status: "active"}, nil)

	mockedRepo := new(mockClusterRepo)
	mockedRepo.On("Update", mock.Anything, cluster).Return(nil)

	prov := newTestProvisioner(withLBService(mockedLB), withClusterRepo(mockedRepo))

	err := prov.ensureAPIServerLB(context.Background(), cluster)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", *cluster.APIServerLBAddress)
	mockedRepo.AssertCalled(t, "Update", mock.Anything, cluster)
}

func TestEnsureAPIServerLB_IPAssignedAfterPolling(t *testing.T) {
	t.Parallel()
	cluster := testCluster()

	lbID := uuid.New()
	mockedLB := new(mockLBService)
	mockedLB.On("List", mock.Anything).Return([]*domain.LoadBalancer{}, nil)
	mockedLB.On("Create", mock.Anything, "lb-k8s-"+cluster.Name, cluster.VpcID, 6443, "round-robin", cluster.ID.String()).
		Return(&domain.LoadBalancer{ID: lbID, Name: "lb-k8s-" + cluster.Name, VpcID: cluster.VpcID, IP: "", Status: "provisioning"}, nil)
	mockedLB.On("Get", mock.Anything, lbID.String()).
		Return(&domain.LoadBalancer{ID: lbID, Name: "lb-k8s-" + cluster.Name, VpcID: cluster.VpcID, IP: "10.0.0.2", Status: "active"}, nil)

	prov := newTestProvisioner(withLBService(mockedLB))

	err := prov.ensureAPIServerLB(context.Background(), cluster)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.2", *cluster.APIServerLBAddress)
}

func TestEnsureAPIServerLB_RepoUpdateFails(t *testing.T) {
	t.Parallel()
	cluster := testCluster()

	lbID := uuid.New()
	mockedLB := new(mockLBService)
	mockedLB.On("List", mock.Anything).Return([]*domain.LoadBalancer{}, nil)
	mockedLB.On("Create", mock.Anything, "lb-k8s-"+cluster.Name, cluster.VpcID, 6443, "round-robin", cluster.ID.String()).
		Return(&domain.LoadBalancer{ID: lbID, Name: "lb-k8s-" + cluster.Name, VpcID: cluster.VpcID, IP: "10.0.0.1", Status: "active"}, nil)

	mockedRepo := new(mockClusterRepo)
	mockedRepo.On("Update", mock.Anything, cluster).Return(errors.New("failed to update cluster"))

	prov := newTestProvisioner(withLBService(mockedLB), withClusterRepo(mockedRepo))

	err := prov.ensureAPIServerLB(context.Background(), cluster)
	assert.ErrorContains(t, err, "failed to update cluster")
}

func TestEnsureAPIServerLB_ListErrorFallsThroughToCreate(t *testing.T) {
	t.Parallel()
	cluster := testCluster()

	lbID := uuid.New()
	mockedLB := new(mockLBService)
	mockedLB.On("List", mock.Anything).Return(nil, errors.New("list error"))
	mockedLB.On("Create", mock.Anything, "lb-k8s-"+cluster.Name, cluster.VpcID, 6443, "round-robin", cluster.ID.String()).
		Return(&domain.LoadBalancer{ID: lbID, Name: "lb-k8s-" + cluster.Name, VpcID: cluster.VpcID, IP: "10.0.0.1", Status: "active"}, nil)

	mockedRepo := new(mockClusterRepo)
	mockedRepo.On("Update", mock.Anything, cluster).Return(nil)

	prov := newTestProvisioner(withLBService(mockedLB), withClusterRepo(mockedRepo))

	err := prov.ensureAPIServerLB(context.Background(), cluster)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", *cluster.APIServerLBAddress)
}

// --- test helpers ---

func testCluster() *domain.Cluster {
	return &domain.Cluster{
		ID:        uuid.New(),
		Name:      "test-cluster",
		VpcID:     uuid.New(),
		UserID:    uuid.New(),
		TenantID:  uuid.New(),
		HAEnabled: true,
	}
}

type provisionerOption func(*KubeadmProvisioner)

func withLBService(svc ports.LBService) provisionerOption {
	return func(p *KubeadmProvisioner) {
		p.lbSvc = svc
	}
}

func withClusterRepo(repo ports.ClusterRepository) provisionerOption {
	return func(p *KubeadmProvisioner) {
		p.repo = repo
	}
}

func newTestProvisioner(opts ...provisionerOption) *KubeadmProvisioner {
	p := &KubeadmProvisioner{
		instSvc:     &mockInstanceService{},
		repo:        &mockClusterRepo{},
		secretSvc:   &mockSecretService{},
		sgSvc:       &MockSecurityGroupService{},
		storageSvc:  &mockStorageService{},
		lbSvc:       &mockLBService{},
		logger:      slog.Default(),
		templateDir: "internal/repositories/k8s/templates",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}