package httputil

import (
	"strings"

	"github.com/gin-gonic/gin"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

const (
	ActionCreate  = "create"
	ActionRead    = "read"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionExecute = "execute"
)

type Permission struct {
	Resource string
	Action   string
}

func (p Permission) String() string {
	return p.Resource + ":" + p.Action
}

func RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := roleFromContext(c)
		if !HasPermission(role, Permission{Resource: resource, Action: action}) {
			Error(c, errors.New(errors.Forbidden, "insufficient permissions"))
			c.Abort()
			return
		}
		c.Next()
	}
}

func HasPermission(role string, perm Permission) bool {
	role = strings.ToLower(role)
	role = domain.NormalizeRole(role)
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	if perms["*:*"] {
		return true
	}
	return perms[perm.String()]
}

func roleFromContext(c *gin.Context) string {
	if roleVal, ok := c.Get("userRole"); ok {
		if role, ok := roleVal.(string); ok && role != "" {
			return role
		}
	}
	return appcontext.UserRoleFromContext(c.Request.Context())
}

var rolePermissions = map[string]map[string]bool{
	domain.RoleOwner:     allPermissions(),
	domain.RoleAdmin:     adminPermissions(),
	domain.RoleDeveloper: developerPermissions(),
	domain.RoleViewer:    viewerPermissions(),
}

func allPermissions() map[string]bool {
	return map[string]bool{
		"*:*": true,
	}
}

func adminPermissions() map[string]bool {
	perms := developerPermissions()
	perms["auth:"+ActionRead] = true
	return perms
}

func developerPermissions() map[string]bool {
	perms := map[string]bool{}
	for _, resource := range []string{
		"instances",
		"vpcs",
		"storage",
		"events",
		"volumes",
		"dashboard",
		"loadbalancers",
		"databases",
		"secrets",
		"functions",
		"caches",
		"autoscaling",
	} {
		grantAllActions(perms, resource)
	}
	perms["auth:"+ActionRead] = true
	return perms
}

func viewerPermissions() map[string]bool {
	perms := map[string]bool{}
	for _, resource := range []string{
		"instances",
		"vpcs",
		"storage",
		"events",
		"volumes",
		"dashboard",
		"loadbalancers",
		"databases",
		"secrets",
		"functions",
		"caches",
		"autoscaling",
	} {
		perms[resource+":"+ActionRead] = true
	}
	perms["auth:"+ActionRead] = true
	return perms
}

func grantAllActions(perms map[string]bool, resource string) {
	perms[resource+":"+ActionCreate] = true
	perms[resource+":"+ActionRead] = true
	perms[resource+":"+ActionUpdate] = true
	perms[resource+":"+ActionDelete] = true
	perms[resource+":"+ActionExecute] = true
}
