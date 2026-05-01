package render

import (
	"strconv"
	"strings"

	"github.com/CVinit/telegram-admin-bot/internal/workflow"
)

func RenderSalesOverview(view *workflow.SalesOverviewView) string {
	if view == nil {
		return "销售统计不可用。"
	}

	currency := strings.TrimSpace(view.Currency)
	if currency == "" {
		currency = "-"
	}

	lines := []string{
		view.Title,
		"范围: " + fallback(view.RangeKey),
		"时区: " + fallback(view.Timezone),
		"GMV: " + renderMoney(view.GMVPaid, currency),
		"利润: " + renderMoney(view.TotalProfit, currency),
		"已支付订单: " + strconv.FormatInt(view.PaidOrders, 10),
		"已完成订单: " + strconv.FormatInt(view.CompletedOrders, 10),
		"支付成功率: " + fallback(view.PaymentSuccessRate),
		"用户余额总额: " + renderMoney(view.TotalUserBalance, currency),
	}

	return strings.Join(lines, "\n")
}

func renderMoney(amount, currency string) string {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		amount = "0"
	}
	return amount + " " + fallback(currency)
}

func fallback(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}
