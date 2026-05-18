// Package services implements core business workflows.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// InstanceService manages compute instance lifecycle (containers or VMs).
//
// Supports multiple backends (Docker, Libvirt) and networking modes (bridge, VPC).
// Handles instance CRUD, port mapping, volume attachment, and resource monitoring.
//
// All methods are safe for concurrent use and return domain errors.

const (
	// NanoCPUsPerVCPU is the number of nanocpus per vCPU (1 vCPU = 1e9 nanocpus).
	NanoCPUsPerVCPU = int64(1e9)
	// BytesPerMB is the number of bytes per megabyte.
	BytesPerMB = int64(1024 * 1024)
	// maxStatsSize bounds instance stats JSON decoding to prevent memory exhaustion.
	maxStatsSize = 1 * 1024 * 1024 // 1 MB
)

type InstanceService struct {
	repo             ports.InstanceRepository
	vpcRepo          ports.VpcRepository
	subnetRepo       ports.SubnetRepository
	volumeRepo       ports.VolumeRepository
	instanceTypeRepo ports.InstanceTypeRepository
	rbacSvc          ports.RBACService
	compute          ports.ComputeBackend
	network          ports.NetworkBackend
	eventSvc         ports.EventService
	auditSvc         ports.AuditService
	dnsSvc           ports.DNSService
	logSvc           ports.LogService
	taskQueue        ports.TaskQueue
	tenantSvc        ports.TenantService
	sshKeySvc        ports.SSHKeyService
	dockerNetwork    string
	logger           *slog.Logger
}

// InstanceServiceParams holds dependencies for InstanceService creation.
// Uses parameter object pattern for cleaner dependency injection.
type InstanceServiceParams struct {
	Repo             ports.InstanceRepository
	VpcRepo          ports.VpcRepository
	SubnetRepo       ports.SubnetRepository
	VolumeRepo       ports.VolumeRepository
	InstanceTypeRepo ports.InstanceTypeRepository
	RBAC             ports.RBACService
	Compute          ports.ComputeBackend
	Network          ports.NetworkBackend
	EventSvc         ports.EventService
	AuditSvc         ports.AuditService
	DNSSvc           ports.DNSService
	LogSvc           ports.LogService
	TaskQueue        ports.TaskQueue // Optional
	TenantSvc        ports.TenantService
	SSHKeySvc        ports.SSHKeyService
	DockerNetwork    string // Optional
	Logger           *slog.Logger
}

// NewInstanceService creates a new InstanceService with the given dependencies.
func NewInstanceService(params InstanceServiceParams) *InstanceService {
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &InstanceService{
		repo:             params.Repo,
		vpcRepo:          params.VpcRepo,
		subnetRepo:       params.SubnetRepo,
		volumeRepo:       params.VolumeRepo,
		instanceTypeRepo: params.InstanceTypeRepo,
		rbacSvc:          params.RBAC,
		compute:          params.Compute,
		network:          params.Network,
		eventSvc:         params.EventSvc,
		auditSvc:         params.AuditSvc,
		dnsSvc:           params.DNSSvc,
		logSvc:           params.LogSvc,
		taskQueue:        params.TaskQueue,
		tenantSvc:        params.TenantSvc,
		sshKeySvc:        params.SSHKeySvc,
		dockerNetwork:    params.DockerNetwork,
		logger:           logger,
	}
}

// LaunchInstance provisions a new instance, sets up its network (if VPC/Subnet provided),
// and attaches any requested volumes.
func (s *InstanceService) LaunchInstance(ctx context.Context, params ports.LaunchParams) (*domain.Instance, error) {
	ctx, span := otel.Tracer("instance-service").Start(ctx, "LaunchInstance")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceLaunch, "*"); err != nil {
		return nil, err
	}

	span.SetAttributes(
		attribute.String("instance.name", params.Name),
		attribute.String("instance.image", params.Image),
	)

	// 1. Validate ports if provided
	_, err := s.parseAndValidatePorts(params.Ports)
	if err != nil {
		return nil, err
	}

	// 2. Resolve Instance Type
	instanceType := params.InstanceType
	if instanceType == "" {
		instanceType = "basic-2"
	}
	it, err := s.instanceTypeRepo.GetByID(ctx, instanceType)
	if err != nil {
		return nil, errors.New(errors.InvalidInput, fmt.Sprintf("invalid instance type: %s", instanceType))
	}

	// 3. Quota Check & Reservation

	// Resolve SSH Key if provided
	var userData string
	if params.SSHKeyID != nil {
		key, err := s.sshKeySvc.GetKey(ctx, *params.SSHKeyID)
		if err != nil {
			return nil, err
		}
		// Use a shell script for maximum compatibility with CirrOS and Ubuntu
		userData = fmt.Sprintf("#!/bin/sh\n"+
			"for user in cirros ubuntu root; do\n"+
			"  home=\"/home/$user\"\n"+
			"  if [ \"$user\" = \"root\" ]; then home=\"/root\"; fi\n"+
			"  if [ -d \"$home\" ]; then\n"+
			"    mkdir -p \"$home/.ssh\"\n"+
			"    echo '%s' >> \"$home/.ssh/authorized_keys\"\n"+
			"    chown -R \"$user:$user\" \"$home/.ssh\" 2>/dev/null || true\n"+
			"    chmod 700 \"$home/.ssh\"\n"+
			"    chmod 600 \"$home/.ssh/authorized_keys\"\n"+
			"  fi\n"+
			"done\n", key.PublicKey)
	}

	// Check instances quota
	if err := s.tenantSvc.CheckQuota(ctx, tenantID, "instances", 1); err != nil {
		return nil, err
	}

	// Check & Reserve vCPU/Memory quota
	// Note: We use atomic increment/decrement to manage usage state
	if err := s.tenantSvc.CheckQuota(ctx, tenantID, "vcpus", it.VCPUs); err != nil {
		return nil, err
	}
	if err := s.tenantSvc.CheckQuota(ctx, tenantID, "memory", it.MemoryMB/1024); err != nil {
		return nil, err
	}

	// Reserve resources
	if err := s.tenantSvc.IncrementUsage(ctx, tenantID, "vcpus", it.VCPUs); err != nil {
		return nil, err
	}
	if err := s.tenantSvc.IncrementUsage(ctx, tenantID, "memory", it.MemoryMB/1024); err != nil {
		// Rollback vCPUs if memory fails
		_ = s.tenantSvc.DecrementUsage(ctx, tenantID, "vcpus", it.VCPUs)
		return nil, err
	}

	// 4. Create domain entity
	inst := &domain.Instance{
		ID:           uuid.New(),
		UserID:       userID,
		TenantID:     tenantID,
		Name:         params.Name,
		Image:        params.Image,
		Status:       domain.StatusStarting,
		Ports:        params.Ports,
		VpcID:        params.VpcID,
		SubnetID:     params.SubnetID,
		InstanceType: instanceType,
		Version:      1,
		VolumeBinds:  params.VolumeBinds,
		Env:          params.Env,
		Cmd:          params.Cmd,
		CPULimit:     params.CPULimit,
		MemoryLimit:  params.MemoryLimit,
		DiskLimit:    params.DiskLimit,
		Metadata:     params.Metadata,
		Labels:       params.Labels,
		SSHKeyID:     params.SSHKeyID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.Create(ctx, inst); err != nil {
		// Rollback quota reservation
		_ = s.tenantSvc.DecrementUsage(ctx, tenantID, "vcpus", it.VCPUs)
		_ = s.tenantSvc.DecrementUsage(ctx, tenantID, "memory", it.MemoryMB/1024)
		return nil, err
	}

	// 4. Enqueue provision task
	job := domain.ProvisionJob{
		InstanceID:  inst.ID,
		UserID:      inst.UserID,
		TenantID:    inst.TenantID,
		Volumes:     params.Volumes,
		VolumeBinds: params.VolumeBinds,
		Env:         params.Env,
		Cmd:         params.Cmd,
		CPULimit:    params.CPULimit,
		MemoryLimit: params.MemoryLimit,
		DiskLimit:   params.DiskLimit,
		Metadata:    params.Metadata,
		Labels:      params.Labels,
		UserData:    userData,
	}

	s.logger.Info("enqueueing provision job", "instance_id", inst.ID, "queue", "provision_queue", "tenant_id", inst.TenantID)
	if err := s.taskQueue.Enqueue(ctx, "provision_queue", job); err != nil {
		s.logger.Error("failed to enqueue provision job", "instance_id", inst.ID, "error", err)
		// Rollback quota reservation on enqueue failure
		_ = s.tenantSvc.DecrementUsage(ctx, tenantID, "vcpus", it.VCPUs)
		_ = s.tenantSvc.DecrementUsage(ctx, tenantID, "memory", it.MemoryMB/1024)
		return nil, errors.Wrap(errors.Internal, "failed to enqueue provisioning task", err)
	}

	return inst, nil
}

