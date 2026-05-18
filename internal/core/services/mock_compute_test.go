package services_test

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/mock"
)

// MockInstanceRepo
type MockInstanceRepo struct{ mock.Mock }

func (m *MockInstanceRepo) Create(ctx context.Context, i *domain.Instance) error {
	return m.Called(ctx, i).Error(0)
}
func (m *MockInstanceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}
func (m *MockInstanceRepo) List(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	args := m.Called(ctx, tagFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Instance), args.Error(1)
}
func (m *MockInstanceRepo) ListAll(ctx context.Context) ([]*domain.Instance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Instance), args.Error(1)
}
func (m *MockInstanceRepo) Update(ctx context.Context, i *domain.Instance) error {
	return m.Called(ctx, i).Error(0)
}
func (m *MockInstanceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockInstanceRepo) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}
func (m *MockInstanceRepo) ListBySubnet(ctx context.Context, id uuid.UUID) ([]*domain.Instance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Instance), args.Error(1)
}
func (m *MockInstanceRepo) ListByVPC(ctx context.Context, id uuid.UUID) ([]*domain.Instance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Instance), args.Error(1)
}

// MockInstanceService
type MockInstanceService struct{ mock.Mock }

func (m *MockInstanceService) LaunchInstance(ctx context.Context, params ports.LaunchParams) (*domain.Instance, error) {
	args := m.Called(ctx, params)
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *MockInstanceService) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (*domain.Instance, error) {
	args := m.Called(ctx, opts)
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *MockInstanceService) StartInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *MockInstanceService) StopInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *MockInstanceService) PauseInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *MockInstanceService) ResumeInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *MockInstanceService) ListInstances(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	args := m.Called(ctx, tagFilter)
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *MockInstanceService) GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error) {
	args := m.Called(ctx, idOrName)
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *MockInstanceService) GetInstanceLogs(ctx context.Context, idOrName string) (string, error) {
	args := m.Called(ctx, idOrName)
	return args.String(0), args.Error(1)
}
func (m *MockInstanceService) GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error) {
	args := m.Called(ctx, idOrName)
	r0, _ := args.Get(0).(*domain.InstanceStats)
	return r0, args.Error(1)
}
func (m *MockInstanceService) GetConsoleURL(ctx context.Context, idOrName string) (string, error) {
	args := m.Called(ctx, idOrName)
	return args.String(0), args.Error(1)
}
func (m *MockInstanceService) TerminateInstance(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *MockInstanceService) Exec(ctx context.Context, idOrName string, cmd []string) (string, error) {
	args := m.Called(ctx, idOrName, cmd)
	return args.String(0), args.Error(1)
}
func (m *MockInstanceService) UpdateInstanceMetadata(ctx context.Context, id uuid.UUID, metadata, labels map[string]string) error {
	args := m.Called(ctx, id, metadata, labels)
	return args.Error(0)
}
func (m *MockInstanceService) ResizeInstance(ctx context.Context, idOrName, newInstanceType string) (*domain.Instance, error) {
	args := m.Called(ctx, idOrName, newInstanceType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}
func (m *MockInstanceService) GetTags(ctx context.Context, id uuid.UUID) (map[string]string, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(map[string]string)
	return r0, args.Error(1)
}
func (m *MockInstanceService) SetTags(ctx context.Context, id uuid.UUID, labels map[string]string) error {
	return m.Called(ctx, id, labels).Error(0)
}
func (m *MockInstanceService) RemoveTag(ctx context.Context, id uuid.UUID, key string) error {
	return m.Called(ctx, id, key).Error(0)
}
func (m *MockInstanceService) Provision(ctx context.Context, job domain.ProvisionJob) error {
	return m.Called(ctx, job).Error(0)
}

// MockComputeBackend
type MockComputeBackend struct{ mock.Mock }

func (m *MockComputeBackend) CreateInstance(ctx context.Context, i *domain.Instance) (string, error) {
	args := m.Called(ctx, i)
	return args.String(0), args.Error(1)
}
func (m *MockComputeBackend) DeleteInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockComputeBackend) StartInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockComputeBackend) StopInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockComputeBackend) PauseInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockComputeBackend) ResumeInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockComputeBackend) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *MockComputeBackend) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *MockComputeBackend) ListInstances(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}
func (m *MockComputeBackend) AttachVolume(ctx context.Context, id, path string) (string, string, error) {
	args := m.Called(ctx, id, path)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *MockComputeBackend) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	args := m.Called(ctx, id, cmd)
	return args.String(0), args.Error(1)
}
func (m *MockComputeBackend) CreateNetwork(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}
func (m *MockComputeBackend) DeleteNetwork(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockComputeBackend) GetInstanceIP(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
func (m *MockComputeBackend) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	args := m.Called(ctx, opts)
	r1, _ := args.Get(1).([]string)
	return args.String(0), r1, args.Error(2)
}
func (m *MockComputeBackend) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	args := m.Called(ctx, opts)
	r1, _ := args.Get(1).([]string)
	return args.String(0), r1, args.Error(2)
}
func (m *MockComputeBackend) WaitTask(ctx context.Context, id string) (int64, error) {
	args := m.Called(ctx, id)
	val := args.Get(0)
	if i, ok := val.(int64); ok {
		return i, args.Error(1)
	}
	return int64(args.Int(0)), args.Error(1)
}
func (m *MockComputeBackend) GetInstancePort(ctx context.Context, id string, port string) (int, error) {
	args := m.Called(ctx, id, port)
	return args.Int(0), args.Error(1)
}
func (m *MockComputeBackend) Ping(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *MockComputeBackend) Type() string {
	return m.Called().String(0)
}
func (m *MockComputeBackend) DetachVolume(ctx context.Context, id string, path string) (string, error) {
	args := m.Called(ctx, id, path)
	return args.String(0), args.Error(1)
}
func (m *MockComputeBackend) GetConsoleURL(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
func (m *MockComputeBackend) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	return m.Called(ctx, id, cpu, memory).Error(0)
}
func (m *MockComputeBackend) CreateSnapshot(ctx context.Context, id, name string) error {
	return m.Called(ctx, id, name).Error(0)
}
func (m *MockComputeBackend) RestoreSnapshot(ctx context.Context, id, name string) error {
	return m.Called(ctx, id, name).Error(0)
}
func (m *MockComputeBackend) DeleteSnapshot(ctx context.Context, id, name string) error {
	return m.Called(ctx, id, name).Error(0)
}
func (m *MockComputeBackend) ResetCircuitBreaker() {}

