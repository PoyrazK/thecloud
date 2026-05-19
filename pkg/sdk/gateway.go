// Package sdk provides the official Go SDK for the platform.
package sdk

import "fmt"

// GatewayRoute describes an API gateway route.
type GatewayRoute struct {
	ID          string   `json:"id"`
	UserID      string   `json:"user_id"`
	Name        string   `json:"name"`
	PathPrefix  string   `json:"path_prefix"`
	PathPattern string   `json:"path_pattern"`
	PatternType string   `json:"pattern_type"`
	ParamNames  []string `json:"param_names"`
	TargetURL   string   `json:"target_url"`
	Methods     []string `json:"methods"`
	StripPrefix bool     `json:"strip_prefix"`
	RateLimit   int      `json:"rate_limit"`
	Priority    int      `json:"priority"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

func (c *Client) CreateGatewayRoute(name, prefix, target string, methods []string, strip bool, rateLimit int, priority int) (*GatewayRoute, error) {
	req := struct {
		Name        string   `json:"name"`
		PathPrefix  string   `json:"path_prefix"`
		TargetURL   string   `json:"target_url"`
		Methods     []string `json:"methods"`
		StripPrefix bool     `json:"strip_prefix"`
		RateLimit   int      `json:"rate_limit"`
		Priority    int      `json:"priority"`
	}{
		Name:        name,
		PathPrefix:  prefix,
		TargetURL:   target,
		Methods:     methods,
		StripPrefix: strip,
		RateLimit:   rateLimit,
		Priority:    priority,
	}

	var route GatewayRoute
	err := c.post("/gateway/routes", req, &route)
	return &route, err
}

func (c *Client) ListGatewayRoutes() ([]GatewayRoute, error) {
	var res Response[[]GatewayRoute]
	err := c.get("/gateway/routes", &res)
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (c *Client) DeleteGatewayRoute(id string) error {
	return c.delete(fmt.Sprintf("/gateway/routes/%s", id), nil)
}
