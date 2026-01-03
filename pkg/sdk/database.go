package sdk

import (
	"fmt"
	"time"
)

type Database struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Engine      string    `json:"engine"`
	Version     string    `json:"version"`
	Status      string    `json:"status"`
	VpcID       *string   `json:"vpc_id,omitempty"`
	ContainerID string    `json:"container_id,omitempty"`
	Port        int       `json:"port"`
	Username    string    `json:"username"`
	Password    string    `json:"password,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateDatabaseInput struct {
	Name    string  `json:"name"`
	Engine  string  `json:"engine"`
	Version string  `json:"version"`
	VpcID   *string `json:"vpc_id,omitempty"`
}

func (c *Client) CreateDatabase(name, engine, version string, vpcID *string) (*Database, error) {
	input := CreateDatabaseInput{
		Name:    name,
		Engine:  engine,
		Version: version,
		VpcID:   vpcID,
	}
	var resp Response[Database]
	if err := c.post("/databases", input, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ListDatabases() ([]*Database, error) {
	var resp Response[[]*Database]
	if err := c.get("/databases", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetDatabase(id string) (*Database, error) {
	var resp Response[Database]
	if err := c.get("/databases/"+id, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteDatabase(id string) error {
	return c.delete("/databases/"+id, nil)
}

func (c *Client) GetDatabaseConnectionString(id string) (string, error) {
	var resp Response[map[string]string]
	if err := c.get("/databases/"+id+"/connection", &resp); err != nil {
		return "", err
	}
	return resp.Data["connection_string"], nil
}

func (c *Client) GetDatabaseLogs(id string) (string, error) {
	resp, err := c.resty.R().Get(c.apiURL + "/databases/" + id + "/logs")
	if err != nil {
		return "", err
	}
	if resp.IsError() {
		return "", fmt.Errorf("api error: %s", resp.String())
	}
	return string(resp.Body()), nil
}
