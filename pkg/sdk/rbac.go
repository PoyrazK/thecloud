package sdk

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

func (c *Client) CreateRole(name, description string, permissions []domain.Permission) (*domain.Role, error) {
	req := map[string]interface{}{
		"name":        name,
		"description": description,
		"permissions": permissions,
	}
	var role domain.Role
	err := c.post("/rbac/roles", req, &role)
	return &role, err
}

func (c *Client) ListRoles() ([]domain.Role, error) {
	var roles []domain.Role
	err := c.get("/rbac/roles", &roles)
	return roles, err
}

func (c *Client) GetRole(idOrName string) (*domain.Role, error) {
	var role domain.Role
	err := c.get(fmt.Sprintf("/rbac/roles/%s", idOrName), &role)
	return &role, err
}

func (c *Client) DeleteRole(id uuid.UUID) error {
	return c.delete(fmt.Sprintf("/rbac/roles/%s", id))
}

func (c *Client) UpdateRole(id uuid.UUID, name, description string, permissions []domain.Permission) (*domain.Role, error) {
	req := map[string]interface{}{
		"name":        name,
		"description": description,
		"permissions": permissions,
	}
	var role domain.Role
	err := c.put(fmt.Sprintf("/rbac/roles/%s", id), req, &role)
	return &role, err
}

func (c *Client) BindRole(userID uuid.UUID, roleName string) error {
	req := map[string]interface{}{
		"user_id":   userID,
		"role_name": roleName,
	}
	return c.post("/rbac/bindings", req, nil)
}

func (c *Client) ListRoleBindings() ([]domain.User, error) {
	var users []domain.User
	err := c.get("/rbac/bindings", &users)
	return users, err
}
