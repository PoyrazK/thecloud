package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type SubnetHandler struct {
	svc ports.SubnetService
}

// NewSubnetHandler creates a SubnetHandler that handles HTTP endpoints for subnets using the provided SubnetService.
// The svc parameter is used to perform subnet-related business logic invoked by the handler methods.
func NewSubnetHandler(svc ports.SubnetService) *SubnetHandler {
	return &SubnetHandler{svc: svc}
}

func (h *SubnetHandler) Create(c *gin.Context) {
	vpcIDStr := c.Param("vpc_id")
	vpcID, err := uuid.Parse(vpcIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid vpc_id"})
		return
	}

	var req struct {
		Name             string `json:"name" binding:"required"`
		CIDRBlock        string `json:"cidr_block" binding:"required"`
		AvailabilityZone string `json:"availability_zone"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subnet, err := h.svc.CreateSubnet(c.Request.Context(), vpcID, req.Name, req.CIDRBlock, req.AvailabilityZone)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, subnet)
}

func (h *SubnetHandler) List(c *gin.Context) {
	vpcIDStr := c.Param("vpc_id")
	vpcID, err := uuid.Parse(vpcIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid vpc_id"})
		return
	}

	subnets, err := h.svc.ListSubnets(c.Request.Context(), vpcID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, subnets)
}

func (h *SubnetHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	// For simple Get, we use id only for now. Or we could pass vpcID if we had it in path.
	// The service GetSubnet requires vpcID if searching by name, but here we use ID.
	subnet, err := h.svc.GetSubnet(c.Request.Context(), idStr, uuid.Nil)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, subnet)
}

func (h *SubnetHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.svc.DeleteSubnet(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "subnet deleted"})
}