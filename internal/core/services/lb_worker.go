package services

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type LBWorker struct {
	lbRepo       ports.LBRepository
	instanceRepo ports.InstanceRepository
	proxyAdapter ports.LBProxyAdapter
}

func NewLBWorker(lbRepo ports.LBRepository, instanceRepo ports.InstanceRepository, proxyAdapter ports.LBProxyAdapter) *LBWorker {
	return &LBWorker{
		lbRepo:       lbRepo,
		instanceRepo: instanceRepo,
		proxyAdapter: proxyAdapter,
	}
}

func (w *LBWorker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Println("Load Balancer Worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Load Balancer Worker stopping")
			return
		case <-ticker.C:
			w.processCreatingLBs(ctx)
			w.processDeletingLBs(ctx)
			w.processActiveLBs(ctx)
			w.processHealthChecks(ctx)
		}
	}
}

func (w *LBWorker) processCreatingLBs(ctx context.Context) {
	lbs, err := w.lbRepo.ListAll(ctx)
	if err != nil {
		log.Printf("Worker: failed to list LBs: %v", err)
		return
	}

	for _, lb := range lbs {
		if lb.Status == domain.LBStatusCreating {
			gCtx := appcontext.WithUserID(ctx, lb.UserID)
			w.deployLB(gCtx, lb)
		}
	}
}

func (w *LBWorker) processDeletingLBs(ctx context.Context) {
	lbs, err := w.lbRepo.ListAll(ctx)
	if err != nil {
		return
	}

	for _, lb := range lbs {
		if lb.Status == domain.LBStatusDeleted {
			gCtx := appcontext.WithUserID(ctx, lb.UserID)
			w.cleanupLB(gCtx, lb)
		}
	}
}

func (w *LBWorker) deployLB(ctx context.Context, lb *domain.LoadBalancer) {
	log.Printf("Worker: deploying LB %s", lb.ID)

	targets, err := w.lbRepo.ListTargets(ctx, lb.ID)
	if err != nil {
		log.Printf("Worker: failed to list targets for LB %s: %v", lb.ID, err)
		return
	}

	_, err = w.proxyAdapter.DeployProxy(ctx, lb, targets)
	if err != nil {
		log.Printf("Worker: failed to deploy proxy for LB %s: %v", lb.ID, err)
		return
	}

	lb.Status = domain.LBStatusActive
	if err := w.lbRepo.Update(ctx, lb); err != nil {
		log.Printf("Worker: failed to update status for LB %s: %v", lb.ID, err)
	} else {
		log.Printf("Worker: LB %s is now ACTIVE", lb.ID)
	}
}

func (w *LBWorker) cleanupLB(ctx context.Context, lb *domain.LoadBalancer) {
	log.Printf("Worker: cleaning up LB %s", lb.ID)

	err := w.proxyAdapter.RemoveProxy(ctx, lb.ID)
	if err != nil {
		log.Printf("Worker: failed to remove proxy for LB %s: %v", lb.ID, err)
	}

	if err := w.lbRepo.Delete(ctx, lb.ID); err != nil {
		log.Printf("Worker: failed to delete LB %s from DB: %v", lb.ID, err)
	} else {
		log.Printf("Worker: LB %s fully removed", lb.ID)
	}
}

func (w *LBWorker) processActiveLBs(ctx context.Context) {
	lbs, err := w.lbRepo.ListAll(ctx)
	if err != nil {
		return
	}

	for _, lb := range lbs {
		if lb.Status == domain.LBStatusActive {
			gCtx := appcontext.WithUserID(ctx, lb.UserID)
			targets, err := w.lbRepo.ListTargets(gCtx, lb.ID)
			if err != nil {
				continue
			}
			// Update configuration (e.g. if targets changed)
			if err := w.proxyAdapter.UpdateProxyConfig(gCtx, lb, targets); err != nil {
				log.Printf("Worker: failed to update proxy config for LB %s: %v", lb.ID, err)
			}
		}
	}
}

func (w *LBWorker) processHealthChecks(ctx context.Context) {
	lbs, err := w.lbRepo.ListAll(ctx)
	if err != nil {
		return
	}

	for _, lb := range lbs {
		if lb.Status == domain.LBStatusActive {
			gCtx := appcontext.WithUserID(ctx, lb.UserID)
			w.checkLBHealth(gCtx, lb)
		}
	}
}

func (w *LBWorker) checkLBHealth(ctx context.Context, lb *domain.LoadBalancer) {
	targets, err := w.lbRepo.ListTargets(ctx, lb.ID)
	if err != nil {
		return
	}

	changed := false
	for _, t := range targets {
		inst, err := w.instanceRepo.GetByID(ctx, t.InstanceID)
		if err != nil {
			continue
		}

		// Simple TCP check to container name (Nginx uses this in the same network)
		// But here we are outside the network.
		// Wait, if we use container names, they are only resolvable inside the docker network.
		// For host-based health check, we'd need to know the host port.
		// For simplicity, let's assume we can reach the container port if we are on the same bridge or if we use the container IP.
		// In a real simulator, we might want to "exec" a ping inside the LB container.

		status := "unhealthy"
		// This is a naive check. In a real system, we'd use the proxy status or internal probes.
		// For this simulator, we'll try to connect to the mapped host port if available.
		// (Assuming ports are mapped as "hostPort:containerPort")
		hostPort := ""
		if inst.Ports != "" {
			// Find the port mapping for t.Port
			mappings := parsePorts(inst.Ports)
			for h, c := range mappings {
				if c == t.Port {
					hostPort = h
					break
				}
			}
		}

		if hostPort != "" {
			conn, err := net.DialTimeout("tcp", "localhost:"+hostPort, 2*time.Second)
			if err == nil {
				status = "healthy"
				_ = conn.Close()
			}
		}

		if t.Health != status {
			_ = w.lbRepo.UpdateTargetHealth(ctx, lb.ID, t.InstanceID, status)
			changed = true
		}
	}

	if changed {
		// If health changed, we might want to update the proxy config if it supports it,
		// or just let Nginx handle it (Nginx Open Source doesn't have active health checks easily).
		// For now, we just update the DB so the CLI can show it.
		log.Printf("Worker: health changed for LB %s", lb.ID)
		// w.proxyAdapter.UpdateProxyConfig(ctx, lb, targets) // Optional
	}
}

func parsePorts(ports string) map[string]int {
	res := make(map[string]int)
	pairs := splitCommas(ports)
	for _, p := range pairs {
		parts := splitColons(p)
		if len(parts) == 2 {
			var hPort string
			var cPort int
			_, _ = fmt.Sscanf(parts[0], "%s", &hPort)
			_, _ = fmt.Sscanf(parts[1], "%d", &cPort)
			res[hPort] = cPort
		}
	}
	return res
}

func splitCommas(s string) []string {
	var res []string
	current := ""
	for _, r := range s {
		if r == ',' {
			res = append(res, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		res = append(res, current)
	}
	return res
}

func splitColons(s string) []string {
	var res []string
	current := ""
	for _, r := range s {
		if r == ':' {
			res = append(res, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		res = append(res, current)
	}
	return res
}
