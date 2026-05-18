// Package k8s implements Kubernetes provisioning adapters.
package k8s

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	cloudcrypto "github.com/poyrazk/thecloud/pkg/crypto"
)

const (
	defaultUser     = "ubuntu"
	adminKubeconfig = "/etc/kubernetes/admin.conf"
	etcdManifest    = "/etc/kubernetes/manifests/etcd.yaml"
)

var controlPlaneManifestPaths = []string{
	"/etc/kubernetes/manifests/etcd.yaml",
	"/etc/kubernetes/manifests/kube-apiserver.yaml",
	"/etc/kubernetes/manifests/kube-controller-manager.yaml",
	"/etc/kubernetes/manifests/kube-scheduler.yaml",
}

// KubeadmProvisioner implements ports.ClusterProvisioner using kubeadm and Cloud-Init.
type KubeadmProvisioner struct {
	instSvc         ports.InstanceService
	repo            ports.ClusterRepository
	secretSvc       ports.SecretService
	sgSvc           ports.SecurityGroupService
	storageSvc      ports.StorageService
	lbSvc           ports.LBService
	logger          *slog.Logger
	templateDir     string
	executorFactory func(ctx context.Context, cluster *domain.Cluster, ip string) (NodeExecutor, error)
}

// NewKubeadmProvisioner constructs a new KubeadmProvisioner.
func NewKubeadmProvisioner(
	instSvc ports.InstanceService,
	repo ports.ClusterRepository,
	secretSvc ports.SecretService,
	sgSvc ports.SecurityGroupService,
	storageSvc ports.StorageService,
	lbSvc ports.LBService,
	logger *slog.Logger,
) *KubeadmProvisioner {
	templateDir := "internal/repositories/k8s/templates"
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		// Fallback for running tests from within the k8s repo
		if _, err := os.Stat("templates"); err == nil {
			templateDir = "templates"
		}
	}

	return &KubeadmProvisioner{
		instSvc:     instSvc,
		repo:        repo,
		secretSvc:   secretSvc,
		sgSvc:       sgSvc,
		storageSvc:  storageSvc,
		lbSvc:       lbSvc,
		logger:      logger,
		templateDir: templateDir,
	}
}

func (p *KubeadmProvisioner) Provision(ctx context.Context, cluster *domain.Cluster) error {
	p.logger.Info("starting real provisioning for cluster", "cluster_id", cluster.ID, "name", cluster.Name)

	// Phase 1: Ensure Security Group
	if err := p.ensureClusterSecurityGroup(ctx, cluster); err != nil {
		return p.failCluster(ctx, cluster, "failed to ensure security group", err)
	}

	// Phase 2: Handle HA LB if enabled
	if cluster.HAEnabled {
		if err := p.ensureAPIServerLB(ctx, cluster); err != nil {
			return p.failCluster(ctx, cluster, "failed to ensure API Server LB", err)
		}
	}

	// Phase 3: Provision Control Plane via Cloud-Init
	if err := p.provisionControlPlane(ctx, cluster); err != nil {
		return err
	}

	// Phase 4: Generate Join Token (requires SSH to CP)
	if err := p.refreshJoinToken(ctx, cluster); err != nil {
		return err
	}

	// Phase 5: Provision Workers via Cloud-Init (using join token)
	if err := p.provisionWorkers(ctx, cluster); err != nil {
		return err
	}

	cluster.Status = domain.ClusterStatusRunning
	if err := p.repo.Update(ctx, cluster); err != nil {
		return fmt.Errorf("provisioning succeeded but failed to persist running status for cluster %s: %w", cluster.ID, err)
	}
	return nil
}

