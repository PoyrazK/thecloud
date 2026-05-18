// Package noop provides no-op implementations of interfaces for testing and benchmarks.
package noop

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

// NoopInstanceRepository is a no-op instance repository.
type NoopInstanceRepository struct{}

func (r *NoopInstanceRepository) Create(ctx context.Context, i *domain.Instance) error { return nil }
func (r *NoopInstanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	return &domain.Instance{ID: id}, nil
}
func (r *NoopInstanceRepository) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	return &domain.Instance{ID: uuid.New(), Name: name}, nil
}
func (r *NoopInstanceRepository) List(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	return []*domain.Instance{}, nil
}
func (r *NoopInstanceRepository) ListAll(ctx context.Context) ([]*domain.Instance, error) {
	return []*domain.Instance{}, nil
}
func (r *NoopInstanceRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Instance, error) {
	return []*domain.Instance{}, nil
}
func (r *NoopInstanceRepository) ListBySubnet(ctx context.Context, subnetID uuid.UUID) ([]*domain.Instance, error) {
	return []*domain.Instance{}, nil
}
func (r *NoopInstanceRepository) ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.Instance, error) {
	return []*domain.Instance{}, nil
}
func (r *NoopInstanceRepository) Update(ctx context.Context, i *domain.Instance) error { return nil }

func (r *NoopInstanceRepository) Delete(ctx context.Context, id uuid.UUID) error { return nil }

// NoopVpcRepository
type NoopVpcRepository struct{}

func (r *NoopVpcRepository) Create(ctx context.Context, v *domain.VPC) error { return nil }
func (r *NoopVpcRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.VPC, error) {
	return &domain.VPC{ID: id}, nil
}
func (r *NoopVpcRepository) GetByName(ctx context.Context, name string) (*domain.VPC, error) {
	return &domain.VPC{ID: uuid.New(), Name: name}, nil
}
func (r *NoopVpcRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.VPC, error) {
	return nil, errors.New(errors.NotFound, "idempotency key not found")
}
func (r *NoopVpcRepository) List(ctx context.Context) ([]*domain.VPC, error) {
	return []*domain.VPC{}, nil
}
func (r *NoopVpcRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.VPC, error) {
	return []*domain.VPC{}, nil
}
func (r *NoopVpcRepository) Update(ctx context.Context, v *domain.VPC) error { return nil }
func (r *NoopVpcRepository) Delete(ctx context.Context, id uuid.UUID) error  { return nil }

// NoopSubnetRepository
type NoopSubnetRepository struct{}

func (r *NoopSubnetRepository) Create(ctx context.Context, s *domain.Subnet) error { return nil }
func (r *NoopSubnetRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Subnet, error) {
	return &domain.Subnet{ID: id}, nil
}
func (r *NoopSubnetRepository) GetByName(ctx context.Context, vpcID uuid.UUID, name string) (*domain.Subnet, error) {
	return &domain.Subnet{ID: uuid.New(), Name: name, VPCID: vpcID}, nil
}
func (r *NoopSubnetRepository) ListByVpcID(ctx context.Context, vpcID uuid.UUID) ([]*domain.Subnet, error) {
	return []*domain.Subnet{}, nil
}
func (r *NoopSubnetRepository) ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.Subnet, error) {
	return []*domain.Subnet{}, nil
}
func (r *NoopSubnetRepository) Update(ctx context.Context, s *domain.Subnet) error { return nil }
func (r *NoopSubnetRepository) Delete(ctx context.Context, id uuid.UUID) error     { return nil }

// NoopInstanceTypeRepository
type NoopInstanceTypeRepository struct{}

func (r *NoopInstanceTypeRepository) Create(ctx context.Context, t *domain.InstanceType) (*domain.InstanceType, error) {
	return t, nil
}
func (r *NoopInstanceTypeRepository) GetByID(ctx context.Context, id string) (*domain.InstanceType, error) {
	return &domain.InstanceType{ID: id}, nil
}
func (r *NoopInstanceTypeRepository) List(ctx context.Context) ([]*domain.InstanceType, error) {
	return []*domain.InstanceType{}, nil
}
func (r *NoopInstanceTypeRepository) Update(ctx context.Context, t *domain.InstanceType) (*domain.InstanceType, error) {
	return t, nil
}
func (r *NoopInstanceTypeRepository) Delete(ctx context.Context, id string) error { return nil }

// NoopComputeBackend is a no-op compute backend.
type NoopComputeBackend struct{}

// Interface assertion
var _ ports.ComputeBackend = (*NoopComputeBackend)(nil)

func NewNoopComputeBackend() *NoopComputeBackend {
	return &NoopComputeBackend{}
}

