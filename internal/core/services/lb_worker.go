// Package services implements core business workflows.
package services

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

const defaultBatchLimit = 100

// LBWorkerOption configures an LBWorker on construction.
type LBWorkerOption func(*LBWorker)

// WithBatchLimit sets the pagination batch size for worker queries.
// Defaults to 100 if not set.
func WithBatchLimit(n int) LBWorkerOption {
	return func(w *LBWorker) { w.batchLimit = n }
}

// LBWorker reconciles load balancer state and health checks.
type LBWorker struct {
	lbRepo       ports.LBRepository
	instanceRepo ports.InstanceRepository
	proxyAdapter ports.LBProxyAdapter
	dialer       PortDialer
	batchLimit   int
}

// PortDialer defines an interface for dialing network connections.
type PortDialer interface {
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
}

type realDialer struct{}

func (d *realDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, timeout)
}

// NewLBWorker constructs an LBWorker with its dependencies.
func NewLBWorker(lbRepo ports.LBRepository, instanceRepo ports.InstanceRepository, proxyAdapter ports.LBProxyAdapter, opts ...LBWorkerOption) *LBWorker {
	w := &LBWorker{
		lbRepo:       lbRepo,
		instanceRepo: instanceRepo,
		proxyAdapter: proxyAdapter,
		dialer:       &realDialer{},
		batchLimit:   defaultBatchLimit,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
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
	offset := 0
	for {
		lbs, err := w.lbRepo.ListByStatus(ctx, string(domain.LBStatusCreating), w.batchLimit, offset)
		if err != nil {
			log.Printf("Worker: failed to list creating LBs: %v", err)
			return
		}
		if len(lbs) == 0 {
			return
		}
		for _, lb := range lbs {
			gCtx := appcontext.WithUserID(ctx, lb.UserID)
			w.deployLB(gCtx, lb)
		}
		if len(lbs) < w.batchLimit {
			return
		}
		offset += w.batchLimit
	}
}

func (w *LBWorker) processDeletingLBs(ctx context.Context) {
	offset := 0
	for {
		lbs, err := w.lbRepo.ListByStatus(ctx, string(domain.LBStatusDeleted), w.batchLimit, offset)
		if err != nil {
			return
		}
		if len(lbs) == 0 {
			return
		}
		for _, lb := range lbs {
			gCtx := appcontext.WithTenantID(ctx, lb.TenantID)
			w.cleanupLB(gCtx, lb)
		}
		if len(lbs) < w.batchLimit {
			return
		}
		offset += w.batchLimit
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
	offset := 0
	for {
		lbs, err := w.lbRepo.ListByStatus(ctx, string(domain.LBStatusActive), w.batchLimit, offset)
		if err != nil {
			return
		}
		if len(lbs) == 0 {
			return
		}
		for _, lb := range lbs {
			gCtx := appcontext.WithUserID(ctx, lb.UserID)
			targets, err := w.lbRepo.ListTargets(gCtx, lb.ID)
			if err != nil {
				continue
			}
			if err := w.proxyAdapter.UpdateProxyConfig(gCtx, lb, targets); err != nil {
				log.Printf("Worker: failed to update proxy config for LB %s: %v", lb.ID, err)
			}
		}
		if len(lbs) < w.batchLimit {
			return
		}
		offset += w.batchLimit
	}
}

func (w *LBWorker) processHealthChecks(ctx context.Context) {
	offset := 0
	for {
		lbs, err := w.lbRepo.ListByStatus(ctx, string(domain.LBStatusActive), w.batchLimit, offset)
		if err != nil {
			return
		}
		if len(lbs) == 0 {
			return
		}
		for _, lb := range lbs {
			gCtx := appcontext.WithUserID(ctx, lb.UserID)
			w.checkLBHealth(gCtx, lb)
		}
		if len(lbs) < w.batchLimit {
			return
		}
		offset += w.batchLimit
	}
}

func (w *LBWorker) checkLBHealth(ctx context.Context, lb *domain.LoadBalancer) {
	targets, err := w.lbRepo.ListTargets(ctx, lb.ID)
	if err != nil {
		return
	}

	changed := false
	for _, t := range targets {
		if w.checkTargetHealth(ctx, lb, t) {
			changed = true
		}
	}

	if changed {
		log.Printf("Worker: health changed for LB %s", lb.ID)
	}
}

func (w *LBWorker) checkTargetHealth(ctx context.Context, lb *domain.LoadBalancer, t *domain.LBTarget) bool {
	inst, err := w.instanceRepo.GetByID(ctx, t.InstanceID)
	if err != nil {
		return false
	}

	hostPort := getHostPort(inst.Ports, t.Port)
	status := "unhealthy"
	if hostPort != "" {
		if w.isPortOpen(hostPort) {
			status = "healthy"
		}
	}

	if t.Health != status {
		_ = w.lbRepo.UpdateTargetHealth(ctx, lb.ID, t.InstanceID, status)
		return true
	}
	return false
}

func getHostPort(portsStr string, targetPort int) string {
	if portsStr == "" {
		return ""
	}
	mappings := parsePorts(portsStr)
	for h, c := range mappings {
		if c == targetPort {
			return h
		}
	}
	return ""
}

func (w *LBWorker) isPortOpen(port string) bool {
	conn, err := w.dialer.DialTimeout("tcp", "localhost:"+port, 2*time.Second)
	if err == nil {
		_ = conn.Close()
		return true
	}
	return false
}

func parsePorts(portsStr string) map[string]int {
	res := make(map[string]int)
	if portsStr == "" {
		return res
	}
	pairs := strings.Split(portsStr, ",")
	for _, p := range pairs {
		parts := strings.Split(p, ":")
		if len(parts) == 2 {
			var cPort int
			_, _ = fmt.Sscanf(parts[1], "%d", &cPort)
			res[parts[0]] = cPort
		}
	}
	return res
}