func (p *KubeadmProvisioner) provisionControlPlane(ctx context.Context, cluster *domain.Cluster) error {
	// Securely fetch API credentials
	apiKey := os.Getenv("CLOUD_API_KEY")
	if apiKey == "" {
		return p.failCluster(ctx, cluster, "CLOUD_API_KEY is required but not set in environment", nil)
	}
	apiURL := os.Getenv("CLOUD_API_URL")
	if apiURL == "" {
		return p.failCluster(ctx, cluster, "CLOUD_API_URL is required but not set in environment", nil)
	}

	// Validate API URL is HTTPS for production safety
	if !strings.HasPrefix(apiURL, "https://") {
		// We allow http for localhost/local testing environments, but enforce https for others
		if !strings.Contains(apiURL, "localhost") && !strings.Contains(apiURL, "127.0.0.1") && !strings.Contains(apiURL, "local") {
			return p.failCluster(ctx, cluster, "CLOUD_API_URL must use HTTPS for production environments", nil)
		}
	}

	hostPrivateKey, hostPublicKey, err := cloudcrypto.GenerateSSHKeyPair()
	if err != nil {
		return p.failCluster(ctx, cluster, "failed to generate control plane host key", err)
	}

	userData, err := p.renderTemplate("control_plane.yaml", map[string]interface{}{
		"ClusterID":            cluster.ID.String(),
		"PodCIDR":              cluster.PodCIDR,
		"ServiceCIDR":          cluster.ServiceCIDR,
		"HAEnabled":            cluster.HAEnabled,
		"LBAddress":            cluster.APIServerLBAddress,
		"CloudAPIKey":          apiKey,
		"CloudAPIURL":          apiURL,
		"SSHHostPrivateKeyB64": base64.StdEncoding.EncodeToString([]byte(hostPrivateKey)),
		"SSHHostPublicKeyB64":  base64.StdEncoding.EncodeToString([]byte(hostPublicKey)),
	})
	if err != nil {
		return p.failCluster(ctx, cluster, "failed to render control plane template", err)
	}

	nodeName := fmt.Sprintf("%s-cp-0", cluster.Name)
	inst, err := p.instSvc.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{
		Name:      nodeName,
		ImageName: "ubuntu:22.04", // Use canonical docker hub image
		NetworkID: cluster.VpcID.String(),
		UserData:  userData,
		Metadata: map[string]string{
			"thecloud.io/cluster-id":          cluster.ID.String(),
			"thecloud.io/node-role":           string(domain.NodeRoleControlPlane),
			"thecloud.io/ssh-host-public-key": strings.TrimSpace(hostPublicKey),
		},
	})
	if err != nil {
		return p.failCluster(ctx, cluster, "failed to launch control plane instance", err)
	}

	node := &domain.ClusterNode{
		ID:         uuid.New(),
		ClusterID:  cluster.ID,
		InstanceID: inst.ID,
		Role:       domain.NodeRoleControlPlane,
		Status:     "provisioning",
		JoinedAt:   time.Now(),
	}
	if err := p.repo.AddNode(ctx, node); err != nil {
		return err
	}

	// Wait for instance to have an IP
	masterIP := p.waitForIP(ctx, node.InstanceID)
	if masterIP == "" {
		return p.failCluster(ctx, cluster, "control plane node failed to get an IP", nil)
	}
	cluster.ControlPlaneIPs = append(cluster.ControlPlaneIPs, masterIP)
	if err := p.repo.Update(ctx, cluster); err != nil {
		return p.failCluster(ctx, cluster, "failed to persist control plane IP", err)
	}

	// Wait for kubeadm init to finish and kubeconfig to be available via SSH
	p.logger.Info("waiting for kubeadm init to complete", "ip", masterIP)

	// Implementation of waitForKubeconfig using SSH with retries
	kubeconfig, err := p.waitForKubeconfig(ctx, cluster, masterIP)
	if err != nil {
		return p.failCluster(ctx, cluster, "failed to retrieve kubeconfig", err)
	}

	encryptedKubeconfig, err := p.secretSvc.Encrypt(ctx, cluster.UserID, kubeconfig)
	if err != nil {
		return p.failCluster(ctx, cluster, "failed to encrypt kubeconfig", err)
	}
	cluster.KubeconfigEncrypted = encryptedKubeconfig

	if err := p.repo.Update(ctx, cluster); err != nil {
		return p.failCluster(ctx, cluster, "failed to persist encrypted kubeconfig", err)
	}
	return nil
}