func (b *NoopComputeBackend) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	return uuid.New().String(), []string{}, nil
}
func (b *NoopComputeBackend) StartInstance(ctx context.Context, id string) error  { return nil }
func (b *NoopComputeBackend) StopInstance(ctx context.Context, id string) error   { return nil }
func (b *NoopComputeBackend) DeleteInstance(ctx context.Context, id string) error { return nil }
func (b *NoopComputeBackend) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (b *NoopComputeBackend) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (b *NoopComputeBackend) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	return 80, nil
}
func (b *NoopComputeBackend) GetInstanceIP(ctx context.Context, id string) (string, error) {
	return "10.0.0.2", nil
}
func (b *NoopComputeBackend) GetConsoleURL(ctx context.Context, id string) (string, error) {
	return "http://console", nil
}
func (b *NoopComputeBackend) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", nil
}
func (b *NoopComputeBackend) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	return uuid.New().String(), []string{}, nil
}
func (b *NoopComputeBackend) WaitTask(ctx context.Context, id string) (int64, error) {
	return 0, nil
}
func (b *NoopComputeBackend) CreateNetwork(ctx context.Context, name string) (string, error) {
	return uuid.New().String(), nil
}
func (b *NoopComputeBackend) DeleteNetwork(ctx context.Context, id string) error { return nil }
func (b *NoopComputeBackend) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	return "/dev/vdb", "", nil
}
func (b *NoopComputeBackend) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	return "", nil
}
func (b *NoopComputeBackend) Ping(ctx context.Context) error                      { return nil }
func (b *NoopComputeBackend) Type() string                                        { return "noop" }
func (b *NoopComputeBackend) PauseInstance(ctx context.Context, id string) error  { return nil }
func (b *NoopComputeBackend) ResumeInstance(ctx context.Context, id string) error { return nil }
func (b *NoopComputeBackend) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	return nil
}
func (b *NoopComputeBackend) CreateSnapshot(ctx context.Context, id, name string) error  { return nil }
func (b *NoopComputeBackend) RestoreSnapshot(ctx context.Context, id, name string) error { return nil }
func (b *NoopComputeBackend) DeleteSnapshot(ctx context.Context, id, name string) error  { return nil }
func (b *NoopComputeBackend) ResetCircuitBreaker()                                       {}

// NoopDNSService is a no-op DNS service.
type NoopDNSService struct{}

func (s *NoopDNSService) RegisterInstance(ctx context.Context, i *domain.Instance) error { return nil }
func (s *NoopDNSService) UnregisterInstance(ctx context.Context, i *domain.Instance) error {
	return nil
}

// NoopLogService is a no-op log service.
type NoopLogService struct{}