// MockClusterRepo
type MockClusterRepo struct{ mock.Mock }

func (m *MockClusterRepo) Create(ctx context.Context, cluster *domain.Cluster) error {
	return m.Called(ctx, cluster).Error(0)
}
func (m *MockClusterRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.Cluster)
	return r0, args.Error(1)
}
func (m *MockClusterRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Cluster, error) {
	args := m.Called(ctx, userID)
	r0, _ := args.Get(0).([]*domain.Cluster)
	return r0, args.Error(1)
}
func (m *MockClusterRepo) ListAll(ctx context.Context) ([]*domain.Cluster, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.Cluster)
	return r0, args.Error(1)
}
func (m *MockClusterRepo) Update(ctx context.Context, cluster *domain.Cluster) error {
	return m.Called(ctx, cluster).Error(0)
}
func (m *MockClusterRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockClusterRepo) AddNode(ctx context.Context, n *domain.ClusterNode) error {
	return m.Called(ctx, n).Error(0)
}
func (m *MockClusterRepo) GetNodes(ctx context.Context, clusterID uuid.UUID) ([]*domain.ClusterNode, error) {
	args := m.Called(ctx, clusterID)
	r0, _ := args.Get(0).([]*domain.ClusterNode)
	return r0, args.Error(1)
}
func (m *MockClusterRepo) UpdateNode(ctx context.Context, n *domain.ClusterNode) error {
	return m.Called(ctx, n).Error(0)
}
func (m *MockClusterRepo) DeleteNode(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockClusterRepo) AddNodeGroup(ctx context.Context, ng *domain.NodeGroup) error {
	return m.Called(ctx, ng).Error(0)
}
func (m *MockClusterRepo) GetNodeGroups(ctx context.Context, clusterID uuid.UUID) ([]domain.NodeGroup, error) {
	args := m.Called(ctx, clusterID)
	r0, _ := args.Get(0).([]domain.NodeGroup)
	return r0, args.Error(1)
}
func (m *MockClusterRepo) UpdateNodeGroup(ctx context.Context, ng *domain.NodeGroup) error {
	return m.Called(ctx, ng).Error(0)
}
func (m *MockClusterRepo) DeleteNodeGroup(ctx context.Context, clusterID uuid.UUID, name string) error {
	return m.Called(ctx, clusterID, name).Error(0)
}

