// Package sdk provides the official Go SDK for the platform.
package sdk

import (
	"fmt"
	"time"
)

const autoscalingAPIErrorFormat = "api error: %s"

// ScalingGroup describes an autoscaling group.
type ScalingGroup struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	VpcID          string    `json:"vpc_id"`
	LoadBalancerID string    `json:"load_balancer_id,omitempty"`
	Image          string    `json:"image"`
	Ports          string    `json:"ports,omitempty"`
	MinInstances   int       `json:"min_instances"`
	MaxInstances   int       `json:"max_instances"`
	DesiredCount   int       `json:"desired_count"`
	CurrentCount   int       `json:"current_count"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateScalingGroupRequest defines parameters for creating a scaling group.
type CreateScalingGroupRequest struct {
	Name           string  `json:"name"`
	VpcID          string  `json:"vpc_id"`
	LoadBalancerID *string `json:"load_balancer_id,omitempty"`
	Image          string  `json:"image"`
	Ports          string  `json:"ports"`
	MinInstances   int     `json:"min_instances"`
	MaxInstances   int     `json:"max_instances"`
	DesiredCount   int     `json:"desired_count"`
}

func (c *Client) CreateScalingGroup(req CreateScalingGroupRequest) (*ScalingGroup, error) {
	var respData Response[ScalingGroup]
	resp, err := c.resty.R().
		SetBody(req).
		SetResult(&respData).
		Post(c.apiURL + "/autoscaling/groups")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf(autoscalingAPIErrorFormat, resp.String())
	}
	return &respData.Data, nil
}

func (c *Client) ListScalingGroups() ([]ScalingGroup, error) {
	var respData Response[[]ScalingGroup]
	resp, err := c.resty.R().
		SetResult(&respData).
		Get(c.apiURL + "/autoscaling/groups")

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf(autoscalingAPIErrorFormat, resp.String())
	}
	return respData.Data, nil
}

func (c *Client) GetScalingGroup(idOrName string) (*ScalingGroup, error) {
	id, err := c.resolveID("scaling-group", func() ([]interface{}, error) {
		groups, err := c.ListScalingGroups()
		return interfaceSlice(groups), err
	}, func(v interface{}) string { return v.(ScalingGroup).ID }, func(v interface{}) string { return v.(ScalingGroup).Name }, idOrName)
	if err != nil {
		return nil, err
	}
	var respData Response[ScalingGroup]
	resp, err := c.resty.R().
		SetResult(&respData).
		Get(c.apiURL + "/autoscaling/groups/" + id)

	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf(autoscalingAPIErrorFormat, resp.String())
	}
	return &respData.Data, nil
}

func (c *Client) DeleteScalingGroup(idOrName string) error {
	id, err := c.resolveID("scaling-group", func() ([]interface{}, error) {
		groups, err := c.ListScalingGroups()
		return interfaceSlice(groups), err
	}, func(v interface{}) string { return v.(ScalingGroup).ID }, func(v interface{}) string { return v.(ScalingGroup).Name }, idOrName)
	if err != nil {
		return err
	}
	resp, err := c.resty.R().Delete(c.apiURL + "/autoscaling/groups/" + id)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf(autoscalingAPIErrorFormat, resp.String())
	}
	return nil
}

// CreatePolicyRequest defines parameters for creating a scaling policy.
type CreatePolicyRequest struct {
	Name        string  `json:"name"`
	MetricType  string  `json:"metric_type"`
	TargetValue float64 `json:"target_value"`
	ScaleOut    int     `json:"scale_out_step"`
	ScaleIn     int     `json:"scale_in_step"`
	CooldownSec int     `json:"cooldown_sec"`
}

func (c *Client) CreateScalingPolicy(groupIDOrName string, req CreatePolicyRequest) error {
	id, err := c.resolveID("scaling-group", func() ([]interface{}, error) {
		groups, err := c.ListScalingGroups()
		return interfaceSlice(groups), err
	}, func(v interface{}) string { return v.(ScalingGroup).ID }, func(v interface{}) string { return v.(ScalingGroup).Name }, groupIDOrName)
	if err != nil {
		return err
	}
	resp, err := c.resty.R().
		SetBody(req).
		Post(fmt.Sprintf("%s/autoscaling/groups/%s/policies", c.apiURL, id))

	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf(autoscalingAPIErrorFormat, resp.String())
	}
	return nil
}

func (c *Client) DeleteScalingPolicy(id string) error {
	resp, err := c.resty.R().Delete(c.apiURL + "/autoscaling/policies/" + id)
	if err != nil {
		return err
	}
	if resp.IsError() {
		return fmt.Errorf(autoscalingAPIErrorFormat, resp.String())
	}
	return nil
}