func (s *NoopLogService) StreamLogs(ctx context.Context, instanceID string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (s *NoopLogService) GetLogs(ctx context.Context, instanceID string) (string, error) {
	return "", nil
}

// NoopEventService is a no-op event service.
type NoopEventService struct{}

func (e *NoopEventService) RecordEvent(ctx context.Context, action, resourceID, resourceType string, metadata map[string]interface{}) error {
	return nil
}
func (e *NoopEventService) ListEvents(ctx context.Context, limit int) ([]*domain.Event, error) {
	return []*domain.Event{}, nil
}

// NoopAuditService implements ports.AuditService.
type NoopAuditService struct{}

func (a *NoopAuditService) Log(ctx context.Context, userID uuid.UUID, action, resourceType, resourceID string, details map[string]interface{}) error {
	return nil
}
func (a *NoopAuditService) ListLogs(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	return []*domain.AuditLog{}, nil
}

// NoopClusterRepository is a no-op cluster repository.
type NoopClusterRepository struct{}

func (r *NoopClusterRepository) Create(ctx context.Context, c *domain.Cluster) error { return nil }
func (r *NoopClusterRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	return &domain.Cluster{ID: id}, nil
}
func (r *NoopClusterRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Cluster, error) {
	return []*domain.Cluster{}, nil
}
func (r *NoopClusterRepository) ListAll(ctx context.Context) ([]*domain.Cluster, error) {
	return []*domain.Cluster{}, nil
}
func (r *NoopClusterRepository) Update(ctx context.Context, c *domain.Cluster) error      { return nil }
func (r *NoopClusterRepository) Delete(ctx context.Context, id uuid.UUID) error           { return nil }
func (r *NoopClusterRepository) AddNode(ctx context.Context, n *domain.ClusterNode) error { return nil }
func (r *NoopClusterRepository) GetNodes(ctx context.Context, clusterID uuid.UUID) ([]*domain.ClusterNode, error) {
	return []*domain.ClusterNode{}, nil
}
func (r *NoopClusterRepository) UpdateNode(ctx context.Context, n *domain.ClusterNode) error {
	return nil
}
func (r *NoopClusterRepository) DeleteNode(ctx context.Context, id uuid.UUID) error { return nil }

// NoopClusterService
type NoopClusterService struct{}

func (s *NoopClusterService) CreateCluster(ctx context.Context, params ports.CreateClusterParams) (*domain.Cluster, error) {
	return &domain.Cluster{ID: uuid.New(), Name: params.Name}, nil
}
func (s *NoopClusterService) DeleteCluster(ctx context.Context, id uuid.UUID) error { return nil }
func (s *NoopClusterService) GetCluster(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	return &domain.Cluster{ID: id}, nil
}
func (s *NoopClusterService) ListClusters(ctx context.Context, userID uuid.UUID) ([]*domain.Cluster, error) {
	return []*domain.Cluster{}, nil
}
func (s *NoopClusterService) GetKubeconfig(ctx context.Context, id uuid.UUID, role string) (string, error) {
	return "", nil
}
func (s *NoopClusterService) RepairCluster(ctx context.Context, id uuid.UUID) error { return nil }
func (s *NoopClusterService) ScaleCluster(ctx context.Context, id uuid.UUID, workers int) error {
	return nil
}
func (s *NoopClusterService) GetClusterHealth(ctx context.Context, id uuid.UUID) (*ports.ClusterHealth, error) {
	return &ports.ClusterHealth{Status: domain.ClusterStatusRunning}, nil
}
func (s *NoopClusterService) UpgradeCluster(ctx context.Context, id uuid.UUID, version string) error {
	return nil
}
func (s *NoopClusterService) RotateSecrets(ctx context.Context, id uuid.UUID) error { return nil }
func (s *NoopClusterService) CreateBackup(ctx context.Context, id uuid.UUID) error  { return nil }
func (s *NoopClusterService) RestoreBackup(ctx context.Context, id uuid.UUID, backupPath string) error {
	return nil
}
func (s *NoopClusterService) SetBackupPolicy(ctx context.Context, id uuid.UUID, params ports.BackupPolicyParams) error {
	return nil
}

func (s *NoopClusterService) AddNodeGroup(ctx context.Context, clusterID uuid.UUID, params ports.NodeGroupParams) (*domain.NodeGroup, error) {
	return &domain.NodeGroup{ClusterID: clusterID, Name: params.Name}, nil
}
func (s *NoopClusterService) UpdateNodeGroup(ctx context.Context, clusterID uuid.UUID, name string, params ports.UpdateNodeGroupParams) (*domain.NodeGroup, error) {
	return &domain.NodeGroup{ClusterID: clusterID, Name: name}, nil
}
func (s *NoopClusterService) DeleteNodeGroup(ctx context.Context, clusterID uuid.UUID, name string) error {
	return nil
}

func (s *NoopClusterService) AddNode(ctx context.Context, clusterID uuid.UUID, role string) (*domain.ClusterNode, error) {
	return &domain.ClusterNode{ID: uuid.New(), ClusterID: clusterID, Role: domain.NodeRole(role)}, nil
}
func (s *NoopClusterService) RemoveNode(ctx context.Context, clusterID, nodeID uuid.UUID) error {
	return nil
}

// NoopSecretService is a no-op secret service.
type NoopSecretService struct{}

func (s *NoopSecretService) CreateSecret(ctx context.Context, name, value, desc string) (*domain.Secret, error) {
	return &domain.Secret{ID: uuid.New(), Name: name}, nil
}
func (s *NoopSecretService) GetSecret(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	return &domain.Secret{ID: id}, nil
}
func (s *NoopSecretService) GetSecretByName(ctx context.Context, name string) (*domain.Secret, error) {
	return &domain.Secret{ID: uuid.New(), Name: name}, nil
}
func (s *NoopSecretService) ListSecrets(ctx context.Context) ([]*domain.Secret, error) {
	return []*domain.Secret{}, nil
}
func (s *NoopSecretService) DeleteSecret(ctx context.Context, id uuid.UUID) error { return nil }
func (s *NoopSecretService) Encrypt(ctx context.Context, userID uuid.UUID, plain string) (string, error) {
	return plain, nil
}
func (s *NoopSecretService) Decrypt(ctx context.Context, userID uuid.UUID, cipher string) (string, error) {
	return cipher, nil
}

// NoopSecurityGroupService is a no-op security group service.
type NoopSecurityGroupService struct{}

func (s *NoopSecurityGroupService) CreateGroup(ctx context.Context, vpcID uuid.UUID, name, desc string) (*domain.SecurityGroup, error) {
	return &domain.SecurityGroup{ID: uuid.New(), Name: name}, nil
}
func (s *NoopSecurityGroupService) GetGroup(ctx context.Context, idOrName string, vpcID uuid.UUID) (*domain.SecurityGroup, error) {
	return &domain.SecurityGroup{ID: uuid.New(), Name: idOrName}, nil
}
func (s *NoopSecurityGroupService) ListGroups(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error) {
	return []*domain.SecurityGroup{}, nil
}
func (s *NoopSecurityGroupService) DeleteGroup(ctx context.Context, id uuid.UUID) error { return nil }
func (s *NoopSecurityGroupService) AddRule(ctx context.Context, groupID uuid.UUID, rule domain.SecurityRule) (*domain.SecurityRule, error) {
	return &rule, nil
}
func (s *NoopSecurityGroupService) RemoveRule(ctx context.Context, ruleID uuid.UUID) error {
	return nil
}
func (s *NoopSecurityGroupService) AttachToInstance(ctx context.Context, instID, groupID uuid.UUID) error {
	return nil
}
func (s *NoopSecurityGroupService) DetachFromInstance(ctx context.Context, instID, groupID uuid.UUID) error {
	return nil
}

// NoopStorageService is a no-op storage service.
type NoopStorageService struct{}

func (s *NoopStorageService) Upload(ctx context.Context, bucket, key string, r io.Reader, providedChecksum string) (*domain.Object, error) {
	return &domain.Object{Key: key}, nil
}
func (s *NoopStorageService) Download(ctx context.Context, bucket, key string) (io.ReadCloser, *domain.Object, error) {
	return io.NopCloser(strings.NewReader("data")), &domain.Object{Bucket: bucket, Key: key}, nil
}
func (s *NoopStorageService) ListObjects(ctx context.Context, bucket string) ([]*domain.Object, error) {
	return []*domain.Object{}, nil
}
func (s *NoopStorageService) DeleteObject(ctx context.Context, bucket, key string) error { return nil }
func (s *NoopStorageService) DownloadVersion(ctx context.Context, bucket, key, versionID string) (io.ReadCloser, *domain.Object, error) {
	return io.NopCloser(strings.NewReader("data")), &domain.Object{Bucket: bucket, Key: key, VersionID: versionID}, nil
}
func (s *NoopStorageService) ListVersions(ctx context.Context, bucket, key string) ([]*domain.Object, error) {
	return []*domain.Object{}, nil
}
func (s *NoopStorageService) DeleteVersion(ctx context.Context, bucket, key, versionID string) error {
	return nil
}
func (s *NoopStorageService) CreateBucket(ctx context.Context, name string, isPublic bool) (*domain.Bucket, error) {
	return &domain.Bucket{Name: name, IsPublic: isPublic}, nil
}
func (s *NoopStorageService) GetBucket(ctx context.Context, name string) (*domain.Bucket, error) {
	return &domain.Bucket{Name: name}, nil
}
func (s *NoopStorageService) DeleteBucket(ctx context.Context, name string, force bool) error {
	return nil
}
func (s *NoopStorageService) ListBuckets(ctx context.Context) ([]*domain.Bucket, error) {
	return []*domain.Bucket{}, nil
}
func (s *NoopStorageService) SetBucketVersioning(ctx context.Context, name string, enabled bool) error {
	return nil
}
func (s *NoopStorageService) GetClusterStatus(ctx context.Context) (*domain.StorageCluster, error) {
	return &domain.StorageCluster{}, nil
}
func (s *NoopStorageService) CreateMultipartUpload(ctx context.Context, bucket, key string) (*domain.MultipartUpload, error) {
	return &domain.MultipartUpload{Bucket: bucket, Key: key}, nil
}
func (s *NoopStorageService) UploadPart(ctx context.Context, uploadID uuid.UUID, partNumber int, r io.Reader, providedChecksum string) (*domain.Part, error) {
	return &domain.Part{UploadID: uploadID, PartNumber: partNumber}, nil
}
func (s *NoopStorageService) CompleteMultipartUpload(ctx context.Context, uploadID uuid.UUID) (*domain.Object, error) {
	return &domain.Object{}, nil
}
func (s *NoopStorageService) AbortMultipartUpload(ctx context.Context, uploadID uuid.UUID) error {
	return nil
}
func (s *NoopStorageService) CleanupDeleted(ctx context.Context, limit int) (int, error) {
	return 0, nil
}
func (s *NoopStorageService) CleanupPendingUploads(ctx context.Context, olderThan time.Duration, limit int) (int, error) {
	return 0, nil
}
func (s *NoopStorageService) GeneratePresignedURL(ctx context.Context, bucket, key, method string, expiry time.Duration) (*domain.PresignedURL, error) {
	return &domain.PresignedURL{}, nil
}

// NoopLBService is a no-op LB service.
type NoopLBService struct{}

func (s *NoopLBService) Create(ctx context.Context, name string, vpcID uuid.UUID, port int, algo string, idempotencyKey string) (*domain.LoadBalancer, error) {
	return &domain.LoadBalancer{ID: uuid.New(), Name: name}, nil
}
func (s *NoopLBService) Get(ctx context.Context, id string) (*domain.LoadBalancer, error) {
	uid, _ := uuid.Parse(id)
	return &domain.LoadBalancer{ID: uid}, nil
}
func (s *NoopLBService) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	return []*domain.LoadBalancer{}, nil
}
func (s *NoopLBService) Delete(ctx context.Context, id string) error { return nil }
func (s *NoopLBService) AddTarget(ctx context.Context, lbID, instanceID uuid.UUID, port int, weight int) error {
	return nil
}
func (s *NoopLBService) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error {
	return nil
}
func (s *NoopLBService) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	return []*domain.LBTarget{}, nil
}

