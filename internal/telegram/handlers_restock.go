package telegram

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/render"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/workflow"
)

const restockUsageText = "用法:\n/restock <product_id> [sku_id]\n下一行开始粘贴卡密，或上传 csv/txt 并把命令写在 caption。"

type restockWorkflow interface {
	BuildPreviewFromText(ctx context.Context, sessionView *session.SessionView, productID, skuID uint, rawText string) (*workflow.RestockPreviewView, error)
	BuildPreviewFromFile(ctx context.Context, sessionView *session.SessionView, productID, skuID uint, fileName string, content []byte) (*workflow.RestockPreviewView, error)
	Confirm(ctx context.Context, sessionView *session.SessionView, actionKey string) (*workflow.RestockResultView, error)
}

func (r *Router) WithRestockWorkflow(restock restockWorkflow) *Router {
	r.restock = restock
	return r
}

func (r *Router) handleRestock(ctx context.Context, update IncomingUpdate, args []string) (*Response, error) {
	if r.restock == nil {
		return r.handlePlaceholder(ctx, update, "补自动库存")
	}

	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	productID, skuID, err := parseRestockArgs(args)
	if err != nil {
		return &Response{Text: restockUsageText}, nil
	}

	var preview *workflow.RestockPreviewView
	if len(update.DocumentData) > 0 {
		preview, err = r.restock.BuildPreviewFromFile(ctx, view, productID, skuID, update.DocumentName, update.DocumentData)
	} else {
		body := extractMessageBody(update.Text)
		if strings.TrimSpace(body) == "" {
			return &Response{Text: restockUsageText}, nil
		}
		preview, err = r.restock.BuildPreviewFromText(ctx, view, productID, skuID, body)
	}
	if err != nil {
		return &Response{Text: "生成补库存预览失败: " + err.Error()}, nil
	}

	return &Response{
		Text:         render.RenderRestockPreview(preview),
		KeyboardKind: KeyboardKindInline,
		Keyboard: [][]Button{
			{{Text: "确认导入", CallbackData: preview.ActionKey}},
		},
	}, nil
}

func (r *Router) handleRestockConfirm(ctx context.Context, update IncomingUpdate) (*Response, error) {
	if r.restock == nil {
		return &Response{Text: "补自动库存功能尚未接入。"}, nil
	}

	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	result, err := r.restock.Confirm(ctx, view, strings.TrimSpace(update.CallbackData))
	if err != nil {
		if errors.Is(err, workflow.ErrConfirmationNotFound) {
			return &Response{
				Text:         "确认已失效或已被消费，请重新生成补库存预览。",
				CallbackID:   update.CallbackID,
				CallbackText: "确认已失效",
			}, nil
		}
		return &Response{
			Text:         "执行补库存失败: " + err.Error(),
			CallbackID:   update.CallbackID,
			CallbackText: "执行失败",
		}, nil
	}

	return &Response{
		Text:         render.RenderRestockResult(result),
		CallbackID:   update.CallbackID,
		CallbackText: "补库存已执行",
	}, nil
}

func parseRestockArgs(args []string) (uint, uint, error) {
	if len(args) == 0 {
		return 0, 0, errors.New("missing product ID")
	}

	productID64, err := strconv.ParseUint(strings.TrimSpace(args[0]), 10, 64)
	if err != nil || productID64 == 0 {
		return 0, 0, errors.New("invalid product ID")
	}

	var skuID uint64
	if len(args) > 1 {
		skuID, err = strconv.ParseUint(strings.TrimSpace(args[1]), 10, 64)
		if err != nil {
			return 0, 0, errors.New("invalid sku ID")
		}
	}

	return uint(productID64), uint(skuID), nil
}

func extractMessageBody(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	idx := strings.IndexByte(text, '\n')
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(text[idx+1:])
}