// LaunchInstanceWithOptions provisions an instance using structured options.
func (s *InstanceService) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (*domain.Instance, error) {
	ctx, span := otel.Tracer("instance-service").Start(ctx, "LaunchInstanceWithOptions")
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceLaunch, "*"); err != nil {
		return nil, err
	}

	inst := &domain.Instance{
		ID:           uuid.New(),
		UserID:       userID,
		TenantID:     tenantID,
		Name:         opts.Name,
		Image:        opts.ImageName,
		Status:       domain.StatusStarting,
		Ports:        strings.Join(opts.Ports, ","),
		VolumeBinds:  opts.VolumeBinds,
		Env:          opts.Env,
		Cmd:          opts.Cmd,
		CPULimit:     opts.CPULimit,
		MemoryLimit:  opts.MemoryLimit,
		DiskLimit:    opts.DiskLimit,
		InstanceType: "custom", // Marking as custom since we are passing raw constraints or defaults
		Version:      1,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if opts.NetworkID != "" {
		vpcID, err := uuid.Parse(opts.NetworkID)
		if err != nil {
			return nil, errors.New(errors.InvalidInput, "invalid network id format")
		}
		inst.VpcID = &vpcID
	}

	if err := s.repo.Create(ctx, inst); err != nil {
		return nil, err
	}

	// 4. Enqueue provision task with full options
	job := domain.ProvisionJob{
		InstanceID:  inst.ID,
		UserID:      inst.UserID,
		TenantID:    inst.TenantID,
		UserData:    opts.UserData,
		Ports:       opts.Ports,
		VolumeBinds: opts.VolumeBinds,
		Env:         opts.Env,
		Cmd:         opts.Cmd,
		CPULimit:    opts.CPULimit,
		MemoryLimit: opts.MemoryLimit,
		DiskLimit:   opts.DiskLimit,
	}

	if err := s.taskQueue.Enqueue(ctx, "provision_queue", job); err != nil {
		s.logger.Error("failed to enqueue provision job", "instance_id", inst.ID, "error", err)
		return nil, errors.Wrap(errors.Internal, "failed to enqueue provisioning task", err)
	}

	return inst, nil
}

// Provision contains the heavy lifting of instance launch, called by background workers.
func (s *InstanceService) Provision(ctx context.Context, job domain.ProvisionJob) error {
	instanceID := job.InstanceID
	userData := job.UserData
	volumes := job.Volumes

	inst, err := s.repo.GetByID(ctx, instanceID)
	if err != nil {
		return err
	}

	// 1. Resolve Networking
	networkID, err := s.provisionNetwork(ctx, inst)
	if err != nil {
		s.updateStatus(ctx, inst)
		return err
	}

	// 2. Resolve Volumes
	volumeBinds, attachedVolumes, err := s.resolveVolumes(ctx, volumes)
	if err != nil {
		s.updateStatus(ctx, inst)
		return err
	}

	// 3. Create Instance
	it, itErr := s.instanceTypeRepo.GetByID(ctx, inst.InstanceType)
	if itErr != nil {
		s.updateStatus(ctx, inst)
		return errors.Wrap(errors.Internal, "failed to resolve instance type for provisioning", itErr)
	}

	// Use limits from instance type but override if custom values provided in inst/job
	cpuLimit := int64(it.VCPUs)
	if inst.CPULimit > 0 {
		cpuLimit = inst.CPULimit
	}
	memLimit := int64(it.MemoryMB) * 1024 * 1024
	if inst.MemoryLimit > 0 {
		memLimit = inst.MemoryLimit
	}
	diskLimit := int64(it.DiskGB) * 1024 * 1024 * 1024
	if inst.DiskLimit > 0 {
		diskLimit = inst.DiskLimit
	}

	dockerName := s.formatContainerName(inst.ID)
	portList, _ := s.parseAndValidatePorts(inst.Ports)
	containerID, allocatedPorts, err := s.compute.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{
		Name:        dockerName,
		ImageName:   inst.Image,
		Ports:       portList,
		NetworkID:   networkID,
		VolumeBinds: volumeBinds,
		Env:         inst.Env,
		Cmd:         inst.Cmd,
		CPULimit:    cpuLimit,
		MemoryLimit: memLimit,
		DiskLimit:   diskLimit,
		UserData:    userData,
	})
	if err != nil {
		platform.InstanceOperationsTotal.WithLabelValues("launch", "failure").Inc()
		s.updateStatus(ctx, inst)
		return errors.Wrap(errors.Internal, "failed to launch container", err)
	}

	// Update ports with actually allocated ones if any
	if len(allocatedPorts) > 0 {
		inst.Ports = strings.Join(allocatedPorts, ",")
	}

	// 4. Finalize
	return s.finalizeProvision(ctx, inst, containerID, attachedVolumes)
}
func (s *InstanceService) provisionNetwork(ctx context.Context, inst *domain.Instance) (string, error) {
	if s.compute.Type() == "noop" && inst.VpcID == nil && inst.SubnetID == nil {
		inst.PrivateIP = "127.0.0.1"
		return "", nil
	}

	networkID, allocatedIP, ovsPort, err := s.resolveNetworkConfig(ctx, inst.VpcID, inst.SubnetID)
	if err != nil {
		return "", err
	}

	inst.PrivateIP = allocatedIP
	inst.OvsPort = ovsPort
	return networkID, nil
}

