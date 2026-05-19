package k8s

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockSecretSvc struct{ mock.Mock }

func (m *mockSecretSvc) CreateSecret(ctx context.Context, n, v, d string) (*domain.Secret, error) {
	return nil, nil
}
func (m *mockSecretSvc) GetSecret(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	return nil, nil
}
func (m *mockSecretSvc) GetSecretByName(ctx context.Context, n string) (*domain.Secret, error) {
	return nil, nil
}
func (m *mockSecretSvc) ListSecrets(ctx context.Context) ([]*domain.Secret, error) { return nil, nil }
func (m *mockSecretSvc) DeleteSecret(ctx context.Context, id uuid.UUID) error      { return nil }
func (m *mockSecretSvc) Encrypt(ctx context.Context, u uuid.UUID, p string) (string, error) {
	return "", nil
}
func (m *mockSecretSvc) Decrypt(ctx context.Context, u uuid.UUID, c string) (string, error) {
	return "", nil
}

type mockSGSvc struct{ mock.Mock }

func (m *mockSGSvc) CreateGroup(ctx context.Context, v uuid.UUID, n, d string) (*domain.SecurityGroup, error) {
	return nil, nil
}
func (m *mockSGSvc) GetGroup(ctx context.Context, id string, v uuid.UUID) (*domain.SecurityGroup, error) {
	return nil, nil
}
func (m *mockSGSvc) ListGroups(ctx context.Context, v uuid.UUID) ([]*domain.SecurityGroup, error) {
	return nil, nil
}
func (m *mockSGSvc) DeleteGroup(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockSGSvc) AddRule(ctx context.Context, idOrName string, r domain.SecurityRule) (*domain.SecurityRule, error) {
	return nil, nil
}
func (m *mockSGSvc) RemoveRule(ctx context.Context, ruleID uuid.UUID) error               { return nil }
func (m *mockSGSvc) AttachToInstance(ctx context.Context, instID, sgID uuid.UUID) error   { return nil }
func (m *mockSGSvc) DetachFromInstance(ctx context.Context, instID, sgID uuid.UUID) error { return nil }

type mockStorageSvc struct{ mock.Mock }

func (m *mockStorageSvc) Upload(ctx context.Context, b, k string, r io.Reader, providedChecksum string) (*domain.Object, error) {
	return nil, nil
}
func (m *mockStorageSvc) Download(ctx context.Context, b, k string) (io.ReadCloser, *domain.Object, error) {
	return nil, nil, nil
}
func (m *mockStorageSvc) ListObjects(ctx context.Context, b string) ([]*domain.Object, error) {
	return nil, nil
}
func (m *mockStorageSvc) DeleteObject(ctx context.Context, b, k string) error { return nil }
func (m *mockStorageSvc) DownloadVersion(ctx context.Context, b, k, v string) (io.ReadCloser, *domain.Object, error) {
	return nil, nil, nil
}
func (m *mockStorageSvc) ListVersions(ctx context.Context, b, k string) ([]*domain.Object, error) {
	return nil, nil
}
func (m *mockStorageSvc) DeleteVersion(ctx context.Context, b, k, v string) error { return nil }
func (m *mockStorageSvc) CreateBucket(ctx context.Context, n string, p bool) (*domain.Bucket, error) {
	return nil, nil
}
func (m *mockStorageSvc) GetBucket(ctx context.Context, n string) (*domain.Bucket, error) {
	return nil, nil
}
func (m *mockStorageSvc) DeleteBucket(ctx context.Context, n string, force bool) error    { return nil }
func (m *mockStorageSvc) ListBuckets(ctx context.Context) ([]*domain.Bucket, error)       { return nil, nil }
func (m *mockStorageSvc) SetBucketVersioning(ctx context.Context, n string, e bool) error { return nil }
func (m *mockStorageSvc) GetClusterStatus(ctx context.Context) (*domain.StorageCluster, error) {
	return nil, nil
}
func (m *mockStorageSvc) CreateMultipartUpload(ctx context.Context, b, k string) (*domain.MultipartUpload, error) {
	return nil, nil
}
func (m *mockStorageSvc) UploadPart(ctx context.Context, u uuid.UUID, n int, r io.Reader, providedChecksum string) (*domain.Part, error) {
	return nil, nil
}
func (m *mockStorageSvc) CompleteMultipartUpload(ctx context.Context, u uuid.UUID) (*domain.Object, error) {
	return nil, nil
}
func (m *mockStorageSvc) AbortMultipartUpload(ctx context.Context, uploadID uuid.UUID) error {
	return nil
}
func (m *mockStorageSvc) CleanupDeleted(ctx context.Context, limit int) (int, error) { return 0, nil }
func (m *mockStorageSvc) CleanupPendingUploads(ctx context.Context, olderThan time.Duration, limit int) (int, error) {
	return 0, nil
}
func (m *mockStorageSvc) GeneratePresignedURL(ctx context.Context, bucket, key, method string, expiry time.Duration) (*domain.PresignedURL, error) {
	return nil, nil
}

