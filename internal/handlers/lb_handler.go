package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type LBHandler struct {
	svc ports.LBService
}

func NewLBHandler(svc ports.LBService) *LBHandler {
	return &LBHandler{svc: svc}
}

type CreateLBRequest struct {
	Name      string `json:"name" binding:"required"`
	VpcID     string `json:"vpc_id" binding:"required"`
	Port      int    `json:"port" binding:"required"`
	Algorithm string `json:"algorithm"`
}

type AddTargetRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	Port       int    `json:"port" binding:"required"`
	Weight     int    `json:"weight"`
}

// Create creates a load balancer
// @Summary Create a new load balancer
// @Description Creates a new load balancer in a VPC
// @Tags loadbalancers
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body CreateLBRequest true "LB creation request"
// @Param Idempotency-Key header string false "Idempotency key"
// @Success 202 {object} domain.LoadBalancer
// @Failure 400 {object} httputil.Response
// @Router /lb [post]
func (h *LBHandler) Create(c *gin.Context) {
	var req CreateLBRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	vpcID, err := uuid.Parse(req.VpcID)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid vpc_id format"))
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")

	lb, err := h.svc.Create(c.Request.Context(), req.Name, vpcID, req.Port, req.Algorithm, idempotencyKey)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusAccepted, lb)
}

// List returns all load balancers
// @Summary List all load balancers
// @Description Gets a list of all load balancers
// @Tags loadbalancers
// @Produce json
// @Security APIKeyAuth
// @Success 200 {array} domain.LoadBalancer
// @Router /lb [get]
func (h *LBHandler) List(c *gin.Context) {
	lbs, err := h.svc.List(c.Request.Context())
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, lbs)
}

// Get returns load balancer details
// @Summary Get load balancer details
// @Description Gets detailed information about a specific load balancer
// @Tags loadbalancers
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "LB ID"
// @Success 200 {object} domain.LoadBalancer
// @Failure 404 {object} httputil.Response
// @Router /lb/{id} [get]
func (h *LBHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid id format"))
		return
	}

	lb, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, lb)
}

// Delete deletes a load balancer
// @Summary Delete a load balancer
// @Description Removes a load balancer and stops associated proxy
// @Tags loadbalancers
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "LB ID"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /lb/{id} [delete]
func (h *LBHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid id format"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, gin.H{"message": "load balancer deletion initiated"})
}

// AddTarget adds a target to a load balancer
// @Summary Add a target to a load balancer
// @Description Registers a compute instance to receive traffic from the load balancer
// @Tags loadbalancers
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "LB ID"
// @Param request body AddTargetRequest true "Target details"
// @Success 201 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Router /lb/{id}/targets [post]
func (h *LBHandler) AddTarget(c *gin.Context) {
	lbIDStr := c.Param("id")
	lbID, err := uuid.Parse(lbIDStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid lb_id format"))
		return
	}

	var req AddTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid request body"))
		return
	}

	instID, err := uuid.Parse(req.InstanceID)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid instance_id format"))
		return
	}

	if err := h.svc.AddTarget(c.Request.Context(), lbID, instID, req.Port, req.Weight); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, gin.H{"message": "target added"})
}

// RemoveTarget removes a target from a load balancer
// @Summary Remove a target from a load balancer
// @Description Deregisters a compute instance from the load balancer
// @Tags loadbalancers
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "LB ID"
// @Param instanceId path string true "Instance ID"
// @Success 200 {object} httputil.Response
// @Failure 404 {object} httputil.Response
// @Router /lb/{id}/targets/{instanceId} [delete]
func (h *LBHandler) RemoveTarget(c *gin.Context) {
	lbIDStr := c.Param("id")
	lbID, err := uuid.Parse(lbIDStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid lb_id format"))
		return
	}

	instIDStr := c.Param("instanceId")
	instID, err := uuid.Parse(instIDStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid instance_id format"))
		return
	}

	if err := h.svc.RemoveTarget(c.Request.Context(), lbID, instID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "target removed"})
}

// ListTargets returns all targets for a load balancer
// @Summary List all targets for a load balancer
// @Description Gets a list of targets for a load balancer
// @Tags loadbalancers
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "LB ID"
// @Success 200 {array} domain.LBTarget
// @Router /lb/{id}/targets [get]
func (h *LBHandler) ListTargets(c *gin.Context) {
	lbIDStr := c.Param("id")
	lbID, err := uuid.Parse(lbIDStr)
	if err != nil {
		httputil.Error(c, errors.New(errors.InvalidInput, "invalid lb_id format"))
		return
	}

	targets, err := h.svc.ListTargets(c.Request.Context(), lbID)
	if err != nil {
		httputil.Error(c, err)
		return
	}
	httputil.Success(c, http.StatusOK, targets)
}
