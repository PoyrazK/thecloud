// Package services implements core business workflows.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"github.com/poyrazk/thecloud/pkg/util"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerNameCache = "cache-service"

const (
	defaultRedisPort = "6379"
	// maxCacheStatsSize bounds cache stats JSON decoding to prevent memory exhaustion.
	maxCacheStatsSize = 1 * 1024 * 1024 // 1 MB
)

// CacheService manages cache clusters and their lifecycle.
type CacheService struct {
	repo     ports.CacheRepository
	rbacSvc  ports.RBACService
	compute  ports.ComputeBackend
	vpcRepo  ports.VpcRepository
	eventSvc ports.EventService
	auditSvc ports.AuditService
	logger   *slog.Logger
}

// NewCacheService constructs a CacheService with its dependencies.
func NewCacheService(
	repo ports.CacheRepository,
	rbacSvc ports.RBACService,
	compute ports.ComputeBackend,
	vpcRepo ports.VpcRepository,
	eventSvc ports.EventService,
	auditSvc ports.AuditService,
	logger *slog.Logger,
) *CacheService {
	return &CacheService{
		repo:     repo,
		rbacSvc:  rbacSvc,
		compute:  compute,
		vpcRepo:  vpcRepo,
		eventSvc: eventSvc,
		auditSvc: auditSvc,
		logger:   logger,
	}
}