type mockLBSvc struct{ mock.Mock }

func (m *mockLBSvc) Create(ctx context.Context, n string, v uuid.UUID, p int, a string, i string) (*domain.LoadBalancer, error) {
	return nil, nil
}
func (m *mockLBSvc) Get(ctx context.Context, idOrName string) (*domain.LoadBalancer, error) {
	return nil, nil
}
func (m *mockLBSvc) List(ctx context.Context) ([]*domain.LoadBalancer, error) { return nil, nil }
func (m *mockLBSvc) Delete(ctx context.Context, idOrName string) error        { return nil }
func (m *mockLBSvc) AddTarget(ctx context.Context, lbID, instanceID uuid.UUID, port int, weight int) error {
	return nil
}
func (m *mockLBSvc) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error { return nil }
func (m *mockLBSvc) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	return nil, nil
}

func setupProvisionerUnit(t *testing.T) (*KubeadmProvisioner, *mockInstanceService, *mockClusterRepo) {
	t.Helper()
	mockInst := new(mockInstanceService)
	mockRepo := new(mockClusterRepo)
	mockSecret := new(mockSecretSvc)
	mockSG := new(mockSGSvc)
	mockStorage := new(mockStorageSvc)
	mockLB := new(mockLBSvc)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	p := NewKubeadmProvisioner(mockInst, mockRepo, mockSecret, mockSG, mockStorage, mockLB, logger)
	return p, mockInst, mockRepo
}

func TestKubeadmProvisionerSimpleOps(t *testing.T) {
	p, _, _ := setupProvisionerUnit(t)
	ctx := context.Background()
	cluster := &domain.Cluster{ID: uuid.New(), Status: domain.ClusterStatusProvisioning}

	t.Run("GetStatus", func(t *testing.T) {
		status, err := p.GetStatus(ctx, cluster)
		require.NoError(t, err)
		assert.Equal(t, domain.ClusterStatusProvisioning, status)
	})

	t.Run("Repair", func(t *testing.T) {
		err := p.Repair(ctx, cluster)
		require.NoError(t, err)
	})
}

func TestKubeadmProvisionerScale(t *testing.T) {
	p, mockInst, _ := setupProvisionerUnit(t)
	ctx := context.Background()
	cluster := &domain.Cluster{
		ID:              uuid.New(),
		UserID:          uuid.New(),
		Name:            "test-cluster",
		WorkerCount:     1,
		ControlPlaneIPs: []string{"10.0.0.1"},
	}

	t.Run("Scale Up", func(t *testing.T) {
		// Mock launching a new instance for the worker node
		mockInst.On("LaunchInstanceWithOptions", ctx, mock.Anything).Return(&domain.Instance{ID: uuid.New()}, nil).Once()

		err := p.Scale(ctx, cluster)
		require.NoError(t, err)
	})
}

func TestKubeadmProvisionerUpgrade(t *testing.T) {
	p, mockInst, mockRepo := setupProvisionerUnit(t)
	ctx := context.Background()
	cluster := &domain.Cluster{
		ID:              uuid.New(),
		Name:            "test-cluster",
		ControlPlaneIPs: []string{"10.0.0.1"},
	}
	nodes := []*domain.ClusterNode{
		{ID: uuid.New(), InstanceID: uuid.New(), Role: domain.NodeRoleControlPlane},
	}
	inst := &domain.Instance{ID: nodes[0].InstanceID, PrivateIP: "10.0.0.1"}

	mockRepo.On("GetNodes", ctx, cluster.ID).Return(nodes, nil).Once()
	mockInst.On("GetInstance", ctx, nodes[0].InstanceID.String()).Return(inst, nil).Once()
	mockInst.On("ListInstances", ctx, mock.Anything).Return([]*domain.Instance{inst}, nil).Once()
	mockInst.On("Exec", ctx, inst.ID.String(), mock.Anything).Return("success", nil)

	err := p.Upgrade(ctx, cluster, "v1.29.0")
	require.NoError(t, err)
}