// NoopTaskQueue is a no-op task queue.
type NoopTaskQueue struct{}

func (q *NoopTaskQueue) Enqueue(ctx context.Context, queue string, payload interface{}) error {
	return nil
}
func (q *NoopTaskQueue) Dequeue(ctx context.Context, queue string) (string, error) { return "", nil }

// --- New No-Ops (for benchmarks and system tests) ---

type NoopVolumeRepository struct{}

func (r *NoopVolumeRepository) Create(ctx context.Context, v *domain.Volume) error { return nil }
func (r *NoopVolumeRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Volume, error) {
	return &domain.Volume{ID: id}, nil
}
func (r *NoopVolumeRepository) GetByName(ctx context.Context, name string) (*domain.Volume, error) {
	return &domain.Volume{ID: uuid.New(), Name: name}, nil
}
func (r *NoopVolumeRepository) List(ctx context.Context) ([]*domain.Volume, error) {
	return []*domain.Volume{}, nil
}
func (r *NoopVolumeRepository) ListByInstanceID(ctx context.Context, id uuid.UUID) ([]*domain.Volume, error) {
	return []*domain.Volume{}, nil
}
func (r *NoopVolumeRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Volume, error) {
	return []*domain.Volume{}, nil
}
func (r *NoopVolumeRepository) Update(ctx context.Context, v *domain.Volume) error { return nil }
func (r *NoopVolumeRepository) Delete(ctx context.Context, id uuid.UUID) error     { return nil }