func (s *InstanceService) finalizeProvision(ctx context.Context, inst *domain.Instance, containerID string, attachedVolumes []*domain.Volume) error {
	if err := s.plumbNetwork(ctx, inst, containerID); err != nil {
		s.logger.Warn("failed to plumb network", "error", err)
	}

	inst.Status = domain.StatusRunning
	inst.ContainerID = containerID

	// If IP was not allocated during provision (e.g. Docker dynamic), fetch it now
	if inst.PrivateIP == "" {
		ip, err := s.compute.GetInstanceIP(ctx, containerID)
		if err == nil && ip != "" {
			inst.PrivateIP = ip
		} else {
			s.logger.Warn("failed to get instance IP from backend", "instance_id", inst.ID, "error", err)
		}
	}

	// 5. Register DNS (if applicable)
	if s.dnsSvc != nil && inst.PrivateIP != "" {
		if err := s.dnsSvc.RegisterInstance(ctx, inst, inst.PrivateIP); err != nil {
			s.logger.Warn("failed to register instance DNS", "error", err, "instance", inst.Name)
			// Don't fail provisioning for DNS failure
		}
	}

	if err := s.repo.Update(ctx, inst); err != nil {
		return err
	}

	s.updateVolumesAfterLaunch(ctx, attachedVolumes, inst.ID)

	if err := s.eventSvc.RecordEvent(ctx, "INSTANCE_LAUNCH", inst.ID.String(), "INSTANCE", map[string]interface{}{
		"name":  inst.Name,
		"image": inst.Image,
		"ip":    inst.PrivateIP,
	}); err != nil {
		s.logger.Warn("failed to record event", "action", "INSTANCE_LAUNCH", "instance_id", inst.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, inst.UserID, "instance.launch", "instance", inst.ID.String(), map[string]interface{}{
		"name":  inst.Name,
		"image": inst.Image,
		"ip":    inst.PrivateIP,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "instance.launch", "instance_id", inst.ID, "error", err)
	}

	return nil
}

func (s *InstanceService) updateStatus(ctx context.Context, inst *domain.Instance) {
	inst.Status = domain.StatusError
	_ = s.repo.Update(ctx, inst)
}

func (s *InstanceService) parseAndValidatePorts(ports string) ([]string, error) {
	if ports == "" {
		return nil, nil
	}

	portList := strings.Split(ports, ",")
	if len(portList) > domain.MaxPortsPerInstance {
		return nil, errors.New(errors.TooManyPorts, fmt.Sprintf("max %d ports allowed", domain.MaxPortsPerInstance))
	}

	for _, p := range portList {
		if err := validatePortMapping(p); err != nil {
			return nil, err
		}
	}

	return portList, nil
}

func validatePortMapping(p string) error {
	idx := strings.Index(p, ":")
	if idx == -1 || strings.Contains(p[idx+1:], ":") {
		return errors.New(errors.InvalidPortFormat, "port format must be host:container")
	}

	hostPart := p[:idx]
	containerPart := p[idx+1:]

	hostPort, err := parsePort(hostPart)
	if err != nil {
		return errors.New(errors.InvalidPortFormat, fmt.Sprintf("invalid host port: %s", hostPart))
	}
	containerPort, err := parsePort(containerPart)
	if err != nil {
		return errors.New(errors.InvalidPortFormat, fmt.Sprintf("invalid container port: %s", containerPart))
	}

	if hostPort < domain.MinPort || hostPort > domain.MaxPort {
		return errors.New(errors.InvalidPortFormat, fmt.Sprintf("host port %d out of range (%d-%d)", hostPort, domain.MinPort, domain.MaxPort))
	}
	if containerPort < domain.MinPort || containerPort > domain.MaxPort {
		return errors.New(errors.InvalidPortFormat, fmt.Sprintf("container port %d out of range (%d-%d)", containerPort, domain.MinPort, domain.MaxPort))
	}

	return nil
}

func parsePort(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty port")
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return port, nil
}

// StartInstance boots up a stopped instance.
func (s *InstanceService) StartInstance(ctx context.Context, idOrName string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceUpdate, idOrName); err != nil {
		return err
	}

	// 1. Get from DB
	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return err
	}

	if inst.Status == domain.StatusRunning {
		return nil // Already running
	}

	// 2. Call Compute backend
	target := inst.ContainerID
	if target == "" {
		// Try to recover ID from name if missing
		target = s.formatContainerName(inst.ID)
	}

	if err := s.compute.StartInstance(ctx, target); err != nil {
		platform.InstanceOperationsTotal.WithLabelValues("start", "failure").Inc()
		s.logger.Error("failed to start instance", "instance_id", inst.ID, "container_id", target, "error", err)
		return errors.Wrap(errors.Internal, "failed to start instance", err)
	}

	// 3. Update Metrics & Status
	platform.InstancesTotal.WithLabelValues("stopped", s.compute.Type()).Dec()
	platform.InstancesTotal.WithLabelValues("running", s.compute.Type()).Inc()
	platform.InstanceOperationsTotal.WithLabelValues("start", "success").Inc()

	s.logger.Info("instance started", "instance_id", inst.ID)

	inst.Status = domain.StatusRunning
	if err := s.repo.Update(ctx, inst); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, inst.UserID, "instance.start", "instance", inst.ID.String(), map[string]interface{}{
		"name": inst.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "instance.start", "instance_id", inst.ID, "error", err)
	}

	return nil
}

// StopInstance halts a running instance's associated compute resource (e.g., container).
func (s *InstanceService) StopInstance(ctx context.Context, idOrName string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceUpdate, idOrName); err != nil {
		return err
	}

	// 1. Get from DB (handles both Name and UUID)
	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return err
	}

	if inst.Status == domain.StatusStopped {
		return nil // Already stopped
	}

	// 2. Call Docker stop
	target := inst.ContainerID
	if target == "" {
		// Fallback to Reconstruction
		target = s.formatContainerName(inst.ID)
	}

	if err := s.compute.StopInstance(ctx, target); err != nil {
		platform.InstanceOperationsTotal.WithLabelValues("stop", "failure").Inc()
		s.logger.Error("failed to stop instance", "backend", s.compute.Type(), "id", target, "error", err)
		return errors.Wrap(errors.Internal, fmt.Sprintf("failed to stop %s instance", s.compute.Type()), err)
	}

	platform.InstancesTotal.WithLabelValues("running", s.compute.Type()).Dec()
	platform.InstancesTotal.WithLabelValues("stopped", s.compute.Type()).Inc()
	platform.InstanceOperationsTotal.WithLabelValues("stop", "success").Inc()

	s.logger.Info("instance stopped", "instance_id", inst.ID)

	// 3. Update DB
	inst.Status = domain.StatusStopped
	if err := s.repo.Update(ctx, inst); err != nil {
		return err
	}

	if err := s.auditSvc.Log(ctx, inst.UserID, "instance.stop", "instance", inst.ID.String(), map[string]interface{}{
		"name": inst.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "instance.stop", "instance_id", inst.ID, "error", err)
	}

	return nil
}

// PauseInstance freezes a running instance (CPU halted, memory/network retained).
func (s *InstanceService) PauseInstance(ctx context.Context, idOrName string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceUpdate, idOrName); err != nil {
		return err
	}

	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return err
	}

	if inst.Status != domain.StatusRunning {
		return errors.New(errors.Conflict, "instance must be RUNNING to pause, got: "+string(inst.Status))
	}

	target := inst.ContainerID
	if target == "" {
		target = s.formatContainerName(inst.ID)
	}

	if err := s.compute.PauseInstance(ctx, target); err != nil {
		platform.InstanceOperationsTotal.WithLabelValues("pause", "failure").Inc()
		if errors.Is(err, errors.Conflict) {
			s.logger.Warn("pause not possible in current state", "container_id", target, "error", err)
			return errors.New(errors.Conflict, err.Error())
		}
		s.logger.Error("failed to pause container", "container_id", target, "error", err)
		return errors.Wrap(errors.Internal, "failed to pause container", err)
	}

	oldStatus := inst.Status
	inst.Status = domain.StatusPaused
	if err := s.repo.Update(ctx, inst); err != nil {
		// Best-effort rollback: undo the pause since DB update failed
		// Call compensating backend first, then restore DB status
		if resumeErr := s.compute.ResumeInstance(ctx, target); resumeErr != nil {
			s.logger.Warn("failed to undo pause after repo error",
				"instance_id", inst.ID, "resume_error", resumeErr)
		}
		inst.Status = oldStatus
		if rollbackErr := s.repo.Update(ctx, inst); rollbackErr != nil {
			s.logger.Warn("failed to rollback pause after repo error",
				"instance_id", inst.ID, "pause_error", err, "rollback_error", rollbackErr)
		}
		return err
	}

	if err := s.auditSvc.Log(ctx, inst.UserID, "instance.pause", "instance", inst.ID.String(), map[string]interface{}{
		"name": inst.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "instance.pause", "instance_id", inst.ID, "error", err)
	}

	platform.InstanceOperationsTotal.WithLabelValues("pause", "success").Inc()
	s.logger.Info("instance paused", "instance_id", inst.ID)
	return nil
}