func (p *KubeadmProvisioner) provisionWorkers(ctx context.Context, cluster *domain.Cluster) error {
	p.logger.Info("provisioning worker nodes", "count", cluster.WorkerCount)

	apiServer := cluster.ControlPlaneIPs[0]
	if cluster.HAEnabled && cluster.APIServerLBAddress != nil {
		apiServer = *cluster.APIServerLBAddress
	}

	// For legacy compatibility, if no NodeGroups exist, create them from cluster.WorkerCount
	if len(cluster.NodeGroups) == 0 {
		cluster.NodeGroups = []domain.NodeGroup{
			{
				Name:         "default-pool",
				InstanceType: "standard-1",
				CurrentSize:  cluster.WorkerCount,
			},
		}
	}

	var provisioningErrors []string
	for _, ng := range cluster.NodeGroups {
		for i := 0; i < ng.CurrentSize; i++ {
			hostPrivateKey, hostPublicKey, err := cloudcrypto.GenerateSSHKeyPair()
			if err != nil {
				errMsg := fmt.Sprintf("failed to generate worker host key %s-%d: %v", ng.Name, i, err)
				p.logger.Error(errMsg)
				provisioningErrors = append(provisioningErrors, errMsg)
				continue
			}

			workerUserData, err := p.renderTemplate("worker.yaml", map[string]interface{}{
				"APIServerAddress":     apiServer,
				"JoinToken":            cluster.JoinToken,
				"CACertHash":           cluster.CACertHash,
				"SSHHostPrivateKeyB64": base64.StdEncoding.EncodeToString([]byte(hostPrivateKey)),
				"SSHHostPublicKeyB64":  base64.StdEncoding.EncodeToString([]byte(hostPublicKey)),
			})
			if err != nil {
				errMsg := fmt.Sprintf("failed to render worker template %s-%d: %v", ng.Name, i, err)
				p.logger.Error(errMsg)
				provisioningErrors = append(provisioningErrors, errMsg)
				continue
			}

			workerName := fmt.Sprintf("%s-%s-%d", cluster.Name, ng.Name, i)
			workerInst, err := p.instSvc.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{
				Name:      workerName,
				ImageName: "ubuntu:22.04", // Use canonical image name consistent with control-plane
				NetworkID: cluster.VpcID.String(),
				UserData:  workerUserData,
				Metadata: map[string]string{
					"thecloud.io/cluster-id":          cluster.ID.String(),
					"thecloud.io/node-group":          ng.Name,
					"thecloud.io/node-role":           string(domain.NodeRoleWorker),
					"thecloud.io/ssh-host-public-key": strings.TrimSpace(hostPublicKey),
				},
			})
			if err != nil {
				errMsg := fmt.Sprintf("failed to create worker node %s: %v", workerName, err)
				p.logger.Error(errMsg)
				provisioningErrors = append(provisioningErrors, errMsg)
				continue
			}

			node := &domain.ClusterNode{
				ID:         uuid.New(),
				ClusterID:  cluster.ID,
				InstanceID: workerInst.ID,
				Role:       domain.NodeRoleWorker,
				Status:     "provisioning",
				JoinedAt:   time.Now(),
			}
			if err := p.repo.AddNode(ctx, node); err != nil {
				errMsg := fmt.Sprintf("failed to add worker node %s to repository: %v", workerName, err)
				p.logger.Error(errMsg)
				provisioningErrors = append(provisioningErrors, errMsg)
			}
		}
	}

	if len(provisioningErrors) > 0 {
		return fmt.Errorf("worker provisioning encountered errors: %s", strings.Join(provisioningErrors, "; "))
	}

	return nil
}

