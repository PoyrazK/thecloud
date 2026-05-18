package docker

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockInstanceRepo struct {
	mock.Mock
}

func (m *mockInstanceRepo) Create(ctx context.Context, instance *domain.Instance) error {
	args := m.Called(ctx, instance)
	return args.Error(0)
}

func (m *mockInstanceRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}

func (m *mockInstanceRepo) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}

func (m *mockInstanceRepo) List(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	args := m.Called(ctx, tagFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceRepo) ListAll(ctx context.Context) ([]*domain.Instance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstanceRepo) ListBySubnet(ctx context.Context, subnetID uuid.UUID) ([]*domain.Instance, error) {
	args := m.Called(ctx, subnetID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}

func (m *mockInstanceRepo) Update(ctx context.Context, instance *domain.Instance) error {
	args := m.Called(ctx, instance)
	return args.Error(0)
}

func (m *mockInstanceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockVpcRepo struct {
	mock.Mock
}

func (m *mockVpcRepo) Create(ctx context.Context, vpc *domain.VPC) error {
	args := m.Called(ctx, vpc)
	return args.Error(0)
}

func (m *mockVpcRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.VPC, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.VPC)
	return r0, args.Error(1)
}

func (m *mockVpcRepo) GetByName(ctx context.Context, name string) (*domain.VPC, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.VPC)
	return r0, args.Error(1)
}

func (m *mockVpcRepo) List(ctx context.Context) ([]*domain.VPC, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.VPC)
	return r0, args.Error(1)
}

func (m *mockVpcRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.VPC, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.VPC)
	return r0, args.Error(1)
}

func (m *mockVpcRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockVpcRepo) Update(ctx context.Context, vpc *domain.VPC) error {
	args := m.Called(ctx, vpc)
	return args.Error(0)
}

func TestLBProxyAdapterGenerateNginxConfig(t *testing.T) {
	instRepo := new(mockInstanceRepo)
	vpcRepo := new(mockVpcRepo)
	adapter := &LBProxyAdapter{
		instanceRepo: instRepo,
		vpcRepo:      vpcRepo,
	}

	ctx := context.Background()
	lb := &domain.LoadBalancer{
		ID:        uuid.New(),
		Port:      80,
		Algorithm: "round-robin",
	}

	inst1ID := uuid.New()
	inst2ID := uuid.New()

	targets := []*domain.LBTarget{
		{InstanceID: inst1ID, Port: 8080, Weight: 1},
		{InstanceID: inst2ID, Port: 9090, Weight: 2},
	}

	inst1 := &domain.Instance{ID: inst1ID, ContainerID: "c1"}
	inst2 := &domain.Instance{ID: inst2ID, ContainerID: "c2"}

	instRepo.On("GetByID", ctx, inst1ID).Return(inst1, nil)
	instRepo.On("GetByID", ctx, inst2ID).Return(inst2, nil)

	t.Run("round-robin config", func(t *testing.T) {
		conf, err := adapter.generateNginxConfig(ctx, lb, targets)
		require.NoError(t, err)
		assert.Contains(t, conf, "server thecloud-"+inst1ID.String()[:8]+":8080 weight=1;")
		assert.Contains(t, conf, "server thecloud-"+inst2ID.String()[:8]+":9090 weight=2;")
		assert.Contains(t, conf, "listen 80;")
		assert.NotContains(t, conf, "least_conn;")
	})

	t.Run("least-conn config", func(t *testing.T) {
		lb.Algorithm = "least-conn"
		conf, err := adapter.generateNginxConfig(ctx, lb, targets)
		require.NoError(t, err)
		assert.Contains(t, conf, "least_conn;")
	})
}