// ResumeInstance resumes a paused instance back to running state.
func (s *InstanceService) ResumeInstance(ctx context.Context, idOrName string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceUpdate, idOrName); err != nil {
		return err
	}

	inst, err := s.GetInstance(ctx, idOrName)
	if err != nil {
		return err
	}

	if inst.Status != domain.StatusPaused {
		return errors.New(errors.Conflict, "instance must be PAUSED to resume, got: "+string(inst.Status))
	}

	target := inst.ContainerID
	if target == "" {
		target = s.formatContainerName(inst.ID)
	}

	if err := s.compute.ResumeInstance(ctx, target); err != nil {
		platform.InstanceOperationsTotal.WithLabelValues("resume", "failure").Inc()
		oldStatus := inst.Status
		if errors.Is(err, errors.Conflict) {
			s.logger.Warn("resume not possible in current state",
				"container_id", target, "instance_id", inst.ID, "error", err)
			return errors.New(errors.Conflict, err.Error())
		}
		s.logger.Error("failed to resume container, instance left in PAUSED state",
			"container_id", target, "instance_id", inst.ID, "error", err)
		inst.Status = oldStatus
		if repoErr := s.repo.Update(ctx, inst); repoErr != nil {
			s.logger.Error("failed to persist instance status after resume failure",
				"instance_id", inst.ID, "resume_error", err, "persist_error", repoErr)
		}
		return errors.Wrap(errors.Internal, "failed to resume container", err)
	}

	inst.Status = domain.StatusRunning
	if err := s.repo.Update(ctx, inst); err != nil {
		// Best-effort rollback: undo the resume since DB update failed
		// Call compensating backend first, then restore DB status
		if pauseErr := s.compute.PauseInstance(ctx, target); pauseErr != nil {
			s.logger.Warn("failed to undo resume after repo error",
				"instance_id", inst.ID, "pause_error", pauseErr)
		}
		inst.Status = domain.StatusPaused
		if rollbackErr := s.repo.Update(ctx, inst); rollbackErr != nil {
			s.logger.Warn("failed to rollback resume after repo error",
				"instance_id", inst.ID, "resume_error", err, "rollback_error", rollbackErr)
		}
		return err
	}

	if err := s.auditSvc.Log(ctx, inst.UserID, "instance.resume", "instance", inst.ID.String(), map[string]interface{}{
		"name": inst.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "instance.resume", "instance_id", inst.ID, "error", err)
	}

	platform.InstanceOperationsTotal.WithLabelValues("resume", "success").Inc()
	s.logger.Info("instance resumed", "instance_id", inst.ID)
	return nil
}

// ListInstances returns all instances owned by the current user.
// tagFilter optionally restricts results to instances matching all given label constraints.
func (s *InstanceService) ListInstances(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.List(ctx, tagFilter)
}

// GetInstance retrieves an instance by its UUID or name.
func (s *InstanceService) GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceRead, idOrName); err != nil {
		return nil, err
	}

	// 1. Try to parse as UUID
	id, uuidErr := uuid.Parse(idOrName)
	if uuidErr == nil {
		return s.repo.GetByID(ctx, id)
	}
	// 2. Fallback to name lookup
	return s.repo.GetByName(ctx, idOrName)
}

// GetInstanceLogs retrieves the execution logs from the instance's compute resource.
func (s *InstanceService) GetInstanceLogs(ctx context.Context, idOrName string) (string, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceRead, idOrName); err != nil {
		return "", err
	}

	inst, err := s.repo.GetByName(ctx, idOrName) // Use underlying repo to avoid double RBAC if GetInstance is used
	if err != nil {
		id, uuidErr := uuid.Parse(idOrName)
		if uuidErr == nil {
			inst, err = s.repo.GetByID(ctx, id)
		}
	}
	if err != nil || inst == nil {
		return "", errors.New(errors.NotFound, "instance not found")
	}

	if inst.ContainerID == "" {
		return "", errors.New(errors.InstanceNotRunning, "instance has no active container")
	}

	stream, err := s.compute.GetInstanceLogs(ctx, inst.ContainerID)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to get instance logs", err)
	}
	defer func() { _ = stream.Close() }()

	bytes, err := io.ReadAll(stream)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to read logs", err)
	}

	return string(bytes), nil
}

// GetConsoleURL returns the VNC console URL for an instance.
func (s *InstanceService) GetConsoleURL(ctx context.Context, idOrName string) (string, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceRead, idOrName); err != nil {
		return "", err
	}

	inst, err := s.repo.GetByName(ctx, idOrName)
	if err != nil {
		id, uuidErr := uuid.Parse(idOrName)
		if uuidErr == nil {
			inst, err = s.repo.GetByID(ctx, id)
		}
	}
	if err != nil || inst == nil {
		return "", errors.New(errors.NotFound, "instance not found")
	}

	id := inst.ID.String()
	if inst.ContainerID != "" {
		id = inst.ContainerID
	}

	url, err := s.compute.GetConsoleURL(ctx, id)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to get console URL", err)
	}
	return url, nil
}

func (s *InstanceService) ResizeInstance(ctx context.Context, idOrName, newInstanceType string) (*domain.Instance, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceResize, idOrName); err != nil {
		return nil, err
	}

	inst, err := s.resolveInstance(ctx, idOrName)
	if err != nil || inst == nil {
		return nil, errors.New(errors.NotFound, "instance not found")
	}

	oldIT, newIT, err := s.resolveInstanceTypes(ctx, inst.InstanceType, newInstanceType)
	if err != nil {
		return nil, err
	}

	if oldIT.ID == newIT.ID {
		s.logger.Info("instance already at target type, skipping resize", "instance_id", inst.ID, "type", oldIT.ID)
		return inst, nil
	}

	if err := s.validateResize(inst); err != nil {
		return nil, err
	}

	target := inst.ContainerID
	if target == "" {
		target = s.formatContainerName(inst.ID)
	}

	if err := s.completeResize(ctx, tenantID, inst, target, oldIT, newIT, newInstanceType); err != nil {
		return nil, err
	}

	s.logger.Info("instance resized", "instance_id", inst.ID, "old_type", oldIT.ID, "new_type", newIT.ID)
	return inst, nil
}

func (s *InstanceService) resolveInstance(ctx context.Context, idOrName string) (*domain.Instance, error) {
	inst, err := s.repo.GetByName(ctx, idOrName)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			id, uuidErr := uuid.Parse(idOrName)
			if uuidErr == nil {
				inst, err = s.repo.GetByID(ctx, id)
			}
		}
		if err != nil {
			return nil, err
		}
	}
	return inst, nil
}

func (s *InstanceService) resolveInstanceTypes(ctx context.Context, currentType, newType string) (*domain.InstanceType, *domain.InstanceType, error) {
	oldIT, err := s.instanceTypeRepo.GetByID(ctx, currentType)
	if err != nil {
		return nil, nil, errors.Wrap(errors.InvalidInput, "current instance type not found", err)
	}
	newIT, err := s.instanceTypeRepo.GetByID(ctx, newType)
	if err != nil {
		return nil, nil, errors.Wrap(errors.InvalidInput, "invalid instance type: "+newType, err)
	}
	return oldIT, newIT, nil
}

