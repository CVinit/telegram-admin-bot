package dujiao

import "context"

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (c *Client) Login(ctx context.Context, username, password string) (*LoginResponse, error) {
	req := loginRequest{
		Username: username,
		Password: password,
	}
	var out LoginResponse
	if err := c.doJSON(ctx, "POST", "/admin/login", "", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetAuthzMe(ctx context.Context, token string) (*AuthzMeResponse, error) {
	var out AuthzMeResponse
	if err := c.doJSON(ctx, "GET", "/admin/authz/me", token, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
