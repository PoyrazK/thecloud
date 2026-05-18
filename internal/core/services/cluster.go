// Package services implements core business logic.
package services

import (
	"context"
	"log/slog"
	"time"

	semver "github.com/Masterminds/semver/v3"
	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/crypto"
)

// ClusterService implements the managed Kubernetes service.
type ClusterService struct {
	repo        ports.ClusterRepository
	rbacSvc     ports.RBACService
	provisioner ports.ClusterProvisioner
	vpcSvc      ports.VpcService
	instanceSvc ports.InstanceService
	secretSvc   ports.SecretService
	taskQueue   ports.TaskQueue
	logger      *slog.Logger
}

// ClusterServiceParams holds dependencies for ClusterService.
type ClusterServiceParams struct {
	Repo        ports.ClusterRepository
	RBAC        ports.RBACService
	Provisioner ports.ClusterProvisioner
	VpcSvc      ports.VpcService
	InstanceSvc ports.InstanceService
	SecretSvc   ports.SecretService
	TaskQueue   ports.TaskQueue
	Logger      *slog.Logger
}

// NewClusterService creates a new ClusterService with the provided parameters.
func NewClusterService(params ClusterServiceParams) (*ClusterService, error) {
	if params.TaskQueue == nil {
		return nil, errors.New(errors.Internal, "taskQueue cannot be nil")
	}
	return &ClusterService{
		repo:        params.Repo,
		rbacSvc:     params.RBAC,
		provisioner: params.Provisioner,
		vpcSvc:      params.VpcSvc,
		instanceSvc: params.InstanceSvc,
		secretSvc:   params.SecretSvc,
		taskQueue:   params.TaskQueue,
		logger:      params.Logger,
	}, nil
}

// Default cluster configuration values.
const (
	defaultClusterVersion      = "v1.29.0"
	defaultWorkerCount         = 2
	defaultBackupRetentionDays = 7
)

// CreateCluster initiates the provisioning of a new Kubernetes cluster.
func (s *ClusterService) CreateCluster(ctx context.Context, params ports.CreateClusterParams) (*domain.Cluster, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterCreate, "*"); err != nil {
		return nil, err
	}

	// 1. Verify VPC exists and belongs to user
	vpc, err := s.vpcSvc.GetVPC(ctx, params.VpcID.String())
	if err != nil {
		return nil, errors.Wrap(errors.NotFound, "vpc not found", err)
	}

	// Default version if not specified
	if params.Version == "" {
		params.Version = defaultClusterVersion
	}
	if params.Workers == 0 {
		params.Workers = defaultWorkerCount
	}

	// 2. Generate SSH Key Pair for the cluster
	privKey, _, err := crypto.GenerateSSHKeyPair()
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to generate cluster ssh key", err)
	}

	// Encrypt the SSH key before storing
	encryptedKey, err := s.secretSvc.Encrypt(ctx, userID, privKey)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to encrypt cluster ssh key", err)
	}

	// 3. Create cluster record in database
	cluster := &domain.Cluster{
		ID:                     uuid.New(),
		Name:                   params.Name,
		UserID:                 userID,
		TenantID:               tenantID,
		VpcID:                  vpc.ID,
		Version:                params.Version,
		WorkerCount:            params.Workers,
		NetworkIsolation:       params.NetworkIsolation,
		HAEnabled:              params.HAEnabled,
		Status:                 domain.ClusterStatusPending,
		PodCIDR:                params.PodCIDR,
		ServiceCIDR:            params.ServiceCIDR,
		SSHPrivateKeyEncrypted: encryptedKey,
		BackupRetentionDays:    defaultBackupRetentionDays,
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now(),
	}

	if cluster.PodCIDR == "" {
		cluster.PodCIDR = "10.244.0.0/16"
	}
	if cluster.ServiceCIDR == "" {
		cluster.ServiceCIDR = "10.96.0.0/12"
	}

	// Add default Node Group
	defaultNG := domain.NodeGroup{
		ID:           uuid.New(),
		ClusterID:    cluster.ID,
		Name:         "default-pool",
		InstanceType: "standard-1",
		MinSize:      1,
		MaxSize:      10,
		CurrentSize:  params.Workers,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now(),
	}
	cluster.NodeGroups = []domain.NodeGroup{defaultNG}

	// Atomically create cluster and its default node group
	if err := s.repo.Create(ctx, cluster); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to create cluster record", err)
	}

	// Persist Node Group
	if err := s.repo.AddNodeGroup(ctx, &cluster.NodeGroups[0]); err != nil {
		// Ideally this should be in a transaction with repo.Create
		return nil, errors.Wrap(errors.Internal, "failed to create default node group", err)
	}

	// 3. Enqueue provisioning job
	job := domain.ClusterJob{
		ClusterID: cluster.ID,
		UserID:    cluster.UserID,
		TenantID:  cluster.TenantID,
		Type:      domain.ClusterJobProvision,
	}

	if err := s.taskQueue.Enqueue(ctx, "k8s_jobs", job); err != nil {
		s.logger.Error("failed to enqueue cluster provision job", "cluster_id", cluster.ID, "error", err)
		cluster.Status = domain.ClusterStatusFailed
		if updateErr := s.repo.Update(ctx, cluster); updateErr != nil {
			s.logger.Error("failed to update cluster status after enqueue failure", "cluster_id", cluster.ID, "error", updateErr)
		}
		return nil, errors.Wrap(errors.Internal, "failed to enqueue provisioning task", err)
	}

	return cluster, nil
}

