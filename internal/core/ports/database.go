package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type DatabaseRepository interface {
	Create(ctx context.Context, db *domain.Database) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Database, error)
	List(ctx context.Context) ([]*domain.Database, error)
	Update(ctx context.Context, db *domain.Database) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type DatabaseService interface {
	CreateDatabase(ctx context.Context, name, engine, version string, vpcID *uuid.UUID) (*domain.Database, error)
	GetDatabase(ctx context.Context, id uuid.UUID) (*domain.Database, error)
	ListDatabases(ctx context.Context) ([]*domain.Database, error)
	DeleteDatabase(ctx context.Context, id uuid.UUID) error
	GetConnectionString(ctx context.Context, id uuid.UUID) (string, error)
	GetDatabaseLogs(ctx context.Context, id uuid.UUID) (string, error)
}
