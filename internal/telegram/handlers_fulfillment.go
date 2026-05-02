package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/CVinit/telegram-admin-bot/internal/render"
	"github.com/CVinit/telegram-admin-bot/internal/session"
	"github.com/CVinit/telegram-admin-bot/internal/workflow"
)

const (
	shipUsageText        = "用法:\n/ship <order_no>\n下一行开始填写发货内容，若每行是 key=value 则会转成结构化 delivery_data。"
	batchShipUsageText   = "用法:\n/batch_ship [status=paid|fulfilling|pending] [product_id=<id>] [sku_id=<id>] [product=<keyword>] [from=<date>] [to=<date>] [limit=<n>]\n下一行开始填写发货内容。指定 product_id 后，每行一条卡密，Bot 会按付款顺序和订单数量自动分配。"
	pendingShipUsageText = "用法:\n/pending_ship [status=paid|fulfilling|pending] [product_id=<id>] [sku_id=<id>] [product=<keyword>] [from=<date>] [to=<date>] [limit=<n>]"
)

type fulfillmentWorkflow interface {
	BuildSinglePreview(ctx context.Context, sessionView *session.SessionView, orderNo string, rawDelivery string) (*workflow.SingleFulfillmentPreviewView, error)
	ConfirmSingle(ctx context.Context, sessionView *session.SessionView, actionKey string) (*workflow.SingleFulfillmentResultView, error)
	BuildPendingList(ctx context.Context, sessionView *session.SessionView, filter workflow.BatchFulfillmentFilter) (*workflow.PendingFulfillmentListView, error)
	BuildBatchPreview(ctx context.Context, sessionView *session.SessionView, filter workflow.BatchFulfillmentFilter, rawDelivery string) (*workflow.BatchFulfillmentPreviewView, error)
	ConfirmBatch(ctx context.Context, sessionView *session.SessionView, actionKey string) (*workflow.BatchFulfillmentResultView, error)
	ListProductsForFulfillment(ctx context.Context, sessionView *session.SessionView, filter workflow.BatchFulfillmentFilter) (*workflow.ProductFulfillmentListView, error)
	BuildProductShipPreview(ctx context.Context, sessionView *session.SessionView, productID uint, skuID uint, rawDelivery string) (*workflow.ProductShipPreviewView, error)
	ConfirmProductShip(ctx context.Context, sessionView *session.SessionView, actionKey string) (*workflow.BatchFulfillmentResultView, error)
}

func (r *Router) WithFulfillmentWorkflow(flow fulfillmentWorkflow) *Router {
	r.fulfillment = flow
	return r
}

func (r *Router) handleShip(ctx context.Context, update IncomingUpdate, args []string) (*Response, error) {
	if r.fulfillment == nil {
		return r.handlePlaceholder(ctx, update, "单个发货")
	}

	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return &Response{Text: shipUsageText}, nil
	}
	body := extractMessageBody(update.Text)
	if strings.TrimSpace(body) == "" {
		return &Response{Text: shipUsageText}, nil
	}

	preview, err := r.fulfillment.BuildSinglePreview(ctx, view, args[0], body)
	if err != nil {
		return &Response{Text: "生成单个发货预览失败: " + err.Error()}, nil
	}
	return &Response{
		Text:         render.RenderSingleFulfillmentPreview(preview),
		KeyboardKind: KeyboardKindInline,
		Keyboard: [][]Button{
			{{Text: "确认发货", CallbackData: preview.ActionKey}},
		},
	}, nil
}

func (r *Router) handlePendingShip(ctx context.Context, update IncomingUpdate, args []string) (*Response, error) {
	if r.fulfillment == nil {
		return r.handlePlaceholder(ctx, update, "待发货订单")
	}

	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	filter, err := parseBatchShipFilter(args)
	if err != nil {
		return &Response{Text: pendingShipUsageText}, nil
	}
	list, err := r.fulfillment.BuildPendingList(ctx, view, filter)
	if err != nil {
		return &Response{Text: "查询待发货订单失败: " + err.Error()}, nil
	}
	return &Response{Text: render.RenderPendingFulfillmentList(list)}, nil
}

