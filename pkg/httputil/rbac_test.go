package httputil

import (
	"testing"

	"github.com/poyrazk/thecloud/internal/core/domain"
)

func TestHasPermission(t *testing.T) {
	if !HasPermission(domain.RoleDeveloper, Permission{Resource: "instances", Action: ActionCreate}) {
		t.Fatal("expected developer to have instances:create")
	}
	if HasPermission(domain.RoleViewer, Permission{Resource: "instances", Action: ActionDelete}) {
		t.Fatal("expected viewer to not have instances:delete")
	}
	if !HasPermission(domain.RoleUser, Permission{Resource: "instances", Action: ActionRead}) {
		t.Fatal("expected legacy user role to map to developer permissions")
	}
	if HasPermission(domain.RoleAdmin, Permission{Resource: "auth", Action: ActionUpdate}) {
		t.Fatal("expected admin to not have auth:update")
	}
	if !HasPermission(domain.RoleOwner, Permission{Resource: "auth", Action: ActionUpdate}) {
		t.Fatal("expected owner to have auth:update")
	}
}
