package telegram

import (
	"context"
	"errors"
	"strings"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/workflow"
)

type sessionService interface {
	Login(ctx context.Context, telegramUser int64, username, password string) (*session.SessionView, error)
	RequireSession(ctx context.Context, telegramUser int64) (*session.SessionView, error)
	Logout(ctx context.Context, telegramUser int64) error
}

type Router struct {
	sessions sessionService
	sales    salesWorkflow
	restock  restockWorkflow
}

func NewRouter(sessions sessionService) *Router {
	return &Router{sessions: sessions}
}

func (r *Router) Handle(ctx context.Context, update IncomingUpdate) (*Response, error) {
	if r == nil {
		return nil, errors.New("router is nil")
	}
	if r.sessions == nil {
		return nil, errors.New("session service is nil")
	}

	if strings.TrimSpace(update.CallbackData) != "" {
		return r.handleCallback(ctx, update)
	}

	command, args := parseCommand(update.Text)
	if command == "" {
		return &Response{Text: helpSummaryText}, nil
	}
	if isSensitiveCommand(command) && !update.IsPrivateChat() {
		return &Response{Text: privateChatOnlyText}, nil
	}

	switch command {
	case "/login":
		return r.handleLogin(ctx, update, args)
	case "/logout":
		return r.handleLogout(ctx, update)
	case "/session":
		return r.handleSession(ctx, update)
	case "/help":
		return r.handleHelp(ctx, update)
	case "/sales":
		return r.handleSales(ctx, update, args)
	case "/restock":
		return r.handleRestock(ctx, update, args)
	case "/ship":
		return r.handlePlaceholder(ctx, update, "单个发货")
	case "/batch_ship":
		return r.handlePlaceholder(ctx, update, "批量发货")
	default:
		return &Response{Text: helpSummaryText}, nil
	}
}

func (r *Router) handleCallback(ctx context.Context, update IncomingUpdate) (*Response, error) {
	switch strings.TrimSpace(update.CallbackData) {
	case CallbackMenuHome:
		return r.handleMenuHome(ctx, update)
	default:
		if strings.HasPrefix(strings.TrimSpace(update.CallbackData), workflow.RestockConfirmPrefix) {
			return r.handleRestockConfirm(ctx, update)
		}
		return &Response{
			Text:         "未识别的菜单操作。",
			CallbackID:   update.CallbackID,
			CallbackText: "未识别的菜单操作",
		}, nil
	}
}

func parseCommand(text string) (string, []string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	header := text
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		header = text[:idx]
	}
	fields := strings.Fields(header)
	if len(fields) == 0 {
		return "", nil
	}
	command := fields[0]
	if alias := menuAliasToCommand(command); alias != "" {
		aliasFields := strings.Fields(alias)
		if len(aliasFields) == 0 {
			return "", nil
		}
		return trimCommandToken(aliasFields[0]), append(aliasFields[1:], fields[1:]...)
	}
	if !strings.HasPrefix(command, "/") {
		return "", nil
	}
	return trimCommandToken(command), fields[1:]
}

func isSensitiveCommand(command string) bool {
	switch command {
	case "/login", "/logout", "/session", "/sales", "/restock", "/ship", "/batch_ship":
		return true
	default:
		return false
	}
}
