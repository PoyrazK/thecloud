// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Cache describes a cache instance.
type Cache struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Engine      string    `json:"engine"`
	Version     string    `json:"version"`
	Status      string    `json:"status"`
	VpcID       *string   `json:"vpc_id,omitempty"`
	ContainerID string    `json:"container_id,omitempty"`
	Port        int       `json:"port"`
	Password    string    `json:"password,omitempty"` // Only returned on Create/Get usually?
	MemoryMB    int       `json:"memory_mb"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateCacheInput defines parameters for creating a cache.
type CreateCacheInput struct {
	Name     string  `json:"name"`
	Version  string  `json:"version"`
	MemoryMB int     `json:"memory_mb"`
	VpcID    *string `json:"vpc_id,omitempty"`
}

// CacheStats summarizes cache runtime metrics.
type CacheStats struct {
	UsedMemoryBytes  int64 `json:"used_memory_bytes"`
	MaxMemoryBytes   int64 `json:"max_memory_bytes"`
	ConnectedClients int   `json:"connected_clients"`
	TotalKeys        int64 `json:"total_keys"`
}

const cachesPath = "/caches/"

func (c *Client) CreateCache(name, version string, memoryMB int, vpcID *string) (*Cache, error) {
	input := CreateCacheInput{
		Name:     name,
		Version:  version,
		MemoryMB: memoryMB,
		VpcID:    vpcID,
	}
	var resp Response[Cache]
	if err := c.post("/caches", input, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ListCaches() ([]*Cache, error) {
	var resp Response[[]*Cache]
	if err := c.get("/caches", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetCache(id string) (*Cache, error) {
	var resp Response[Cache]
	if err := c.get(cachesPath+id, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteCache(id string) error {
	return c.delete(cachesPath+id, nil)
}

func (c *Client) GetCacheConnectionString(id string) (string, error) {
	var resp Response[map[string]string]
	if err := c.get(cachesPath+id+"/connection", &resp); err != nil {
		return "", err
	}
	return resp.Data["connection_string"], nil
}

func (c *Client) FlushCache(id string) error {
	return c.post(cachesPath+id+"/flush", nil, nil)
}

func (c *Client) GetCacheStats(id string) (*CacheStats, error) {
	var resp Response[CacheStats]
	if err := c.get(cachesPath+id+"/stats", &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ResizeCache(idOrName string, newMemoryMB int) error {
	// Resolve name to ID if needed
	id := idOrName
	if _, err := uuid.Parse(idOrName); err != nil {
		caches, err := c.ListCaches()
		if err != nil {
			return err
		}
		for _, cache := range caches {
			if cache.Name == idOrName {
				id = cache.ID
				break
			}
		}
	}
	body := map[string]int{"memory_mb": newMemoryMB}
	path := fmt.Sprintf("%s%s/resize", cachesPath, id)
	return c.post(path, body, nil)
}