// GetCluster retrieves cluster details by ID.
func (s *ClusterService) GetCluster(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterRead, id.String()); err != nil {
		return nil, err
	}

	cluster, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, errors.New(errors.NotFound, "cluster not found")
	}
	return cluster, nil
}

// ListClusters retrieves all clusters for a user.
func (s *ClusterService) ListClusters(ctx context.Context, userID uuid.UUID) ([]*domain.Cluster, error) {
	uID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, uID, tenantID, domain.PermissionClusterRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListByUserID(ctx, userID)
}

// DeleteCluster removes a cluster and its associated resources.
func (s *ClusterService) DeleteCluster(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterDelete, id.String()); err != nil {
		return err
	}

	cluster, err := s.GetCluster(ctx, id)
	if err != nil {
		return err
	}

	cluster.Status = domain.ClusterStatusDeleting
	cluster.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, cluster); err != nil {
		return errors.Wrap(errors.Internal, "failed to update cluster status", err)
	}

	// Enqueue deprovision job
	job := domain.ClusterJob{
		ClusterID: cluster.ID,
		UserID:    cluster.UserID,
		TenantID:  cluster.TenantID,
		Type:      domain.ClusterJobDeprovision,
	}

	if err := s.taskQueue.Enqueue(ctx, "k8s_jobs", job); err != nil {
		return errors.Wrap(errors.Internal, "failed to enqueue cluster deprovision job", err)
	}

	return nil
}

// GetKubeconfig retrieves the encrypted kubeconfig for a cluster, optionally for a specific guest role.
func (s *ClusterService) GetKubeconfig(ctx context.Context, id uuid.UUID, role string) (string, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterRead, id.String()); err != nil {
		return "", err
	}

	cluster, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	if cluster.Status != domain.ClusterStatusRunning {
		return "", errors.New(errors.InvalidInput, "kubeconfig is only available when cluster is running")
	}

	// For admin role, use the stored kubeconfig
	if role == "" || role == "admin" {
		if cluster.KubeconfigEncrypted == "" {
			return "", errors.New(errors.NotFound, "kubeconfig not found for cluster")
		}

		// Decrypt the kubeconfig
		decrypted, err := s.secretSvc.Decrypt(ctx, cluster.UserID, cluster.KubeconfigEncrypted)
		if err != nil {
			s.logger.Error("failed to decrypt kubeconfig", "cluster_id", cluster.ID, "error", err)
			return "", errors.Wrap(errors.Internal, "failed to decrypt kubeconfig", err)
		}
		return decrypted, nil
	}

	// For other roles, generate dynamically from provisioner
	return s.provisioner.GetKubeconfig(ctx, cluster, role)
}

