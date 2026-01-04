package sdk

import "fmt"

type GatewayRoute struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	PathPrefix  string `json:"path_prefix"`
	TargetURL   string `json:"target_url"`
	StripPrefix bool   `json:"strip_prefix"`
	RateLimit   int    `json:"rate_limit"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (c *Client) CreateGatewayRoute(name, prefix, target string, strip bool, rateLimit int) (*GatewayRoute, error) {
	req := struct {
		Name        string `json:"name"`
		PathPrefix  string `json:"path_prefix"`
		TargetURL   string `json:"target_url"`
		StripPrefix bool   `json:"strip_prefix"`
		RateLimit   int    `json:"rate_limit"`
	}{
		Name:        name,
		PathPrefix:  prefix,
		TargetURL:   target,
		StripPrefix: strip,
		RateLimit:   rateLimit,
	}

	var route GatewayRoute
	err := c.post("/gateway/routes", req, &route)
	return &route, err
}

func (c *Client) ListGatewayRoutes() ([]GatewayRoute, error) {
	var routes []GatewayRoute
	err := c.get("/gateway/routes", &routes)
	return routes, err
}

func (c *Client) DeleteGatewayRoute(id string) error {
	return c.delete(fmt.Sprintf("/gateway/routes/%s", id), nil)
}
