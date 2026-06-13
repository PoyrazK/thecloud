// Package mock provides mock implementations for ports and services for testing.
package mock

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

// MockGlobalLBRepo is a mock implementation of the GlobalLBRepository port.
type MockGlobalLBRepo struct {
	mu        sync.RWMutex
	GLBs      map[uuid.UUID]*domain.GlobalLoadBalancer
	Endpoints map[uuid.UUID][]*domain.GlobalEndpoint
}

// NewMockGlobalLBRepo creates a new instance of MockGlobalLBRepo.
func NewMockGlobalLBRepo() *MockGlobalLBRepo {
	return &MockGlobalLBRepo{
		GLBs:      make(map[uuid.UUID]*domain.GlobalLoadBalancer),
		Endpoints: make(map[uuid.UUID][]*domain.GlobalEndpoint),
	}
}

func (m *MockGlobalLBRepo) Create(ctx context.Context, glb *domain.GlobalLoadBalancer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GLBs[glb.ID] = glb
	return nil
}

func (m *MockGlobalLBRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.GlobalLoadBalancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if glb, ok := m.GLBs[id]; ok {
		glbCopy := *glb
		return &glbCopy, nil
	}
	return nil, nil
}

func (m *MockGlobalLBRepo) GetByHostname(ctx context.Context, hostname string) (*domain.GlobalLoadBalancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, glb := range m.GLBs {
		if glb.Hostname == hostname {
			glbCopy := *glb
			return &glbCopy, nil
		}
	}
	return nil, nil
}

func (m *MockGlobalLBRepo) List(ctx context.Context, userID uuid.UUID) ([]*domain.GlobalLoadBalancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*domain.GlobalLoadBalancer
	for _, glb := range m.GLBs {
		if glb.UserID == userID {
			list = append(list, glb)
		}
	}
	return list, nil
}

func (m *MockGlobalLBRepo) Update(ctx context.Context, glb *domain.GlobalLoadBalancer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GLBs[glb.ID] = glb
	return nil
}

func (m *MockGlobalLBRepo) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if glb, ok := m.GLBs[id]; ok && glb.UserID == userID {
		delete(m.GLBs, id)
	}
	return nil
}

func (m *MockGlobalLBRepo) AddEndpoint(ctx context.Context, ep *domain.GlobalEndpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Endpoints[ep.GlobalLBID] = append(m.Endpoints[ep.GlobalLBID], ep)
	return nil
}

func (m *MockGlobalLBRepo) RemoveEndpoint(ctx context.Context, endpointID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// inefficient but mock
	for glbID, eps := range m.Endpoints {
		var newEps []*domain.GlobalEndpoint
		for _, ep := range eps {
			if ep.ID != endpointID {
				newEps = append(newEps, ep)
			}
		}
		m.Endpoints[glbID] = newEps
	}
	return nil
}

func (m *MockGlobalLBRepo) GetEndpointByID(ctx context.Context, endpointID uuid.UUID) (*domain.GlobalEndpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, eps := range m.Endpoints {
		for _, ep := range eps {
			if ep.ID == endpointID {
				epCopy := *ep
				return &epCopy, nil
			}
		}
	}
	return nil, nil
}

func (m *MockGlobalLBRepo) ListEndpoints(ctx context.Context, glbID uuid.UUID) ([]*domain.GlobalEndpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	eps := m.Endpoints[glbID]
	epsCopy := make([]*domain.GlobalEndpoint, len(eps))
	copy(epsCopy, eps)
	return epsCopy, nil
}

func (m *MockGlobalLBRepo) UpdateEndpointHealth(ctx context.Context, epID uuid.UUID, healthy bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, eps := range m.Endpoints {
		for _, ep := range eps {
			if ep.ID == epID {
				ep.Healthy = healthy
				return nil
			}
		}
	}
	return nil
}

// Ensure interface satisfaction
var _ ports.GlobalLBRepository = (*MockGlobalLBRepo)(nil)