func (s *InstanceService) validateResize(inst *domain.Instance) error {
	if inst.ContainerID == "" {
		return errors.New(errors.InvalidInput, "instance has no active container, not yet provisioned")
	}
	if inst.Status != domain.StatusRunning && inst.Status != domain.StatusStopped {
		return errors.New(errors.Conflict, "instance state must be RUNNING or STOPPED to resize, got: "+string(inst.Status))
	}
	return nil
}

// rollbackQuotaChanges reverses quota modifications made before the compute resize attempt.
// It logs failures but does not return errors, since undo is not guaranteed to be possible.
func (s *InstanceService) rollbackQuotaChanges(ctx context.Context, tenantID uuid.UUID, deltaCPU, deltaMemMB, memoryGB int) {
	if deltaCPU > 0 {
		if err := s.tenantSvc.DecrementUsage(ctx, tenantID, "vcpus", deltaCPU); err != nil {
			s.logger.Error("rollback vcpu decrement failed", "error", err, "tenant_id", tenantID, "delta", deltaCPU)
		}
	} else if deltaCPU < 0 {
		if err := s.tenantSvc.IncrementUsage(ctx, tenantID, "vcpus", -deltaCPU); err != nil {
			s.logger.Error("rollback vcpu increment failed", "error", err, "tenant_id", tenantID, "delta", -deltaCPU)
		}
	}
	if deltaMemMB > 0 {
		if err := s.tenantSvc.DecrementUsage(ctx, tenantID, "memory", memoryGB); err != nil {
			s.logger.Error("rollback memory decrement failed", "error", err, "tenant_id", tenantID)
		}
	} else if deltaMemMB < 0 {
		if err := s.tenantSvc.IncrementUsage(ctx, tenantID, "memory", -memoryGB); err != nil {
			s.logger.Error("rollback memory increment failed", "error", err, "tenant_id", tenantID)
		}
	}
}

func (s *InstanceService) completeResize(ctx context.Context, tenantID uuid.UUID, inst *domain.Instance, target string, oldIT, newIT *domain.InstanceType, newInstanceType string) error {
	deltaCPU := newIT.VCPUs - oldIT.VCPUs
	deltaMemMB := newIT.MemoryMB - oldIT.MemoryMB
	memoryGB := deltaMemMB / 1024

	// 1. Quota changes first — fail fast before any VM state change
	if deltaCPU > 0 {
		if err := s.tenantSvc.CheckQuota(ctx, tenantID, "vcpus", deltaCPU); err != nil {
			return err
		}
		if err := s.tenantSvc.IncrementUsage(ctx, tenantID, "vcpus", deltaCPU); err != nil {
			platform.InstanceOperationsTotal.WithLabelValues("resize", "quota_failure").Inc()
			return errors.Wrap(errors.Internal, "failed to increment vCPU quota for resize", err)
		}
	} else if deltaCPU < 0 {
		if err := s.tenantSvc.DecrementUsage(ctx, tenantID, "vcpus", -deltaCPU); err != nil {
			platform.InstanceOperationsTotal.WithLabelValues("resize", "quota_decrement_failure").Inc()
			return errors.Wrap(errors.Internal, "failed to decrement vCPU quota for resize", err)
		}
	}
	if deltaMemMB > 0 {
		if err := s.tenantSvc.CheckQuota(ctx, tenantID, "memory", memoryGB); err != nil {
			// Rollback vCPU increment since memory quota check failed
			if deltaCPU > 0 {
				if decErr := s.tenantSvc.DecrementUsage(ctx, tenantID, "vcpus", deltaCPU); decErr != nil {
					return errors.Wrap(errors.Internal,
						fmt.Sprintf("memory quota check failed (%v), vCPU rollback also failed (%v)", err, decErr), err)
				}
			}
			return err
		}
		if err := s.tenantSvc.IncrementUsage(ctx, tenantID, "memory", memoryGB); err != nil {
			platform.InstanceOperationsTotal.WithLabelValues("resize", "quota_failure").Inc()
			// Rollback vCPU increment since memory increment failed
			if deltaCPU > 0 {
				if decErr := s.tenantSvc.DecrementUsage(ctx, tenantID, "vcpus", deltaCPU); decErr != nil {
					return errors.Wrap(errors.Internal,
						fmt.Sprintf("memory increment failed (%v), vCPU rollback also failed (%v)", err, decErr), err)
				}
			}
			return errors.Wrap(errors.Internal, "failed to increment memory quota for resize", err)
		}
	} else if deltaMemMB < 0 {
		if err := s.tenantSvc.DecrementUsage(ctx, tenantID, "memory", -memoryGB); err != nil {
			platform.InstanceOperationsTotal.WithLabelValues("resize", "quota_decrement_failure").Inc()
			return errors.Wrap(errors.Internal, "failed to decrement memory quota for resize", err)
		}
	}

	// 2. Compute resize (now that quota is settled)
	newCpuNano := int64(newIT.VCPUs) * NanoCPUsPerVCPU
	const maxMemoryMB = math.MaxInt64 / BytesPerMB
	if int64(newIT.MemoryMB) > maxMemoryMB {
		return fmt.Errorf("memory size overflows int64: %d MB", newIT.MemoryMB)
	}
	newMemoryBytes := int64(newIT.MemoryMB) * BytesPerMB
	if err := s.compute.ResizeInstance(ctx, target, newCpuNano, newMemoryBytes); err != nil {
		platform.InstanceOperationsTotal.WithLabelValues("resize", "failure").Inc()
		// Rollback quota changes since compute resize failed; log errors but continue since undo is not possible
		s.rollbackQuotaChanges(ctx, tenantID, deltaCPU, deltaMemMB, memoryGB)
		return errors.Wrap(errors.Internal, "failed to resize instance", err)
	}

	// 3. DB update with optimistic locking
	inst.InstanceType = newInstanceType
	inst.Version++
	if err := s.repo.Update(ctx, inst); err != nil {
		// Check if it's a conflict error (another resize beat us)
		if isConflictError(err) {
			// Conflict: another resize already committed - the compute is already at new size anyway
			// Log and return success since the resize did happen on compute backend
			s.logger.Warn("instance update conflict after resize, compute already at target", "instance_id", inst.ID)
			return nil
		}
		// Non-conflict error (e.g. network failure) - rollback and fail
		oldCpuNano := int64(oldIT.VCPUs) * NanoCPUsPerVCPU
		oldMemoryBytes := int64(oldIT.MemoryMB) * BytesPerMB
		var rollbackErrs []error

		if resizeErr := s.compute.ResizeInstance(ctx, target, oldCpuNano, oldMemoryBytes); resizeErr != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("compute resize rollback (target=%s, old_cpu_nano=%d, old_memory_bytes=%d): %w", target, oldCpuNano, oldMemoryBytes, resizeErr))
		}
		// Quota rollback for DB update failure (quota was successfully updated before compute resize)
		if deltaCPU > 0 {
			if decErr := s.tenantSvc.DecrementUsage(ctx, tenantID, "vcpus", deltaCPU); decErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("vcpu decrement rollback (tenant_id=%s, delta_cpu=%d): %w", tenantID, deltaCPU, decErr))
			}
		} else if deltaCPU < 0 {
			if incErr := s.tenantSvc.IncrementUsage(ctx, tenantID, "vcpus", -deltaCPU); incErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("vcpu increment rollback (tenant_id=%s, delta_cpu=%d): %w", tenantID, -deltaCPU, incErr))
			}
		}
		if deltaMemMB > 0 {
			if decErr := s.tenantSvc.DecrementUsage(ctx, tenantID, "memory", memoryGB); decErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("memory decrement rollback (tenant_id=%s, delta_mem_gb=%d): %w", tenantID, memoryGB, decErr))
			}
		} else if deltaMemMB < 0 {
			if incErr := s.tenantSvc.IncrementUsage(ctx, tenantID, "memory", -memoryGB); incErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("memory increment rollback (tenant_id=%s, delta_mem_gb=%d): %w", tenantID, -memoryGB, incErr))
			}
		}

		if len(rollbackErrs) > 0 {
			return errors.Wrap(errors.Internal, fmt.Sprintf("failed to update instance record (instance_id=%s), rollback attempted: %v", inst.ID, rollbackErrs), err)
		}
		return errors.Wrap(errors.Internal, "failed to update instance record, rollback attempted", err)
	}

	platform.InstanceOperationsTotal.WithLabelValues("resize", "success").Inc()
	s.recordInstanceResizeEvent(ctx, inst, oldIT, newIT, deltaCPU, deltaMemMB)
	return nil
}