type NoopFunctionRepository struct{}

func (r *NoopFunctionRepository) Create(ctx context.Context, fn *domain.Function) error { return nil }
func (r *NoopFunctionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Function, error) {
	return &domain.Function{ID: id}, nil
}
func (r *NoopFunctionRepository) GetByName(ctx context.Context, userID uuid.UUID, name string) (*domain.Function, error) {
	return &domain.Function{ID: uuid.New(), Name: name}, nil
}
func (r *NoopFunctionRepository) List(ctx context.Context, userID uuid.UUID) ([]*domain.Function, error) {
	return []*domain.Function{}, nil
}
func (r *NoopFunctionRepository) Update(ctx context.Context, id uuid.UUID, u *domain.FunctionUpdate) error {
	return nil
}
func (r *NoopFunctionRepository) Delete(ctx context.Context, id uuid.UUID) error { return nil }
func (r *NoopFunctionRepository) GetInvocations(ctx context.Context, fnID uuid.UUID, limit int) ([]*domain.Invocation, error) {
	return []*domain.Invocation{}, nil
}
func (r *NoopFunctionRepository) CreateInvocation(ctx context.Context, inv *domain.Invocation) error {
	return nil
}

type NoopFileStore struct{}

func (s *NoopFileStore) Write(ctx context.Context, bucket, key string, r io.Reader) (int64, error) {
	return 0, nil
}
func (s *NoopFileStore) Read(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (s *NoopFileStore) Delete(ctx context.Context, bucket, key string) error { return nil }
func (s *NoopFileStore) GetClusterStatus(ctx context.Context) (*domain.StorageCluster, error) {
	return &domain.StorageCluster{}, nil
}
func (s *NoopFileStore) Assemble(ctx context.Context, bucket, key string, parts []string) (int64, error) {
	return 0, nil
}

type NoopDatabaseRepository struct{}

func (r *NoopDatabaseRepository) Create(ctx context.Context, db *domain.Database) error { return nil }
func (r *NoopDatabaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	return &domain.Database{ID: id}, nil
}
func (r *NoopDatabaseRepository) List(ctx context.Context) ([]*domain.Database, error) {
	return []*domain.Database{}, nil
}
func (r *NoopDatabaseRepository) ListReplicas(ctx context.Context, primaryID uuid.UUID) ([]*domain.Database, error) {
	return []*domain.Database{}, nil
}
func (r *NoopDatabaseRepository) Update(ctx context.Context, db *domain.Database) error { return nil }
func (r *NoopDatabaseRepository) Delete(ctx context.Context, id uuid.UUID) error        { return nil }

type NoopUserRepository struct{}

func (r *NoopUserRepository) Create(ctx context.Context, u *domain.User) error { return nil }
func (r *NoopUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return &domain.User{ID: id}, nil
}
func (r *NoopUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}
func (r *NoopUserRepository) Update(ctx context.Context, u *domain.User) error { return nil }
func (r *NoopUserRepository) List(ctx context.Context) ([]*domain.User, error) {
	return []*domain.User{}, nil
}
func (r *NoopUserRepository) Delete(ctx context.Context, id uuid.UUID) error { return nil }

type NoopIdentityService struct{}

func (s *NoopIdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	return &domain.APIKey{ID: uuid.New()}, nil
}
func (s *NoopIdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	return &domain.APIKey{ID: uuid.New()}, nil
}
func (s *NoopIdentityService) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	return &domain.APIKey{ID: id}, nil
}
func (s *NoopIdentityService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	return []*domain.APIKey{}, nil
}
func (s *NoopIdentityService) RevokeKey(ctx context.Context, userID, id uuid.UUID) error { return nil }
func (s *NoopIdentityService) RotateKey(ctx context.Context, userID, id uuid.UUID) (*domain.APIKey, error) {
	return &domain.APIKey{ID: id}, nil
}
func (s *NoopIdentityService) CreateServiceAccount(ctx context.Context, tenantID uuid.UUID, name, role string) (*domain.ServiceAccountWithSecret, error) {
	return &domain.ServiceAccountWithSecret{ServiceAccount: domain.ServiceAccount{ID: uuid.New(), TenantID: tenantID, Name: name, Role: role, Enabled: true}}, nil
}
func (s *NoopIdentityService) GetServiceAccount(ctx context.Context, id uuid.UUID) (*domain.ServiceAccount, error) {
	return &domain.ServiceAccount{ID: id}, nil
}
func (s *NoopIdentityService) ListServiceAccounts(ctx context.Context, tenantID uuid.UUID) ([]*domain.ServiceAccount, error) {
	return []*domain.ServiceAccount{}, nil
}
func (s *NoopIdentityService) UpdateServiceAccount(ctx context.Context, sa *domain.ServiceAccount) error {
	return nil
}
func (s *NoopIdentityService) DeleteServiceAccount(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (s *NoopIdentityService) ValidateClientCredentials(ctx context.Context, clientID, clientSecret string) (string, error) {
	return "noop-token", nil
}
func (s *NoopIdentityService) ValidateAccessToken(ctx context.Context, token string) (*domain.ServiceAccountClaims, error) {
	return &domain.ServiceAccountClaims{ServiceAccountID: uuid.New(), TenantID: uuid.New(), Role: "service"}, nil
}
func (s *NoopIdentityService) RotateServiceAccountSecret(ctx context.Context, saID uuid.UUID) (string, error) {
	return "sa_newsecret", nil
}
func (s *NoopIdentityService) RevokeServiceAccountSecret(ctx context.Context, saID, secretID uuid.UUID) error {
	return nil
}
func (s *NoopIdentityService) ListServiceAccountSecrets(ctx context.Context, saID uuid.UUID) ([]*domain.ServiceAccountSecret, error) {
	return []*domain.ServiceAccountSecret{}, nil
}
func (s *NoopIdentityService) TokenTTL() time.Duration {
	return time.Hour
}

type NoopIdentityRepository struct{}

func (r *NoopIdentityRepository) CreateAPIKey(ctx context.Context, key *domain.APIKey) error {
	return nil
}
func (r *NoopIdentityRepository) GetAPIKeyByKey(ctx context.Context, key string) (*domain.APIKey, error) {
	return &domain.APIKey{Key: key}, nil
}
func (r *NoopIdentityRepository) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	return &domain.APIKey{ID: id}, nil
}
func (r *NoopIdentityRepository) ListAPIKeysByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	return []*domain.APIKey{}, nil
}
func (r *NoopIdentityRepository) DeleteAPIKey(ctx context.Context, id uuid.UUID) error { return nil }

