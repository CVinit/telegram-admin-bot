package telegram

import (
	"context"
	"errors"
	"strings"

	"github.com/CVinit/telegram-admin-bot/internal/session"
)

func (r *Router) handleLogin(ctx context.Context, update IncomingUpdate, args []string) (*Response, error) {
	if len(args) < 2 {
		return &Response{Text: "用法: /login <username> <password>"}, nil
	}

	view, err := r.sessions.Login(ctx, update.TelegramUser, strings.TrimSpace(args[0]), args[1])
	if err != nil {
		return &Response{Text: "登录失败: " + err.Error()}, nil
	}

	return &Response{
		Text:         "已登录管理员 " + strings.TrimSpace(view.Username) + "。",
		Keyboard:     buildHomeKeyboard(),
		KeyboardKind: KeyboardKindReply,
	}, nil
}

func (r *Router) handleLogout(ctx context.Context, update IncomingUpdate) (*Response, error) {
	err := r.sessions.Logout(ctx, update.TelegramUser)
	if err != nil {
		return &Response{Text: "退出登录失败: " + err.Error()}, nil
	}
	return &Response{
		Text:           "已退出当前管理员会话。",
		RemoveKeyboard: true,
	}, nil
}

func (r *Router) renderSessionRequired(err error) *Response {
	switch {
	case errors.Is(err, session.ErrSessionRequired):
		return &Response{Text: notLoggedInSummary}
	case errors.Is(err, session.ErrSessionExpired):
		return &Response{Text: "当前登录已过期，请重新使用 /login。"}
	default:
		return &Response{Text: "读取会话失败: " + err.Error()}
	}
}
