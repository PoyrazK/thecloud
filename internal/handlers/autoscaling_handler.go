package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type AutoScalingHandler struct {
	svc ports.AutoScalingService
}

func NewAutoScalingHandler(svc ports.AutoScalingService) *AutoScalingHandler {
	return &AutoScalingHandler{svc: svc}
}

type CreateGroupRequest struct {
	Name           string     `json:"name" binding:"required"`
	VpcID          uuid.UUID  `json:"vpc_id" binding:"required"`
	LoadBalancerID *uuid.UUID `json:"load_balancer_id"`
	Image          string     `json:"image" binding:"required"`
	Ports          string     `json:"ports"`
	MinInstances   int        `json:"min_instances"` // 0 is valid
	MaxInstances   int        `json:"max_instances" binding:"required"`
	DesiredCount   int        `json:"desired_count" binding:"required"`
}

// CreateGroup creates a new scaling group
// @Summary Create a new scaling group
// @Description Creates an auto-scaling group for instances
// @Tags autoscaling
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body CreateGroupRequest true "ASG creation request"
// @Param Idempotency-Key header string false "Idempotency key"
// @Success 201 {object} domain.ScalingGroup
// @Failure 400 {object} httputil.Response
// @Router /autoscaling/groups [post]
func (h *AutoScalingHandler) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	key := c.GetHeader("Idempotency-Key")

	group, err := h.svc.CreateGroup(c.Request.Context(), req.Name, req.VpcID, req.Image, req.Ports, req.MinInstances, req.MaxInstances, req.DesiredCount, req.LoadBalancerID, key)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, group)
}

// ListGroups returns all scaling groups
// @Summary List all scaling groups
// @Description Gets a list of all auto-scaling groups
// @Tags autoscaling
// @Produce json
// @Security APIKeyAuth
// @Success 200 {array} domain.ScalingGroup
// @Router /autoscaling/groups [get]
func (h *AutoScalingHandler) ListGroups(c *gin.Context) {
	groups, err := h.svc.ListGroups(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, groups)
}

// GetGroup returns scaling group details
// @Summary Get scaling group details
// @Description Gets detailed information about a specific auto-scaling group
// @Tags autoscaling
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "ASG ID"
// @Success 200 {object} domain.ScalingGroup
// @Failure 404 {object} httputil.Response
// @Router /autoscaling/groups/{id} [get]
func (h *AutoScalingHandler) GetGroup(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid group id"))
		return
	}

	group, err := h.svc.GetGroup(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, group)
}

// DeleteGroup deletes a scaling group
// @Summary Delete a scaling group
// @Description Removes an auto-scaling group
// @Tags autoscaling
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "ASG ID"
// @Success 204
// @Failure 404 {object} httputil.Response
// @Router /autoscaling/groups/{id} [delete]
func (h *AutoScalingHandler) DeleteGroup(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid group id"))
		return
	}

	if err := h.svc.DeleteGroup(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusNoContent, nil)
}

type CreateASPolicyRequest struct {
	Name        string  `json:"name" binding:"required"`
	MetricType  string  `json:"metric_type" binding:"required"`
	TargetValue float64 `json:"target_value" binding:"required"`
	ScaleOut    int     `json:"scale_out_step" binding:"required"`
	ScaleIn     int     `json:"scale_in_step" binding:"required"`
	CooldownSec int     `json:"cooldown_sec" binding:"required"`
}

// CreatePolicy creates a new scaling policy
// @Summary Create a new scaling policy
// @Description Adds a scaling policy to an auto-scaling group
// @Tags autoscaling
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "ASG ID"
// @Param request body CreateASPolicyRequest true "Policy creation request"
// @Success 201 {object} domain.ScalingPolicy
// @Failure 400 {object} httputil.Response
// @Router /autoscaling/groups/{id}/policies [post]
func (h *AutoScalingHandler) CreatePolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid group id"))
		return
	}

	var req CreateASPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
		return
	}

	policy, err := h.svc.CreatePolicy(c.Request.Context(), id, req.Name, req.MetricType, req.TargetValue, req.ScaleOut, req.ScaleIn, req.CooldownSec)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, policy)
}

// DeletePolicy deletes a scaling policy
// @Summary Delete a scaling policy
// @Description Removes a scaling policy
// @Tags autoscaling
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Policy ID"
// @Success 204
// @Failure 404 {object} httputil.Response
// @Router /autoscaling/policies/{id} [delete]
func (h *AutoScalingHandler) DeletePolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid policy id"))
		return
	}

	if err := h.svc.DeletePolicy(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusNoContent, nil)
}
