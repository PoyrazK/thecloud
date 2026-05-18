// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"context"
	"time"
)

// FunctionSchedule describes a scheduled function invocation.
type FunctionSchedule struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	TenantID   string    `json:"tenant_id"`
	FunctionID string    `json:"function_id"`
	Name       string    `json:"name"`
	Schedule   string    `json:"schedule"`
	Payload    string    `json:"payload,omitempty"`
	Status     string    `json:"status"`
	LastRunAt  string    `json:"last_run_at,omitempty"`
	NextRunAt  string    `json:"next_run_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// FunctionScheduleRun records a single execution of a function schedule.
type FunctionScheduleRun struct {
	ID           string `json:"id"`
	ScheduleID   string `json:"schedule_id"`
	InvocationID string `json:"invocation_id"`
	Status       string `json:"status"`
	StatusCode   int    `json:"status_code"`
	DurationMs   int64  `json:"duration_ms"`
	ErrorMessage string `json:"error_message,omitempty"`
	StartedAt    string `json:"started_at"`
}

const functionSchedulesPath = "/function-schedules"

func (c *Client) CreateFunctionSchedule(functionID, name, schedule string, payload []byte) (*FunctionSchedule, error) {
	reqBody := struct {
		FunctionID string `json:"function_id"`
		Name       string `json:"name"`
		Schedule   string `json:"schedule"`
		Payload    string `json:"payload"`
	}{
		FunctionID: functionID,
		Name:       name,
		Schedule:   schedule,
		Payload:    string(payload),
	}

	var resp Response[FunctionSchedule]
	if err := c.post(functionSchedulesPath, reqBody, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) CreateFunctionScheduleContext(ctx context.Context, functionID, name, schedule string, payload []byte) (*FunctionSchedule, error) {
	reqBody := struct {
		FunctionID string `json:"function_id"`
		Name       string `json:"name"`
		Schedule   string `json:"schedule"`
		Payload    string `json:"payload"`
	}{
		FunctionID: functionID,
		Name:       name,
		Schedule:   schedule,
		Payload:    string(payload),
	}

	var resp Response[FunctionSchedule]
	if err := c.postContext(ctx, functionSchedulesPath, reqBody, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ListFunctionSchedules() ([]*FunctionSchedule, error) {
	var resp Response[[]*FunctionSchedule]
	if err := c.get(functionSchedulesPath, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetFunctionSchedule(idOrName string) (*FunctionSchedule, error) {
	id, err := c.resolveID("function-schedule", func() ([]interface{}, error) {
		schedules, err := c.ListFunctionSchedules()
		return interfaceSlicePtr(schedules), err
	}, func(v interface{}) string { return v.(*FunctionSchedule).ID }, func(v interface{}) string { return v.(*FunctionSchedule).Name }, idOrName)
	if err != nil {
		return nil, err
	}
	var resp Response[FunctionSchedule]
	if err := c.get(functionSchedulesPath+"/"+id, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteFunctionSchedule(idOrName string) error {
	id, err := c.resolveID("function-schedule", func() ([]interface{}, error) {
		schedules, err := c.ListFunctionSchedules()
		return interfaceSlicePtr(schedules), err
	}, func(v interface{}) string { return v.(*FunctionSchedule).ID }, func(v interface{}) string { return v.(*FunctionSchedule).Name }, idOrName)
	if err != nil {
		return err
	}
	return c.delete(functionSchedulesPath+"/"+id, nil)
}

func (c *Client) PauseFunctionSchedule(idOrName string) error {
	id, err := c.resolveID("function-schedule", func() ([]interface{}, error) {
		schedules, err := c.ListFunctionSchedules()
		return interfaceSlicePtr(schedules), err
	}, func(v interface{}) string { return v.(*FunctionSchedule).ID }, func(v interface{}) string { return v.(*FunctionSchedule).Name }, idOrName)
	if err != nil {
		return err
	}
	var resp Response[any]
	return c.post(functionSchedulesPath+"/"+id+"/pause", nil, &resp)
}

func (c *Client) ResumeFunctionSchedule(idOrName string) error {
	id, err := c.resolveID("function-schedule", func() ([]interface{}, error) {
		schedules, err := c.ListFunctionSchedules()
		return interfaceSlicePtr(schedules), err
	}, func(v interface{}) string { return v.(*FunctionSchedule).ID }, func(v interface{}) string { return v.(*FunctionSchedule).Name }, idOrName)
	if err != nil {
		return err
	}
	var resp Response[any]
	return c.post(functionSchedulesPath+"/"+id+"/resume", nil, &resp)
}

func (c *Client) GetFunctionScheduleRuns(idOrName string) ([]*FunctionScheduleRun, error) {
	id, err := c.resolveID("function-schedule", func() ([]interface{}, error) {
		schedules, err := c.ListFunctionSchedules()
		return interfaceSlicePtr(schedules), err
	}, func(v interface{}) string { return v.(*FunctionSchedule).ID }, func(v interface{}) string { return v.(*FunctionSchedule).Name }, idOrName)
	if err != nil {
		return nil, err
	}
	var resp Response[[]*FunctionScheduleRun]
	if err := c.get(functionSchedulesPath+"/"+id+"/runs", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