// recordInstanceResizeEvent records the resize event and audit log.
func (s *InstanceService) recordInstanceResizeEvent(ctx context.Context, inst *domain.Instance, oldIT, newIT *domain.InstanceType, deltaCPU, deltaMemMB int) {
	params := map[string]interface{}{
		"name":            inst.Name,
		"old_type":        oldIT.ID,
		"new_type":        newIT.ID,
		"delta_vcpus":     deltaCPU,
		"delta_memory_mb": deltaMemMB,
	}
	if err := s.eventSvc.RecordEvent(ctx, "INSTANCE_RESIZE", inst.ID.String(), "INSTANCE", params); err != nil {
		s.logger.Warn("failed to record event", "action", "INSTANCE_RESIZE", "instance_id", inst.ID, "error", err)
	}
	if err := s.auditSvc.Log(ctx, inst.UserID, "instance.resize", "instance", inst.ID.String(), params); err != nil {
		s.logger.Warn("failed to log audit event", "action", "instance.resize", "instance_id", inst.ID, "error", err)
	}
}

// isConflictError returns true if the error is a conflict type (version mismatch)
func isConflictError(err error) bool {
	var e errors.Error
	if errors.As(err, &e) {
		return e.Type == errors.Conflict
	}
	return false
}

func (s *InstanceService) TerminateInstance(ctx context.Context, idOrName string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceTerminate, idOrName); err != nil {
		return err
	}

	inst, err := s.repo.GetByName(ctx, idOrName)
	if err != nil {
		id, uuidErr := uuid.Parse(idOrName)
		if uuidErr == nil {
			inst, err = s.repo.GetByID(ctx, id)
		}
	}
	if err != nil || inst == nil {
		return errors.New(errors.NotFound, "instance not found")
	}

	// Ingest logs before termination if LogService is available
	if s.logSvc != nil && inst.ContainerID != "" {
		logs, err := s.compute.GetInstanceLogs(ctx, inst.ContainerID)
		if err == nil {
			defer func() { _ = logs.Close() }()
			logBytes, _ := io.ReadAll(logs)
			if len(logBytes) > 0 {
				lines := strings.Split(string(logBytes), "\n")
				entries := make([]*domain.LogEntry, 0, len(lines))
				for _, line := range lines {
					if strings.TrimSpace(line) == "" {
						continue
					}
					entries = append(entries, &domain.LogEntry{
						ID:           uuid.New(),
						TenantID:     inst.TenantID,
						ResourceID:   inst.ID.String(),
						ResourceType: "instance",
						Level:        "INFO",
						Message:      line,
						Timestamp:    time.Now(),
					})
				}
				if len(entries) > 0 {
					if ingestErr := s.logSvc.IngestLogs(ctx, entries); ingestErr != nil {
						s.logger.Warn("failed to ingest logs during termination", "instance_id", inst.ID, "error", ingestErr)
					}
				}
			}
		}
	}

	if err := s.removeInstanceContainer(ctx, inst); err != nil {
		platform.InstanceOperationsTotal.WithLabelValues("terminate", "failure").Inc()
		return err
	}

	s.updateTerminationMetrics(inst)

	if err := s.releaseAttachedVolumes(ctx, inst.ID); err != nil {
		s.logger.Warn("failed to release volumes during termination", "instance_id", inst.ID, "error", err)
	}

	return s.finalizeTermination(ctx, inst)
}

func (s *InstanceService) updateTerminationMetrics(inst *domain.Instance) {
	switch inst.Status {
	case domain.StatusRunning:
		platform.InstancesTotal.WithLabelValues("running", s.compute.Type()).Dec()
	case domain.StatusStopped:
		platform.InstancesTotal.WithLabelValues("stopped", s.compute.Type()).Dec()
	}
	platform.InstanceOperationsTotal.WithLabelValues("terminate", "success").Inc()

	if s.dnsSvc != nil {
		_ = s.dnsSvc.UnregisterInstance(context.Background(), inst.ID)
	}
}

func (s *InstanceService) finalizeTermination(ctx context.Context, inst *domain.Instance) error {
	// Release Quota FIRST — before delete so failure is recoverable
	it, err := s.instanceTypeRepo.GetByID(ctx, inst.InstanceType)
	if err != nil {
		s.logger.Error("failed to resolve instance type for quota release", "instance_id", inst.ID, "type", inst.InstanceType, "error", err)
	}
	if it != nil {
		if err := s.tenantSvc.DecrementUsage(ctx, inst.TenantID, "instances", 1); err != nil {
			s.logger.Error("failed to decrement instance quota", "instance_id", inst.ID, "error", err)
		}
		if err := s.tenantSvc.DecrementUsage(ctx, inst.TenantID, "vcpus", it.VCPUs); err != nil {
			s.logger.Error("failed to decrement vcpu quota", "instance_id", inst.ID, "error", err)
		}
		if err := s.tenantSvc.DecrementUsage(ctx, inst.TenantID, "memory", it.MemoryMB/1024); err != nil {
			s.logger.Error("failed to decrement memory quota", "instance_id", inst.ID, "error", err)
		}
	}

	// Delete AFTER quota release — safe to fail now
	if err := s.repo.Delete(ctx, inst.ID); err != nil {
		return err
	}

	if err := s.eventSvc.RecordEvent(ctx, "INSTANCE_TERMINATE", inst.ID.String(), "INSTANCE", map[string]interface{}{}); err != nil {
		s.logger.Warn("failed to record event", "action", "INSTANCE_TERMINATE", "instance_id", inst.ID, "error", err)
	}
	if err := s.auditSvc.Log(ctx, inst.UserID, "instance.terminate", "instance", inst.ID.String(), map[string]interface{}{
		"name": inst.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "instance.terminate", "instance_id", inst.ID, "error", err)
	}

	return nil
}

func (s *InstanceService) removeInstanceContainer(ctx context.Context, inst *domain.Instance) error {
	containerID := inst.ContainerID
	if containerID == "" {
		// Fallback to Reconstruction for legacy or missing ID
		containerID = s.formatContainerName(inst.ID)
	}

	if err := s.compute.DeleteInstance(ctx, containerID); err != nil {
		s.logger.Warn("failed to remove instance", "backend", s.compute.Type(), "id", containerID, "error", err)
		return errors.Wrap(errors.Internal, fmt.Sprintf("failed to remove %s instance", s.compute.Type()), err)
	}

	s.logger.Info("instance terminated", "instance_id", inst.ID)
	return nil
}