type NoopCacheRepository struct{}

func (r *NoopCacheRepository) Create(ctx context.Context, c *domain.Cache) error { return nil }
func (r *NoopCacheRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Cache, error) {
	return &domain.Cache{ID: id, TenantID: tenantID}, nil
}
func (r *NoopCacheRepository) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*domain.Cache, error) {
	return &domain.Cache{ID: uuid.New(), Name: name, TenantID: tenantID}, nil
}
func (r *NoopCacheRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Cache, error) {
	return []*domain.Cache{}, nil
}
func (r *NoopCacheRepository) Update(ctx context.Context, c *domain.Cache) error        { return nil }
func (r *NoopCacheRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error { return nil }

type NoopLBRepository struct {
	mu             sync.Mutex
	idempotencyMap map[string]uuid.UUID
}

func (r *NoopLBRepository) Create(ctx context.Context, lb *domain.LoadBalancer) error { return nil }
func (r *NoopLBRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.LoadBalancer, error) {
	return &domain.LoadBalancer{ID: id}, nil
}
func (r *NoopLBRepository) GetByName(ctx context.Context, name string) (*domain.LoadBalancer, error) {
	return &domain.LoadBalancer{ID: uuid.New(), Name: name}, nil
}
func (r *NoopLBRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.LoadBalancer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.idempotencyMap == nil {
		r.idempotencyMap = make(map[string]uuid.UUID)
	}
	id, ok := r.idempotencyMap[key]
	if !ok {
		id = uuid.New()
		r.idempotencyMap[key] = id
	}
	return &domain.LoadBalancer{ID: id}, nil
}
func (r *NoopLBRepository) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	return []*domain.LoadBalancer{}, nil
}
func (r *NoopLBRepository) ListAll(ctx context.Context) ([]*domain.LoadBalancer, error) {
	return []*domain.LoadBalancer{}, nil
}
func (r *NoopLBRepository) ListByStatus(ctx context.Context, status string, limit, offset int) ([]*domain.LoadBalancer, error) {
	return []*domain.LoadBalancer{}, nil
}
func (r *NoopLBRepository) Update(ctx context.Context, lb *domain.LoadBalancer) error { return nil }
func (r *NoopLBRepository) Delete(ctx context.Context, id uuid.UUID) error            { return nil }
func (r *NoopLBRepository) AddTarget(ctx context.Context, target *domain.LBTarget) error {
	return nil
}
func (r *NoopLBRepository) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error {
	return nil
}
func (r *NoopLBRepository) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	return []*domain.LBTarget{}, nil
}
func (r *NoopLBRepository) UpdateTargetHealth(ctx context.Context, lbID, instanceID uuid.UUID, health string) error {
	return nil
}
func (r *NoopLBRepository) GetTargetsForInstance(ctx context.Context, instanceID uuid.UUID) ([]*domain.LBTarget, error) {
	return []*domain.LBTarget{}, nil
}

