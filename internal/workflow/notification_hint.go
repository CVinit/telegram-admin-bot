package workflow

import (
	"context"
	"strings"

	"github.com/CVinit/telegram-admin-bot/internal/dujiao"
	"github.com/CVinit/telegram-admin-bot/internal/session"
)

const (
	NotificationStatusExpectedAttempt = "expected_attempt"
	NotificationStatusExpectedSkip    = "expected_skip"
	NotificationStatusUnknown         = "unknown"
)

type NotificationTargetHint struct {
	Status string
	Reason string
}

type NotificationHint struct {
	Email    NotificationTargetHint
	Telegram NotificationTargetHint
}

type fulfillmentAPI interface {
	ListOrders(ctx context.Context, token string, params dujiao.ListOrdersParams) (*dujiao.OrderListResponse, error)
	GetOrder(ctx context.Context, token string, id uint) (*dujiao.OrderDetail, error)
	CreateFulfillment(ctx context.Context, token string, req dujiao.CreateFulfillmentRequest) (*dujiao.FulfillmentResponse, error)
	GetProduct(ctx context.Context, token string, id uint) (*dujiao.ProductDetail, error)
	GetUser(ctx context.Context, token string, id uint) (*dujiao.AdminUserDetail, error)
	GetSMTPSettings(ctx context.Context, token string) (*dujiao.SMTPSettings, error)
	GetTelegramBotRuntimeStatus(ctx context.Context, token string) (*dujiao.TelegramBotRuntimeStatus, error)
	ListChannelClients(ctx context.Context, token string) ([]dujiao.ChannelClient, error)
}

type fulfillmentAuditLogger interface {
	LogActionStarted(ctx context.Context, telegramUser int64, adminID uint, action, target string, metadata map[string]any) error
	LogActionSucceeded(ctx context.Context, telegramUser int64, adminID uint, action, target, result string, metadata map[string]any) error
	LogActionFailed(ctx context.Context, telegramUser int64, adminID uint, action, target, result string, metadata map[string]any) error
}

type FulfillmentWorkflow struct {
	api           fulfillmentAPI
	confirmations *ConfirmationService
	audit         fulfillmentAuditLogger
}

func NewFulfillmentWorkflow(api fulfillmentAPI, confirmations *ConfirmationService, audit fulfillmentAuditLogger) *FulfillmentWorkflow {
	return &FulfillmentWorkflow{
		api:           api,
		confirmations: confirmations,
		audit:         audit,
	}
}

func (w *FulfillmentWorkflow) BuildNotificationHint(ctx context.Context, sessionView *session.SessionView, order *dujiao.OrderDetail) (*NotificationHint, error) {
	if w == nil || w.api == nil {
		return nil, errNilFulfillmentAPI
	}
	if sessionView == nil {
		return nil, errSessionRequired
	}
	if order == nil {
		return nil, errOrderRequired
	}

	hint := &NotificationHint{
		Email:    NotificationTargetHint{Status: NotificationStatusUnknown},
		Telegram: NotificationTargetHint{Status: NotificationStatusUnknown},
	}
	token := strings.TrimSpace(sessionView.JWTToken)

	receiverEmail := resolveReceiverEmail(order)
	switch {
	case receiverEmail == "":
		hint.Email = NotificationTargetHint{Status: NotificationStatusExpectedSkip, Reason: "missing receiver email"}
	default:
		smtp, err := w.api.GetSMTPSettings(ctx, token)
		if err != nil {
			hint.Email = NotificationTargetHint{Status: NotificationStatusUnknown, Reason: "smtp settings unavailable"}
		} else if smtp == nil {
			hint.Email = NotificationTargetHint{Status: NotificationStatusUnknown, Reason: "smtp settings missing"}
		} else if !smtp.Enabled {
			hint.Email = NotificationTargetHint{Status: NotificationStatusExpectedSkip, Reason: "smtp disabled"}
		} else if !smtp.OrderNotificationEnabled {
			hint.Email = NotificationTargetHint{Status: NotificationStatusExpectedSkip, Reason: "order notification email disabled"}
		} else if isPlaceholderEmail(receiverEmail) {
			hint.Email = NotificationTargetHint{Status: NotificationStatusExpectedSkip, Reason: "telegram placeholder email"}
		} else {
			hint.Email = NotificationTargetHint{Status: NotificationStatusExpectedAttempt}
		}
	}

	if order.UserID == 0 {
		hint.Telegram = NotificationTargetHint{Status: NotificationStatusExpectedSkip, Reason: "guest order"}
		return hint, nil
	}

	user, err := w.api.GetUser(ctx, token, order.UserID)
	if err != nil {
		hint.Telegram = NotificationTargetHint{Status: NotificationStatusUnknown, Reason: "user detail unavailable"}
		return hint, nil
	}
	if user == nil {
		hint.Telegram = NotificationTargetHint{Status: NotificationStatusExpectedSkip, Reason: "user detail missing"}
		return hint, nil
	}
	if !hasTelegramIdentity(user) {
		hint.Telegram = NotificationTargetHint{Status: NotificationStatusExpectedSkip, Reason: "user not bound to telegram"}
		return hint, nil
	}

	clients, err := w.api.ListChannelClients(ctx, token)
	if err != nil {
		hint.Telegram = NotificationTargetHint{Status: NotificationStatusUnknown, Reason: "channel clients unavailable"}
		return hint, nil
	}
	client := activeTelegramBotClient(clients)
	if client == nil {
		hint.Telegram = NotificationTargetHint{Status: NotificationStatusExpectedSkip, Reason: "telegram bot channel inactive"}
		return hint, nil
	}
	if strings.TrimSpace(client.CallbackURL) == "" {
		hint.Telegram = NotificationTargetHint{Status: NotificationStatusExpectedSkip, Reason: "telegram callback url missing"}
		return hint, nil
	}

	runtime, err := w.api.GetTelegramBotRuntimeStatus(ctx, token)
	if err != nil {
		hint.Telegram = NotificationTargetHint{Status: NotificationStatusUnknown, Reason: "telegram runtime unavailable"}
		return hint, nil
	}
	if runtime == nil {
		hint.Telegram = NotificationTargetHint{Status: NotificationStatusUnknown, Reason: "telegram runtime missing"}
		return hint, nil
	}
	if !runtime.Connected {
		hint.Telegram = NotificationTargetHint{Status: NotificationStatusExpectedSkip, Reason: "telegram runtime disconnected"}
		return hint, nil
	}

	hint.Telegram = NotificationTargetHint{Status: NotificationStatusExpectedAttempt}
	return hint, nil
}

func resolveReceiverEmail(order *dujiao.OrderDetail) string {
	if order == nil {
		return ""
	}
	if email := strings.TrimSpace(order.UserEmail); email != "" {
		return email
	}
	return strings.TrimSpace(order.GuestEmail)
}

func hasTelegramIdentity(user *dujiao.AdminUserDetail) bool {
	if user == nil {
		return false
	}
	for _, identity := range user.OAuthIdentities {
		if strings.TrimSpace(identity.Provider) == "telegram" && strings.TrimSpace(identity.ProviderUserID) != "" {
			return true
		}
	}
	return false
}

func activeTelegramBotClient(clients []dujiao.ChannelClient) *dujiao.ChannelClient {
	for i := range clients {
		if strings.TrimSpace(clients[i].ChannelType) == "telegram_bot" && clients[i].Status == 1 {
			return &clients[i]
		}
	}
	return nil
}

func isPlaceholderEmail(email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	return strings.HasPrefix(email, "telegram_") && strings.HasSuffix(email, "@login.local")
}