func (p *KubeadmProvisioner) renderTemplate(name string, data interface{}) (string, error) {
	path := filepath.Join(p.templateDir, name)
	tpl, err := template.ParseFiles(path)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (p *KubeadmProvisioner) refreshJoinToken(ctx context.Context, cluster *domain.Cluster) error {
	if cluster.TokenExpiresAt != nil && cluster.TokenExpiresAt.After(time.Now().Add(10*time.Minute)) {
		return nil
	}

	p.logger.Info("refreshing join token", "cluster_id", cluster.ID)

	masterIP := cluster.ControlPlaneIPs[0]
	exec, err := p.getExecutor(ctx, cluster, masterIP)
	if err != nil {
		return err
	}

	// Create new token
	out, err := exec.Run(ctx, "kubeadm token create --print-join-command")
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create join token", err)
	}

	// Parse join command
	// Example: kubeadm join 10.0.0.10:6443 --token abcdef.0123456789abcdef --discovery-token-ca-cert-hash sha256:hash
	fields := strings.Fields(out)
	for i, f := range fields {
		if f == "--token" && i+1 < len(fields) {
			cluster.JoinToken = fields[i+1]
		}
		if f == "--discovery-token-ca-cert-hash" && i+1 < len(fields) {
			cluster.CACertHash = fields[i+1]
		}
	}

	expiry := time.Now().Add(23 * time.Hour)
	cluster.TokenExpiresAt = &expiry

	return p.repo.Update(ctx, cluster)
}

func (p *KubeadmProvisioner) waitForIP(ctx context.Context, instID uuid.UUID) string {
	for i := 0; i < 30; i++ {
		inst, err := p.instSvc.GetInstance(ctx, instID.String())
		if err == nil && inst.PrivateIP != "" {
			ip := inst.PrivateIP
			if idx := strings.Index(ip, "/"); idx != -1 {
				ip = ip[:idx]
			}
			return ip
		}
		time.Sleep(5 * time.Second)
	}
	return ""
}

func (p *KubeadmProvisioner) waitForKubeconfig(ctx context.Context, cluster *domain.Cluster, ip string) (string, error) {
	exec, err := p.getExecutor(ctx, cluster, ip)
	if err != nil {
		return "", err
	}

	for i := 0; i < 60; i++ { // Wait up to 10 mins
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		out, err := exec.Run(ctx, "cat "+adminKubeconfig)
		if err == nil {
			return out, nil
		}

		timer := time.NewTimer(10 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
			timer.Stop()
		}
	}
	return "", fmt.Errorf("timed out waiting for kubeconfig")
}

func (p *KubeadmProvisioner) getExecutor(ctx context.Context, cluster *domain.Cluster, ip string) (NodeExecutor, error) {
	if p.executorFactory != nil {
		return p.executorFactory(ctx, cluster, ip)
	}
	ip = normalizeInstanceIP(ip)

	// 1. Try to find instance by IP to use ServiceExecutor (preferred for Docker/Local/Managed)
	// This avoids needing SSH access if we have direct control via the backend.
	instances, err := p.instSvc.ListInstances(ctx, nil)
	if err == nil {
		for _, inst := range instances {
			if normalizeInstanceIP(inst.PrivateIP) == ip {
				// We found the managed instance! Use the ServiceExecutor.
				return NewServiceExecutor(p.instSvc, inst.ID), nil
			}
		}
	}

	// 2. Fallback to SSH
	if cluster.SSHPrivateKeyEncrypted == "" {
		return nil, errors.New(errors.Internal, "no SSH key found for node access")
	}

	decryptedKey, err := p.secretSvc.Decrypt(ctx, cluster.UserID, cluster.SSHPrivateKeyEncrypted)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to decrypt SSH key", err)
	}

	hostPublicKey := ""
	if err == nil {
		for _, inst := range instances {
			if normalizeInstanceIP(inst.PrivateIP) == ip {
				hostPublicKey = inst.Metadata["thecloud.io/ssh-host-public-key"]
				break
			}
		}
	}
	if hostPublicKey == "" {
		return nil, errors.New(errors.Internal, "no pinned SSH host key found for node access")
	}

	return NewSSHExecutor(ip, defaultUser, decryptedKey, hostPublicKey), nil
}

