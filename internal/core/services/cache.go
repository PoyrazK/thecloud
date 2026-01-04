package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/util"
)

type CacheService struct {
	repo     ports.CacheRepository
	docker   ports.DockerClient
	vpcRepo  ports.VpcRepository
	eventSvc ports.EventService
	logger   *slog.Logger
}

func NewCacheService(
	repo ports.CacheRepository,
	docker ports.DockerClient,
	vpcRepo ports.VpcRepository,
	eventSvc ports.EventService,
	logger *slog.Logger,
) *CacheService {
	return &CacheService{
		repo:     repo,
		docker:   docker,
		vpcRepo:  vpcRepo,
		eventSvc: eventSvc,
		logger:   logger,
	}
}

func (s *CacheService) CreateCache(ctx context.Context, name, version string, memoryMB int, vpcID *uuid.UUID) (*domain.Cache, error) {
	userID := appcontext.UserIDFromContext(ctx)

	// Generate password
	password, err := util.GenerateRandomPassword(16)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to generate password", err)
	}

	// Prepare domain object
	cacheID := uuid.New()
	cache := &domain.Cache{
		ID:        cacheID,
		UserID:    userID,
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

	// Persist initial state
	if err := s.repo.Create(ctx, cache); err != nil {
		return nil, err
	}

	// Network config
	networkID := ""
	if vpcID != nil {
		vpc, err := s.vpcRepo.GetByID(ctx, *vpcID)
		if err != nil {
			s.logger.Error("failed to get VPC", "vpc_id", vpcID, "error", err)
			return nil, err
		}
		networkID = vpc.NetworkID
	}

	// Docker config
	dockerName := fmt.Sprintf("thecloud-cache-%s", cache.ID.String()[:8])
	imageName := fmt.Sprintf("redis:%s-alpine", version)

	// Redis Command Arguments
	cmd := []string{
		"redis-server",
		"--appendonly", "yes",
		"--save", "", // Disable RDB
		"--requirepass", password,
		"--maxmemory", fmt.Sprintf("%dmb", memoryMB),
		"--maxmemory-policy", "allkeys-lru",
		"--tcp-keepalive", "300",
	}

	// Expose default port
	portMapping := []string{"0:6379"}

	containerID, err := s.docker.CreateContainer(ctx, dockerName, imageName, portMapping, networkID, nil, nil, cmd)
	if err != nil {
		s.logger.Error("failed to create cache container", "error", err)
		cache.Status = domain.CacheStatusFailed
		_ = s.repo.Delete(ctx, cache.ID) // Rollback
		return nil, errors.Wrap(errors.Internal, "failed to launch cache container", err)
	}

	// Get assigned port
	port, err := s.docker.GetContainerPort(ctx, containerID, "6379")
	if err != nil {
		s.logger.Error("failed to get cache port", "error", err)
		// Don't fail completely, try to recover later or let user retry
	}

	// Wait for container to be ready (healthcheck mock)
	// In real world we would wait for healthcheck. Here we assume generic success if started.

	// Update cache with container info
	cache.Status = domain.CacheStatusRunning
	cache.ContainerID = containerID
	cache.Port = port

	if err := s.repo.Update(ctx, cache); err != nil {
		s.logger.Warn("failed to update cache status after launch", "id", cache.ID, "error", err)
	}

	_ = s.eventSvc.RecordEvent(ctx, "CACHE_CREATE", cache.ID.String(), "CACHE", map[string]interface{}{
		"name":    cache.Name,
		"version": cache.Version,
		"memory":  cache.MemoryMB,
	})

	return cache, nil
}

func (s *CacheService) GetCache(ctx context.Context, idOrName string) (*domain.Cache, error) {
	return s.getCacheByIDOrName(ctx, idOrName)
}

func (s *CacheService) ListCaches(ctx context.Context) ([]*domain.Cache, error) {
	userID := appcontext.UserIDFromContext(ctx)
	return s.repo.List(ctx, userID)
}

