package k8s_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/core/ports/mocks"
	"github.com/poyrazk/thecloud/internal/repositories/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockInstanceService struct{ mock.Mock }

func (m *MockInstanceService) LaunchInstance(ctx context.Context, params ports.LaunchParams) (*domain.Instance, error) {
	args := m.Called(ctx, params)
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *MockInstanceService) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (*domain.Instance, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *MockInstanceService) Provision(ctx context.Context, job domain.ProvisionJob) error {
	return m.Called(ctx, job).Error(0)
}
func (m *MockInstanceService) GetInstance(ctx context.Context, id string) (*domain.Instance, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *MockInstanceService) TerminateInstance(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockInstanceService) StartInstance(ctx context.Context, id string) error {
	return nil
}
func (m *MockInstanceService) StopInstance(ctx context.Context, id string) error {
	return nil
}
func (m *MockInstanceService) PauseInstance(ctx context.Context, id string) error {
	return nil
}
func (m *MockInstanceService) ResumeInstance(ctx context.Context, id string) error {
	return nil
}
func (m *MockInstanceService) ListInstances(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	args := m.Called(ctx, tagFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *MockInstanceService) GetInstanceLogs(ctx context.Context, id string) (string, error) {
	return "", nil
}
func (m *MockInstanceService) GetInstanceStats(ctx context.Context, id string) (*domain.InstanceStats, error) {
	return nil, nil
}
func (m *MockInstanceService) GetConsoleURL(ctx context.Context, id string) (string, error) {
	return "", nil
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
	var r0 map[string]string
	if args.Get(0) != nil {
		r0, _ = args.Get(0).(map[string]string)
	}
	return r0, args.Error(1)
}
func (m *MockInstanceService) SetTags(ctx context.Context, id uuid.UUID, labels map[string]string) error {
	return m.Called(ctx, id, labels).Error(0)
}
func (m *MockInstanceService) RemoveTag(ctx context.Context, id uuid.UUID, key string) error {
	return m.Called(ctx, id, key).Error(0)
}

type MockClusterRepo struct{ mock.Mock }

func (m *MockClusterRepo) Create(ctx context.Context, c *domain.Cluster) error { return nil }
func (m *MockClusterRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	return nil, nil
}
func (m *MockClusterRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Cluster, error) {
	return nil, nil
}
func (m *MockClusterRepo) ListAll(ctx context.Context) ([]*domain.Cluster, error) {
	return nil, nil
}
func (m *MockClusterRepo) Update(ctx context.Context, c *domain.Cluster) error {
	return m.Called(ctx, c).Error(0)
}
func (m *MockClusterRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *MockClusterRepo) AddNode(ctx context.Context, n *domain.ClusterNode) error {
	return m.Called(ctx, n).Error(0)
}
func (m *MockClusterRepo) GetNodes(ctx context.Context, clusterID uuid.UUID) ([]*domain.ClusterNode, error) {
	args := m.Called(ctx, clusterID)
	r0, _ := args.Get(0).([]*domain.ClusterNode)
	return r0, args.Error(1)
}
func (m *MockClusterRepo) DeleteNode(ctx context.Context, nodeID uuid.UUID) error {
	return m.Called(ctx, nodeID).Error(0)
}
func (m *MockClusterRepo) UpdateNode(ctx context.Context, n *domain.ClusterNode) error {
	return m.Called(ctx, n).Error(0)
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

type MockSecurityGroupService struct{ mock.Mock }

func (m *MockSecurityGroupService) CreateGroup(ctx context.Context, vpcID uuid.UUID, name, description string) (*domain.SecurityGroup, error) {
	args := m.Called(ctx, vpcID, name, description)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.SecurityGroup)
	return r0, args.Error(1)
}
func (m *MockSecurityGroupService) GetGroup(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.SecurityGroup, error) {
	args := m.Called(ctx, idOrName, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.SecurityGroup)
	return r0, args.Error(1)
}
func (m *MockSecurityGroupService) ListGroups(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error) {
	args := m.Called(ctx, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.SecurityGroup)
	return r0, args.Error(1)
}
func (m *MockSecurityGroupService) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockSecurityGroupService) AddRule(ctx context.Context, idOrName string, rule domain.SecurityRule) (*domain.SecurityRule, error) {
	args := m.Called(ctx, idOrName, rule)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.SecurityRule)
	return r0, args.Error(1)
}
func (m *MockSecurityGroupService) RemoveRule(ctx context.Context, ruleID uuid.UUID) error {
	return m.Called(ctx, ruleID).Error(0)
}
func (m *MockSecurityGroupService) AttachToInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	return m.Called(ctx, instanceID, groupID).Error(0)
}
func (m *MockSecurityGroupService) DetachFromInstance(ctx context.Context, instanceID, groupID uuid.UUID) error {
	return m.Called(ctx, instanceID, groupID).Error(0)
}