func (s *CacheService) CreateCache(ctx context.Context, name, version string, memoryMB int, vpcID *uuid.UUID) (*domain.Cache, error) {
	tracer := otel.Tracer(tracerNameCache)
	_, span := tracer.Start(ctx, "CacheService.CreateCache",
		trace.WithAttributes(
			attribute.String("cache.name", name),
			attribute.String("cache.version", version),
			attribute.Int("cache.memory_mb", memoryMB),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCacheCreate, "*"); err != nil {
		span.RecordError(err)
		return nil, err
	}

	if s.compute.Type() == "libvirt" {
		return nil, errors.New(errors.InvalidInput, "managed cache requires docker compute backend (COMPUTE_BACKEND=libvirt is not supported)")
	}

	password, err := util.GenerateRandomPassword(16)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to generate password", err)
	}

	cache := &domain.Cache{
		ID:        uuid.New(),
		UserID:    userID,
		TenantID:  tenantID,
		Name:      name,
		Engine:    domain.EngineRedis,
		Version:   version,
		Status:    domain.CacheStatusCreating,
		VpcID:     vpcID,
		Password:  password,
		MemoryMB:  memoryMB,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	networkID, err := s.resolveNetworkID(ctx, vpcID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, cache); err != nil {
		return nil, err
	}

	containerID, allocatedPorts, err := s.launchCacheContainer(ctx, cache, networkID)
	if err != nil {
		if delErr := s.repo.Delete(ctx, cache.ID, tenantID); delErr != nil {
			s.logger.Error("failed to delete failed cache record", "id", cache.ID, "error", delErr)
		}
		return nil, errors.Wrap(errors.Internal, "failed to launch cache container", err)
	}

	port, err := s.parseAllocatedPort(allocatedPorts, defaultRedisPort)
	if err != nil || port == 0 {
		var portErr error
		port, portErr = s.compute.GetInstancePort(ctx, containerID, defaultRedisPort)
		if portErr != nil {
			s.logger.Error("failed to resolve cache port", "container_id", containerID, "error", portErr)
			if delErr := s.compute.DeleteInstance(ctx, containerID); delErr != nil {
				s.logger.Error("failed to clean up cache container after port resolution failure", "container_id", containerID, "error", delErr)
			}
			cache.Status = domain.CacheStatusFailed
			if upErr := s.repo.Update(ctx, cache); upErr != nil {
				s.logger.Error("failed to update cache status to failed", "id", cache.ID, "error", upErr)
			}
			return nil, errors.Wrap(errors.Internal, "failed to resolve cache port", portErr)
		}
	}

	cache.Status = domain.CacheStatusRunning
	cache.ContainerID = containerID
	cache.Port = port

	if err := s.repo.Update(ctx, cache); err != nil {
		s.logger.Warn("failed to update cache status after launch", "id", cache.ID, "error", err)
	}

	s.logCacheCreation(ctx, cache, name)

	return cache, nil
}

// parseAllocatedPort extracts the host port from allocated port mapping strings.
// Expected format is "hostPort:containerPort" (e.g. "8080:6379").
//
//nolint:unparam // targetPort is always defaultRedisPort but kept as param for future extensibility
func (s *CacheService) parseAllocatedPort(allocatedPorts []string, targetPort string) (int, error) {
	for _, p := range allocatedPorts {
		parts := strings.Split(p, ":")
		if len(parts) == 2 && parts[1] == targetPort {
			hp, err := strconv.Atoi(parts[0])
			if err != nil {
				return 0, err
			}
			return hp, nil
		}
	}
	return 0, nil
}

func (s *CacheService) resolveNetworkID(ctx context.Context, vpcID *uuid.UUID) (string, error) {
	if vpcID == nil {
		return "", nil
	}
	vpc, err := s.vpcRepo.GetByID(ctx, *vpcID)
	if err != nil {
		s.logger.Error("failed to get VPC", "vpc_id", vpcID, "error", err)
		return "", err
	}
	return vpc.NetworkID, nil
}

func (s *CacheService) launchCacheContainer(ctx context.Context, cache *domain.Cache, networkID string) (string, []string, error) {
	dockerName := fmt.Sprintf("thecloud-cache-%s", cache.ID.String()[:8])
	imageName := fmt.Sprintf("redis:%s-alpine", cache.Version)

	cmd := []string{
		"redis-server",
		"--appendonly", "yes",
		"--save", "",
		"--requirepass", cache.Password,
		"--maxmemory", fmt.Sprintf("%dmb", cache.MemoryMB),
		"--maxmemory-policy", "allkeys-lru",
		"--tcp-keepalive", "300",
	}

	containerID, allocatedPorts, err := s.compute.LaunchInstanceWithOptions(ctx, ports.CreateInstanceOptions{
		Name:      dockerName,
		ImageName: imageName,
		Ports:     []string{"0:" + defaultRedisPort},
		NetworkID: networkID,
		Cmd:       cmd,
	})
	if err != nil {
		s.logger.Error("failed to create cache container", "error", err)
		return "", nil, err
	}
	return containerID, allocatedPorts, nil
}

func (s *CacheService) logCacheCreation(ctx context.Context, cache *domain.Cache, originalName string) {
	if err := s.eventSvc.RecordEvent(ctx, "CACHE_CREATE", cache.ID.String(), "CACHE", map[string]interface{}{
		"name":    cache.Name,
		"version": cache.Version,
		"memory":  cache.MemoryMB,
	}); err != nil {
		s.logger.Warn("failed to record event", "action", "CACHE_CREATE", "cache_id", cache.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, cache.UserID, "cache.create", "cache", cache.ID.String(), map[string]interface{}{
		"name": originalName,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "cache.create", "cache_id", cache.ID, "error", err)
	}

	platform.CacheInstancesTotal.WithLabelValues("running").Inc()
}

func (s *CacheService) GetCache(ctx context.Context, idOrName string) (*domain.Cache, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCacheRead, idOrName); err != nil {
		return nil, err
	}

	return s.getCacheByIDOrName(ctx, idOrName)
}

func (s *CacheService) ListCaches(ctx context.Context) ([]*domain.Cache, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCacheRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.List(ctx, tenantID)
}

func (s *CacheService) DeleteCache(ctx context.Context, idOrName string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCacheDelete, idOrName); err != nil {
		return err
	}

	cache, err := s.getCacheByIDOrName(ctx, idOrName)
	if err != nil {
		return err
	}

	if cache.ContainerID != "" {
		if err := s.compute.StopInstance(ctx, cache.ContainerID); err != nil {
			s.logger.Warn("failed to stop cache container", "container_id", cache.ContainerID, "error", err)
		}
		if err := s.compute.DeleteInstance(ctx, cache.ContainerID); err != nil {
			s.logger.Warn("failed to remove cache container", "container_id", cache.ContainerID, "error", err)
		}
	}

	if err := s.repo.Delete(ctx, cache.ID, tenantID); err != nil {
		return err
	}

	if err := s.eventSvc.RecordEvent(ctx, "CACHE_DELETE", cache.ID.String(), "CACHE", nil); err != nil {
		s.logger.Warn("failed to record event", "action", "CACHE_DELETE", "cache_id", cache.ID, "error", err)
	}

	if err := s.auditSvc.Log(ctx, cache.UserID, "cache.delete", "cache", cache.ID.String(), map[string]interface{}{
		"name": cache.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "cache.delete", "cache_id", cache.ID, "error", err)
	}

	platform.CacheInstancesTotal.WithLabelValues("running").Dec()

	return nil
}