func (s *CacheService) DeleteCache(ctx context.Context, idOrName string) error {
	cache, err := s.getCacheByIDOrName(ctx, idOrName)
	if err != nil {
		return err
	}

	if cache.ContainerID != "" {
		if err := s.docker.StopContainer(ctx, cache.ContainerID); err != nil {
			s.logger.Warn("failed to stop cache container", "container_id", cache.ContainerID, "error", err)
		}
		if err := s.docker.RemoveContainer(ctx, cache.ContainerID); err != nil {
			s.logger.Warn("failed to remove cache container", "container_id", cache.ContainerID, "error", err)
		}
	}

	if err := s.repo.Delete(ctx, cache.ID); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "CACHE_DELETE", cache.ID.String(), "CACHE", nil)
	return nil
}

func (s *CacheService) GetConnectionString(ctx context.Context, idOrName string) (string, error) {
	cache, err := s.getCacheByIDOrName(ctx, idOrName)
	if err != nil {
		return "", err
	}
	// format: redis://:password@host:port
	// We assume localhost for now as we don't have public IPs yet
	return fmt.Sprintf("redis://:%s@localhost:%d", cache.Password, cache.Port), nil
}

func (s *CacheService) getCacheByIDOrName(ctx context.Context, idOrName string) (*domain.Cache, error) {
	id, err := uuid.Parse(idOrName)
	if err == nil {
		return s.repo.GetByID(ctx, id)
	}
	userID := appcontext.UserIDFromContext(ctx)
	return s.repo.GetByName(ctx, userID, idOrName)
}

func (s *CacheService) FlushCache(ctx context.Context, idOrName string) error {
	cache, err := s.getCacheByIDOrName(ctx, idOrName)
	if err != nil {
		return err
	}

	if cache.Status != domain.CacheStatusRunning {
		return errors.New(errors.InstanceNotRunning, "cache is not running")
	}

	// Exec FLUSHALL inside the container
	// We need to pass the password if set.
	cmd := []string{"redis-cli"}
	if cache.Password != "" {
		cmd = append(cmd, "-a", cache.Password)
	}
	cmd = append(cmd, "FLUSHALL")

	output, err := s.docker.Exec(ctx, cache.ContainerID, cmd)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to flush cache: "+output, err)
	}

	return nil
}

func (s *CacheService) GetCacheStats(ctx context.Context, idOrName string) (*ports.CacheStats, error) {
	cache, err := s.getCacheByIDOrName(ctx, idOrName)
	if err != nil {
		return nil, err
	}

	if cache.Status != domain.CacheStatusRunning {
		return nil, errors.New(errors.InstanceNotRunning, "cache is not running")
	}

	stream, err := s.docker.GetContainerStats(ctx, cache.ContainerID)
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
	if err := json.NewDecoder(stream).Decode(&dockerStats); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to decode stats", err)
	}

	result := &ports.CacheStats{
		UsedMemoryBytes:  int64(dockerStats.MemoryStats.Usage),
		MaxMemoryBytes:   int64(dockerStats.MemoryStats.Limit),
		ConnectedClients: 0,
		TotalKeys:        0,
	}

	// Try to get Redis Internal Stats
	cmd := []string{"redis-cli"}
	if cache.Password != "" {
		cmd = append(cmd, "-a", cache.Password)
	}
	cmd = append(cmd, "INFO")

	output, err := s.docker.Exec(ctx, cache.ContainerID, cmd)
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

func parseRedisKeys(info string) int64 {
	// Look for Keyspace section, e.g. db0:keys=1,expires=0,avg_ttl=0
	// We want to sum all keys across all DBs
	var total int64
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "db") && strings.Contains(line, "keys=") {
			// db0:keys=1,...
			parts := strings.Split(line, ":")
			if len(parts) < 2 {
				continue
			}
			stats := parts[1] // keys=1,expires=0...
			pairs := strings.Split(stats, ",")
			for _, pair := range pairs {
				if strings.HasPrefix(pair, "keys=") {
					kv := strings.Split(pair, "=")
					if len(kv) == 2 {
						val, _ := strconv.ParseInt(kv[1], 10, 64)
						total += val
					}
				}
			}
		}
	}
	return total
}
