package domain

const (
	RoleOwner     = "owner"
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
	RoleViewer    = "viewer"
	RoleUser      = "user" // Legacy/default role used by existing data.
)

var ValidRoles = map[string]struct{}{
	RoleOwner:     {},
	RoleAdmin:     {},
	RoleDeveloper: {},
	RoleViewer:    {},
	RoleUser:      {},
}

func IsValidRole(role string) bool {
	_, ok := ValidRoles[role]
	return ok
}

// NormalizeRole maps legacy roles to their modern equivalents for permission checks.
func NormalizeRole(role string) string {
	if role == RoleUser {
		return RoleDeveloper
	}
	return role
}
