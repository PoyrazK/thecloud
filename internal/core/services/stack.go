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
	instanceSvc ports.InstanceService
	vpcSvc      ports.VpcService
	volumeSvc   ports.VolumeService
	snapshotSvc ports.SnapshotService
	logger      *slog.Logger
}

func NewStackService(
	repo ports.StackRepository,
	instanceSvc ports.InstanceService,
	vpcSvc ports.VpcService,
	volumeSvc ports.VolumeService,
	snapshotSvc ports.SnapshotService,
	logger *slog.Logger,
) *stackService {
	return &stackService{
		repo:        repo,
		instanceSvc: instanceSvc,
		vpcSvc:      vpcSvc,
		volumeSvc:   volumeSvc,
		snapshotSvc: snapshotSvc,
		logger:      logger,
	}
}

type Template struct {
	Resources map[string]ResourceDefinition `yaml:"Resources"`
}

type ResourceDefinition struct {
	Type       string                 `yaml:"Type"`
	Properties map[string]interface{} `yaml:"Properties"`
}

func (s *stackService) CreateStack(ctx context.Context, name, templateStr string, parameters map[string]string) (*domain.Stack, error) {
	userID := appcontext.UserIDFromContext(ctx)

	paramsJSON, _ := json.Marshal(parameters)
	stack := &domain.Stack{
		ID:         uuid.New(),
		UserID:     userID,
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
	// Process in background
	// Create a copy for the goroutine to avoid data race with the returned stack
	stackCopy := *stack
	go s.processStack(&stackCopy)

	return stack, nil
}

func (s *stackService) processStack(stack *domain.Stack) {
	ctx := context.Background()
	ctx = appcontext.WithUserID(ctx, stack.UserID)

	var t Template
	if err := yaml.Unmarshal([]byte(stack.Template), &t); err != nil {
		s.updateStackStatus(ctx, stack, domain.StackStatusCreateFailed, "Invalid template YAML")
		return
	}

	logicalToPhysical := make(map[string]uuid.UUID)

	// Simple non-topological sort for now: Create VPCs first, then everything else
	// A real implementation would build a dependency graph

	// Pass 1: VPCs
	for logicalID, res := range t.Resources {
		if res.Type == "VPC" {
			id, err := s.createVPC(ctx, stack.ID, logicalID, res.Properties)
			if err != nil {
				s.logger.Error("VPC creation failed, rolling back", "error", err)
				s.startRollback(ctx, stack, fmt.Sprintf("Failed to create VPC %s: %v", logicalID, err))
				return
			}
			logicalToPhysical[logicalID] = id
		}
	}

	// Pass 2: Volumes
	for logicalID, res := range t.Resources {
		if res.Type == "Volume" {
			id, err := s.createVolume(ctx, stack.ID, logicalID, res.Properties)
			if err != nil {
				s.logger.Error("Volume creation failed, rolling back", "error", err)
				s.startRollback(ctx, stack, fmt.Sprintf("Failed to create Volume %s: %v", logicalID, err))
				return
			}
			logicalToPhysical[logicalID] = id
		}
	}

	// Pass 3: Instances
	for logicalID, res := range t.Resources {
		if res.Type == "Instance" {
			id, err := s.createInstance(ctx, stack.ID, logicalID, s.resolveRefs(res.Properties, logicalToPhysical))
			if err != nil {
				s.logger.Error("Instance creation failed, rolling back", "error", err)
				s.startRollback(ctx, stack, fmt.Sprintf("Failed to create Instance %s: %v", logicalID, err))
				return
			}
			logicalToPhysical[logicalID] = id
		}
	}

	// Pass 4: Snapshots
	for logicalID, res := range t.Resources {
		if res.Type == "Snapshot" {
			_, err := s.createSnapshot(ctx, stack.ID, logicalID, s.resolveRefs(res.Properties, logicalToPhysical))
			if err != nil {
				s.logger.Error("Snapshot creation failed, rolling back", "error", err)
				s.startRollback(ctx, stack, fmt.Sprintf("Failed to create Snapshot %s: %v", logicalID, err))
				return
			}
		}
	}

	s.updateStackStatus(ctx, stack, domain.StackStatusCreateComplete, "")
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
		_ = s.vpcSvc.DeleteVPC(ctx, physicalID)
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

	vpc, err := s.vpcSvc.CreateVPC(ctx, name, cidr)
	if err != nil {
		return uuid.Nil, err
	}

	_ = s.repo.AddResource(ctx, &domain.StackResource{
		ID:           uuid.New(),
		StackID:      stackID,
		LogicalID:    logicalID,
		PhysicalID:   vpc.ID.String(),
		ResourceType: "VPC",
		Status:       "CREATE_COMPLETE",
		CreatedAt:    time.Now(),
	})

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

	_ = s.repo.AddResource(ctx, &domain.StackResource{
		ID:           uuid.New(),
		StackID:      stackID,
		LogicalID:    logicalID,
		PhysicalID:   vol.ID.String(),
		ResourceType: "Volume",
		Status:       "CREATE_COMPLETE",
		CreatedAt:    time.Now(),
	})

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

	_ = s.repo.AddResource(ctx, &domain.StackResource{
		ID:           uuid.New(),
		StackID:      stackID,
		LogicalID:    logicalID,
		PhysicalID:   snap.ID.String(),
		ResourceType: "Snapshot",
		Status:       "CREATE_IN_PROGRESS",
		CreatedAt:    time.Now(),
	})

	return snap.ID, nil
}

func (s *stackService) createInstance(ctx context.Context, stackID uuid.UUID, logicalID string, props map[string]interface{}) (uuid.UUID, error) {
	name, _ := props["Name"].(string)
	image, _ := props["Image"].(string)
	// cpu, mem are currently not used in LaunchInstance but we can keep them in template
	vpcID, _ := props["VpcID"].(uuid.UUID)

	if name == "" {
		name = fmt.Sprintf("%s-%s", logicalID, stackID.String()[:8])
	}

	var vpcIDPtr *uuid.UUID
	if vpcID != uuid.Nil {
		vpcIDPtr = &vpcID
	}

	inst, err := s.instanceSvc.LaunchInstance(ctx, name, image, "80", vpcIDPtr, nil, nil)
	if err != nil {
		return uuid.Nil, err
	}

	_ = s.repo.AddResource(ctx, &domain.StackResource{
		ID:           uuid.New(),
		StackID:      stackID,
		LogicalID:    logicalID,
		PhysicalID:   inst.ID.String(),
		ResourceType: "Instance",
		Status:       "CREATE_COMPLETE",
		CreatedAt:    time.Now(),
	})

	return inst.ID, nil
}

func (s *stackService) updateStackStatus(ctx context.Context, stack *domain.Stack, status domain.StackStatus, reason string) {
	stack.Status = status
	stack.StatusReason = reason
	stack.UpdatedAt = time.Now()
	_ = s.repo.Update(ctx, stack)
}

func (s *stackService) GetStack(ctx context.Context, id uuid.UUID) (*domain.Stack, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *stackService) ListStacks(ctx context.Context) ([]*domain.Stack, error) {
	userID := appcontext.UserIDFromContext(ctx)
	return s.repo.ListByUserID(ctx, userID)
}

func (s *stackService) DeleteStack(ctx context.Context, id uuid.UUID) error {
	// 1. Get stack
	stack, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 2. Perform background deletion
	go func() {
		bgCtx := context.Background()
		bgCtx = appcontext.WithUserID(bgCtx, stack.UserID)

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