func (p *KubeadmProvisioner) Deprovision(ctx context.Context, cluster *domain.Cluster) error {
	p.logger.Info("deprovisioning cluster", "cluster_id", cluster.ID)

	nodes, err := p.repo.GetNodes(ctx, cluster.ID)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		_ = p.instSvc.TerminateInstance(ctx, node.InstanceID.String())
		_ = p.repo.DeleteNode(ctx, node.ID)
	}

	if cluster.HAEnabled {
		lbName := fmt.Sprintf("lb-k8s-%s", cluster.Name)
		lbs, err := p.lbSvc.List(ctx)
		if err == nil {
			for _, lb := range lbs {
				if lb.Name == lbName && lb.VpcID == cluster.VpcID {
					p.logger.Info("cleaning up cluster load balancer", "cluster_id", cluster.ID, "lb_id", lb.ID)
					_ = p.lbSvc.Delete(ctx, lb.ID.String())
				}
			}
		}
	}

	return nil
}

func (p *KubeadmProvisioner) failCluster(ctx context.Context, cluster *domain.Cluster, msg string, err error) error {
	cluster.Status = domain.ClusterStatusFailed
	if updateErr := p.repo.Update(ctx, cluster); updateErr != nil {
		// We're already returning a failure; log so the persistence
		// gap is visible in operations rather than silently dropped.
		p.logger.Error("failed to persist failed cluster status",
			"cluster_id", cluster.ID, "error", updateErr, "original_error", err)
	}
	p.logger.Error(msg, "cluster_id", cluster.ID, "error", err)
	return fmt.Errorf("%s: %w", msg, err)
}

func (p *KubeadmProvisioner) GetStatus(ctx context.Context, cluster *domain.Cluster) (domain.ClusterStatus, error) {
	return cluster.Status, nil
}

func (p *KubeadmProvisioner) Repair(ctx context.Context, cluster *domain.Cluster) error { return nil }

func (p *KubeadmProvisioner) Scale(ctx context.Context, cluster *domain.Cluster) error {
	return p.provisionWorkers(ctx, cluster)
}

func (p *KubeadmProvisioner) GetKubeconfig(ctx context.Context, cluster *domain.Cluster, role string) (string, error) {
	if cluster.KubeconfigEncrypted == "" {
		return "", errors.New(errors.NotFound, "kubeconfig not found")
	}
	return p.secretSvc.Decrypt(ctx, cluster.UserID, cluster.KubeconfigEncrypted)
}

func (p *KubeadmProvisioner) GetHealth(ctx context.Context, cluster *domain.Cluster) (*ports.ClusterHealth, error) {
	if len(cluster.ControlPlaneIPs) == 0 {
		return nil, errors.New(errors.NotFound, "no control plane IPs found")
	}

	masterIP := cluster.ControlPlaneIPs[0]
	exec, err := p.getExecutor(ctx, cluster, masterIP)
	if err != nil {
		return nil, err
	}

	// Check API Server
	_, err = exec.Run(ctx, "kubectl --kubeconfig "+adminKubeconfig+" get nodes")
	health := &ports.ClusterHealth{
		APIServer: err == nil,
	}

	if err != nil {
		health.Message = "API server is unreachable"
		//nolint:nilerr
		return health, nil
	}

	// Get Node Status
	out, err := exec.Run(ctx, "kubectl --kubeconfig "+adminKubeconfig+" get nodes --no-headers")
	if err == nil {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		health.NodesTotal = len(lines)
		for _, line := range lines {
			if strings.Contains(line, " Ready ") {
				health.NodesReady++
			}
		}
		health.Message = fmt.Sprintf("%d/%d nodes are ready", health.NodesReady, health.NodesTotal)
	}

	return health, nil
}

func (p *KubeadmProvisioner) Upgrade(ctx context.Context, cluster *domain.Cluster, version string) error {
	p.logger.Info("upgrading cluster", "cluster_id", cluster.ID, "to_version", version)

	if len(cluster.ControlPlaneIPs) == 0 {
		return errors.New(errors.InvalidInput, "no control plane nodes found")
	}

	// 1. Upgrade Control Plane nodes one by one
	nodes, err := p.repo.GetNodes(ctx, cluster.ID)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		if node.Role == domain.NodeRoleControlPlane {
			if err := p.upgradeNode(ctx, cluster, node, version); err != nil {
				return err
			}
		}
	}

	// 2. Upgrade Worker nodes one by one
	for _, node := range nodes {
		if node.Role == domain.NodeRoleWorker {
			if err := p.upgradeNode(ctx, cluster, node, version); err != nil {
				return err
			}
		}
	}

	cluster.Version = version
	return p.repo.Update(ctx, cluster)
}

