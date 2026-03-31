package dujiao

import "context"

func (c *Client) ListChannelClients(ctx context.Context, token string) ([]ChannelClient, error) {
	var out []ChannelClient
	if err := c.doJSON(ctx, "GET", "/admin/channel-clients", token, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
