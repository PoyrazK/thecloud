package k8s

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/mock"
)

type mockInstanceService struct{ mock.Mock }

func (m *mockInstanceService) LaunchInstance(ctx context.Context, params ports.LaunchParams) (*domain.Instance, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceService) Provision(ctx context.Context, job domain.ProvisionJob) error {
	return m.Called(ctx, job).Error(0)
}
func (m *mockInstanceService) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (*domain.Instance, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceService) StartInstance(ctx context.Context, idOrName string) error  { return nil }
func (m *mockInstanceService) StopInstance(ctx context.Context, idOrName string) error   { return nil }
func (m *mockInstanceService) PauseInstance(ctx context.Context, idOrName string) error  { return nil }
func (m *mockInstanceService) ResumeInstance(ctx context.Context, idOrName string) error { return nil }
func (m *mockInstanceService) ListInstances(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	args := m.Called(ctx, tagFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceService) GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceService) GetInstanceLogs(ctx context.Context, idOrName string) (string, error) {
	return "", nil
}
func (m *mockInstanceService) GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error) {
	return nil, nil
}
func (m *mockInstanceService) GetConsoleURL(ctx context.Context, idOrName string) (string, error) {
	return "", nil
}
func (m *mockInstanceService) TerminateInstance(ctx context.Context, idOrName string) error {
	for _, call := range m.ExpectedCalls {
		if call.Method == "TerminateInstance" {
			return m.Called(ctx, idOrName).Error(0)
		}
	}
	return nil
}
func (m *mockInstanceService) Exec(ctx context.Context, idOrName string, cmd []string) (string, error) {
	args := m.Called(ctx, idOrName, cmd)
	if args.Get(0) == nil {
		return "", args.Error(1)
	}
	return args.String(0), args.Error(1)
}

func (m *mockInstanceService) UpdateInstanceMetadata(ctx context.Context, id uuid.UUID, metadata, labels map[string]string) error {
	args := m.Called(ctx, id, metadata, labels)
	return args.Error(0)
}
func (m *mockInstanceService) ResizeInstance(ctx context.Context, idOrName, newInstanceType string) (*domain.Instance, error) {
	args := m.Called(ctx, idOrName, newInstanceType)
	var r0 *domain.Instance
	if args.Get(0) != nil {
		r0, _ = args.Get(0).(*domain.Instance)
	}
	return r0, args.Error(1)
}
func (m *mockInstanceService) GetTags(ctx context.Context, id uuid.UUID) (map[string]string, error) {
	args := m.Called(ctx, id)
	var r0 map[string]string
	if args.Get(0) != nil {
		r0, _ = args.Get(0).(map[string]string)
	}
	return r0, args.Error(1)
}
func (m *mockInstanceService) SetTags(ctx context.Context, id uuid.UUID, labels map[string]string) error {
	return m.Called(ctx, id, labels).Error(0)
}
func (m *mockInstanceService) RemoveTag(ctx context.Context, id uuid.UUID, key string) error {
	return m.Called(ctx, id, key).Error(0)
}

type mockClusterRepo struct{ mock.Mock }

func (m *mockClusterRepo) Create(ctx context.Context, c *domain.Cluster) error { return nil }
func (m *mockClusterRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	return nil, nil
}
func (m *mockClusterRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Cluster, error) {
	return nil, nil
}
func (m *mockClusterRepo) ListAll(ctx context.Context) ([]*domain.Cluster, error) {
	return nil, nil
}
func (m *mockClusterRepo) Update(ctx context.Context, c *domain.Cluster) error {
	for _, call := range m.ExpectedCalls {
		if call.Method == "Update" {
			return m.Called(ctx, c).Error(0)
		}
	}
	return nil
}
func (m *mockClusterRepo) Delete(ctx context.Context, id uuid.UUID) error           { return nil }
func (m *mockClusterRepo) AddNode(ctx context.Context, n *domain.ClusterNode) error { return nil }
func (m *mockClusterRepo) GetNodes(ctx context.Context, clusterID uuid.UUID) ([]*domain.ClusterNode, error) {
	args := m.Called(ctx, clusterID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.ClusterNode)
	return r0, args.Error(1)
}
func (m *mockClusterRepo) DeleteNode(ctx context.Context, nodeID uuid.UUID) error {
	for _, call := range m.ExpectedCalls {
		if call.Method == "DeleteNode" {
			return m.Called(ctx, nodeID).Error(0)
		}
	}
	return nil
}
func (m *mockClusterRepo) UpdateNode(ctx context.Context, n *domain.ClusterNode) error { return nil }

func (m *mockClusterRepo) AddNodeGroup(ctx context.Context, ng *domain.NodeGroup) error { return nil }
func (m *mockClusterRepo) GetNodeGroups(ctx context.Context, clusterID uuid.UUID) ([]domain.NodeGroup, error) {
	return []domain.NodeGroup{}, nil
}
func (m *mockClusterRepo) UpdateNodeGroup(ctx context.Context, ng *domain.NodeGroup) error {
	return nil
}
func (m *mockClusterRepo) DeleteNodeGroup(ctx context.Context, clusterID uuid.UUID, name string) error {
	return nil
}

type mockSecretService struct{ mock.Mock }

func (m *mockSecretService) CreateSecret(ctx context.Context, name, value, desc string) (*domain.Secret, error) {
	return nil, nil
}
func (m *mockSecretService) GetSecret(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	return nil, nil
}
func (m *mockSecretService) GetSecretByName(ctx context.Context, name string) (*domain.Secret, error) {
	return nil, nil
}
func (m *mockSecretService) ListSecrets(ctx context.Context) ([]*domain.Secret, error) {
	return nil, nil
}
func (m *mockSecretService) DeleteSecret(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockSecretService) Encrypt(ctx context.Context, userID uuid.UUID, plain string) (string, error) {
	args := m.Called(ctx, userID, plain)
	return args.String(0), args.Error(1)
}
func (m *mockSecretService) Decrypt(ctx context.Context, userID uuid.UUID, cipher string) (string, error) {
	args := m.Called(ctx, userID, cipher)
	return args.String(0), args.Error(1)
}

type mockLBService struct{ mock.Mock }

func (m *mockLBService) Create(ctx context.Context, name string, vpcID uuid.UUID, port int, algo string, idempotencyKey string) (*domain.LoadBalancer, error) {
	return nil, nil
}
func (m *mockLBService) Get(ctx context.Context, idOrName string) (*domain.LoadBalancer, error) {
	return nil, nil
}
func (m *mockLBService) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LoadBalancer), args.Error(1)
}
func (m *mockLBService) Delete(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *mockLBService) AddTarget(ctx context.Context, lbID, instanceID uuid.UUID, port int, weight int) error {
	return nil
}
func (m *mockLBService) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error {
	return nil
}
func (m *mockLBService) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	return nil, nil
}
