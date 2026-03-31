package dujiao

import "context"

func (c *Client) GetSMTPSettings(ctx context.Context, token string) (*SMTPSettings, error) {
	var out SMTPSettings
	if err := c.doJSON(ctx, "GET", "/admin/settings/smtp", token, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetTelegramBotRuntimeStatus(ctx context.Context, token string) (*TelegramBotRuntimeStatus, error) {
	var out TelegramBotRuntimeStatus
	if err := c.doJSON(ctx, "GET", "/admin/settings/telegram-bot/runtime-status", token, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