func (s *CacheService) GetConnectionString(ctx context.Context, idOrName string) (string, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCacheRead, idOrName); err != nil {
		return "", err
	}

	cache, err := s.getCacheByIDOrName(ctx, idOrName)
	if err != nil {
		return "", err
	}
	// format: redis://:password@host:port
	// We assume localhost for now as we don't have public IPs yet
	return fmt.Sprintf("redis://:%s@localhost:%d", cache.Password, cache.Port), nil
}

func (s *CacheService) getCacheByIDOrName(ctx context.Context, idOrName string) (*domain.Cache, error) {
	tenantID := appcontext.TenantIDFromContext(ctx)
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.repo.GetByID(ctx, id, tenantID)
	}
	return s.repo.GetByName(ctx, tenantID, idOrName)
}

func (s *CacheService) FlushCache(ctx context.Context, idOrName string) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCacheDelete, idOrName); err != nil { // Using Delete perm for flush
		return err
	}

	cache, err := s.getCacheByIDOrName(ctx, idOrName)
	if err != nil {
		return err
	}

	if cache.Status != domain.CacheStatusRunning {
		return errors.New(errors.InstanceNotRunning, "cache is not running")
	}

	if s.compute.Type() == "libvirt" {
		return errors.New(errors.InvalidInput, "cache flush requires docker compute backend (COMPUTE_BACKEND=libvirt is not supported)")
	}

	// Exec FLUSHALL inside the container
	// We need to pass the password if set.
	cmd := []string{"redis-cli"}
	if cache.Password != "" {
		cmd = append(cmd, "-a", cache.Password)
	}
	cmd = append(cmd, "FLUSHALL")

	output, err := s.compute.Exec(ctx, cache.ContainerID, cmd)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to flush cache: "+output, err)
	}

	if err := s.auditSvc.Log(ctx, cache.UserID, "cache.flush", "cache", cache.ID.String(), map[string]interface{}{}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "cache.flush", "cache_id", cache.ID, "error", err)
	}

	return nil
}

func (s *CacheService) GetCacheStats(ctx context.Context, idOrName string) (*ports.CacheStats, error) {
	tracer := otel.Tracer(tracerNameCache)
	_, span := tracer.Start(ctx, "CacheService.GetCacheStats",
		trace.WithAttributes(
			attribute.String("cache.id_or_name", idOrName),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCacheRead, idOrName); err != nil {
		span.RecordError(err)
		return nil, err
	}

	cache, err := s.getCacheByIDOrName(ctx, idOrName)
	if err != nil {
		return nil, err
	}

	if cache.Status != domain.CacheStatusRunning {
		return nil, errors.New(errors.InstanceNotRunning, "cache is not running")
	}

	if s.compute.Type() == "libvirt" {
		return nil, errors.New(errors.InvalidInput, "cache stats requires docker compute backend (COMPUTE_BACKEND=libvirt is not supported)")
	}

	stream, err := s.compute.GetInstanceStats(ctx, cache.ContainerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()

	// Parse Docker Stats (standard JSON)
	var dockerStats struct {
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
	}
	if err := json.NewDecoder(io.LimitReader(stream, maxCacheStatsSize)).Decode(&dockerStats); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to decode stats", err)
	}

	used := dockerStats.MemoryStats.Usage
	limit := dockerStats.MemoryStats.Limit

	var usedInt64 int64
	if used > math.MaxInt64 {
		usedInt64 = math.MaxInt64
	} else {
		usedInt64 = int64(used)
	}

	var limitInt64 int64
	if limit > math.MaxInt64 {
		limitInt64 = math.MaxInt64
	} else {
		limitInt64 = int64(limit)
	}

	result := &ports.CacheStats{
		UsedMemoryBytes:  usedInt64,
		MaxMemoryBytes:   limitInt64,
		ConnectedClients: 0,
		TotalKeys:        0,
	}

	// Try to get Redis Internal Stats
	cmd := []string{"redis-cli"}
	if cache.Password != "" {
		cmd = append(cmd, "-a", cache.Password)
	}
	cmd = append(cmd, "INFO")

	output, err := s.compute.Exec(ctx, cache.ContainerID, cmd)
	if err == nil {
		result.ConnectedClients = parseRedisClients(output)
		result.TotalKeys = parseRedisKeys(output)
	} else {
		s.logger.Warn("failed to get redis internal stats", "error", err)
	}

	return result, nil
}

func parseRedisClients(info string) int {
	// Look for connected_clients:N
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "connected_clients:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				val, _ := strconv.Atoi(parts[1])
				return val
			}
		}
	}
	return 0
}

