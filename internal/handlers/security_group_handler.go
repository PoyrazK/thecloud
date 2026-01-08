package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

type SecurityGroupHandler struct {
	svc ports.SecurityGroupService
}

func NewSecurityGroupHandler(svc ports.SecurityGroupService) *SecurityGroupHandler {
	return &SecurityGroupHandler{svc: svc}
}

func (h *SecurityGroupHandler) Create(c *gin.Context) {
	var req struct {
		VPCID       uuid.UUID `json:"vpc_id" binding:"required"`
		Name        string    `json:"name" binding:"required"`
		Description string    `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sg, err := h.svc.CreateGroup(c.Request.Context(), req.VPCID, req.Name, req.Description)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, sg)
}

func (h *SecurityGroupHandler) List(c *gin.Context) {
	vpcIDStr := c.Query("vpc_id")
	if vpcIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vpc_id is required"})
		return
	}

	vpcID, err := uuid.Parse(vpcIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid vpc_id"})
		return
	}

	groups, err := h.svc.ListGroups(c.Request.Context(), vpcID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, groups)
}

func (h *SecurityGroupHandler) Get(c *gin.Context) {
	id := c.Param("id")
	vpcIDStr := c.Query("vpc_id")

	var vpcID uuid.UUID
	if vpcIDStr != "" {
		vpcID, _ = uuid.Parse(vpcIDStr)
	}

	sg, err := h.svc.GetGroup(c.Request.Context(), id, vpcID)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, sg)
}

func (h *SecurityGroupHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.svc.DeleteGroup(c.Request.Context(), id); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "security group deleted"})
}

func (h *SecurityGroupHandler) AddRule(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group_id"})
		return
	}

	var req domain.SecurityRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule, err := h.svc.AddRule(c.Request.Context(), groupID, req)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, rule)
}

func (h *SecurityGroupHandler) Attach(c *gin.Context) {
	var req struct {
		InstanceID uuid.UUID `json:"instance_id" binding:"required"`
		GroupID    uuid.UUID `json:"group_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.AttachToInstance(c.Request.Context(), req.InstanceID, req.GroupID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "security group attached"})
}
