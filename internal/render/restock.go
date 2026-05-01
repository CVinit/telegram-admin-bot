package render

import (
	"strconv"
	"strings"
	"time"

	"github.com/CVinit/telegram-admin-bot/internal/workflow"
)

func RenderRestockPreview(view *workflow.RestockPreviewView) string {
	if view == nil {
		return "补库存预览不可用。"
	}

	lines := []string{
		"补库存预览",
		"商品: " + view.ProductName,
		"Product ID: " + strconv.FormatUint(uint64(view.ProductID), 10),
		"SKU: " + fallback(view.SKULabel),
		"导入条数: " + strconv.Itoa(view.SecretCount),
		"批次号: " + fallback(view.BatchNo),
		"来源: " + fallback(view.Source),
		"确认有效期至: " + view.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if len(view.Samples) > 0 {
		lines = append(lines, "样例: "+strings.Join(view.Samples, " | "))
	}
	return strings.Join(lines, "\n")
}

func RenderRestockResult(view *workflow.RestockResultView) string {
	if view == nil {
		return "补库存结果不可用。"
	}

	lines := []string{
		"补库存已完成",
		"商品: " + view.ProductName,
		"Product ID: " + strconv.FormatUint(uint64(view.ProductID), 10),
		"SKU: " + fallback(view.SKULabel),
		"批次号: " + fallback(view.BatchNo),
		"新增数量: " + strconv.Itoa(view.Created),
		"提交数量: " + strconv.Itoa(view.SecretCount),
	}
	return strings.Join(lines, "\n")
}