type MockStorageService struct{ mock.Mock }

func (m *MockStorageService) Upload(ctx context.Context, bucket, key string, r io.Reader, providedChecksum string) (*domain.Object, error) {
	return nil, nil
}
func (m *MockStorageService) Download(ctx context.Context, bucket, key string) (io.ReadCloser, *domain.Object, error) {
	return nil, nil, nil
}
func (m *MockStorageService) ListObjects(ctx context.Context, bucket string) ([]*domain.Object, error) {
	return nil, nil
}
func (m *MockStorageService) DeleteObject(ctx context.Context, bucket, key string) error {
	return nil
}
func (m *MockStorageService) DownloadVersion(ctx context.Context, bucket, key, versionID string) (io.ReadCloser, *domain.Object, error) {
	return nil, nil, nil
}
func (m *MockStorageService) ListVersions(ctx context.Context, bucket, key string) ([]*domain.Object, error) {
	return nil, nil
}
func (m *MockStorageService) DeleteVersion(ctx context.Context, bucket, key, versionID string) error {
	return nil
}
func (m *MockStorageService) CreateBucket(ctx context.Context, name string, isPublic bool) (*domain.Bucket, error) {
	return &domain.Bucket{Name: name}, nil
}
func (m *MockStorageService) GetBucket(ctx context.Context, name string) (*domain.Bucket, error) {
	return &domain.Bucket{Name: name}, nil
}
func (m *MockStorageService) DeleteBucket(ctx context.Context, name string, force bool) error {
	return nil
}
func (m *MockStorageService) ListBuckets(ctx context.Context) ([]*domain.Bucket, error) {
	return nil, nil
}
func (m *MockStorageService) GetClusterStatus(ctx context.Context) (*domain.StorageCluster, error) {
	return nil, nil
}
func (m *MockStorageService) SetBucketVersioning(ctx context.Context, name string, enabled bool) error {
	return nil
}
func (m *MockStorageService) GeneratePresignedURL(ctx context.Context, bucket, key, method string, expiry time.Duration) (*domain.PresignedURL, error) {
	return nil, nil
}

func (m *MockStorageService) CreateMultipartUpload(ctx context.Context, bucket, key string) (*domain.MultipartUpload, error) {
	return nil, nil
}
func (m *MockStorageService) UploadPart(ctx context.Context, uploadID uuid.UUID, partNumber int, r io.Reader, providedChecksum string) (*domain.Part, error) {
	return nil, nil
}
func (m *MockStorageService) CompleteMultipartUpload(ctx context.Context, uploadID uuid.UUID) (*domain.Object, error) {
	return nil, nil
}
func (m *MockStorageService) AbortMultipartUpload(ctx context.Context, uploadID uuid.UUID) error {
	return nil
}
func (m *MockStorageService) CleanupDeleted(ctx context.Context, limit int) (int, error) {
	return 0, nil
}
func (m *MockStorageService) CleanupPendingUploads(ctx context.Context, olderThan time.Duration, limit int) (int, error) {
	return 0, nil
}

type MockLBService struct{ mock.Mock }

func (m *MockLBService) Create(ctx context.Context, name string, vpcID uuid.UUID, port int, algo string, idempotencyKey string) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, name, vpcID, port, algo, idempotencyKey)
	r0, _ := args.Get(0).(*domain.LoadBalancer)
	return r0, args.Error(1)
}
func (m *MockLBService) Get(ctx context.Context, idOrName string) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.LoadBalancer)
	return r0, args.Error(1)
}
func (m *MockLBService) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	return nil, nil
}
func (m *MockLBService) Delete(ctx context.Context, idOrName string) error {
	return m.Called(ctx, idOrName).Error(0)
}
func (m *MockLBService) AddTarget(ctx context.Context, lbID, instanceID uuid.UUID, port int, weight int) error {
	return m.Called(ctx, lbID, instanceID, port, weight).Error(0)
}
func (m *MockLBService) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error {
	return nil
}
func (m *MockLBService) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	return nil, nil
}