// MockClusterProvisioner
type MockClusterProvisioner struct{ mock.Mock }

func (m *MockClusterProvisioner) Provision(ctx context.Context, cluster *domain.Cluster) error {
	return m.Called(ctx, cluster).Error(0)
}
func (m *MockClusterProvisioner) Deprovision(ctx context.Context, cluster *domain.Cluster) error {
	return m.Called(ctx, cluster).Error(0)
}
func (m *MockClusterProvisioner) GetStatus(ctx context.Context, cluster *domain.Cluster) (domain.ClusterStatus, error) {
	args := m.Called(ctx, cluster)
	r0, _ := args.Get(0).(domain.ClusterStatus)
	return r0, args.Error(1)
}
func (m *MockClusterProvisioner) Repair(ctx context.Context, cluster *domain.Cluster) error {
	return m.Called(ctx, cluster).Error(0)
}
func (m *MockClusterProvisioner) Scale(ctx context.Context, cluster *domain.Cluster) error {
	return m.Called(ctx, cluster).Error(0)
}
func (m *MockClusterProvisioner) GetKubeconfig(ctx context.Context, cluster *domain.Cluster, role string) (string, error) {
	args := m.Called(ctx, cluster, role)
	return args.String(0), args.Error(1)
}
func (m *MockClusterProvisioner) Upgrade(ctx context.Context, cluster *domain.Cluster, version string) error {
	return m.Called(ctx, cluster, version).Error(0)
}
func (m *MockClusterProvisioner) RotateSecrets(ctx context.Context, cluster *domain.Cluster) error {
	return m.Called(ctx, cluster).Error(0)
}
func (m *MockClusterProvisioner) CreateBackup(ctx context.Context, cluster *domain.Cluster) error {
	return m.Called(ctx, cluster).Error(0)
}
func (m *MockClusterProvisioner) Restore(ctx context.Context, cluster *domain.Cluster, backupPath string) error {
	return m.Called(ctx, cluster, backupPath).Error(0)
}
func (m *MockClusterProvisioner) GetHealth(ctx context.Context, cluster *domain.Cluster) (*ports.ClusterHealth, error) {
	args := m.Called(ctx, cluster)
	r0, _ := args.Get(0).(*ports.ClusterHealth)
	return r0, args.Error(1)
}

// MockAutoScalingRepo
type MockAutoScalingRepo struct{ mock.Mock }

