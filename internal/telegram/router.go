package telegram

import (
	"context"
	"errors"
	"strings"

	"github.com/CVinit/telegram-admin-bot/internal/session"
	"github.com/CVinit/telegram-admin-bot/internal/workflow"
)

type sessionService interface {
	Login(ctx context.Context, telegramUser int64, username, password string) (*session.SessionView, error)
	RequireSession(ctx context.Context, telegramUser int64) (*session.SessionView, error)
	Logout(ctx context.Context, telegramUser int64) error
}

type Router struct {
	sessions           sessionService
	sales              salesWorkflow
	restock            restockWorkflow
	fulfillment        fulfillmentWorkflow
	pendingProductShip map[int64]pendingProductShipState
}

type pendingProductShipState struct {
	ProductID   uint
	SKUID       uint
	ProductName string
}

func NewRouter(sessions sessionService) *Router {
	return &Router{
		sessions:           sessions,
		pendingProductShip: make(map[int64]pendingProductShipState),
	}
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

	// Check if user is in pending product ship state (waiting for card secrets)
	if r.fulfillment != nil && update.TelegramUser != 0 && strings.TrimSpace(update.Text) != "" {
		if state, ok := r.pendingProductShip[update.TelegramUser]; ok {
			delete(r.pendingProductShip, update.TelegramUser)
			return r.handleProductShipWithSecrets(ctx, update, state)
		}
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
		return r.handleShip(ctx, update, args)
	case "/pending_ship":
		return r.handlePendingShip(ctx, update, args)
	case "/batch_ship":
		return r.handleBatchShip(ctx, update, args)
	case "/ship_by_product":
		return r.handleShipByProduct(ctx, update, args)
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
		if strings.HasPrefix(strings.TrimSpace(update.CallbackData), workflow.FulfillmentBatchConfirmPrefix) {
			return r.handleBatchFulfillmentConfirm(ctx, update)
		}
		if strings.HasPrefix(strings.TrimSpace(update.CallbackData), workflow.FulfillmentConfirmPrefix) {
			return r.handleFulfillmentConfirm(ctx, update)
		}
		if strings.HasPrefix(strings.TrimSpace(update.CallbackData), workflow.ProductShipConfirmPrefix) {
			return r.handleProductShipConfirm(ctx, update)
		}
		if strings.HasPrefix(strings.TrimSpace(update.CallbackData), workflow.ProductListConfirmPrefix) {
			return r.handleProductListSelect(ctx, update)
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
	case "/login", "/logout", "/session", "/sales", "/restock", "/ship", "/pending_ship", "/batch_ship", "/ship_by_product":
		return true
	default:
		return false
	}
}
