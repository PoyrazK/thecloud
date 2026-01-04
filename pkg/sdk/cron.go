package sdk

import "fmt"

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
	var jobs []CronJob
	err := c.get("/cron/jobs", &jobs)
	return jobs, err
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