func (p *KubeadmProvisioner) upgradeNode(ctx context.Context, cluster *domain.Cluster, node *domain.ClusterNode, version string) error {
	inst, err := p.instSvc.GetInstance(ctx, node.InstanceID.String())
	if err != nil {
		return err
	}

	exec, err := p.getExecutor(ctx, cluster, inst.PrivateIP)
	if err != nil {
		return err
	}

	v := strings.TrimPrefix(version, "v")
	minor := v[:strings.LastIndex(v, ".")]

	p.logger.Info("upgrading node", "node_id", node.ID, "ip", inst.PrivateIP)

	cmds := []string{
		"apt-get update",
		fmt.Sprintf("apt-get install -y --allow-change-held-packages kubeadm=%s-*", v),
	}

	if node.Role == domain.NodeRoleControlPlane {
		cmds = append(cmds, fmt.Sprintf("kubeadm upgrade apply v%s -y", minor))
	} else {
		cmds = append(cmds, "kubeadm upgrade node")
	}

	cmds = append(cmds, fmt.Sprintf("apt-get install -y --allow-change-held-packages kubelet=%s-* kubectl=%s-*", v, v))
	cmds = append(cmds, "systemctl daemon-reload", "systemctl restart kubelet")

	for _, cmd := range cmds {
		if _, err := exec.Run(ctx, cmd); err != nil {
			return errors.Wrap(errors.Internal, "failed to run upgrade command", err)
		}
	}

	return nil
}

func (p *KubeadmProvisioner) RotateSecrets(ctx context.Context, cluster *domain.Cluster) error {
	p.logger.Info("rotating cluster secrets", "cluster_id", cluster.ID)

	if len(cluster.ControlPlaneIPs) == 0 {
		return errors.New(errors.InvalidInput, "no control plane node for rotation")
	}

	masterIP := cluster.ControlPlaneIPs[0]
	exec, err := p.getExecutor(ctx, cluster, masterIP)
	if err != nil {
		return err
	}

	// 1. Renew certs
	if _, err := exec.Run(ctx, "kubeadm certs renew all"); err != nil {
		return err
	}

	// 2. Update Kubeconfig in DB
	kubeconfig, err := exec.Run(ctx, "cat "+adminKubeconfig)
	if err != nil {
		return err
	}

	encryptedKubeconfig, err := p.secretSvc.Encrypt(ctx, cluster.UserID, kubeconfig)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to encrypt kubeconfig", err)
	}
	cluster.KubeconfigEncrypted = encryptedKubeconfig

	return p.repo.Update(ctx, cluster)
}

func (p *KubeadmProvisioner) CreateBackup(ctx context.Context, cluster *domain.Cluster) error {
	p.logger.Info("creating cluster backup", "cluster_id", cluster.ID)

	if len(cluster.ControlPlaneIPs) == 0 {
		return errors.New(errors.InvalidInput, "no control plane node for backup")
	}

	masterIP := cluster.ControlPlaneIPs[0]
	exec, err := p.getExecutor(ctx, cluster, masterIP)
	if err != nil {
		return err
	}

	// Backup etcd
	backupCmd := "ETCDCTL_API=3 etcdctl --endpoints=https://127.0.0.1:2379 " +
		"--cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt " +
		"--key=/etc/kubernetes/pki/etcd/server.key snapshot save /tmp/snapshot.db"

	if _, err := exec.Run(ctx, backupCmd); err != nil {
		return errors.Wrap(errors.Internal, "etcd snapshot failed", err)
	}

	// Upload to storage service
	// We'd need a way to stream files or cat them
	snapshotData, err := exec.Run(ctx, "base64 /tmp/snapshot.db")
	if err != nil {
		return err
	}

	if p.storageSvc != nil {
		key := fmt.Sprintf("k8s-backups/%s/%d.db.b64", cluster.ID, time.Now().Unix())
		_, err = p.storageSvc.Upload(ctx, "k8s-backups", key, strings.NewReader(snapshotData), "")
		return err
	}

	return nil
}