func TestKubeadmProvisionerDeprovision(t *testing.T) {
	ctx := context.Background()
	instSvc := new(MockInstanceService)
	repo := new(MockClusterRepo)
	secretSvc := new(mocks.SecretService)
	sgSvc := new(MockSecurityGroupService)
	storageSvc := new(MockStorageService)
	lbSvc := new(MockLBService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	p := k8s.NewKubeadmProvisioner(instSvc, repo, secretSvc, sgSvc, storageSvc, lbSvc, logger)

	cluster := &domain.Cluster{ID: uuid.New()}
	nodeID := uuid.New()
	instanceID := uuid.New()
	nodes := []*domain.ClusterNode{
		{ID: nodeID, InstanceID: instanceID},
	}

	repo.On("GetNodes", ctx, cluster.ID).Return(nodes, nil)
	instSvc.On("TerminateInstance", ctx, instanceID.String()).Return(nil)
	repo.On("DeleteNode", ctx, nodeID).Return(nil)
	sgSvc.On("GetGroup", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("not found"))

	err := p.Deprovision(ctx, cluster)

	assert.NoError(t, err)
}

func TestKubeadmProvisionerProvisionHA(t *testing.T) {
	t.Setenv("CLOUD_API_KEY", "test-key")
	t.Setenv("CLOUD_API_URL", "https://api.local")

	ctx := context.Background()
	instSvc := new(MockInstanceService)
	repo := new(MockClusterRepo)
	secretSvc := new(mocks.SecretService)
	sgSvc := new(MockSecurityGroupService)
	storageSvc := new(MockStorageService)
	lbSvc := new(MockLBService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	p := k8s.NewKubeadmProvisioner(instSvc, repo, secretSvc, sgSvc, storageSvc, lbSvc, logger)

	cluster := &domain.Cluster{
		ID:        uuid.New(),
		Name:      "ha-cluster",
		VpcID:     uuid.New(),
		Version:   "v1.29.0",
		HAEnabled: true,
	}

	repo.On("Update", mock.Anything, mock.Anything).Return(nil).Maybe()
	repo.On("AddNode", mock.Anything, mock.Anything).Return(nil).Maybe()
	repo.On("UpdateNode", mock.Anything, mock.Anything).Return(nil).Maybe()
	sgSvc.On("GetGroup", mock.Anything, mock.Anything, mock.Anything).Return(&domain.SecurityGroup{ID: uuid.New()}, nil)
	sgSvc.On("AttachToInstance", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	secretSvc.On("Encrypt", mock.Anything, mock.Anything, mock.Anything).Return("encrypted", nil).Maybe()

	// 1. LB Creation
	lbID := uuid.New()
	lb := &domain.LoadBalancer{ID: lbID, IP: "10.0.0.100"}
	lbSvc.On("Create", mock.Anything, mock.Anything, cluster.VpcID, 6443, "round-robin", mock.Anything).Return(lb, nil)

	// 2. 3 Master Nodes Creation
	var allNodes []*domain.ClusterNode
	for i := 0; i < 3; i++ {
		nodeID := uuid.New()
		instance := &domain.Instance{ID: nodeID, PrivateIP: fmt.Sprintf("10.0.0.%d", 10+i)}

		allNodes = append(allNodes, &domain.ClusterNode{
			ID:         uuid.New(),
			ClusterID:  cluster.ID,
			InstanceID: nodeID,
			Role:       domain.NodeRoleControlPlane,
		})

		instSvc.On("LaunchInstanceWithOptions", mock.Anything, mock.Anything).Return(instance, nil).Once()
		instSvc.On("GetInstance", mock.Anything, nodeID.String()).Return(instance, nil).Maybe()
		lbSvc.On("AddTarget", mock.Anything, lbID, nodeID, 6443, 10).Return(nil)
	}
	repo.On("GetNodes", mock.Anything, cluster.ID).Return(allNodes, nil).Maybe()

	// General Exec matches
	kubeadmOutput := `
kubeadm join 10.0.0.100:6443 --token abc --discovery-token-ca-cert-hash sha256:123

kubeadm join 10.0.0.100:6443 --token abc --discovery-token-ca-cert-hash sha256:123 --control-plane --certificate-key xyz
`
	// Collect instances for ListInstances mock
	instances := make([]*domain.Instance, 0, len(allNodes))
	for _, n := range allNodes {
		inst, _ := instSvc.GetInstance(ctx, n.InstanceID.String())
		instances = append(instances, inst)
	}

	instSvc.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(kubeadmOutput, nil).Maybe()
	instSvc.On("ListInstances", mock.Anything, mock.Anything).Return(instances, nil).Maybe()

	err := p.Provision(ctx, cluster)

	assert.NoError(t, err)
}
