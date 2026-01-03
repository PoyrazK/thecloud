package sdk

import (
	"fmt"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	resty  *resty.Client
	apiURL string
}

func NewClient(apiURL, apiKey string) *Client {
	client := resty.New()
	client.SetHeader("X-API-Key", apiKey)
	return &Client{
		resty:  client,
		apiURL: apiURL,
	}
}

type Response[T any] struct {
	Data  T              `json:"data"`
	Error *ErrorResponse `json:"error,omitempty"`
}

type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (c *Client) get(path string, result interface{}) error {
	resp, err := c.resty.R().
		SetResult(result).
		Get(c.apiURL + path)

	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("api error: %s", resp.String())
	}

	return nil
}

func (c *Client) post(path string, body interface{}, result interface{}) error {
	req := c.resty.R()
	if body != nil {
		req.SetBody(body)
	}
	if result != nil {
		req.SetResult(result)
	}

	resp, err := req.Post(c.apiURL + path)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("api error: %s", resp.String())
	}

	return nil
}

func (c *Client) delete(path string, result interface{}) error {
	req := c.resty.R()
	if result != nil {
		req.SetResult(result)
	}

	resp, err := req.Delete(c.apiURL + path)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("api error: %s", resp.String())
	}

	return nil
}

func (c *Client) put(path string, body interface{}, result interface{}) error {
	req := c.resty.R()
	if body != nil {
		req.SetBody(body)
	}
	if result != nil {
		req.SetResult(result)
	}

	resp, err := req.Put(c.apiURL + path)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if resp.IsError() {
		return fmt.Errorf("api error: %s", resp.String())
	}

	return nil
}