func (p *KubeadmProvisioner) Restore(ctx context.Context, cluster *domain.Cluster, path string) error {
	p.logger.Info("restoring cluster from backup", "cluster_id", cluster.ID, "path", path)

	if len(cluster.ControlPlaneIPs) == 0 {
		return errors.New(errors.InvalidInput, "no control plane node for restore")
	}
	if cluster.HAEnabled || len(cluster.ControlPlaneIPs) > 1 {
		return errors.New(errors.InvalidInput, "restore is only supported for single-control-plane clusters")
	}

	masterIP := cluster.ControlPlaneIPs[0]
	exec, err := p.getExecutor(ctx, cluster, masterIP)
	if err != nil {
		return err
	}

	// 1. Download backup from storage
	if p.storageSvc == nil {
		return errors.New(errors.Internal, "storage service not available")
	}

	rc, _, err := p.storageSvc.Download(ctx, "k8s-backups", path)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to download backup from storage", err)
	}
	defer func() { _ = rc.Close() }()

	var data io.Reader = rc
	if strings.HasSuffix(path, ".b64") {
		data = base64.NewDecoder(base64.StdEncoding, rc)
	}

	// 2. Prepare node: Stop control plane by moving manifests
	p.logger.Info("stopping control plane pods", "cluster_id", cluster.ID)
	manifestBackupDir := fmt.Sprintf("/var/lib/thecloud/manifests-backup-%d", time.Now().Unix())
	if err := p.moveControlPlaneManifests(ctx, exec, manifestBackupDir); err != nil {
		return errors.Wrap(errors.Internal, "failed to prepare node for restore", err)
	}

	if err := p.waitForEtcdToStop(ctx, exec, 20*time.Second); err != nil {
		if restoreErr := p.restoreControlPlaneManifests(ctx, exec, manifestBackupDir); restoreErr != nil {
			return errors.Wrap(errors.Internal, "failed waiting for etcd to stop and restore manifests", restoreErr)
		}
		return errors.Wrap(errors.Internal, "failed waiting for etcd to stop", err)
	}

	// 3. Upload snapshot to node
	p.logger.Info("uploading snapshot to node", "cluster_id", cluster.ID)
	if err := exec.WriteFile(ctx, "/tmp/restore-snapshot.db", data); err != nil {
		// Attempt to recover manifests before failing
		if restoreErr := p.restoreControlPlaneManifests(ctx, exec, manifestBackupDir); restoreErr != nil {
			return errors.Wrap(errors.Internal, "failed to upload snapshot to node and restore manifests", restoreErr)
		}
		return errors.Wrap(errors.Internal, "failed to upload snapshot to node", err)
	}

	// 4. Perform etcd restore
	p.logger.Info("restoring etcd data directory", "cluster_id", cluster.ID)
	restoreCmd := "ETCDCTL_API=3 etcdctl snapshot restore /tmp/restore-snapshot.db --data-dir /var/lib/etcd-restored"
	if _, err := exec.Run(ctx, restoreCmd); err != nil {
		if restoreErr := p.restoreControlPlaneManifests(ctx, exec, manifestBackupDir); restoreErr != nil {
			return errors.Wrap(errors.Internal, fmt.Sprintf("etcd snapshot restore failed: %v; additionally failed to restore manifests", err), restoreErr)
		}
		return errors.Wrap(errors.Internal, "etcd snapshot restore failed", err)
	}

	// 5. Swap data directories
	etcdBackupDir := fmt.Sprintf("/var/lib/etcd-backup-%d", time.Now().Unix())
	swapCmds := []string{
		fmt.Sprintf("if [ -d /var/lib/etcd ]; then mv /var/lib/etcd %s; fi", shellQuote(etcdBackupDir)),
		"mv /var/lib/etcd-restored /var/lib/etcd",
		"chown -R 0:0 /var/lib/etcd", // Ensure permissions
	}
	for _, cmd := range swapCmds {
		if _, err := exec.Run(ctx, cmd); err != nil {
			if rollbackErr := p.rollbackEtcdRestore(ctx, exec, manifestBackupDir, etcdBackupDir); rollbackErr != nil {
				return errors.Wrap(errors.Internal, fmt.Sprintf("failed to swap etcd directories: %v; additionally failed to rollback restore", err), rollbackErr)
			}
			return errors.Wrap(errors.Internal, "failed to swap etcd directories", err)
		}
	}

	// 6. Restart control plane
	p.logger.Info("restarting control plane pods", "cluster_id", cluster.ID)
	if err := p.restoreControlPlaneManifests(ctx, exec, manifestBackupDir); err != nil {
		return errors.Wrap(errors.Internal, "failed to restart control plane pods", err)
	}
	if err := p.waitForControlPlaneReady(ctx, exec, 2*time.Minute); err != nil {
		return errors.Wrap(errors.Internal, "control plane did not become ready after restore", err)
	}

	// 7. Cleanup
	if _, err := exec.Run(ctx, "rm /tmp/restore-snapshot.db"); err != nil {
		return errors.Wrap(errors.Internal, "failed to clean up restore snapshot", err)
	}

	p.logger.Info("cluster restore completed successfully", "cluster_id", cluster.ID)
	return nil
}

