package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

type ContainerWorker struct {
	repo        ports.ContainerRepository
	instanceSvc ports.InstanceService
	eventSvc    ports.EventService
}

func NewContainerWorker(repo ports.ContainerRepository, instanceSvc ports.InstanceService, eventSvc ports.EventService) *ContainerWorker {
	return &ContainerWorker{
		repo:        repo,
		instanceSvc: instanceSvc,
		eventSvc:    eventSvc,
	}
}

func (w *ContainerWorker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	log.Println("CloudContainers Worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("CloudContainers Worker stopping")
			return
		case <-ticker.C:
			w.Reconcile(ctx)
		}
	}
}

func (w *ContainerWorker) Reconcile(ctx context.Context) {
	deployments, err := w.repo.ListAllDeployments(ctx)
	if err != nil {
		log.Printf("ContainerWorker: failed to list deployments: %v", err)
		return
	}

	for _, dep := range deployments {
		w.reconcileDeployment(ctx, dep)
	}
}

func (w *ContainerWorker) reconcileDeployment(ctx context.Context, dep *domain.Deployment) {
	// Wrap context with user ID
	uCtx := appcontext.WithUserID(ctx, dep.UserID)

	containerIDs, err := w.repo.GetContainers(uCtx, dep.ID)
	if err != nil {
		log.Printf("ContainerWorker: failed to get containers for %s: %v", dep.Name, err)
		return
	}

	current := len(containerIDs)

	if dep.Status == domain.DeploymentStatusDeleting {
		if current == 0 {
			_ = w.repo.DeleteDeployment(uCtx, dep.ID)
			return
		}
		for _, id := range containerIDs {
			_ = w.terminateContainer(uCtx, dep, id)
		}
		return
	}

	if current < dep.Replicas {
		needed := dep.Replicas - current
		for i := 0; i < needed; i++ {
			if err := w.launchContainer(uCtx, dep); err != nil {
				log.Printf("ContainerWorker: failed to launch container for %s: %v", dep.Name, err)
				break
			}
		}
	} else if current > dep.Replicas {
		excess := current - dep.Replicas
		for i := 0; i < excess; i++ {
			if err := w.terminateContainer(uCtx, dep, containerIDs[i]); err != nil {
				log.Printf("ContainerWorker: failed to terminate container for %s: %v", dep.Name, err)
				break
			}
		}
	}

	// Update status
	newStatus := domain.DeploymentStatusReady
	if len(containerIDs) != dep.Replicas {
		newStatus = domain.DeploymentStatusScaling
	}

	if dep.Status != newStatus || dep.CurrentCount != len(containerIDs) {
		dep.Status = newStatus
		dep.CurrentCount = len(containerIDs)
		_ = w.repo.UpdateDeployment(uCtx, dep)
	}
}

func (w *ContainerWorker) launchContainer(ctx context.Context, dep *domain.Deployment) error {
	name := fmt.Sprintf("dep-%s-%d", dep.Name, time.Now().UnixNano())

	// Deployments usually run in a default VPC or we could add VPC support to deployments
	// For now using nil VPC (default network)
	inst, err := w.instanceSvc.LaunchInstance(ctx, name, dep.Image, dep.Ports, nil, nil, nil)
	if err != nil {
		return err
	}

	if err := w.repo.AddContainer(ctx, dep.ID, inst.ID); err != nil {
		// Cleanup instance if association fails
		_ = w.instanceSvc.TerminateInstance(ctx, inst.ID.String())
		return err
	}

	return nil
}

func (w *ContainerWorker) terminateContainer(ctx context.Context, dep *domain.Deployment, instanceID uuid.UUID) error {
	if err := w.repo.RemoveContainer(ctx, dep.ID, instanceID); err != nil {
		return err
	}

	return w.instanceSvc.TerminateInstance(ctx, instanceID.String())
}
