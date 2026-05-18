package k8s

import (
	"context"
	"fmt"

	"github.com/poyrazk/thecloud/internal/core/domain"
)

func (p *KubeadmProvisioner) ensureClusterSecurityGroup(ctx context.Context, cluster *domain.Cluster) error {
	sgName := fmt.Sprintf("sg-k8s-%s", cluster.Name)

	sg, err := p.sgSvc.GetGroup(ctx, sgName, cluster.VpcID)
	if err == nil && sg != nil {
		return nil
	}

	p.logger.Info("creating cluster security group", "sg_name", sgName)
	newSG, err := p.sgSvc.CreateGroup(ctx, cluster.VpcID, sgName, "Managed security group for K8s cluster "+cluster.Name)
	if err != nil {
		return fmt.Errorf("failed to create security group: %w", err)
	}

	// Add default rules (K8s control plane ports)
	rules := []domain.SecurityRule{
		{Direction: domain.RuleIngress, Protocol: "tcp", PortMin: 6443, PortMax: 6443, CIDR: "0.0.0.0/0"},
		{Direction: domain.RuleIngress, Protocol: "tcp", PortMin: 22, PortMax: 22, CIDR: "0.0.0.0/0"},
		{Direction: domain.RuleIngress, Protocol: "tcp", PortMin: 2379, PortMax: 2380, CIDR: "10.0.0.0/8"},   // etcd (internal)
		{Direction: domain.RuleIngress, Protocol: "tcp", PortMin: 10250, PortMax: 10250, CIDR: "10.0.0.0/8"}, // kubelet
	}

	for _, rule := range rules {
		_, _ = p.sgSvc.AddRule(ctx, newSG.ID.String(), rule)
	}

	return nil
}
