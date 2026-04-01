package telegram

import (
	"context"
	"errors"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/render"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/workflow"
)

type salesWorkflow interface {
	BuildOverview(ctx context.Context, sessionView *session.SessionView, rangeKey string) (*workflow.SalesOverviewView, error)
}

func (r *Router) WithSalesWorkflow(sales salesWorkflow) *Router {
	r.sales = sales
	return r
}

func (r *Router) handleSales(ctx context.Context, update IncomingUpdate, args []string) (*Response, error) {
	if r.sales == nil {
		return r.handlePlaceholder(ctx, update, "销售统计")
	}

	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	rangeKey := "today"
	if len(args) > 0 {
		rangeKey = args[0]
	}

	overview, err := r.sales.BuildOverview(ctx, view, rangeKey)
	if err != nil {
		if errors.Is(err, session.ErrSessionRequired) || errors.Is(err, session.ErrSessionExpired) {
			return r.renderSessionRequired(err), nil
		}
		return &Response{Text: "查询销售统计失败: " + err.Error()}, nil
	}

	return &Response{Text: render.RenderSalesOverview(overview)}, nil
}
