// Package services implements core business workflows.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"gopkg.in/yaml.v3"
)

type stackService struct {
	repo        ports.StackRepository
	rbacSvc     ports.RBACService
	instanceSvc ports.InstanceService
	vpcSvc      ports.VpcService
	volumeSvc   ports.VolumeService
	snapshotSvc ports.SnapshotService
	logger      *slog.Logger
}

// NewStackService constructs a StackService with its dependencies.
func NewStackService(
	repo ports.StackRepository,
	rbacSvc ports.RBACService,
	instanceSvc ports.InstanceService,
	vpcSvc ports.VpcService,
	volumeSvc ports.VolumeService,
	snapshotSvc ports.SnapshotService,
	logger *slog.Logger,
) *stackService {
	return &stackService{
		repo:        repo,
		rbacSvc:     rbacSvc,
		instanceSvc: instanceSvc,
		vpcSvc:      vpcSvc,
		volumeSvc:   volumeSvc,
		snapshotSvc: snapshotSvc,
		logger:      logger,
	}
}

// Template represents a stack template with logical resources.
type Template struct {
	Resources map[string]ResourceDefinition `yaml:"Resources"`
}

// ResourceDefinition describes a single stack resource.
type ResourceDefinition struct {
	Type       string                 `yaml:"Type"`
	Properties map[string]interface{} `yaml:"Properties"`
}

func (s *stackService) CreateStack(ctx context.Context, name, templateStr string, parameters map[string]string) (*domain.Stack, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStackCreate, "*"); err != nil {
		return nil, err
	}

	paramsJSON, _ := json.Marshal(parameters)
	stack := &domain.Stack{
		ID:         uuid.New(),
		UserID:     userID,
		TenantID:   tenantID,
		Name:       name,
		Template:   templateStr,
		Parameters: paramsJSON,
		Status:     domain.StackStatusCreateInProgress,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, stack); err != nil {
		return nil, err
	}

	// Process in background
	// Create a copy for the goroutine to avoid data race with the returned stack
	stackCopy := *stack
	go s.processStack(&stackCopy)

	return stack, nil
}

func (s *stackService) processStack(stack *domain.Stack) {
	ctx := context.Background()
	ctx = appcontext.WithUserID(ctx, stack.UserID)
	ctx = appcontext.WithTenantID(ctx, stack.TenantID)

	var t Template
	if err := yaml.Unmarshal([]byte(stack.Template), &t); err != nil {
		s.updateStackStatus(ctx, stack, domain.StackStatusCreateFailed, "Invalid template YAML")
		return
	}

	logicalToPhysical := make(map[string]uuid.UUID)

	// Simple non-topological sort for now: Create VPCs first, then everything else
	// a real implementation would build a dependency graph

	// Pass 1: VPCs
	if err := s.createResourcePass(ctx, stack, t, "VPC", logicalToPhysical); err != nil {
		return
	}

	// Pass 2: Volumes
	if err := s.createResourcePass(ctx, stack, t, "Volume", logicalToPhysical); err != nil {
		return
	}

	// Pass 3: Instances
	if err := s.createResourcePass(ctx, stack, t, "Instance", logicalToPhysical); err != nil {
		return
	}

	// Pass 4: Snapshots
	if err := s.createResourcePass(ctx, stack, t, "Snapshot", logicalToPhysical); err != nil {
		return
	}

	s.updateStackStatus(ctx, stack, domain.StackStatusCreateComplete, "")
}

func (s *stackService) createResourcePass(ctx context.Context, stack *domain.Stack, t Template, resourceType string, logicalToPhysical map[string]uuid.UUID) error {
	for logicalID, res := range t.Resources {
		if res.Type != resourceType {
			continue
		}

		id, err := s.createSingleResource(ctx, stack.ID, logicalID, res, resourceType, logicalToPhysical)
		if err != nil {
			s.logger.Error(fmt.Sprintf("%s creation failed, rolling back", resourceType), "error", err)
			s.startRollback(ctx, stack, fmt.Sprintf("Failed to create %s %s: %v", resourceType, logicalID, err))
			return err
		}
		logicalToPhysical[logicalID] = id
	}
	return nil
}