func (r *Router) handleBatchShip(ctx context.Context, update IncomingUpdate, args []string) (*Response, error) {
	if r.fulfillment == nil {
		return r.handlePlaceholder(ctx, update, "批量发货")
	}

	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}
	body := extractMessageBody(update.Text)
	if strings.TrimSpace(body) == "" {
		return &Response{Text: batchShipUsageText}, nil
	}

	filter, err := parseBatchShipFilter(args)
	if err != nil {
		return &Response{Text: batchShipUsageText}, nil
	}
	preview, err := r.fulfillment.BuildBatchPreview(ctx, view, filter, body)
	if err != nil {
		return &Response{Text: "生成批量发货预览失败: " + err.Error()}, nil
	}
	return &Response{
		Text:         render.RenderBatchFulfillmentPreview(preview),
		KeyboardKind: KeyboardKindInline,
		Keyboard: [][]Button{
			{{Text: "确认批量发货", CallbackData: preview.ActionKey}},
		},
	}, nil
}

func (r *Router) handleFulfillmentConfirm(ctx context.Context, update IncomingUpdate) (*Response, error) {
	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	result, err := r.fulfillment.ConfirmSingle(ctx, view, strings.TrimSpace(update.CallbackData))
	if err != nil {
		return &Response{
			Text:         "执行单个发货失败: " + err.Error(),
			CallbackID:   update.CallbackID,
			CallbackText: "执行失败",
		}, nil
	}
	return &Response{
		Text:         render.RenderSingleFulfillmentResult(result),
		CallbackID:   update.CallbackID,
		CallbackText: "发货已执行",
	}, nil
}

func (r *Router) handleBatchFulfillmentConfirm(ctx context.Context, update IncomingUpdate) (*Response, error) {
	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	result, err := r.fulfillment.ConfirmBatch(ctx, view, strings.TrimSpace(update.CallbackData))
	if err != nil {
		return &Response{
			Text:         "执行批量发货失败: " + err.Error(),
			CallbackID:   update.CallbackID,
			CallbackText: "执行失败",
		}, nil
	}
	return &Response{
		Text:         render.RenderBatchFulfillmentResult(result),
		CallbackID:   update.CallbackID,
		CallbackText: "批量发货已执行",
	}, nil
}

func (r *Router) handleShipByProduct(ctx context.Context, update IncomingUpdate, args []string) (*Response, error) {
	if r.fulfillment == nil {
		return r.handlePlaceholder(ctx, update, "按商品发货")
	}

	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	filter := workflow.BatchFulfillmentFilter{
		Status: "paid,fulfilling",
		Limit:  200,
	}

	list, err := r.fulfillment.ListProductsForFulfillment(ctx, view, filter)
	if err != nil {
		return &Response{Text: "查询待发货商品失败: " + err.Error()}, nil
	}
	if len(list.Items) == 0 {
		return &Response{Text: "暂无待发货商品。"}, nil
	}

	lines := []string{"待发货商品清单", ""}
	keyboard := make([][]Button, 0, len(list.Items))
	row := []Button{}
	for i, item := range list.Items {
		productName := item.ProductName
		if item.SKUID > 0 {
			productName += fmt.Sprintf(" (SKU:%d)", item.SKUID)
		}
		lines = append(lines, fmt.Sprintf("%s\n  订单数:%d | 总数:%d | ID:%d-%d",
			productName, item.OrderCount, item.TotalQty, item.ProductID, item.SKUID))
		cbData := fmt.Sprintf("%s:prod_%d_%d", workflow.ProductListConfirmPrefix, item.ProductID, item.SKUID)
		row = append(row, Button{Text: productName, CallbackData: cbData})
		if len(row) == 2 || i == len(list.Items)-1 {
			if len(row) == 1 {
				row = append(row, Button{Text: "", CallbackData: " "})
			}
			keyboard = append(keyboard, row)
			row = nil
		}
	}
	lines = append(lines, "", "点击商品名称，然后发送卡密（每行一个）")
	return &Response{
		Text:         strings.Join(lines, "\n"),
		KeyboardKind: KeyboardKindInline,
		Keyboard:     keyboard,
	}, nil
}

