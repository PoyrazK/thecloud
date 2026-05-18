// Package sdk provides the official Go SDK for the platform.
package sdk

import "time"

// Database describes a managed database instance.
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

// CreateDatabaseInput defines parameters for creating a database.
type CreateDatabaseInput struct {
	Name             string  `json:"name"`
	Engine           string  `json:"engine"`
	Version          string  `json:"version"`
	VpcID            *string `json:"vpc_id,omitempty"`
	AllocatedStorage int     `json:"allocated_storage_gb,omitempty"`
}

const databasesPath = "/databases/"

func (c *Client) CreateDatabase(name, engine, version string, vpcID *string, allocatedStorageGB int) (*Database, error) {
	input := CreateDatabaseInput{
		Name:             name,
		Engine:           engine,
		Version:          version,
		VpcID:            vpcID,
		AllocatedStorage: allocatedStorageGB,
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
	if err := c.get(databasesPath+id, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteDatabase(id string) error {
	return c.delete(databasesPath+id, nil)
}

func (c *Client) GetDatabaseConnectionString(id string) (string, error) {
	var resp Response[map[string]string]
	if err := c.get(databasesPath+id+"/connection", &resp); err != nil {
		return "", err
	}
	return resp.Data["connection_string"], nil
}

func (c *Client) RotateDatabaseCredentials(id string) error {
	return c.post(databasesPath+id+"/rotate-credentials", nil, nil)
}

func (c *Client) ResizeDatabase(id string, newSizeGB int) error {
	body := map[string]int{"allocated_storage": newSizeGB}
	return c.post(databasesPath+id+"/resize", body, nil)
}
