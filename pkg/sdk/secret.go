// Package sdk provides the official Go SDK for the platform.
package sdk

import "time"

// Secret describes a stored secret value.
type Secret struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	Name           string     `json:"name"`
	EncryptedValue string     `json:"encrypted_value"` // This field name is used for plaintext in Get response
	Description    string     `json:"description"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
}

// CreateSecretInput defines parameters for creating a secret.
type CreateSecretInput struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

func (c *Client) CreateSecret(name, value, description string) (*Secret, error) {
	input := CreateSecretInput{
		Name:        name,
		Value:       value,
		Description: description,
	}
	var resp Response[Secret]
	if err := c.post("/secrets", input, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ListSecrets() ([]*Secret, error) {
	var resp Response[[]*Secret]
	if err := c.get("/secrets", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetSecret(idOrName string) (*Secret, error) {
	id, err := c.resolveID("secret", func() ([]interface{}, error) {
		secrets, err := c.ListSecrets()
		return interfaceSlicePtr(secrets), err
	}, func(v interface{}) string { return v.(*Secret).ID }, func(v interface{}) string { return v.(*Secret).Name }, idOrName)
	if err != nil {
		return nil, err
	}
	var resp Response[Secret]
	if err := c.get("/secrets/"+id, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteSecret(idOrName string) error {
	id, err := c.resolveID("secret", func() ([]interface{}, error) {
		secrets, err := c.ListSecrets()
		return interfaceSlicePtr(secrets), err
	}, func(v interface{}) string { return v.(*Secret).ID }, func(v interface{}) string { return v.(*Secret).Name }, idOrName)
	if err != nil {
		return err
	}
	return c.delete("/secrets/"+id, nil)
}
