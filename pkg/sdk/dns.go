package sdk

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
)

// ListDNSZones returns all DNS zones for the current tenant.
func (c *Client) ListDNSZones() ([]domain.DNSZone, error) {
	var resp struct {
		Data []domain.DNSZone `json:"data"`
	}
	err := c.get("/dns/zones", &resp)
	return resp.Data, err
}

// CreateDNSZone creates a new DNS zone.
func (c *Client) CreateDNSZone(name, description string, vpcIDOrName *string) (*domain.DNSZone, error) {
	payload := map[string]interface{}{
		"name":        name,
		"description": description,
	}
	if vpcIDOrName != nil {
		id, err := c.resolveID("vpc", func() ([]interface{}, error) {
			vpcs, err := c.ListVPCs()
			return interfaceSlice(vpcs), err
		}, func(v interface{}) string { return v.(VPC).ID }, func(v interface{}) string { return v.(VPC).Name }, *vpcIDOrName)
		if err != nil {
			return nil, err
		}
		payload["vpc_id"] = id
	}

	var resp struct {
		Data domain.DNSZone `json:"data"`
	}
	err := c.post("/dns/zones", payload, &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetDNSZone returns details of a specific DNS zone.
func (c *Client) GetDNSZone(id string) (*domain.DNSZone, error) {
	var resp struct {
		Data domain.DNSZone `json:"data"`
	}
	err := c.get(fmt.Sprintf("/dns/zones/%s", id), &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// DeleteDNSZone deletes a DNS zone.
func (c *Client) DeleteDNSZone(id string) error {
	return c.delete(fmt.Sprintf("/dns/zones/%s", id), nil)
}

// ListDNSRecords returns all records in a DNS zone.
func (c *Client) ListDNSRecords(zoneID string) ([]domain.DNSRecord, error) {
	var resp struct {
		Data []domain.DNSRecord `json:"data"`
	}
	err := c.get(fmt.Sprintf("/dns/zones/%s/records", zoneID), &resp)
	return resp.Data, err
}

// CreateDNSRecord creates a new record in a zone.
func (c *Client) CreateDNSRecord(zoneID uuid.UUID, name string, recordType domain.RecordType, content string, ttl int, priority *int) (*domain.DNSRecord, error) {
	payload := map[string]interface{}{
		"name":     name,
		"type":     recordType,
		"content":  content,
		"ttl":      ttl,
		"priority": priority,
	}

	var resp struct {
		Data domain.DNSRecord `json:"data"`
	}
	err := c.post(fmt.Sprintf("/dns/zones/%s/records", zoneID), payload, &resp)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// DeleteDNSRecord deletes a DNS record.
func (c *Client) DeleteDNSRecord(recordID string) error {
	return c.delete(fmt.Sprintf("/dns/records/%s", recordID), nil)
}