func (p *KubeadmProvisioner) moveControlPlaneManifests(ctx context.Context, exec NodeExecutor, backupDir string) error {
	if _, err := exec.Run(ctx, fmt.Sprintf("mkdir -p %s", shellQuote(backupDir))); err != nil {
		return err
	}
	moved := false
	for _, manifest := range controlPlaneManifestPaths {
		cmd := fmt.Sprintf("if [ -f %s ]; then mv %s %s/; fi", shellQuote(manifest), shellQuote(manifest), shellQuote(backupDir))
		if _, err := exec.Run(ctx, cmd); err != nil {
			if moved {
				_ = p.restoreControlPlaneManifests(ctx, exec, backupDir)
			}
			return err
		}
		moved = true
	}
	return nil
}

func (p *KubeadmProvisioner) restoreControlPlaneManifests(ctx context.Context, exec NodeExecutor, backupDir string) error {
	for _, manifest := range controlPlaneManifestPaths {
		name := filepath.Base(manifest)
		cmd := fmt.Sprintf("if [ -f %s/%s ]; then mv %s/%s %s; fi", shellQuote(backupDir), shellQuote(name), shellQuote(backupDir), shellQuote(name), shellQuote(manifest))
		if _, err := exec.Run(ctx, cmd); err != nil {
			return err
		}
	}
	return nil
}

func (p *KubeadmProvisioner) waitForEtcdToStop(ctx context.Context, exec NodeExecutor, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			manifestStopped := false
			if _, err := exec.Run(ctx, fmt.Sprintf("test ! -f %s", shellQuote(etcdManifest))); err == nil {
				manifestStopped = true
			}
			state, err := exec.Run(ctx, "if pgrep -f '[e]tcd' >/dev/null; then printf running; else printf stopped; fi")
			if err != nil {
				return err
			}
			etcdStopped := strings.TrimSpace(state) == "stopped"
			if manifestStopped && etcdStopped {
				return nil
			}
		}
	}
}

func (p *KubeadmProvisioner) waitForControlPlaneReady(ctx context.Context, exec NodeExecutor, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := exec.Run(ctx, fmt.Sprintf("KUBECONFIG=%s kubectl get --raw=/readyz >/dev/null", shellQuote(adminKubeconfig))); err == nil {
				return nil
			}
		}
	}
}

func normalizeInstanceIP(ip string) string {
	if idx := strings.Index(ip, "/"); idx != -1 {
		return ip[:idx]
	}
	return ip
}

func (p *KubeadmProvisioner) rollbackEtcdRestore(ctx context.Context, exec NodeExecutor, manifestBackupDir, etcdBackupDir string) error {
	if _, err := exec.Run(ctx, fmt.Sprintf("if [ -d %s ]; then rm -rf /var/lib/etcd && mv %s /var/lib/etcd; fi", shellQuote(etcdBackupDir), shellQuote(etcdBackupDir))); err != nil {
		return err
	}
	return p.restoreControlPlaneManifests(ctx, exec, manifestBackupDir)
}