func (m *MockAutoScalingRepo) CreateGroup(ctx context.Context, group *domain.ScalingGroup) error {
	return m.Called(ctx, group).Error(0)
}
func (m *MockAutoScalingRepo) GetGroupByID(ctx context.Context, id uuid.UUID) (*domain.ScalingGroup, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.ScalingGroup)
	return r0, args.Error(1)
}
func (m *MockAutoScalingRepo) GetGroupByIdempotencyKey(ctx context.Context, key string) (*domain.ScalingGroup, error) {
	args := m.Called(ctx, key)
	r0, _ := args.Get(0).(*domain.ScalingGroup)
	return r0, args.Error(1)
}
func (m *MockAutoScalingRepo) ListGroups(ctx context.Context) ([]*domain.ScalingGroup, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.ScalingGroup)
	return r0, args.Error(1)
}
func (m *MockAutoScalingRepo) ListAllGroups(ctx context.Context) ([]*domain.ScalingGroup, error) {
	ret := m.Called(ctx)
	r0, _ := ret.Get(0).([]*domain.ScalingGroup)
	return r0, ret.Error(1)
}
func (m *MockAutoScalingRepo) CountGroupsByVPC(ctx context.Context, vpcID uuid.UUID) (int, error) {
	args := m.Called(ctx, vpcID)
	return args.Int(0), args.Error(1)
}
func (m *MockAutoScalingRepo) UpdateGroup(ctx context.Context, group *domain.ScalingGroup) error {
	return m.Called(ctx, group).Error(0)
}
func (m *MockAutoScalingRepo) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockAutoScalingRepo) CreatePolicy(ctx context.Context, policy *domain.ScalingPolicy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) GetPoliciesForGroup(ctx context.Context, groupID uuid.UUID) ([]*domain.ScalingPolicy, error) {
	args := m.Called(ctx, groupID)
	r0, _ := args.Get(0).([]*domain.ScalingPolicy)
	return r0, args.Error(1)
}
func (m *MockAutoScalingRepo) GetAllPolicies(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID][]*domain.ScalingPolicy, error) {
	args := m.Called(ctx, groupIDs)
	r0, _ := args.Get(0).(map[uuid.UUID][]*domain.ScalingPolicy)
	return r0, args.Error(1)
}
func (m *MockAutoScalingRepo) UpdatePolicyLastScaled(ctx context.Context, policyID uuid.UUID, t time.Time) error {
	args := m.Called(ctx, policyID, t)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) AddInstanceToGroup(ctx context.Context, groupID, instanceID uuid.UUID) error {
	return m.Called(ctx, groupID, instanceID).Error(0)
}
func (m *MockAutoScalingRepo) RemoveInstanceFromGroup(ctx context.Context, groupID, instanceID uuid.UUID) error {
	return m.Called(ctx, groupID, instanceID).Error(0)
}
func (m *MockAutoScalingRepo) GetInstancesInGroup(ctx context.Context, groupID uuid.UUID) ([]uuid.UUID, error) {
	args := m.Called(ctx, groupID)
	r0, _ := args.Get(0).([]uuid.UUID)
	return r0, args.Error(1)
}
func (m *MockAutoScalingRepo) GetAllScalingGroupInstances(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	args := m.Called(ctx, groupIDs)
	r0, _ := args.Get(0).(map[uuid.UUID][]uuid.UUID)
	return r0, args.Error(1)
}
func (m *MockAutoScalingRepo) GetAverageCPU(ctx context.Context, instanceIDs []uuid.UUID, since time.Time) (float64, error) {
	args := m.Called(ctx, instanceIDs, since)
	r0, _ := args.Get(0).(float64)
	return r0, args.Error(1)
}

// MockClock
type MockClock struct{ mock.Mock }

func (m *MockClock) Now() time.Time {
	args := m.Called()
	r0, _ := args.Get(0).(time.Time)
	return r0
}

// MockInstanceTypeRepo
type MockInstanceTypeRepo struct{ mock.Mock }

func (m *MockInstanceTypeRepo) List(ctx context.Context) ([]*domain.InstanceType, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.InstanceType)
	return r0, args.Error(1)
}
func (m *MockInstanceTypeRepo) GetByID(ctx context.Context, id string) (*domain.InstanceType, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.InstanceType)
	return r0, args.Error(1)
}
func (m *MockInstanceTypeRepo) Create(ctx context.Context, it *domain.InstanceType) (*domain.InstanceType, error) {
	args := m.Called(ctx, it)
	r0, _ := args.Get(0).(*domain.InstanceType)
	return r0, args.Error(1)
}
func (m *MockInstanceTypeRepo) Update(ctx context.Context, it *domain.InstanceType) (*domain.InstanceType, error) {
	args := m.Called(ctx, it)
	r0, _ := args.Get(0).(*domain.InstanceType)
	return r0, args.Error(1)
}
func (m *MockInstanceTypeRepo) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}

// MockInstanceTypeService
type MockInstanceTypeService struct{ mock.Mock }

func (m *MockInstanceTypeService) List(ctx context.Context) ([]*domain.InstanceType, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.InstanceType)
	return r0, args.Error(1)
}
