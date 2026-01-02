package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/poyraz/cloud/internal/core/domain"
	"github.com/poyraz/cloud/internal/core/ports"
	"github.com/poyraz/cloud/internal/platform"
)

type AutoScalingWorker struct {
	repo        ports.AutoScalingRepository
	instanceSvc ports.InstanceService
	lbSvc       ports.LBService
	eventSvc    ports.EventService
	clock       ports.Clock
}

func NewAutoScalingWorker(
	repo ports.AutoScalingRepository,
	instanceSvc ports.InstanceService,
	lbSvc ports.LBService,
	eventSvc ports.EventService,
	clock ports.Clock,
) *AutoScalingWorker {
	return &AutoScalingWorker{
		repo:        repo,
		instanceSvc: instanceSvc,
		lbSvc:       lbSvc,
		eventSvc:    eventSvc,
		clock:       clock,
	}
}

func (w *AutoScalingWorker) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Println("Auto-Scaling Worker started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Auto-Scaling Worker stopping")
			return
		case <-ticker.C:
			platform.AutoScalingEvaluations.Inc()
			w.evaluateAllGroups(ctx)
		}
	}
}

func (w *AutoScalingWorker) evaluateAllGroups(ctx context.Context) {
	groups, err := w.repo.ListGroups(ctx)
	if err != nil {
		log.Printf("AutoScaling: failed to list groups: %v", err)
		return
	}

	// Batch fetch instances to prevent N+1
	groupIDs := make([]uuid.UUID, len(groups))
	for i, g := range groups {
		groupIDs[i] = g.ID
	}
	instancesByGroup, err := w.repo.GetAllScalingGroupInstances(ctx, groupIDs)
	if err != nil {
		log.Printf("AutoScaling: failed to fetch group instances: %v", err)
		return
	}

	for _, group := range groups {
		// Calculate current count from actual instances, not just DB field
		// DB field `CurrentCount` is kept in sync but source of truth is the link table
		instances := instancesByGroup[group.ID]
		if len(instances) != group.CurrentCount {
			// Reconciliation: update group count if mismatched
			group.CurrentCount = len(instances)
			w.repo.UpdateGroup(ctx, group)
		}

		platform.AutoScalingCurrentInstances.WithLabelValues(group.ID.String()).Set(float64(group.CurrentCount))

		w.reconcileInstances(ctx, group, instances)
		w.evaluatePolicies(ctx, group, instances)
	}
}

func (w *AutoScalingWorker) reconcileInstances(ctx context.Context, group *domain.ScalingGroup, instanceIDs []uuid.UUID) {
	// 1. Check if we need to scale out to meet Desired/Min
	current := len(instanceIDs)

	// Force Desired to be within bounds
	if group.DesiredCount < group.MinInstances {
		group.DesiredCount = group.MinInstances
		w.repo.UpdateGroup(ctx, group)
	}
	if group.DesiredCount > group.MaxInstances {
		group.DesiredCount = group.MaxInstances
		w.repo.UpdateGroup(ctx, group)
	}

	if current < group.DesiredCount {
		needed := group.DesiredCount - current
		log.Printf("AutoScaling: Group %s needs %d more instances (Current: %d, Desired: %d)", group.Name, needed, current, group.DesiredCount)
		for i := 0; i < needed; i++ {
			if err := w.scaleOut(ctx, group, nil); err != nil {
				log.Printf("AutoScaling: failed to scale out group %s: %v", group.Name, err)
				break
			}
		}
	} else if current > group.DesiredCount {
		excess := current - group.DesiredCount
		log.Printf("AutoScaling: Group %s has %d excess instances", group.Name, excess)
		for i := 0; i < excess; i++ {
			// Remove oldest first? Or random?
			// For simplicity: last one.
			targetID := instanceIDs[len(instanceIDs)-1-i]
			if err := w.scaleIn(ctx, group, targetID, nil); err != nil {
				log.Printf("AutoScaling: failed to scale in group %s: %v", group.Name, err)
				break
			}
		}
	}
}