// RepairCluster triggers a re-run of critical provisioning steps (CNI, kube-proxy patches).
func (s *ClusterService) RepairCluster(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterUpdate, id.String()); err != nil {
		return err
	}

	cluster, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	go func() {
		bgCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		bgCtx = appcontext.WithUserID(bgCtx, cluster.UserID)
		bgCtx = appcontext.WithTenantID(bgCtx, cluster.TenantID)
		s.logger.Info("starting cluster repair", "cluster_id", cluster.ID)

		if err := s.provisioner.Repair(bgCtx, cluster); err != nil {
			s.logger.Error("cluster repair failed", "cluster_id", cluster.ID, "error", err)
			return
		}

		s.logger.Info("cluster repair completed", "cluster_id", cluster.ID)
	}()

	return nil
}

// ScaleCluster adjusts the number of worker nodes in the cluster.
func (s *ClusterService) ScaleCluster(ctx context.Context, id uuid.UUID, workers int) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterUpdate, id.String()); err != nil {
		return err
	}

	cluster, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if workers < 1 {
		return errors.New(errors.InvalidInput, "at least 1 worker required")
	}

	oldWorkerCount := cluster.WorkerCount
	cluster.WorkerCount = workers
	cluster.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, cluster); err != nil {
		cluster.WorkerCount = oldWorkerCount
		return errors.Wrap(errors.Internal, "failed to update cluster record during scaling", err)
	}

	go func() {
		bgCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		bgCtx = appcontext.WithUserID(bgCtx, cluster.UserID)
		bgCtx = appcontext.WithTenantID(bgCtx, cluster.TenantID)
		s.logger.Info("starting cluster scale", "cluster_id", cluster.ID, "new_workers", workers)

		if err := s.provisioner.Scale(bgCtx, cluster); err != nil {
			s.logger.Error("cluster scaling failed", "cluster_id", cluster.ID, "error", err)
			return
		}

		s.logger.Info("cluster scaling completed", "cluster_id", cluster.ID)
	}()

	return nil
}

// GetClusterHealth retrieves the operational status of the cluster.
func (s *ClusterService) GetClusterHealth(ctx context.Context, id uuid.UUID) (*ports.ClusterHealth, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterRead, id.String()); err != nil {
		return nil, err
	}

	cluster, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.provisioner.GetHealth(ctx, cluster)
}

// UpgradeCluster initiates an asynchronous version upgrade.
func (s *ClusterService) UpgradeCluster(ctx context.Context, id uuid.UUID, version string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterUpdate, id.String()); err != nil {
		return err
	}

	cluster, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if cluster.Status != domain.ClusterStatusRunning {
		return errors.New(errors.Conflict, "cluster must be in running state to upgrade")
	}

	if cluster.Version == version {
		return errors.New(errors.InvalidInput, "cluster is already at this version")
	}

	if err := s.validateUpgrade(cluster.Version, version); err != nil {
		return err
	}

	cluster.Status = domain.ClusterStatusUpgrading
	cluster.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, cluster); err != nil {
		return err
	}

	// Enqueue upgrade job
	job := domain.ClusterJob{
		ClusterID: cluster.ID,
		UserID:    cluster.UserID,
		TenantID:  cluster.TenantID,
		Type:      domain.ClusterJobUpgrade,
		Version:   version,
	}

	if s.taskQueue == nil {
		return errors.New(errors.Internal, "task queue not initialized")
	}

	if err := s.taskQueue.Enqueue(ctx, "k8s_jobs", job); err != nil {
		return errors.Wrap(errors.Internal, "failed to enqueue cluster upgrade job", err)
	}

	return nil
}

func (s *ClusterService) RotateSecrets(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterUpdate, id.String()); err != nil {
		return err
	}

	cluster, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if cluster.Status != domain.ClusterStatusRunning {
		return errors.New(errors.Conflict, "cluster must be in running state to rotate secrets")
	}

	cluster.Status = domain.ClusterStatusUpdating
	if err := s.repo.Update(ctx, cluster); err != nil {
		return err
	}

	if err := s.provisioner.RotateSecrets(ctx, cluster); err != nil {
		cluster.Status = domain.ClusterStatusFailed
		if updateErr := s.repo.Update(ctx, cluster); updateErr != nil {
			s.logger.Error("failed to update cluster status after secret rotation failure", "cluster_id", cluster.ID, "error", updateErr)
		}
		return err
	}

	cluster.Status = domain.ClusterStatusRunning
	return s.repo.Update(ctx, cluster)
}

