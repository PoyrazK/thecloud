// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"bytes"
	"context"
	"fmt"
	"time"
)

// Function describes a serverless function.
type Function struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Runtime   string    `json:"runtime"`
	Handler   string    `json:"handler"`
	CodePath  string    `json:"code_path"`
	Timeout   int       `json:"timeout"`
	MemoryMB  int       `json:"memory_mb"`
	Status    string    `json:"status"`
	EnvVars   []*EnvVar `json:"env_vars,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EnvVar represents a key-value environment variable.
type EnvVar struct {
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	SecretRef string `json:"secret_ref,omitempty"`
}

// FunctionUpdateRequest describes fields that can be updated.
type FunctionUpdateRequest struct {
	Handler  *string   `json:"handler,omitempty"`
	Timeout  *int      `json:"timeout,omitempty"`
	MemoryMB *int      `json:"memory_mb,omitempty"`
	Status   string    `json:"status,omitempty"`
	EnvVars  []*EnvVar `json:"env_vars,omitempty"`
}

// Invocation represents a function invocation result.
type Invocation struct {
	ID         string     `json:"id"`
	FunctionID string     `json:"function_id"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	DurationMs int        `json:"duration_ms"`
	StatusCode int        `json:"status_code"`
	Logs       string     `json:"logs"`
}

const functionsPath = "/functions/"

func (c *Client) CreateFunction(name, runtime, handler string, code []byte) (*Function, error) {
	var resp Response[Function]
	req := c.resty.R().
		SetFileReader("code", "code.zip", bytes.NewReader(code)).
		SetFormData(map[string]string{
			"name":    name,
			"runtime": runtime,
			"handler": handler,
		}).
		SetResult(&resp)

	httpResp, err := req.Post(c.apiURL + "/functions")
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if httpResp.IsError() {
		return nil, fmt.Errorf("api error: %s", httpResp.String())
	}

	return &resp.Data, nil
}

func (c *Client) ListFunctions() ([]*Function, error) {
	var resp Response[[]*Function]
	if err := c.get("/functions", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) ListFunctionsWithPagination(limit, offset int) ([]*Function, *ListResponse[Function], error) {
	var resp Response[ListResponse[Function]]
	if err := c.getWithPagination("/functions", &resp, limit, offset); err != nil {
		return nil, nil, err
	}
	// Convert []Function to []*Function
	result := make([]*Function, len(resp.Data.Data))
	for i := range resp.Data.Data {
		result[i] = &resp.Data.Data[i]
	}
	return result, &resp.Data, nil
}

// ListFunctionsWithContextAndPagination returns functions with context and pagination metadata.
func (c *Client) ListFunctionsWithContextAndPagination(ctx context.Context, limit, offset int) ([]*Function, *ListResponse[Function], error) {
	var resp Response[ListResponse[Function]]
	if err := c.getContextWithPagination(ctx, "/functions", &resp, limit, offset); err != nil {
		return nil, nil, err
	}
	// Convert []Function to []*Function
	result := make([]*Function, len(resp.Data.Data))
	for i := range resp.Data.Data {
		result[i] = &resp.Data.Data[i]
	}
	return result, &resp.Data, nil
}

func (c *Client) ListFunctionsContext(ctx context.Context) ([]*Function, error) {
	var resp Response[[]*Function]
	if err := c.getContext(ctx, "/functions", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetFunction(id string) (*Function, error) {
	var resp Response[Function]
	if err := c.get(functionsPath+id, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteFunction(id string) error {
	return c.delete(functionsPath+id, nil)
}

func (c *Client) UpdateFunction(id string, req *FunctionUpdateRequest) (*Function, error) {
	var resp Response[Function]
	if err := c.patch(functionsPath+id, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) InvokeFunction(idOrName string, payload []byte, async bool) (*Invocation, error) {
	id, err := c.resolveID("function", func() ([]interface{}, error) {
		fns, err := c.ListFunctions()
		return interfaceSlicePtr(fns), err
	}, func(v interface{}) string { return v.(*Function).ID }, func(v interface{}) string { return v.(*Function).Name }, idOrName)
	if err != nil {
		return nil, err
	}
	url := functionsPath + id + "/invoke"
	if async {
		url += "?async=true"
	}

	var resp Response[Invocation]
	// post helper expects a struct for body, but we have []byte.
	// We'll use resty directly or fix helper.
	// The post helper in client.go: req.SetBody(body). Resty handles []byte.
	if err := c.post(url, payload, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetFunctionLogs(id string) ([]*Invocation, error) {
	var resp Response[[]*Invocation]
	if err := c.get(functionsPath+id+"/logs", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
