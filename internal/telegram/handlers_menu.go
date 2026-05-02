package telegram

import (
	"context"
	"strings"
)

const (
	menuTextSales         = "销售统计"
	menuTextSalesToday    = "今日销售"
	menuTextSalesWeek     = "本周销售"
	menuTextSalesMonth    = "本月销售"
	menuTextRestock       = "补自动库存"
	menuTextShip          = "单个发货"
	menuTextPending       = "待发货订单"
	menuTextBatchShip     = "批量发货"
	menuTextShipByProduct = "按商品发货"
	menuTextSession       = "我的会话"
	menuTextHelp          = "帮助"
	menuTextLogout        = "退出登录"
	menuHomeTitle         = "管理员菜单"
	helpSummaryText       = "可用命令: /login /logout /session /help /sales today|week|month /restock /pending_ship /ship /batch_ship"
	notLoggedInSummary    = "当前未登录，请先使用 /login <username> <password>。"
)

func (r *Router) handleHelp(context.Context, IncomingUpdate) (*Response, error) {
	return &Response{Text: helpSummaryText}, nil
}

func (r *Router) handleMenuHome(ctx context.Context, update IncomingUpdate) (*Response, error) {
	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}
	return &Response{
		Text:         menuHomeTitle + "\n当前管理员: " + strings.TrimSpace(view.Username),
		Keyboard:     buildHomeKeyboard(),
		KeyboardKind: KeyboardKindReply,
		CallbackID:   update.CallbackID,
	}, nil
}

func (r *Router) handlePlaceholder(_ context.Context, update IncomingUpdate, feature string) (*Response, error) {
	return &Response{
		Text: feature + notImplementedSuffix,
	}, nil
}

func buildHomeKeyboard() [][]Button {
	return [][]Button{
		{{Text: menuTextSalesToday}, {Text: menuTextSalesWeek}, {Text: menuTextSalesMonth}},
		{{Text: menuTextRestock}},
		{{Text: menuTextPending}, {Text: menuTextShipByProduct}, {Text: menuTextShip}},
		{{Text: menuTextBatchShip}},
		{{Text: menuTextSession}, {Text: menuTextHelp}},
		{{Text: menuTextLogout}},
	}
}

func menuAliasToCommand(text string) string {
	switch strings.TrimSpace(text) {
	case menuTextSalesToday:
		return "/sales today"
	case menuTextSalesWeek:
		return "/sales week"
	case menuTextSalesMonth:
		return "/sales month"
	case menuTextSales:
		return "/sales"
	case menuTextRestock:
		return "/restock"
	case menuTextShip:
		return "/ship"
	case menuTextPending:
		return "/pending_ship"
	case menuTextBatchShip:
		return "/batch_ship"
	case menuTextShipByProduct:
		return "/ship_by_product"
	case menuTextSession:
		return "/session"
	case menuTextHelp:
		return "/help"
	case menuTextLogout:
		return "/logout"
	default:
		return ""
	}
}
