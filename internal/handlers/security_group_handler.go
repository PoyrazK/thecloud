// Package httphandlers provides HTTP handlers for the API.
package httphandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/pkg/httputil"
)

// SecurityGroupHandler handles security group HTTP endpoints.
type SecurityGroupHandler struct {
	svc ports.SecurityGroupService
}

// NewSecurityGroupHandler constructs a SecurityGroupHandler.
func NewSecurityGroupHandler(svc ports.SecurityGroupService) *SecurityGroupHandler {
	return &SecurityGroupHandler{svc: svc}
}

// CreateSecurityGroupRequest is the payload for creating a security group.
type CreateSecurityGroupRequest struct {
	VPCID       uuid.UUID `json:"vpc_id" binding:"required"`
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
}

// AttachDetachSGRequest is the payload for attaching or detaching a security group.
type AttachDetachSGRequest struct {
	InstanceID uuid.UUID `json:"instance_id" binding:"required"`
	GroupID    uuid.UUID `json:"group_id" binding:"required"`
}

// Create creates a new security group
// @Summary Create security group
// @Description Creates a new security group in a VPC
// @Tags security-groups
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body CreateSecurityGroupRequest true "Create request"
// @Success 201 {object} domain.SecurityGroup
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /security-groups [post]
func (h *SecurityGroupHandler) Create(c *gin.Context) {
	var req CreateSecurityGroupRequest

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

// List lists security groups
// @Summary List security groups
// @Description Lists all security groups for a VPC
// @Tags security-groups
// @Produce json
// @Security APIKeyAuth
// @Param vpc_id query string true "VPC ID"
// @Success 200 {array} domain.SecurityGroup
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /security-groups [get]
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

// Get gets a security group
// @Summary Get security group
// @Description Gets details of a specific security group
// @Tags security-groups
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Security Group ID"
// @Param vpc_id query string false "VPC ID (optional if ID is UUID)"
// @Success 200 {object} domain.SecurityGroup
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /security-groups/{id} [get]
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

// Delete deletes a security group
// @Summary Delete security group
// @Description Deletes a security group
// @Tags security-groups
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Security Group ID"
// @Success 204
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /security-groups/{id} [delete]
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

	c.Status(http.StatusNoContent)
}

// AddRule adds a security group rule
// @Summary Add security rule
// @Description Adds a new rule to a security group
// @Tags security-groups
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param id path string true "Security Group ID"
// @Param request body domain.SecurityRule true "Rule details"
// @Success 201 {object} domain.SecurityRule
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /security-groups/{id}/rules [post]
func (h *SecurityGroupHandler) AddRule(c *gin.Context) {
	idOrName := c.Param("id")

	if idOrName != "" {
		if _, err := uuid.Parse(idOrName); err != nil && isUUIDLike(idOrName) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group_id"})
			return
		}
	}

	var req domain.SecurityRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule, err := h.svc.AddRule(c.Request.Context(), idOrName, req)
	if err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusCreated, rule)
}

func isUUIDLike(s string) bool {
	if len(s) < 8 || len(s) > 36 {
		return false
	}
	for _, c := range s {
		if c != '-' && (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// Attach attaches a security group to an instance
// @Summary Attach security group
// @Description Associates a security group with a compute instance
// @Tags security-groups
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body AttachDetachSGRequest true "Attach details"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /security-groups/attach [post]
func (h *SecurityGroupHandler) Attach(c *gin.Context) {
	var req AttachDetachSGRequest

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

// RemoveRule removes a security group rule
// @Summary Remove security rule
// @Description Deletes a specific security rule by ID
// @Tags security-groups
// @Produce json
// @Security APIKeyAuth
// @Param rule_id path string true "Rule ID"
// @Success 204
// @Failure 404 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /security-groups/rules/{rule_id} [delete]
func (h *SecurityGroupHandler) RemoveRule(c *gin.Context) {
	ruleIDStr := c.Param("rule_id")
	ruleID, err := uuid.Parse(ruleIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule_id"})
		return
	}

	if err := h.svc.RemoveRule(c.Request.Context(), ruleID); err != nil {
		httputil.Error(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Detach detaches a security group from an instance
// @Summary Detach security group
// AttachDetachSGRequest is the payload for attaching or detaching a security group.
// @Description Removes a security group association from a compute instance
// @Tags security-groups
// @Accept json
// @Produce json
// @Security APIKeyAuth
// @Param request body AttachDetachSGRequest true "Detach details"
// @Success 200 {object} httputil.Response
// @Failure 400 {object} httputil.Response
// @Failure 500 {object} httputil.Response
// @Router /security-groups/detach [post]
func (h *SecurityGroupHandler) Detach(c *gin.Context) {
	var req AttachDetachSGRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.DetachFromInstance(c.Request.Context(), req.InstanceID, req.GroupID); err != nil {
		httputil.Error(c, err)
		return
	}

	httputil.Success(c, http.StatusOK, gin.H{"message": "security group detached"})
}
