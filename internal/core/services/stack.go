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

	paramsJSON, err := json.Marshal(parameters)
	if err != nil {
		s.logger.Error("failed to marshal stack parameters to JSON", "stackName", name, "error", err)
		return nil, fmt.Errorf("create stack: marshal parameters: %w", err)
	}
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
		if err == domain.ErrStackNameAlreadyExists {
			return nil, fmt.Errorf("stack with name '%s' already exists", name)
		}
		return nil, err
	}

	// Process in background with a detached context to prevent request cancellation
	// from interrupting resource provisioning, but with a reasonable timeout
	stackCopy := *stack
	go s.processStack(context.Background(), &stackCopy)

	return stack, nil
}

func (s *stackService) processStack(ctx context.Context, stack *domain.Stack) {
	// Attach user ID to the context for authorization checks
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
		if err := s.deletePhysicalResource(ctx, res.ResourceType, res.PhysicalID); err != nil {
			s.logger.Error("failed to delete resource during rollback", "resourceType", res.ResourceType, "physicalID", res.PhysicalID, "error", err)
		}
	}

	return s.repo.DeleteResources(ctx, stackID)
}

func (s *stackService) deletePhysicalResource(ctx context.Context, resourceType, physicalID string) error {
	switch resourceType {
	case "Instance":
		return s.instanceSvc.TerminateInstance(ctx, physicalID)
	case "VPC":
		return s.vpcSvc.DeleteVPC(ctx, physicalID)
	case "Volume":
		return s.volumeSvc.DeleteVolume(ctx, physicalID)
	case "Snapshot":
		if physID, err := uuid.Parse(physicalID); err == nil {
			return s.snapshotSvc.DeleteSnapshot(ctx, physID)
		}
		return fmt.Errorf("invalid snapshot physical ID: %s", physicalID)
	}
	return nil
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

	vpc, err := s.vpcSvc.CreateVPC(ctx, name)
	if err != nil {
		return uuid.Nil, err
	}

	if err := s.repo.AddResource(ctx, &domain.StackResource{
		ID:           uuid.New(),
		StackID:      stackID,
		LogicalID:    logicalID,
		PhysicalID:   vpc.ID.String(),
		ResourceType: "VPC",
		Status:       "CREATE_COMPLETE",
		CreatedAt:    time.Now(),
	}); err != nil {
		s.logger.Error("failed to add VPC resource to stack", "stackID", stackID, "logicalID", logicalID, "error", err)
		return uuid.Nil, fmt.Errorf("add VPC resource: %w", err)
	}

	return vpc.ID, nil
}

func (s *stackService) createVolume(ctx context.Context, stackID uuid.UUID, logicalID string, props map[string]interface{}) (uuid.UUID, error) {
	name, _ := props["Name"].(string)
	size := 0
	if rawSize, ok := props["Size"]; ok {
		switch v := rawSize.(type) {
		case int:
			size = v
		case int64:
			size = int(v)
		case float64:
			size = int(v)
		case uint:
			size = int(v)
		case uint64:
			size = int(v)
		}
	}
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

	if err := s.repo.AddResource(ctx, &domain.StackResource{
		ID:           uuid.New(),
		StackID:      stackID,
		LogicalID:    logicalID,
		PhysicalID:   vol.ID.String(),
		ResourceType: "Volume",
		Status:       "CREATE_COMPLETE",
		CreatedAt:    time.Now(),
	}); err != nil {
		s.logger.Error("failed to add Volume resource to stack", "stackID", stackID, "logicalID", logicalID, "error", err)
		return uuid.Nil, fmt.Errorf("add Volume resource: %w", err)
	}

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

	if err := s.repo.AddResource(ctx, &domain.StackResource{
		ID:           uuid.New(),
		StackID:      stackID,
		LogicalID:    logicalID,
		PhysicalID:   snap.ID.String(),
		ResourceType: "Snapshot",
		Status:       "CREATE_IN_PROGRESS",
		CreatedAt:    time.Now(),
	}); err != nil {
		s.logger.Error("failed to add Snapshot resource to stack", "stackID", stackID, "logicalID", logicalID, "error", err)
		return uuid.Nil, fmt.Errorf("add Snapshot resource: %w", err)
	}

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

	port := "80"
	if p, ok := props["Port"].(string); ok && p != "" {
		port = p
	}

	inst, err := s.instanceSvc.LaunchInstance(ctx, name, image, port, vpcIDPtr, nil)
	if err != nil {
		return uuid.Nil, err
	}

	if err := s.repo.AddResource(ctx, &domain.StackResource{
		ID:           uuid.New(),
		StackID:      stackID,
		LogicalID:    logicalID,
		PhysicalID:   inst.ID.String(),
		ResourceType: "Instance",
		Status:       "CREATE_COMPLETE",
		CreatedAt:    time.Now(),
	}); err != nil {
		s.logger.Error("failed to add Instance resource to stack", "stackID", stackID, "logicalID", logicalID, "error", err)
		return uuid.Nil, fmt.Errorf("add Instance resource: %w", err)
	}

	return inst.ID, nil
}

func (s *stackService) updateStackStatus(ctx context.Context, stack *domain.Stack, status domain.StackStatus, reason string) {
	stack.Status = status
	stack.StatusReason = reason
	stack.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, stack); err != nil {
		slog.ErrorContext(ctx, "failed to update stack status", "stack_id", stack.ID, "status", status, "reason", reason, "error", err)
	}
}

func (s *stackService) GetStack(ctx context.Context, id uuid.UUID) (*domain.Stack, error) {
	userID := appcontext.UserIDFromContext(ctx)

	stack, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if stack.UserID != userID {
		// Do not reveal existence of stacks owned by other users
		return nil, fmt.Errorf("stack not found")
	}

	return stack, nil
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

	// 2. Authorization: ensure the requesting user owns the stack
	userID := appcontext.UserIDFromContext(ctx)
	if userID != stack.UserID {
		return fmt.Errorf("user not authorized to delete this stack")
	}

	// 3. Perform background deletion with detached context
	// Use background context to ensure cleanup completes even if request is cancelled
	go func() {
		deleteCtx := context.Background()
		deleteCtx = appcontext.WithUserID(deleteCtx, stack.UserID)

		resources, err := s.repo.ListResources(deleteCtx, id)
		if err != nil {
			s.logger.Error("failed to list stack resources for deletion",
				"stackID", id,
				"error", err,
			)
			return
		}

		// Delete resources in reverse order (naive)
		for i := len(resources) - 1; i >= 0; i-- {
			resource := resources[i]
			if err := s.deletePhysicalResource(deleteCtx, resource.ResourceType, resource.PhysicalID); err != nil {
				s.logger.Error("failed to delete physical resource",
					"stackID", id,
					"resourceType", resource.ResourceType,
					"physicalID", resource.PhysicalID,
					"error", err,
				)
			}
		}

		if err := s.repo.Delete(deleteCtx, id); err != nil {
			s.logger.Error("failed to delete stack record",
				"stackID", id,
				"error", err,
			)
		}
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
