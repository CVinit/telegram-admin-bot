package workflow

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/dujiao"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
)

const FulfillmentBatchConfirmPrefix = "fulfillment_batch_confirm"

type BatchFulfillmentFilter struct {
	Status         string
	CreatedFrom    string
	CreatedTo      string
	ProductKeyword string
	Limit          int
}

type BatchFulfillmentPreviewView struct {
	ActionKey      string
	ExpiresAt      string
	OrderCount     int
	OrderNos       []string
	DeliveryKind   string
	PayloadPreview string
}

type BatchFulfillmentItemResult struct {
	OrderID          uint
	OrderNo          string
	Status           string
	Error            string
	NotificationHint *NotificationHint
}

type BatchFulfillmentResultView struct {
	TotalCount   int
	SuccessCount int
	FailedCount  int
	Items        []BatchFulfillmentItemResult
}

type batchFulfillmentPayload struct {
	OrderIDs     []uint                 `json:"order_ids"`
	OrderNos     []string               `json:"order_nos"`
	Payload      string                 `json:"payload,omitempty"`
	DeliveryData map[string]interface{} `json:"delivery_data,omitempty"`
}

func (w *FulfillmentWorkflow) BuildBatchPreview(ctx context.Context, sessionView *session.SessionView, filter BatchFulfillmentFilter, rawDelivery string) (*BatchFulfillmentPreviewView, error) {
	if w == nil || w.api == nil {
		return nil, errNilFulfillmentAPI
	}
	if w.confirmations == nil {
		return nil, errors.New("confirmation service is nil")
	}
	if sessionView == nil {
		return nil, errSessionRequired
	}

	orderIDs, orderNos, err := w.collectEligibleManualOrders(ctx, sessionView, filter)
	if err != nil {
		return nil, err
	}
	payloadText, deliveryData, deliveryKind, err := parseDeliveryInput(rawDelivery)
	if err != nil {
		return nil, err
	}

	record, err := w.confirmations.Create(ctx, sessionView.TelegramUser, FulfillmentBatchConfirmPrefix, batchFulfillmentPayload{
		OrderIDs:     orderIDs,
		OrderNos:     orderNos,
		Payload:      payloadText,
		DeliveryData: deliveryData,
	})
	if err != nil {
		return nil, err
	}

	return &BatchFulfillmentPreviewView{
		ActionKey:      record.ActionKey,
		ExpiresAt:      record.ExpiresAt.UTC().Format(timeRFC3339),
		OrderCount:     len(orderNos),
		OrderNos:       orderNos,
		DeliveryKind:   deliveryKind,
		PayloadPreview: previewDelivery(payloadText, deliveryData),
	}, nil
}

func (w *FulfillmentWorkflow) ConfirmBatch(ctx context.Context, sessionView *session.SessionView, actionKey string) (*BatchFulfillmentResultView, error) {
	if w == nil || w.api == nil {
		return nil, errNilFulfillmentAPI
	}
	if w.confirmations == nil {
		return nil, errors.New("confirmation service is nil")
	}
	if sessionView == nil {
		return nil, errSessionRequired
	}

	var payload batchFulfillmentPayload
	action, err := w.confirmations.Consume(ctx, sessionView.TelegramUser, actionKey, &payload)
	if err != nil {
		return nil, err
	}

	if w.audit != nil {
		_ = w.audit.LogActionStarted(ctx, sessionView.TelegramUser, sessionView.AdminID, "fulfillment_batch_confirm", "orders:"+strconv.Itoa(len(payload.OrderIDs)), map[string]any{
			"action_key": action.Action,
		})
	}

	result := &BatchFulfillmentResultView{
		TotalCount: len(payload.OrderIDs),
		Items:      make([]BatchFulfillmentItemResult, 0, len(payload.OrderIDs)),
	}
	for idx, orderID := range payload.OrderIDs {
		item := BatchFulfillmentItemResult{
			OrderID: orderID,
		}
		if idx < len(payload.OrderNos) {
			item.OrderNo = payload.OrderNos[idx]
		}

		resp, err := w.api.CreateFulfillment(ctx, strings.TrimSpace(sessionView.JWTToken), dujiao.CreateFulfillmentRequest{
			OrderID:      orderID,
			Payload:      payload.Payload,
			DeliveryData: payload.DeliveryData,
		})
		if err != nil {
			item.Status = "failed"
			item.Error = err.Error()
			result.FailedCount++
			result.Items = append(result.Items, item)
			continue
		}

		order, _ := w.api.GetOrder(ctx, strings.TrimSpace(sessionView.JWTToken), orderID)
		item.NotificationHint, _ = w.BuildNotificationHint(ctx, sessionView, order)
		item.Status = "delivered"
		if resp != nil && item.OrderNo == "" {
			item.OrderNo = payload.OrderNos[idx]
		}
		result.SuccessCount++
		result.Items = append(result.Items, item)
	}

	if w.audit != nil {
		_ = w.audit.LogActionSucceeded(ctx, sessionView.TelegramUser, sessionView.AdminID, "fulfillment_batch_confirm", "orders:"+strconv.Itoa(len(payload.OrderIDs)), fmt.Sprintf("success=%d failed=%d", result.SuccessCount, result.FailedCount), map[string]any{
			"action_key": action.Action,
		})
	}
	return result, nil
}

func (w *FulfillmentWorkflow) collectEligibleManualOrders(ctx context.Context, sessionView *session.SessionView, filter BatchFulfillmentFilter) ([]uint, []string, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	listResp, err := w.api.ListOrders(ctx, strings.TrimSpace(sessionView.JWTToken), dujiao.ListOrdersParams{
		Status:         strings.TrimSpace(filter.Status),
		CreatedFrom:    strings.TrimSpace(filter.CreatedFrom),
		CreatedTo:      strings.TrimSpace(filter.CreatedTo),
		ProductKeyword: strings.TrimSpace(filter.ProductKeyword),
		Page:           1,
		PageSize:       limit,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list orders for batch fulfillment: %w", err)
	}
	if listResp == nil || len(listResp.Items) == 0 {
		return nil, nil, fmt.Errorf("no orders matched the batch filter")
	}

	orderIDs := make([]uint, 0, limit)
	orderNos := make([]string, 0, limit)
	for _, item := range listResp.Items {
		order, err := w.api.GetOrder(ctx, strings.TrimSpace(sessionView.JWTToken), item.ID)
		if err != nil || order == nil {
			continue
		}
		if validateManualFulfillmentOrder(order) != nil {
			continue
		}
		orderIDs = append(orderIDs, order.ID)
		orderNos = append(orderNos, order.OrderNo)
	}
	if len(orderIDs) == 0 {
		return nil, nil, fmt.Errorf("no eligible manual orders matched the batch filter")
	}
	return orderIDs, orderNos, nil
}