type NoopStorageRepository struct{}

func NewNoopStorageRepository() *NoopStorageRepository {
	return &NoopStorageRepository{}
}

func NewNoopStorageBackend() *NoopStorageRepository {
	return &NoopStorageRepository{}
}

func (r *NoopStorageRepository) SaveMeta(ctx context.Context, obj *domain.Object) error { return nil }
func (r *NoopStorageRepository) GetMeta(ctx context.Context, bucket, key string) (*domain.Object, error) {
	return &domain.Object{Bucket: bucket, Key: key}, nil
}
func (r *NoopStorageRepository) List(ctx context.Context, bucket string) ([]*domain.Object, error) {
	return []*domain.Object{}, nil
}
func (r *NoopStorageRepository) SoftDelete(ctx context.Context, bucket, key string) error { return nil }
func (r *NoopStorageRepository) DeleteVersion(ctx context.Context, bucket, key, ver string) error {
	return nil
}
func (r *NoopStorageRepository) GetMetaByVersion(ctx context.Context, bucket, key, ver string) (*domain.Object, error) {
	return &domain.Object{}, nil
}
func (r *NoopStorageRepository) ListVersions(ctx context.Context, bucket, key string) ([]*domain.Object, error) {
	return []*domain.Object{}, nil
}
func (r *NoopStorageRepository) ListDeleted(ctx context.Context, limit int) ([]*domain.Object, error) {
	return []*domain.Object{}, nil
}
func (r *NoopStorageRepository) HardDelete(ctx context.Context, bucket, key, ver string) error {
	return nil
}
func (r *NoopStorageRepository) ListPending(ctx context.Context, olderThan time.Time, limit int) ([]*domain.Object, error) {
	return []*domain.Object{}, nil
}
func (r *NoopStorageRepository) CreateBucket(ctx context.Context, b *domain.Bucket) error { return nil }
func (r *NoopStorageRepository) GetBucket(ctx context.Context, name string) (*domain.Bucket, error) {
	return &domain.Bucket{Name: name}, nil
}
func (r *NoopStorageRepository) DeleteBucket(ctx context.Context, name string) error { return nil }
func (r *NoopStorageRepository) ListBuckets(ctx context.Context, uid string) ([]*domain.Bucket, error) {
	return []*domain.Bucket{}, nil
}
func (r *NoopStorageRepository) SetBucketVersioning(ctx context.Context, name string, enabled bool) error {
	return nil
}
func (r *NoopStorageRepository) SaveMultipartUpload(ctx context.Context, u *domain.MultipartUpload) error {
	return nil
}
func (r *NoopStorageRepository) GetMultipartUpload(ctx context.Context, id uuid.UUID) (*domain.MultipartUpload, error) {
	// Note: Noop implementation always returns a populated object for the given ID.
	// Happy-path only; does not simulate ObjectNotFound errors.
	return &domain.MultipartUpload{ID: id}, nil
}
func (r *NoopStorageRepository) DeleteMultipartUpload(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (r *NoopStorageRepository) SavePart(ctx context.Context, p *domain.Part) error { return nil }
func (r *NoopStorageRepository) ListParts(ctx context.Context, uploadID uuid.UUID) ([]*domain.Part, error) {
	return []*domain.Part{}, nil
}
func (r *NoopStorageRepository) AttachVolume(ctx context.Context, volumeName, instanceID string) (string, error) {
	return "/dev/vdb", nil
}
func (r *NoopStorageRepository) CreateVolume(ctx context.Context, name string, sizeGB int) (string, error) {
	return "vol-1", nil
}
func (r *NoopStorageRepository) DeleteVolume(ctx context.Context, name string) error { return nil }
func (r *NoopStorageRepository) ResizeVolume(ctx context.Context, name string, newSizeGB int) error {
	return nil
}
func (r *NoopStorageRepository) DetachVolume(ctx context.Context, volumeName, instanceID string) error {
	return nil
}
func (r *NoopStorageRepository) CreateSnapshot(ctx context.Context, volumeName, snapshotName string) error {
	return nil
}
func (r *NoopStorageRepository) DeleteSnapshot(ctx context.Context, snapshotName string) error {
	return nil
}
func (r *NoopStorageRepository) RestoreSnapshot(ctx context.Context, volumeName, snapshotName string) error {
	return nil
}
func (r *NoopStorageRepository) Ping(ctx context.Context) error { return nil }
func (r *NoopStorageRepository) Type() string                   { return "noop" }

type NoopStorageBackend struct{}

func NewNoopStorageBackendAdapter() *NoopStorageBackend {
	return &NoopStorageBackend{}
}

func (s *NoopStorageBackend) CreateVolume(ctx context.Context, name string, sizeGB int) (string, error) {
	return "vol-1", nil
}
func (s *NoopStorageBackend) DeleteVolume(ctx context.Context, name string) error { return nil }
func (s *NoopStorageBackend) ResizeVolume(ctx context.Context, name string, newSizeGB int) error {
	return nil
}
func (s *NoopStorageBackend) AttachVolume(ctx context.Context, volumeName, instanceID string) (string, error) {
	return "vol-1", nil
}
func (s *NoopStorageBackend) DetachVolume(ctx context.Context, volumeName, instanceID string) error {
	return nil
}
func (s *NoopStorageBackend) CreateSnapshot(ctx context.Context, volumeName, snapshotName string) error {
	return nil
}
func (s *NoopStorageBackend) DeleteSnapshot(ctx context.Context, snapshotName string) error {
	return nil
}
func (s *NoopStorageBackend) RestoreSnapshot(ctx context.Context, volumeName, snapshotName string) error {
	return nil
}
func (s *NoopStorageBackend) Ping(ctx context.Context) error { return nil }
func (s *NoopStorageBackend) Type() string                   { return "noop" }

// NoopRBACService is a no-op RBAC service.
type NoopRBACService struct{}

func (s *NoopRBACService) Authorize(ctx context.Context, userID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	return nil
}
func (s *NoopRBACService) HasPermission(ctx context.Context, userID, tenantID uuid.UUID, permission domain.Permission, resource string) (bool, error) {
	return true, nil
}
func (s *NoopRBACService) CreateRole(ctx context.Context, role *domain.Role) error { return nil }
func (s *NoopRBACService) GetRoleByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	return &domain.Role{ID: id}, nil
}
func (s *NoopRBACService) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	return &domain.Role{ID: uuid.New(), Name: name}, nil
}
func (s *NoopRBACService) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	return []*domain.Role{}, nil
}
func (s *NoopRBACService) UpdateRole(ctx context.Context, role *domain.Role) error { return nil }
func (s *NoopRBACService) DeleteRole(ctx context.Context, id uuid.UUID) error      { return nil }
func (s *NoopRBACService) AddPermissionToRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return nil
}
func (s *NoopRBACService) RemovePermissionFromRole(ctx context.Context, roleID uuid.UUID, permission domain.Permission) error {
	return nil
}
func (s *NoopRBACService) BindRole(ctx context.Context, userIdentifier string, roleName string) error {
	return nil
}
func (s *NoopRBACService) ListRoleBindings(ctx context.Context) ([]*domain.User, error) {
	return []*domain.User{}, nil
}
func (s *NoopRBACService) EvaluatePolicy(ctx context.Context, userID uuid.UUID, action string, resource string, context map[string]interface{}) (bool, error) {
	return true, nil
}
func (s *NoopRBACService) AuthorizeServiceAccount(ctx context.Context, saID, tenantID uuid.UUID, permission domain.Permission, resource string) error {
	return nil
}

