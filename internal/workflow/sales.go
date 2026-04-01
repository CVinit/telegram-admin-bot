package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/dujiao"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
)

type dashboardOverviewAPI interface {
	GetDashboardOverview(ctx context.Context, token string, query dujiao.DashboardOverviewQuery) (*dujiao.DashboardOverviewResponse, error)
}

type SalesWorkflow struct {
	api dashboardOverviewAPI
}

type SalesOverviewView struct {
	RangeKey           string
	Title              string
	Timezone           string
	Currency           string
	GMVPaid            string
	TotalProfit        string
	PaidOrders         int64
	CompletedOrders    int64
	PaymentSuccessRate string
	TotalUserBalance   string
	SourceRange        string
}

func NewSalesWorkflow(api dashboardOverviewAPI) *SalesWorkflow {
	return &SalesWorkflow{api: api}
}

func (w *SalesWorkflow) BuildOverview(ctx context.Context, sessionView *session.SessionView, rangeKey string) (*SalesOverviewView, error) {
	if w == nil || w.api == nil {
		return nil, errors.New("sales workflow api is nil")
	}
	if sessionView == nil {
		return nil, errors.New("session is required")
	}
	if strings.TrimSpace(sessionView.JWTToken) == "" {
		return nil, errors.New("session jwt token is required")
	}

	normalizedRange, title, err := normalizeSalesRange(rangeKey)
	if err != nil {
		return nil, err
	}

	resp, err := w.api.GetDashboardOverview(ctx, sessionView.JWTToken, dujiao.DashboardOverviewQuery{
		Range: normalizedRange,
	})
	if err != nil {
		return nil, fmt.Errorf("get dashboard overview: %w", err)
	}
	if resp == nil {
		return nil, errors.New("dashboard overview response is nil")
	}

	return &SalesOverviewView{
		RangeKey:           normalizedRange,
		Title:              title,
		Timezone:           strings.TrimSpace(resp.Timezone),
		Currency:           strings.TrimSpace(resp.Currency),
		GMVPaid:            strings.TrimSpace(resp.KPI.GMVPaid),
		TotalProfit:        strings.TrimSpace(resp.KPI.TotalProfit),
		PaidOrders:         resp.KPI.PaidOrders,
		CompletedOrders:    resp.KPI.CompletedOrders,
		PaymentSuccessRate: strings.TrimSpace(resp.KPI.PaymentSuccessRate),
		TotalUserBalance:   strings.TrimSpace(resp.KPI.TotalUserBalance),
		SourceRange:        strings.TrimSpace(resp.Range),
	}, nil
}

func normalizeSalesRange(rangeKey string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(rangeKey)) {
	case "", "today":
		return "today", "今日销售", nil
	case "week":
		return "week", "本周销售", nil
	case "month":
		return "month", "本月销售", nil
	default:
		return "", "", fmt.Errorf("unsupported sales range %q", rangeKey)
	}
}