func (s *ClusterService) CreateBackup(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterUpdate, id.String()); err != nil {
		return err
	}

	cluster, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if cluster.Status != domain.ClusterStatusRunning {
		return errors.New(errors.Conflict, "cluster must be in running state to create backup")
	}

	// Backup is a side operation, we don't necessarily change status to "backing up"
	// unless it's a very long operation that blocks other things.
	return s.provisioner.CreateBackup(ctx, cluster)
}

func (s *ClusterService) RestoreBackup(ctx context.Context, id uuid.UUID, backupPath string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterUpdate, id.String()); err != nil {
		return err
	}

	cluster, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if cluster.Status != domain.ClusterStatusRunning {
		return errors.New(errors.Conflict, "cluster must be in running state to restore backup")
	}

	cluster.Status = domain.ClusterStatusRepairing
	if err := s.repo.Update(ctx, cluster); err != nil {
		return err
	}

	if err := s.provisioner.Restore(ctx, cluster, backupPath); err != nil {
		cluster.Status = domain.ClusterStatusFailed
		if updateErr := s.repo.Update(ctx, cluster); updateErr != nil {
			s.logger.Error("failed to update cluster status after restoration failure", "cluster_id", cluster.ID, "error", updateErr)
		}
		return err
	}

	cluster.Status = domain.ClusterStatusRunning
	return s.repo.Update(ctx, cluster)
}

func (s *ClusterService) SetBackupPolicy(ctx context.Context, id uuid.UUID, params ports.BackupPolicyParams) error {
	cluster, err := s.GetCluster(ctx, id)
	if err != nil {
		return err
	}
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionClusterUpdate, id.String()); err != nil {
		return err
	}
	if params.Schedule == nil && params.RetentionDays == nil {
		return errors.New(errors.InvalidInput, "at least one backup policy field must be provided")
	}

	if params.Schedule != nil {
		cluster.BackupSchedule = *params.Schedule
	}
	if params.RetentionDays != nil {
		if *params.RetentionDays <= 0 {
			return errors.New(errors.InvalidInput, "invalid retention days")
		}
		cluster.BackupRetentionDays = *params.RetentionDays
	} else if params.Schedule != nil && *params.Schedule == "" {
		cluster.BackupRetentionDays = defaultBackupRetentionDays
	}
	cluster.UpdatedAt = time.Now()

	return s.repo.Update(ctx, cluster)
}