func (w *AutoScalingWorker) evaluatePolicies(ctx context.Context, group *domain.ScalingGroup, instanceIDs []uuid.UUID) {
	policies, err := w.repo.GetPoliciesForGroup(ctx, group.ID)
	if err != nil || len(policies) == 0 {
		return
	}

	// We only support 'cpu' metric type for now
	avgCPU, err := w.repo.GetAverageCPU(ctx, instanceIDs, w.clock.Now().Add(-1*time.Minute))
	if err != nil {
		log.Printf("AutoScaling: failed to get metrics for group %s: %v", group.ID, err)
		return
	}

	for _, policy := range policies {
		// Cooldown check
		if policy.LastScaledAt != nil {
			if w.clock.Now().Sub(*policy.LastScaledAt) < time.Duration(policy.CooldownSec)*time.Second {
				continue
			}
		}

		if policy.MetricType == "cpu" {
			if avgCPU > policy.TargetValue {
				// Scale Out
				if group.CurrentCount < group.MaxInstances {
					log.Printf("AutoScaling: Policy %s triggered Scale Out (CPU %.2f > %.2f)", policy.Name, avgCPU, policy.TargetValue)
					// Calculate new desired
					newDesired := group.CurrentCount + policy.ScaleOutStep
					if newDesired > group.MaxInstances {
						newDesired = group.MaxInstances
					}
					group.DesiredCount = newDesired
					w.repo.UpdateGroup(ctx, group)
					// Next tick will reconcile

					// Update policy last scaled
					w.repo.UpdatePolicyLastScaled(ctx, policy.ID, w.clock.Now())
					return // Only trigger one policy per tick per group to avoid conflicts
				}
			} else if avgCPU < (policy.TargetValue - 10.0) { // arbitrary 10% buffer for scale in
				// Scale In
				if group.CurrentCount > group.MinInstances {
					log.Printf("AutoScaling: Policy %s triggered Scale In (CPU %.2f < %.2f)", policy.Name, avgCPU, policy.TargetValue-10.0)
					newDesired := group.CurrentCount - policy.ScaleInStep
					if newDesired < group.MinInstances {
						newDesired = group.MinInstances
					}
					group.DesiredCount = newDesired
					w.repo.UpdateGroup(ctx, group)

					w.repo.UpdatePolicyLastScaled(ctx, policy.ID, w.clock.Now())
					return
				}
			}
		}
	}
}

func (w *AutoScalingWorker) scaleOut(ctx context.Context, group *domain.ScalingGroup, policy *domain.ScalingPolicy) error {
	// Create instance
	name := fmt.Sprintf("%s-%d", group.Name, w.clock.Now().UnixNano()) // Unique name

	// Use dynamic ports to avoid conflicts on the same host
	dynamicPorts := toDynamicPorts(group.Ports)

	inst, err := w.instanceSvc.LaunchInstance(ctx, name, group.Image, dynamicPorts, &group.VpcID, nil)
	if err != nil {
		return err
	}

	// Register with group
	if err := w.repo.AddInstanceToGroup(ctx, group.ID, inst.ID); err != nil {
		return err
	}

	// Add to LB
	if group.LoadBalancerID != nil {
		if err := w.lbSvc.AddTarget(ctx, *group.LoadBalancerID, inst.ID, 80, 1); err != nil {
			log.Printf("AutoScaling: failed to add instance to LB: %v", err)
			// Continue, don't fail the whole scale out.
		}
	}

	platform.AutoScalingScaleOutEvents.Inc()
	w.eventSvc.RecordEvent(ctx, "AUTOSCALING_SCALE_OUT", group.ID.String(), "SCALING_GROUP", map[string]interface{}{
		"instance_id": inst.ID.String(),
		"trigger":     "reconciliation",
	})

	return nil
}

func (w *AutoScalingWorker) scaleIn(ctx context.Context, group *domain.ScalingGroup, instanceID uuid.UUID, policy *domain.ScalingPolicy) error {
	// Remove from LB
	if group.LoadBalancerID != nil {
		if err := w.lbSvc.RemoveTarget(ctx, *group.LoadBalancerID, instanceID); err != nil {
			log.Printf("AutoScaling: failed to remove instance from LB: %v", err)
		}
	}

	// Remove from group
	if err := w.repo.RemoveInstanceFromGroup(ctx, group.ID, instanceID); err != nil {
		return err
	}

	// Terminate instance
	if err := w.instanceSvc.TerminateInstance(ctx, instanceID.String()); err != nil {
		log.Printf("AutoScaling: failed to terminate instance %s: %v", instanceID, err)
		// We already removed it from group, so it's "gone" from ASG perspective
	}

	platform.AutoScalingScaleInEvents.Inc()
	w.eventSvc.RecordEvent(ctx, "AUTOSCALING_SCALE_IN", group.ID.String(), "SCALING_GROUP", map[string]interface{}{
		"instance_id": instanceID.String(),
		"trigger":     "reconciliation",
	})

	return nil
}

func toDynamicPorts(ports string) string {
	if ports == "" {
		return ""
	}
	// "80:80,443:443" -> "0:80,0:443"
	parts := strings.Split(ports, ",")
	var newPorts []string
	for _, p := range parts {
		components := strings.Split(p, ":")
		if len(components) == 2 {
			newPorts = append(newPorts, fmt.Sprintf("0:%s", components[1]))
		} else {
			newPorts = append(newPorts, p)
		}
	}
	return strings.Join(newPorts, ",")
}
