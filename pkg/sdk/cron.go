// Package sdk provides the official Go SDK for the platform.
package sdk

import "fmt"

// CronJob describes a scheduled job.
type CronJob struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	Name          string `json:"name"`
	Schedule      string `json:"schedule"`
	TargetURL     string `json:"target_url"`
	TargetMethod  string `json:"target_method"`
	TargetPayload string `json:"target_payload"`
	Status        string `json:"status"`
	LastRunAt     string `json:"last_run_at,omitempty"`
	NextRunAt     string `json:"next_run_at,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func (c *Client) CreateCronJob(name, schedule, url, method, payload string) (*CronJob, error) {
	req := struct {
		Name          string `json:"name"`
		Schedule      string `json:"schedule"`
		TargetURL     string `json:"target_url"`
		TargetMethod  string `json:"target_method"`
		TargetPayload string `json:"target_payload"`
	}{
		Name:          name,
		Schedule:      schedule,
		TargetURL:     url,
		TargetMethod:  method,
		TargetPayload: payload,
	}

	var job CronJob
	err := c.post("/cron/jobs", req, &job)
	return &job, err
}

func (c *Client) ListCronJobs() ([]CronJob, error) {
	var res Response[[]CronJob]
	err := c.get("/cron/jobs", &res)
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (c *Client) GetCronJob(id string) (*CronJob, error) {
	var job CronJob
	err := c.get(fmt.Sprintf("/cron/jobs/%s", id), &job)
	return &job, err
}

func (c *Client) PauseCronJob(id string) error {
	return c.post(fmt.Sprintf("/cron/jobs/%s/pause", id), nil, nil)
}

func (c *Client) ResumeCronJob(id string) error {
	return c.post(fmt.Sprintf("/cron/jobs/%s/resume", id), nil, nil)
}

func (c *Client) DeleteCronJob(id string) error {
	return c.delete(fmt.Sprintf("/cron/jobs/%s", id), nil)
}

// CronJobRun describes a single execution of a cron job.
type CronJobRun struct {
	ID         string `json:"id"`
	JobID      string `json:"job_id"`
	Status     string `json:"status"`
	StatusCode int    `json:"status_code"`
	Response   string `json:"response"`
	DurationMs int64  `json:"duration_ms"`
	StartedAt  string `json:"started_at"`
}

// GetCronJobRuns returns execution history for a cron job.
func (c *Client) GetCronJobRuns(id string, limit int) ([]CronJobRun, error) {
	var res Response[[]CronJobRun]
	err := c.get(fmt.Sprintf("/cron/jobs/%s/runs?limit=%d", id, limit), &res)
	return res.Data, err
}

// UpdateCronJob updates an existing cron job.
func (c *Client) UpdateCronJob(id string, job *CronJob) (*CronJob, error) {
	var updated CronJob
	err := c.put(fmt.Sprintf("/cron/jobs/%s", id), job, &updated)
	return &updated, err
}
