package libvirt

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

type LBProxyAdapter struct{}

func NewLBProxyAdapter() *LBProxyAdapter {
	return &LBProxyAdapter{}
}

func (a *LBProxyAdapter) DeployProxy(ctx context.Context, lb *domain.LoadBalancer, targets []*domain.LBTarget) (string, error) {
	return "", fmt.Errorf("load balancer not supported on libvirt backend yet")
}

func (a *LBProxyAdapter) RemoveProxy(ctx context.Context, lbID uuid.UUID) error {
	return nil
}

func (a *LBProxyAdapter) UpdateProxyConfig(ctx context.Context, lb *domain.LoadBalancer, targets []*domain.LBTarget) error {
	return fmt.Errorf("load balancer not supported on libvirt backend yet")
}