// releaseAttachedVolumes marks all volumes attached to an instance as available
func (s *InstanceService) releaseAttachedVolumes(ctx context.Context, instanceID uuid.UUID) error {
	volumes, err := s.volumeRepo.ListByInstanceID(ctx, instanceID)
	if err != nil {
		return err
	}

	for _, vol := range volumes {
		vol.Status = domain.VolumeStatusAvailable
		vol.InstanceID = nil
		vol.MountPath = ""
		vol.UpdatedAt = time.Now()

		if err := s.volumeRepo.Update(ctx, vol); err != nil {
			s.logger.Warn("failed to release volume", "volume_id", vol.ID, "error", err)
			continue
		}
		s.logger.Info("volume released during instance termination", "volume_id", vol.ID, "instance_id", instanceID)
	}

	return nil
}

// GetInstanceStats retrieves real-time CPU and Memory usage for an instance.
func (s *InstanceService) GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceRead, idOrName); err != nil {
		return nil, err
	}

	inst, err := s.repo.GetByName(ctx, idOrName)
	if err != nil {
		id, uuidErr := uuid.Parse(idOrName)
		if uuidErr == nil {
			inst, err = s.repo.GetByID(ctx, id)
		}
	}
	if err != nil || inst == nil {
		return nil, errors.New(errors.NotFound, "instance not found")
	}

	if inst.ContainerID == "" {
		return nil, errors.New(errors.InstanceNotRunning, "instance not running")
	}

	stream, err := s.compute.GetInstanceStats(ctx, inst.ContainerID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to get stats stream", err)
	}
	defer func() { _ = stream.Close() }()

	var stats domain.RawDockerStats
	if err := json.NewDecoder(io.LimitReader(stream, maxStatsSize)).Decode(&stats); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to decode stats", err)
	}

	return s.calculateInstanceStats(&stats), nil
}

func (s *InstanceService) calculateInstanceStats(stats *domain.RawDockerStats) *domain.InstanceStats {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage) - float64(stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemCPUUsage) - float64(stats.PreCPUStats.SystemCPUUsage)

	cpuPercent := 0.0
	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * 100.0
	}

	memUsage := float64(stats.MemoryStats.Usage)
	memLimit := float64(stats.MemoryStats.Limit)
	memPercent := 0.0
	if memLimit > 0 {
		memPercent = (memUsage / memLimit) * 100.0
	}

	// Sum network rx/tx across all interfaces
	var rxBytes, txBytes *uint64
	if stats.NetworkStats != nil {
		var r, t uint64
		for _, net := range stats.NetworkStats {
			r += net.RxBytes
			t += net.TxBytes
		}
		rxBytes = &r
		txBytes = &t
	}

	// Sum block read/write bytes
	var readBytes, writeBytes *uint64
	if stats.BlkioStats.IoServiceBytes != nil {
		var r, w uint64
		for _, entry := range stats.BlkioStats.IoServiceBytes {
			switch entry.Op {
			case "read", "Read":
				r += entry.Value
			case "write", "Write":
				w += entry.Value
			}
		}
		readBytes = &r
		writeBytes = &w
	}

	var cpuTime *uint64
	if ct := stats.CPUStats.CPUTime; ct > 0 {
		cpuTime = &ct
	}

	return &domain.InstanceStats{
		CPUPercentage:      cpuPercent,
		MemoryUsageBytes:   memUsage,
		MemoryLimitBytes:   memLimit,
		MemoryPercentage:   memPercent,
		NetworkRxBytes:     rxBytes,
		NetworkTxBytes:     txBytes,
		DiskReadBytes:      readBytes,
		DiskWriteBytes:     writeBytes,
		CPUTimeNanoseconds: cpuTime,
	}
}

func (s *InstanceService) getVolumeByIDOrName(ctx context.Context, idOrName string) (*domain.Volume, error) {
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.volumeRepo.GetByID(ctx, id)
	}
	return s.volumeRepo.GetByName(ctx, idOrName)
}

func (s *InstanceService) updateVolumesAfterLaunch(ctx context.Context, volumes []*domain.Volume, instanceID uuid.UUID) {
	for _, vol := range volumes {
		vol.Status = domain.VolumeStatusInUse
		vol.InstanceID = &instanceID
		vol.UpdatedAt = time.Now()
		if err := s.volumeRepo.Update(ctx, vol); err != nil {
			s.logger.Warn("failed to update volume status", "volume_id", vol.ID, "error", err)
		}
	}
}
func (s *InstanceService) allocateIP(ctx context.Context, subnet *domain.Subnet) (string, error) {
	_, ipNet, err := net.ParseCIDR(subnet.CIDRBlock)
	if err != nil {
		return "", err
	}

	instances, err := s.repo.ListBySubnet(ctx, subnet.ID)
	if err != nil {
		return "", err
	}

	usedIPs := make(map[string]bool)
	for _, inst := range instances {
		if inst.PrivateIP != "" {
			ip := inst.PrivateIP
			if idx := strings.Index(ip, "/"); idx != -1 {
				ip = ip[:idx]
			}
			usedIPs[ip] = true
		}
	}
	gw := subnet.GatewayIP
	if idx := strings.Index(gw, "/"); idx != -1 {
		gw = gw[:idx]
	}
	usedIPs[gw] = true

	// Find first available IP
	ip, err := s.findAvailableIP(ipNet, usedIPs)
	if err != nil {
		return "", err
	}
	return ip, nil
}

func (s *InstanceService) isValidHostIP(ip net.IP, n *net.IPNet) bool {
	if !n.Contains(ip) {
		return false
	}

	// For IPv4, skip network and broadcast addresses
	ip4 := ip.To4()
	if ip4 != nil {
		network := n.IP.To4()
		if ip4.Equal(network) {
			return false
		}

		// Calculate broadcast
		broadcast := make(net.IP, 4)
		for i := 0; i < 4; i++ {
			broadcast[i] = network[i] | ^n.Mask[i]
		}
		if ip4.Equal(broadcast) {
			return false
		}
	}

	return true
}

func (s *InstanceService) resolveNetworkConfig(ctx context.Context, vpcID, subnetID *uuid.UUID) (string, string, string, error) {
	var networkID string
	if vpcID != nil {
		vpc, err := s.vpcRepo.GetByID(ctx, *vpcID)
		if err != nil {
			s.logger.Error("failed to get VPC", "vpc_id", vpcID, "error", err)
			return "", "", "", err
		}
		networkID = vpc.NetworkID
	}

	// Implementation Note: The Docker compute backend utilizes a shared bridge network ('cloud-network')
	// to simulate VPC isolation pending full Open vSwitch (OVS) integration.
	if s.compute.Type() == "docker" {
		networkID = "cloud-network"
		if s.dockerNetwork != "" {
			networkID = s.dockerNetwork
		}

		// If no subnet is configured, we let the backend assign an IP (dynamic).
		// We return empty string here, and LaunchInstance should fetch the real IP later.
		if subnetID == nil {
			return networkID, "", "", nil
		}
	}

	// For libvirt compute with no network backend (NETWORK_BACKEND=noop),
	// return "default" (libvirt's built-in NAT network) instead of OVS bridge.
	// This avoids libvirt failing to find non-existent OVS bridges.
	if s.compute.Type() == "libvirt" && s.network == nil && vpcID != nil {
		return "default", "", "", nil
	}

	if subnetID == nil || s.network == nil {
		return networkID, "", "", nil
	}

	subnet, err := s.subnetRepo.GetByID(ctx, *subnetID)
	if err != nil {
		return "", "", "", errors.Wrap(errors.NotFound, "subnet not found", err)
	}

	// Dynamic IP allocation
	allocatedIP, err := s.allocateIP(ctx, subnet)
	if err != nil {
		return "", "", "", errors.Wrap(errors.ResourceLimitExceeded, "failed to allocate IP in subnet", err)
	}

	ovsPort := "veth-" + uuid.New().String()[:8]
	return networkID, allocatedIP, ovsPort, nil
}