func (s *stackService) createSingleResource(ctx context.Context, stackID uuid.UUID, logicalID string, res ResourceDefinition, resourceType string, refs map[string]uuid.UUID) (uuid.UUID, error) {
	props := res.Properties
	if resourceType == "Instance" || resourceType == "Snapshot" {
		props = s.resolveRefs(res.Properties, refs)
	}

	switch resourceType {
	case "VPC":
		return s.createVPC(ctx, stackID, logicalID, props)
	case "Volume":
		return s.createVolume(ctx, stackID, logicalID, props)
	case "Instance":
		return s.createInstance(ctx, stackID, logicalID, props)
	case "Snapshot":
		return s.createSnapshot(ctx, stackID, logicalID, props)
	default:
		return uuid.Nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

func (s *stackService) startRollback(ctx context.Context, stack *domain.Stack, reason string) {
	s.updateStackStatus(ctx, stack, domain.StackStatusRollbackInProgress, reason)

	if err := s.rollbackStack(ctx, stack.ID); err != nil {
		s.logger.Error("Rollback failed", "stack_id", stack.ID, "error", err)
		s.updateStackStatus(ctx, stack, domain.StackStatusRollbackFailed, fmt.Sprintf("Rollback failed: %v", err))
		return
	}

	s.updateStackStatus(ctx, stack, domain.StackStatusRollbackComplete, reason)
}

func (s *stackService) rollbackStack(ctx context.Context, stackID uuid.UUID) error {
	resources, err := s.repo.ListResources(ctx, stackID)
	if err != nil {
		return err
	}

	// Delete resources in reverse creation order
	for i := len(resources) - 1; i >= 0; i-- {
		res := resources[i]
		s.deletePhysicalResource(ctx, res.ResourceType, res.PhysicalID)
	}

	return s.repo.DeleteResources(ctx, stackID)
}

func (s *stackService) deletePhysicalResource(ctx context.Context, resourceType, physicalID string) {
	switch resourceType {
	case "Instance":
		_ = s.instanceSvc.TerminateInstance(ctx, physicalID)
	case "VPC":
		_ = s.vpcSvc.DeleteVPC(ctx, physicalID, true) // force delete for stack cleanup
	case "Volume":
		_ = s.volumeSvc.DeleteVolume(ctx, physicalID)
	case "Snapshot":
		if physID, err := uuid.Parse(physicalID); err == nil {
			_ = s.snapshotSvc.DeleteSnapshot(ctx, physID)
		}
	}
}

func (s *stackService) resolveRefs(props map[string]interface{}, refs map[string]uuid.UUID) map[string]interface{} {
	newProps := make(map[string]interface{})
	for k, v := range props {
		if m, ok := v.(map[string]interface{}); ok {
			if ref, exists := m["Ref"]; exists {
				if refID, ok := ref.(string); ok {
					if physicalID, found := refs[refID]; found {
						newProps[k] = physicalID
						continue
					}
				}
			}
		}
		newProps[k] = v
	}
	return newProps
}

func (s *stackService) createVPC(ctx context.Context, stackID uuid.UUID, logicalID string, props map[string]interface{}) (uuid.UUID, error) {
	name, _ := props["Name"].(string)
	if name == "" {
		name = fmt.Sprintf("%s-%s", logicalID, stackID.String()[:8])
	}
	cidr, _ := props["CIDRBlock"].(string)

	vpc, err := s.vpcSvc.CreateVPC(ctx, name, cidr, "")
	if err != nil {
		return uuid.Nil, err
	}

	s.recordResource(ctx, stackID, logicalID, vpc.ID.String(), "VPC", "CREATE_COMPLETE")
	return vpc.ID, nil
}

func (s *stackService) createVolume(ctx context.Context, stackID uuid.UUID, logicalID string, props map[string]interface{}) (uuid.UUID, error) {
	name, _ := props["Name"].(string)
	size, _ := props["Size"].(int)
	if size == 0 {
		size = 10
	}

	if name == "" {
		name = fmt.Sprintf("%s-%s", logicalID, stackID.String()[:8])
	}

	vol, err := s.volumeSvc.CreateVolume(ctx, name, size)
	if err != nil {
		return uuid.Nil, err
	}

	s.recordResource(ctx, stackID, logicalID, vol.ID.String(), "Volume", "CREATE_COMPLETE")
	return vol.ID, nil
}

func (s *stackService) createSnapshot(ctx context.Context, stackID uuid.UUID, logicalID string, props map[string]interface{}) (uuid.UUID, error) {
	name, _ := props["Name"].(string)
	volumeID, ok := props["VolumeID"].(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("VolumeID is required for Snapshot")
	}

	if name == "" {
		name = fmt.Sprintf("%s-%s", logicalID, stackID.String()[:8])
	}

	snap, err := s.snapshotSvc.CreateSnapshot(ctx, volumeID, name)
	if err != nil {
		return uuid.Nil, err
	}

	s.recordResource(ctx, stackID, logicalID, snap.ID.String(), "Snapshot", "CREATE_IN_PROGRESS")
	return snap.ID, nil
}

func (s *stackService) createInstance(ctx context.Context, stackID uuid.UUID, logicalID string, props map[string]interface{}) (uuid.UUID, error) {
	name, _ := props["Name"].(string)
	image, _ := props["Image"].(string)
	vpcID, _ := props["VpcID"].(uuid.UUID)

	if name == "" {
		name = fmt.Sprintf("%s-%s", logicalID, stackID.String()[:8])
	}

	var vpcIDPtr *uuid.UUID
	if vpcID != uuid.Nil {
		vpcIDPtr = &vpcID
	}

	inst, err := s.instanceSvc.LaunchInstance(ctx, ports.LaunchParams{
		Name:  name,
		Image: image,
		Ports: "80",
		VpcID: vpcIDPtr,
	})
	if err != nil {
		return uuid.Nil, err
	}

	s.recordResource(ctx, stackID, logicalID, inst.ID.String(), "Instance", "CREATE_COMPLETE")
	return inst.ID, nil
}

func (s *stackService) recordResource(ctx context.Context, stackID uuid.UUID, logicalID, physicalID, resType, status string) {
	_ = s.repo.AddResource(ctx, &domain.StackResource{
		ID:           uuid.New(),
		StackID:      stackID,
		LogicalID:    logicalID,
		PhysicalID:   physicalID,
		ResourceType: resType,
		Status:       status,
		CreatedAt:    time.Now(),
	})
}

func (s *stackService) updateStackStatus(ctx context.Context, stack *domain.Stack, status domain.StackStatus, reason string) {
	stack.Status = status
	stack.StatusReason = reason
	stack.UpdatedAt = time.Now()
	_ = s.repo.Update(ctx, stack)
}

func (s *stackService) GetStack(ctx context.Context, id uuid.UUID) (*domain.Stack, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStackRead, id.String()); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}

func (s *stackService) ListStacks(ctx context.Context) ([]*domain.Stack, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStackRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.ListByUserID(ctx, userID)
}

func (s *stackService) DeleteStack(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStackDelete, id.String()); err != nil {
		return err
	}

	// 1. Get stack
	stack, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 2. Perform background deletion
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		bgCtx = appcontext.WithUserID(bgCtx, stack.UserID)
		bgCtx = appcontext.WithTenantID(bgCtx, stack.TenantID)

		resources, _ := s.repo.ListResources(bgCtx, id)

		// Delete resources in reverse order (naive)
		for i := len(resources) - 1; i >= 0; i-- {
			s.deletePhysicalResource(bgCtx, resources[i].ResourceType, resources[i].PhysicalID)
		}

		_ = s.repo.Delete(bgCtx, id)
	}()

	return nil
}

func (s *stackService) ValidateTemplate(ctx context.Context, template string) (*domain.TemplateValidateResponse, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionStackRead, "*"); err != nil {
		return nil, err
	}

	var t Template
	if err := yaml.Unmarshal([]byte(template), &t); err != nil {
		return &domain.TemplateValidateResponse{
			Valid:  false,
			Errors: []string{fmt.Sprintf("YAML parse error: %v", err)},
		}, nil
	}

	if len(t.Resources) == 0 {
		return &domain.TemplateValidateResponse{
			Valid:  false,
			Errors: []string{"Template must contain at least one resource"},
		}, nil
	}

	return &domain.TemplateValidateResponse{Valid: true}, nil
}
