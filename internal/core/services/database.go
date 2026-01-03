package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/util"
)

type DatabaseService struct {
	repo     ports.DatabaseRepository
	docker   ports.DockerClient
	vpcRepo  ports.VpcRepository
	eventSvc ports.EventService
	logger   *slog.Logger
}

func NewDatabaseService(
	repo ports.DatabaseRepository,
	docker ports.DockerClient,
	vpcRepo ports.VpcRepository,
	eventSvc ports.EventService,
	logger *slog.Logger,
) *DatabaseService {
	return &DatabaseService{
		repo:     repo,
		docker:   docker,
		vpcRepo:  vpcRepo,
		eventSvc: eventSvc,
		logger:   logger,
	}
}

func (s *DatabaseService) CreateDatabase(ctx context.Context, name, engine, version string, vpcID *uuid.UUID) (*domain.Database, error) {
	userID := appcontext.UserIDFromContext(ctx)

	// Validate engine
	dbEngine := domain.DatabaseEngine(engine)
	if dbEngine != domain.EnginePostgres && dbEngine != domain.EngineMySQL {
		return nil, errors.New(errors.InvalidInput, "unsupported database engine")
	}

	// Generate credentials
	password, err := util.GenerateRandomPassword(16)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to generate password", err)
	}
	username := "cloud_user"
	if dbEngine == domain.EngineMySQL {
		username = "root" // Default for many mysql images for easy setup
	}

	// Prepare domain object
	db := &domain.Database{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		Engine:    dbEngine,
		Version:   version,
		Status:    domain.DatabaseStatusCreating,
		VpcID:     vpcID,
		Username:  username,
		Password:  password,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Docker config
	imageName := ""
	var env []string
	defaultPort := ""

	switch dbEngine {
	case domain.EnginePostgres:
		imageName = fmt.Sprintf("postgres:%s-alpine", version)
		env = []string{
			"POSTGRES_USER=" + username,
			"POSTGRES_PASSWORD=" + password,
			"POSTGRES_DB=" + name,
		}
		defaultPort = "5432"
	case domain.EngineMySQL:
		imageName = fmt.Sprintf("mysql:%s", version)
		env = []string{
			"MYSQL_ROOT_PASSWORD=" + password,
			"MYSQL_DATABASE=" + name,
		}
		defaultPort = "3306"
	}

	// VPC config
	networkID := ""
	if vpcID != nil {
		vpc, err := s.vpcRepo.GetByID(ctx, *vpcID)
		if err != nil {
			return nil, errors.Wrap(errors.NotFound, "vpc not found", err)
		}
		networkID = vpc.NetworkID
	}

	// Launch container with dynamic port
	dockerName := fmt.Sprintf("cloud-db-%s-%s", name, db.ID.String()[:8])
	portMapping := []string{"0:" + defaultPort}

	containerID, err := s.docker.CreateContainer(ctx, dockerName, imageName, portMapping, networkID, nil, env, nil)
	if err != nil {
		s.logger.Error("failed to create database container", "error", err)
		return nil, errors.Wrap(errors.Internal, "failed to launch database container", err)
	}

	// 5. Fetch host port
	hostPort, err := s.docker.GetContainerPort(ctx, containerID, defaultPort)
	if err != nil {
		s.logger.Warn("failed to get mapped port", "container_id", containerID, "error", err)
		// We'll continue, but the connection string might be broken if not stored correctly.
	}

	db.ContainerID = containerID
	db.Port = hostPort
	db.Status = domain.DatabaseStatusRunning // For simplicity in MVP, we mark as running once container starts

	// Save to repo
	if err := s.repo.Create(ctx, db); err != nil {
		// Cleanup container if DB save fails
		_ = s.docker.RemoveContainer(ctx, containerID)
		return nil, err
	}

	_ = s.eventSvc.RecordEvent(ctx, "DATABASE_CREATE", db.ID.String(), "DATABASE", map[string]interface{}{
		"name":   db.Name,
		"engine": db.Engine,
	})

	return db, nil
}

func (s *DatabaseService) GetDatabase(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *DatabaseService) ListDatabases(ctx context.Context) ([]*domain.Database, error) {
	return s.repo.List(ctx)
}

func (s *DatabaseService) DeleteDatabase(ctx context.Context, id uuid.UUID) error {
	db, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 1. Remove container
	if db.ContainerID != "" {
		if err := s.docker.RemoveContainer(ctx, db.ContainerID); err != nil {
			s.logger.Warn("failed to remove database container", "container_id", db.ContainerID, "error", err)
		}
	}

	// 2. Delete from repo
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	_ = s.eventSvc.RecordEvent(ctx, "DATABASE_DELETE", id.String(), "DATABASE", nil)

	return nil
}

func (s *DatabaseService) GetConnectionString(ctx context.Context, id uuid.UUID) (string, error) {
	db, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	// In a real cloud, this would be the LB endpoint or internal VPC IP.
	// For this simulation, we'll return a localhost connection string with the mapped port.
	// Note: We need to get the mapped port from Docker if we don't store it accurately.
	// For now, let's assume we store it in db.Port (I should probably implement a way to retrieve it).

	switch db.Engine {
	case domain.EnginePostgres:
		return fmt.Sprintf("postgres://%s:%s@localhost:%d/%s", db.Username, db.Password, db.Port, db.Name), nil
	case domain.EngineMySQL:
		return fmt.Sprintf("%s:%s@tcp(localhost:%d)/%s", db.Username, db.Password, db.Port, db.Name), nil
	default:
		return "", errors.New(errors.Internal, "unknown engine")
	}
}

func (s *DatabaseService) GetDatabaseLogs(ctx context.Context, id uuid.UUID) (string, error) {
	db, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", err
	}

	if db.ContainerID == "" {
		return "", errors.New(errors.NotFound, "database container not found")
	}

	stream, err := s.docker.GetLogs(ctx, db.ContainerID)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	bytes, err := io.ReadAll(stream)
	if err != nil {
		return "", errors.Wrap(errors.Internal, "failed to read logs", err)
	}

	return string(bytes), nil
}