func (s *InstanceService) resolveVolumes(ctx context.Context, volumes []domain.VolumeAttachment) ([]string, []*domain.Volume, error) {
	volumeBinds := make([]string, 0, len(volumes))
	attachedVolumes := make([]*domain.Volume, 0, len(volumes))
	for _, va := range volumes {
		vol, err := s.getVolumeByIDOrName(ctx, va.VolumeIDOrName)
		if err != nil {
			s.logger.Error("failed to get volume", "volume", va.VolumeIDOrName, "error", err)
			return nil, nil, errors.Wrap(errors.NotFound, "volume "+va.VolumeIDOrName+" not found", err)
		}
		if vol.Status != domain.VolumeStatusAvailable {
			return nil, nil, errors.New(errors.InvalidInput, "volume "+vol.Name+" is not available")
		}
		volName := "thecloud-vol-" + vol.ID.String()[:8]
		if vol.BackendPath != "" {
			volName = vol.BackendPath
		}
		volumeBinds = append(volumeBinds, volName+":"+va.MountPath)
		attachedVolumes = append(attachedVolumes, vol)
	}
	return volumeBinds, attachedVolumes, nil
}

func (s *InstanceService) plumbNetwork(ctx context.Context, inst *domain.Instance, _ string) error {
	if inst.OvsPort == "" || s.network == nil {
		return nil
	}

	vethContainer := "eth0-" + inst.ID.String()[:8]
	if err := s.network.CreateVethPair(ctx, inst.OvsPort, vethContainer); err != nil {
		// In Docker/Dev mode without real OVS, this might fail. We log and continue
		// to allow the instance to run (albeit without custom networking).
		s.logger.Warn("failed to create veth pair (networking might be limited)", "error", err)
		return nil
	}

	if inst.VpcID != nil {
		if err := s.attachToVpcBridge(ctx, *inst.VpcID, inst.OvsPort); err != nil {
			return err
		}
	}

	if inst.SubnetID != nil {
		return s.configureVethIP(ctx, *inst.SubnetID, vethContainer, inst.PrivateIP)
	}
	return nil
}

func (s *InstanceService) attachToVpcBridge(ctx context.Context, vpcID uuid.UUID, ovsPort string) error {
	vpc, err := s.vpcRepo.GetByID(ctx, vpcID)
	if err != nil || vpc == nil {
		return err
	}
	return s.network.AttachVethToBridge(ctx, vpc.NetworkID, ovsPort)
}

func (s *InstanceService) configureVethIP(ctx context.Context, subnetID uuid.UUID, vethContainer, privateIP string) error {
	subnet, err := s.subnetRepo.GetByID(ctx, subnetID)
	if err != nil || subnet == nil {
		return err
	}
	_, ipNet, _ := net.ParseCIDR(subnet.CIDRBlock)
	ones, _ := ipNet.Mask.Size()
	return s.network.SetVethIP(ctx, vethContainer, privateIP, strconv.Itoa(ones))
}

func (s *InstanceService) formatContainerName(id uuid.UUID) string {
	return "thecloud-" + id.String()[:8]
}

func (s *InstanceService) findAvailableIP(ipNet *net.IPNet, usedIPs map[string]bool) (string, error) {
	ip := make(net.IP, len(ipNet.IP))
	copy(ip, ipNet.IP)

	ones, _ := ipNet.Mask.Size()
	maxIPs := uint64(1) << (32 - ones)
	if maxIPs > 1<<16 {
		maxIPs = 1 << 16 // cap at 65536 to prevent unbounded iteration
	}

	for iterations := uint64(0); iterations < maxIPs; iterations++ {
		// Increment IP
		for i := len(ip) - 1; i >= 0; i-- {
			ip[i]++
			if ip[i] > 0 {
				break
			}
		}

		if !ipNet.Contains(ip) {
			break
		}

		displayIP := ip.String()
		if ip4 := ip.To4(); ip4 != nil {
			displayIP = ip4.String()
		}

		if !usedIPs[displayIP] && s.isValidHostIP(ip, ipNet) {
			return displayIP, nil
		}
	}
	return "", fmt.Errorf("no available IPs in subnet after %d attempts", maxIPs)
}

func (s *InstanceService) Exec(ctx context.Context, idOrName string, cmd []string) (string, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceUpdate, idOrName); err != nil {
		return "", err
	}

	inst, err := s.repo.GetByName(ctx, idOrName)
	if err != nil {
		id, uuidErr := uuid.Parse(idOrName)
		if uuidErr == nil {
			inst, err = s.repo.GetByID(ctx, id)
		}
	}
	if err != nil || inst == nil {
		return "", errors.New(errors.NotFound, "instance not found")
	}

	if inst.ContainerID == "" {
		return "", errors.New(errors.InstanceNotRunning, "instance not running")
	}

	// Authorization is checked implicitly by GetInstance, which validates ownership/tenancy.
	// Granular RBAC permissions for 'exec' operations are expected to be enforced by the caller.

	output, err := s.compute.Exec(ctx, inst.ContainerID, cmd)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to execute command", err)
	}

	return output, nil
}

// UpdateInstanceMetadata updates the metadata and labels of an instance.
func (s *InstanceService) UpdateInstanceMetadata(ctx context.Context, id uuid.UUID, metadata, labels map[string]string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceUpdate, id.String()); err != nil {
		return err
	}

	inst, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if metadata != nil {
		if inst.Metadata == nil {
			inst.Metadata = make(map[string]string)
		}
		for k, v := range metadata {
			if v == "" {
				delete(inst.Metadata, k)
			} else {
				inst.Metadata[k] = v
			}
		}
	}

	if labels != nil {
		if inst.Labels == nil {
			inst.Labels = make(map[string]string)
		}
		for k, v := range labels {
			if v == "" {
				delete(inst.Labels, k)
			} else {
				inst.Labels[k] = v
			}
		}
	}

	return s.repo.Update(ctx, inst)
}

// GetTags returns the label set for an instance.
func (s *InstanceService) GetTags(ctx context.Context, id uuid.UUID) (map[string]string, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceRead, id.String()); err != nil {
		return nil, err
	}

	inst, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if inst.Labels == nil {
		return map[string]string{}, nil
	}
	return inst.Labels, nil
}

// SetTags replaces an instance's labels with the provided map (add/update).
func (s *InstanceService) SetTags(ctx context.Context, id uuid.UUID, labels map[string]string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceUpdate, id.String()); err != nil {
		return err
	}

	inst, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if inst.Labels == nil {
		inst.Labels = make(map[string]string)
	}
	for k, v := range labels {
		if v == "" {
			delete(inst.Labels, k)
		} else {
			inst.Labels[k] = v
		}
	}

	return s.repo.Update(ctx, inst)
}

// RemoveTag deletes a single label key from an instance.
func (s *InstanceService) RemoveTag(ctx context.Context, id uuid.UUID, key string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionInstanceUpdate, id.String()); err != nil {
		return err
	}

	inst, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if inst.Labels != nil {
		delete(inst.Labels, key)
	}

	return s.repo.Update(ctx, inst)
}
