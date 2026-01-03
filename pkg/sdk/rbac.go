package sdk

type RoleResponse struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func (c *Client) ListRoles() ([]string, error) {
	var res Response[struct {
		Roles []string `json:"roles"`
	}]
	if err := c.get("/auth/roles", &res); err != nil {
		return nil, err
	}
	return res.Data.Roles, nil
}

func (c *Client) GetMyRole() (*RoleResponse, error) {
	var res Response[RoleResponse]
	if err := c.get("/auth/me/role", &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) GetUserRole(userID string) (*RoleResponse, error) {
	var res Response[RoleResponse]
	if err := c.get("/auth/users/"+userID+"/role", &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) UpdateUserRole(userID, role string) (*RoleResponse, error) {
	body := map[string]string{"role": role}
	var res Response[RoleResponse]
	if err := c.put("/auth/users/"+userID+"/role", body, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}