func (s *CacheService) ResizeCache(ctx context.Context, idOrName string, newMemoryMB int) error {
	tracer := otel.Tracer(tracerNameCache)
	_, span := tracer.Start(ctx, "CacheService.ResizeCache",
		trace.WithAttributes(
			attribute.String("cache.id_or_name", idOrName),
			attribute.Int("cache.new_memory_mb", newMemoryMB),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionCacheUpdate, idOrName); err != nil {
		span.RecordError(err)
		return err
	}

	cache, err := s.getCacheByIDOrName(ctx, idOrName)
	if err != nil {
		span.RecordError(err)
		return err
	}

	if newMemoryMB <= cache.MemoryMB {
		return errors.New(errors.InvalidInput, "new memory must be larger than current memory")
	}

	if s.compute.Type() != "docker" {
		return errors.New(errors.InvalidInput, "cache memory resize requires docker compute backend")
	}

	networkID, err := s.resolveNetworkID(ctx, cache.VpcID)
	if err != nil {
		return err
	}

	// Stop and delete old container
	var oldContainerID string
	if cache.ContainerID != "" {
		oldContainerID = cache.ContainerID
		if err := s.compute.StopInstance(ctx, cache.ContainerID); err != nil {
			s.logger.Error("failed to stop cache container for resize", "container_id", cache.ContainerID, "error", err)
		}
		if err := s.compute.DeleteInstance(ctx, cache.ContainerID); err != nil {
			s.logger.Error("failed to delete cache container for resize", "container_id", cache.ContainerID, "error", err)
		}
	}

	// Relaunch with new memory limit
	containerID, allocatedPorts, err := s.launchCacheContainer(ctx, cache, networkID)
	if err != nil {
		// Rollback: attempt to restart old container
		if oldContainerID != "" {
			s.logger.Warn("resize failed, rolling back container", "old_container_id", oldContainerID)
			if restartErr := s.compute.StopInstance(ctx, oldContainerID); restartErr != nil {
				s.logger.Error("failed to stop old container during rollback", "container_id", oldContainerID, "error", restartErr)
			}
			if startErr := s.compute.StartInstance(ctx, oldContainerID); startErr != nil {
				s.logger.Error("failed to restart old container during rollback", "container_id", oldContainerID, "error", startErr)
			}
		}
		return errors.Wrap(errors.Internal, "failed to relaunch cache with new memory", err)
	}

	port, _ := s.parseAllocatedPort(allocatedPorts, defaultRedisPort)
	if port == 0 {
		port, _ = s.compute.GetInstancePort(ctx, containerID, defaultRedisPort)
	}

	cache.ContainerID = containerID
	cache.Port = port
	cache.MemoryMB = newMemoryMB
	cache.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, cache); err != nil {
		return fmt.Errorf("failed to update cache record in database: %w", err)
	}

	if err := s.auditSvc.Log(ctx, cache.UserID, "cache.resize", "cache", cache.ID.String(), map[string]interface{}{
		"name":          cache.Name,
		"old_memory_mb": cache.MemoryMB,
		"new_memory_mb": newMemoryMB,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "cache_id", cache.ID, "error", err)
	}

	return nil
}

func parseRedisKeys(info string) int64 {
	var total int64
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "db") && strings.Contains(line, "keys=") {
			total += parseKeysFromLine(line)
		}
	}
	return total
}

func parseKeysFromLine(line string) int64 {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return 0
	}
	stats := parts[1]
	pairs := strings.Split(stats, ",")
	for _, pair := range pairs {
		if strings.HasPrefix(pair, "keys=") {
			kv := strings.Split(pair, "=")
			if len(kv) == 2 {
				val, _ := strconv.ParseInt(kv[1], 10, 64)
				return val
			}
		}
	}
	return 0
}