func (s *ClusterService) AddNodeGroup(ctx context.Context, clusterID uuid.UUID, params ports.NodeGroupParams) (*domain.NodeGroup, error) {
	cluster, err := s.GetCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	// Validate node group sizing invariants
	if err := s.validateNodeGroupSizing(params.MinSize, params.MaxSize, params.DesiredSize); err != nil {
		return nil, err
	}

	ng := &domain.NodeGroup{
		ID:           uuid.New(),
		ClusterID:    cluster.ID,
		Name:         params.Name,
		InstanceType: params.InstanceType,
		MinSize:      params.MinSize,
		MaxSize:      params.MaxSize,
		CurrentSize:  params.DesiredSize,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.AddNodeGroup(ctx, ng); err != nil {
		return nil, err
	}

	// If desired size > 0, we should trigger a scale operation for this specific group.
	// For now, we will update the global worker count to reflect the addition.
	oldWorkerCount := cluster.WorkerCount
	cluster.WorkerCount += params.DesiredSize
	if err := s.repo.Update(ctx, cluster); err != nil {
		cluster.WorkerCount = oldWorkerCount
		return nil, errors.Wrap(errors.Internal, "failed to update cluster worker count after adding node group", err)
	}

	return ng, nil
}

func (s *ClusterService) UpdateNodeGroup(ctx context.Context, clusterID uuid.UUID, name string, params ports.UpdateNodeGroupParams) (*domain.NodeGroup, error) {
	cluster, err := s.GetCluster(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	var targetGroup *domain.NodeGroup
	for i := range cluster.NodeGroups {
		if cluster.NodeGroups[i].Name == name {
			targetGroup = &cluster.NodeGroups[i]
			break
		}
	}

	if targetGroup == nil {
		return nil, errors.New(errors.NotFound, "node group not found")
	}

	newMin := targetGroup.MinSize
	newMax := targetGroup.MaxSize
	newDesired := targetGroup.CurrentSize

	if params.MinSize != nil {
		newMin = *params.MinSize
	}
	if params.MaxSize != nil {
		newMax = *params.MaxSize
	}
	if params.DesiredSize != nil {
		newDesired = *params.DesiredSize
	}

	// Validate node group sizing invariants
	if err := s.validateNodeGroupSizing(newMin, newMax, newDesired); err != nil {
		return nil, err
	}

	oldMin, oldMax, oldDesired := targetGroup.MinSize, targetGroup.MaxSize, targetGroup.CurrentSize
	targetGroup.MinSize = newMin
	targetGroup.MaxSize = newMax

	oldWorkerCount := cluster.WorkerCount
	if params.DesiredSize != nil {
		diff := newDesired - oldDesired
		cluster.WorkerCount += diff
		targetGroup.CurrentSize = newDesired
	}

	if err := s.repo.UpdateNodeGroup(ctx, targetGroup); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, cluster); err != nil {
		// Rollback in-memory changes on persistence failure
		targetGroup.MinSize = oldMin
		targetGroup.MaxSize = oldMax
		targetGroup.CurrentSize = oldDesired
		cluster.WorkerCount = oldWorkerCount
		return nil, errors.Wrap(errors.Internal, "failed to update cluster worker count after updating node group", err)
	}

	return targetGroup, nil
}

func (s *ClusterService) DeleteNodeGroup(ctx context.Context, clusterID uuid.UUID, name string) error {
	cluster, err := s.GetCluster(ctx, clusterID)
	if err != nil {
		return err
	}

	if name == "default-pool" {
		return errors.New(errors.InvalidInput, "cannot delete default node group")
	}

	var targetNG *domain.NodeGroup
	for i := range cluster.NodeGroups {
		if cluster.NodeGroups[i].Name == name {
			targetNG = &cluster.NodeGroups[i]
			break
		}
	}

	if targetNG == nil {
		return errors.New(errors.NotFound, "node group not found")
	}

	oldWorkerCount := cluster.WorkerCount
	cluster.WorkerCount -= targetNG.CurrentSize

	if err := s.repo.DeleteNodeGroup(ctx, clusterID, name); err != nil {
		cluster.WorkerCount = oldWorkerCount
		return err
	}

	if err := s.repo.Update(ctx, cluster); err != nil {
		cluster.WorkerCount = oldWorkerCount
		return errors.Wrap(errors.Internal, "failed to update cluster worker count after deleting node group", err)
	}

	return nil
}

func (s *ClusterService) validateUpgrade(current, target string) error {
	curVer, err := semver.NewVersion(current)
	if err != nil {
		s.logger.Warn("cluster has invalid current version string", "version", current, "error", err)
		return nil // Proceed anyway if current version is malformed, risk is on the user
	}

	tarVer, err := semver.NewVersion(target)
	if err != nil {
		return errors.New(errors.InvalidInput, "invalid target version format")
	}

	if tarVer.LessThan(curVer) || tarVer.Equal(curVer) {
		return errors.New(errors.InvalidInput, "target version must be higher than current")
	}

	// kubeadm only supports +1 minor version
	if tarVer.Major() != curVer.Major() || tarVer.Minor()-curVer.Minor() > 1 {
		return errors.New(errors.InvalidInput, "cannot skip minor versions or upgrade across major versions")
	}

	return nil
}

func (s *ClusterService) validateNodeGroupSizing(min, max, desired int) error {
	if min < 0 || max < 0 || desired < 0 {
		return errors.New(errors.InvalidInput, "node group sizes must be non-negative")
	}
	if min > max {
		return errors.New(errors.InvalidInput, "min_size cannot be greater than max_size")
	}
	if desired < min || desired > max {
		return errors.New(errors.InvalidInput, "desired_size must be between min_size and max_size")
	}
	return nil
}
