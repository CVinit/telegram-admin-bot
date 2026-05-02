package render

import (
	"strconv"
	"strings"

	"github.com/CVinit/telegram-admin-bot/internal/workflow"
)

func RenderSingleFulfillmentPreview(view *workflow.SingleFulfillmentPreviewView) string {
	if view == nil {
		return "发货预览不可用。"
	}

	lines := []string{
		"单个发货预览",
		"订单号: " + view.OrderNo,
		"Order ID: " + strconv.FormatUint(uint64(view.OrderID), 10),
		"交付模式: " + view.DeliveryKind,
		"内容预览: " + fallback(view.PayloadPreview),
	}
	lines = append(lines, renderNotificationHintLines(view.NotificationHint)...)
	return strings.Join(lines, "\n")
}

func RenderSingleFulfillmentResult(view *workflow.SingleFulfillmentResultView) string {
	if view == nil {
		return "发货结果不可用。"
	}
	lines := []string{
		"单个发货已完成",
		"订单号: " + view.OrderNo,
		"Order ID: " + strconv.FormatUint(uint64(view.OrderID), 10),
		"Fulfillment ID: " + strconv.FormatUint(uint64(view.FulfillmentID), 10),
	}
	lines = append(lines, renderNotificationHintLines(view.NotificationHint)...)
	return strings.Join(lines, "\n")
}

func RenderBatchFulfillmentPreview(view *workflow.BatchFulfillmentPreviewView) string {
	if view == nil {
		return "批量发货预览不可用。"
	}
	lines := []string{
		"批量发货预览",
		"订单数量: " + strconv.Itoa(view.OrderCount),
		"发货数量: " + strconv.Itoa(view.TotalQuantity),
		"交付模式: " + view.DeliveryKind,
		"内容预览: " + fallback(view.PayloadPreview),
	}
	if view.ProductID > 0 {
		lines = append(lines, "Product ID: "+strconv.FormatUint(uint64(view.ProductID), 10))
	}
	if view.SKUID > 0 {
		lines = append(lines, "SKU ID: "+strconv.FormatUint(uint64(view.SKUID), 10))
	}
	if view.SecretCount > 0 {
		lines = append(lines, "卡密数量: "+strconv.Itoa(view.SecretCount))
	}
	if len(view.OrderNos) > 0 {
		lines = append(lines, "订单列表: "+strings.Join(view.OrderNos, ", "))
	}
	return strings.Join(lines, "\n")
}

func RenderPendingFulfillmentList(view *workflow.PendingFulfillmentListView) string {
	if view == nil {
		return "待发货订单不可用。"
	}
	lines := []string{
		"待发货订单",
		"订单数量: " + strconv.Itoa(view.OrderCount),
		"发货数量: " + strconv.Itoa(view.TotalQuantity),
	}
	if len(view.Statuses) > 0 {
		lines = append(lines, "状态: "+strings.Join(view.Statuses, ", "))
	}
	if view.ProductID > 0 {
		lines = append(lines, "Product ID: "+strconv.FormatUint(uint64(view.ProductID), 10))
	}
	if view.SKUID > 0 {
		lines = append(lines, "SKU ID: "+strconv.FormatUint(uint64(view.SKUID), 10))
	}
	if len(view.Items) == 0 {
		lines = append(lines, "暂无符合条件的订单。")
		return strings.Join(lines, "\n")
	}
	for _, item := range view.Items {
		line := item.OrderNo + " | " + item.Status + " | qty=" + strconv.Itoa(item.Quantity)
		if item.ProductID > 0 {
			line += " | product_id=" + strconv.FormatUint(uint64(item.ProductID), 10)
		}
		if item.SKUID > 0 {
			line += " | sku_id=" + strconv.FormatUint(uint64(item.SKUID), 10)
		}
		if strings.TrimSpace(item.PaidAt) != "" {
			line += " | paid_at=" + strings.TrimSpace(item.PaidAt)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func RenderBatchFulfillmentResult(view *workflow.BatchFulfillmentResultView) string {
	if view == nil {
		return "批量发货结果不可用。"
	}
	lines := []string{
		"批量发货已完成",
		"总数: " + strconv.Itoa(view.TotalCount),
		"成功: " + strconv.Itoa(view.SuccessCount),
		"失败: " + strconv.Itoa(view.FailedCount),
	}
	for _, item := range view.Items {
		line := item.OrderNo + " => " + item.Status
		if item.Error != "" {
			line += " (" + item.Error + ")"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func renderNotificationHintLines(hint *workflow.NotificationHint) []string {
	if hint == nil {
		return nil
	}
	return []string{
		"邮件通知: " + hint.Email.Status + renderHintReason(hint.Email.Reason),
		"Telegram 通知: " + hint.Telegram.Status + renderHintReason(hint.Telegram.Reason),
		"提示: 这里只反映预期触发状态，不代表最终送达。",
	}
}

func renderHintReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	return " [" + reason + "]"
}