func (r *Router) handleProductListSelect(ctx context.Context, update IncomingUpdate) (*Response, error) {
	if r.fulfillment == nil {
		return r.handlePlaceholder(ctx, update, "按商品发货")
	}

	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	cbData := strings.TrimSpace(update.CallbackData)
	if !strings.HasPrefix(cbData, workflow.ProductListConfirmPrefix+":prod_") {
		return &Response{Text: "未知的商品选择", CallbackID: update.CallbackID, CallbackText: "未识别"}, nil
	}
	parts := strings.Split(cbData, "_")
	if len(parts) < 4 {
		return &Response{Text: "无效的商品选择", CallbackID: update.CallbackID, CallbackText: "无效"}, nil
	}
	productID, _ := strconv.ParseUint(parts[2], 10, 64)
	skuID, _ := strconv.ParseUint(parts[3], 10, 64)

	productName := fmt.Sprintf("商品%d", productID)
	if prod, err := r.fulfillment.BuildProductShipPreview(ctx, view, uint(productID), uint(skuID), ""); err == nil {
		productName = prod.ProductName
	}

	r.pendingProductShip[update.TelegramUser] = pendingProductShipState{
		ProductID:   uint(productID),
		SKUID:       uint(skuID),
		ProductName: productName,
	}

	return &Response{
		Text:         fmt.Sprintf("已选择: %s\n请发送卡密（每行一个），Bot 将自动按订单及数量分配", productName),
		CallbackID:   update.CallbackID,
		CallbackText: fmt.Sprintf("已选:%s", productName),
	}, nil
}

func (r *Router) handleProductShipWithSecrets(ctx context.Context, update IncomingUpdate, state pendingProductShipState) (*Response, error) {
	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	secrets := extractMessageBody(update.Text)
	if strings.TrimSpace(secrets) == "" {
		return &Response{Text: "请发送卡密（每行一个）"}, nil
	}

	preview, err := r.fulfillment.BuildProductShipPreview(ctx, view, state.ProductID, state.SKUID, secrets)
	if err != nil {
		return &Response{Text: "生成发货预览失败: " + err.Error()}, nil
	}

	return &Response{
		Text:         render.RenderProductShipPreview(preview),
		KeyboardKind: KeyboardKindInline,
		Keyboard: [][]Button{
			{{Text: "确认发货", CallbackData: preview.ActionKey}},
		},
	}, nil
}

func (r *Router) handleProductShipConfirm(ctx context.Context, update IncomingUpdate) (*Response, error) {
	view, err := r.sessions.RequireSession(ctx, update.TelegramUser)
	if err != nil {
		return r.renderSessionRequired(err), nil
	}

	result, err := r.fulfillment.ConfirmProductShip(ctx, view, strings.TrimSpace(update.CallbackData))
	if err != nil {
		return &Response{
			Text:         "执行发货失败: " + err.Error(),
			CallbackID:   update.CallbackID,
			CallbackText: "执行失败",
		}, nil
	}
	return &Response{
		Text:         render.RenderBatchFulfillmentResult(result),
		CallbackID:   update.CallbackID,
		CallbackText: "发货已完成",
	}, nil
}

func parseBatchShipFilter(args []string) (workflow.BatchFulfillmentFilter, error) {
	filter := workflow.BatchFulfillmentFilter{
		Status: "pending",
		Limit:  20,
	}
	for _, arg := range args {
		key, value, ok := strings.Cut(strings.TrimSpace(arg), "=")
		if !ok {
			return workflow.BatchFulfillmentFilter{}, strconv.ErrSyntax
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "status":
			filter.Status = value
		case "product":
			filter.ProductKeyword = value
		case "product_id":
			parsed, err := strconv.ParseUint(value, 10, 64)
			if err != nil || parsed == 0 {
				return workflow.BatchFulfillmentFilter{}, strconv.ErrSyntax
			}
			filter.ProductID = uint(parsed)
		case "sku_id":
			parsed, err := strconv.ParseUint(value, 10, 64)
			if err != nil || parsed == 0 {
				return workflow.BatchFulfillmentFilter{}, strconv.ErrSyntax
			}
			filter.SKUID = uint(parsed)
		case "from":
			filter.CreatedFrom = value
		case "to":
			filter.CreatedTo = value
		case "limit":
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				return workflow.BatchFulfillmentFilter{}, strconv.ErrSyntax
			}
			filter.Limit = parsed
		default:
			return workflow.BatchFulfillmentFilter{}, strconv.ErrSyntax
		}
	}
	if filter.SKUID > 0 && filter.ProductID == 0 {
		return workflow.BatchFulfillmentFilter{}, strconv.ErrSyntax
	}
	return filter, nil
}
