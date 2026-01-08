package sdk

import (
	"fmt"
	"time"
)

type SecurityGroup struct {
	ID          string         `json:"id"`
	VPCID       string         `json:"vpc_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ARN         string         `json:"arn"`
	Rules       []SecurityRule `json:"rules,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type SecurityRule struct {
	ID        string `json:"id"`
	Direction string `json:"direction"`
	Protocol  string `json:"protocol"`
	PortMin   int    `json:"port_min"`
	PortMax   int    `json:"port_max"`
	CIDR      string `json:"cidr"`
	Priority  int    `json:"priority"`
}

func (c *Client) CreateSecurityGroup(vpcID, name, description string) (*SecurityGroup, error) {
	body := map[string]string{
		"vpc_id":      vpcID,
		"name":        name,
		"description": description,
	}
	var res Response[SecurityGroup]
	if err := c.post("/security-groups", body, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) ListSecurityGroups(vpcID string) ([]SecurityGroup, error) {
	var res Response[[]SecurityGroup]
	path := "/security-groups"
	if vpcID != "" {
		path = fmt.Sprintf("%s?vpc_id=%s", path, vpcID)
	}
	if err := c.get(path, &res); err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (c *Client) GetSecurityGroup(id string) (*SecurityGroup, error) {
	var res Response[SecurityGroup]
	if err := c.get(fmt.Sprintf("/security-groups/%s", id), &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) DeleteSecurityGroup(id string) error {
	return c.delete(fmt.Sprintf("/security-groups/%s", id), nil)
}

func (c *Client) AddSecurityRule(groupID string, rule SecurityRule) (*SecurityRule, error) {
	var res Response[SecurityRule]
	if err := c.post(fmt.Sprintf("/security-groups/%s/rules", groupID), rule, &res); err != nil {
		return nil, err
	}
	return &res.Data, nil
}

func (c *Client) AttachSecurityGroup(instanceID, groupID string) error {
	body := map[string]string{
		"instance_id": instanceID,
		"group_id":    groupID,
	}
	return c.post("/security-groups/attach", body, nil)
}