// NoopDatabaseService is a no-op database service.
type NoopDatabaseService struct{}

func (s *NoopDatabaseService) CreateDatabase(ctx context.Context, req ports.CreateDatabaseRequest) (*domain.Database, error) {
	return &domain.Database{ID: uuid.New(), Name: req.Name, Role: domain.RolePrimary}, nil
}
func (s *NoopDatabaseService) DeleteDatabase(ctx context.Context, id uuid.UUID) error { return nil }
func (s *NoopDatabaseService) ModifyDatabase(ctx context.Context, req ports.ModifyDatabaseRequest) (*domain.Database, error) {
	return &domain.Database{ID: req.ID}, nil
}
func (s *NoopDatabaseService) GetConnectionString(ctx context.Context, id uuid.UUID) (string, error) {
	return "postgres://127.0.0.1:5432/db", nil
}
func (s *NoopDatabaseService) CreateDatabaseSnapshot(ctx context.Context, databaseID uuid.UUID, description string) (*domain.Snapshot, error) {
	return &domain.Snapshot{ID: uuid.New()}, nil
}
func (s *NoopDatabaseService) ListDatabaseSnapshots(ctx context.Context, databaseID uuid.UUID) ([]*domain.Snapshot, error) {
	return []*domain.Snapshot{}, nil
}
func (s *NoopDatabaseService) RestoreDatabase(ctx context.Context, req ports.RestoreDatabaseRequest) (*domain.Database, error) {
	return &domain.Database{ID: uuid.New(), Name: req.NewName, Role: domain.RolePrimary}, nil
}
func (s *NoopDatabaseService) CreateReplica(ctx context.Context, primaryID uuid.UUID, name string) (*domain.Database, error) {
	return &domain.Database{ID: uuid.New(), Name: name, Role: domain.RoleReplica}, nil
}
func (s *NoopDatabaseService) PromoteToPrimary(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (s *NoopDatabaseService) GetDatabase(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	return &domain.Database{ID: id, Name: "noop-db", Role: domain.RolePrimary}, nil
}
func (s *NoopDatabaseService) ListDatabases(ctx context.Context) ([]*domain.Database, error) {
	return []*domain.Database{}, nil
}
func (s *NoopDatabaseService) RotateCredentials(ctx context.Context, id uuid.UUID, idempotencyKey string) error {
	return nil
}
func (s *NoopDatabaseService) StopDatabase(ctx context.Context, id uuid.UUID) error {
	return nil
}
func (s *NoopDatabaseService) StartDatabase(ctx context.Context, id uuid.UUID) error {
	return nil
}
