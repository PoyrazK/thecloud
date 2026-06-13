package mock

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type MockLBRepo struct {
	mu  sync.RWMutex
	LBs map[uuid.UUID]*domain.LoadBalancer
}

func NewMockLBRepo() *MockLBRepo {
	return &MockLBRepo{
		LBs: make(map[uuid.UUID]*domain.LoadBalancer),
	}
}

func (m *MockLBRepo) Create(ctx context.Context, lb *domain.LoadBalancer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LBs[lb.ID] = lb
	return nil
}

func (m *MockLBRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.LoadBalancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if lb, ok := m.LBs[id]; ok {
		lbCopy := *lb
		return &lbCopy, nil
	}
	return nil, nil
}

func (m *MockLBRepo) GetByName(ctx context.Context, name string) (*domain.LoadBalancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, lb := range m.LBs {
		if lb.Name == name {
			lbCopy := *lb
			return &lbCopy, nil
		}
	}
	return nil, nil
}

func (m *MockLBRepo) Update(ctx context.Context, lb *domain.LoadBalancer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LBs[lb.ID] = lb
	return nil
}

func (m *MockLBRepo) Delete(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.LBs, id)
	return nil
}

func (m *MockLBRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.LoadBalancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, lb := range m.LBs {
		if lb.IdempotencyKey == key {
			lbCopy := *lb
			return &lbCopy, nil
		}
	}
	return nil, nil
}

func (m *MockLBRepo) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*domain.LoadBalancer, 0, len(m.LBs))
	for _, lb := range m.LBs {
		list = append(list, lb)
	}
	return list, nil
}

func (m *MockLBRepo) ListAll(ctx context.Context) ([]*domain.LoadBalancer, error) {
	return m.List(ctx)
}

func (m *MockLBRepo) ListByStatus(ctx context.Context, status string, limit, offset int) ([]*domain.LoadBalancer, error) {
	return m.List(ctx)
}

func (m *MockLBRepo) AddTarget(ctx context.Context, target *domain.LBTarget) error       { return nil }
func (m *MockLBRepo) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error { return nil }
func (m *MockLBRepo) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	return nil, nil
}
func (m *MockLBRepo) UpdateTargetHealth(ctx context.Context, lbID, instanceID uuid.UUID, health string) error {
	return nil
}
func (m *MockLBRepo) GetTargetsForInstance(ctx context.Context, instanceID uuid.UUID) ([]*domain.LBTarget, error) {
	return nil, nil
}

// Ensure interface satisfaction
var _ ports.LBRepository = (*MockLBRepo)(nil)
