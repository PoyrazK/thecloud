package k8s

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/mock"
)

func TestEnsureClusterSecurityGroup_AlreadyExists(t *testing.T) {
	t.Parallel()
	cluster := testClusterForSecurity()

	mockedSG := new(MockSecurityGroupService)
	mockedSG.On("GetGroup", mock.Anything, "sg-k8s-"+cluster.Name, cluster.VpcID).
		Return(&domain.SecurityGroup{ID: uuid.New(), Name: "sg-k8s-" + cluster.Name}, nil)

	prov := newTestProvisionerForSecurity(withSGService(mockedSG))

	err := prov.ensureClusterSecurityGroup(context.Background(), cluster)
	require.NoError(t, err)
	mockedSG.AssertCalled(t, "GetGroup", mock.Anything, "sg-k8s-"+cluster.Name, cluster.VpcID)
	mockedSG.AssertNotCalled(t, "CreateGroup", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestEnsureClusterSecurityGroup_CreateSuccess(t *testing.T) {
	t.Parallel()
	cluster := testClusterForSecurity()

	newSG := &domain.SecurityGroup{ID: uuid.New(), Name: "sg-k8s-" + cluster.Name}
	mockedSG := new(MockSecurityGroupService)
	mockedSG.On("GetGroup", mock.Anything, "sg-k8s-"+cluster.Name, cluster.VpcID).Return(nil, errors.New("not found"))
	mockedSG.On("CreateGroup", mock.Anything, cluster.VpcID, "sg-k8s-"+cluster.Name, mock.Anything).
		Return(newSG, nil)
	mockedSG.On("AddRule", mock.Anything, newSG.ID, mock.Anything).Return(nil, nil).Maybe()

	prov := newTestProvisionerForSecurity(withSGService(mockedSG))

	err := prov.ensureClusterSecurityGroup(context.Background(), cluster)
	require.NoError(t, err)
	mockedSG.AssertCalled(t, "CreateGroup", mock.Anything, cluster.VpcID, "sg-k8s-"+cluster.Name, mock.Anything)
	mockedSG.AssertNumberOfCalls(t, "AddRule", 4)
}

func TestEnsureClusterSecurityGroup_CreateFails(t *testing.T) {
	t.Parallel()
	cluster := testClusterForSecurity()

	mockedSG := new(MockSecurityGroupService)
	mockedSG.On("GetGroup", mock.Anything, "sg-k8s-"+cluster.Name, cluster.VpcID).Return(nil, errors.New("not found"))
	mockedSG.On("CreateGroup", mock.Anything, cluster.VpcID, "sg-k8s-"+cluster.Name, mock.Anything).
		Return(nil, errors.New("failed to create security group"))

	prov := newTestProvisionerForSecurity(withSGService(mockedSG))

	err := prov.ensureClusterSecurityGroup(context.Background(), cluster)
	assert.ErrorContains(t, err, "failed to create security group")
}

func TestEnsureClusterSecurityGroup_AddRuleFailsSilently(t *testing.T) {
	t.Parallel()
	cluster := testClusterForSecurity()

	newSG := &domain.SecurityGroup{ID: uuid.New(), Name: "sg-k8s-" + cluster.Name}
	mockedSG := new(MockSecurityGroupService)
	mockedSG.On("GetGroup", mock.Anything, "sg-k8s-"+cluster.Name, cluster.VpcID).Return(nil, errors.New("not found"))
	mockedSG.On("CreateGroup", mock.Anything, cluster.VpcID, "sg-k8s-"+cluster.Name, mock.Anything).
		Return(newSG, nil)
	mockedSG.On("AddRule", mock.Anything, newSG.ID, mock.Anything).Return(nil, errors.New("failed to add rule")).Maybe()

	prov := newTestProvisionerForSecurity(withSGService(mockedSG))

	// ensureClusterSecurityGroup does NOT propagate AddRule errors; it logs and continues
	err := prov.ensureClusterSecurityGroup(context.Background(), cluster)
	require.NoError(t, err)
}

func TestEnsureClusterSecurityGroup_GetGroupErrorFallsThroughToCreate(t *testing.T) {
	t.Parallel()
	cluster := testClusterForSecurity()

	newSG := &domain.SecurityGroup{ID: uuid.New(), Name: "sg-k8s-" + cluster.Name}
	mockedSG := new(MockSecurityGroupService)
	mockedSG.On("GetGroup", mock.Anything, "sg-k8s-"+cluster.Name, cluster.VpcID).Return(nil, errors.New("some error"))
	mockedSG.On("CreateGroup", mock.Anything, cluster.VpcID, "sg-k8s-"+cluster.Name, mock.Anything).
		Return(newSG, nil)
	mockedSG.On("AddRule", mock.Anything, newSG.ID, mock.Anything).Return(nil, nil).Maybe()

	prov := newTestProvisionerForSecurity(withSGService(mockedSG))

	err := prov.ensureClusterSecurityGroup(context.Background(), cluster)
	require.NoError(t, err)
	mockedSG.AssertCalled(t, "CreateGroup", mock.Anything, cluster.VpcID, "sg-k8s-"+cluster.Name, mock.Anything)
}

func testClusterForSecurity() *domain.Cluster {
	return &domain.Cluster{
		ID:       uuid.New(),
		Name:     "test-cluster",
		VpcID:    uuid.New(),
		UserID:   uuid.New(),
		TenantID: uuid.New(),
	}
}

type sgProvisionerOption func(*KubeadmProvisioner)

func withSGService(sg ports.SecurityGroupService) sgProvisionerOption {
	return func(p *KubeadmProvisioner) {
		p.sgSvc = sg
	}
}

func newTestProvisionerForSecurity(opts ...sgProvisionerOption) *KubeadmProvisioner {
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