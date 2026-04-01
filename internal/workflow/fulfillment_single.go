package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/dujiao"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
)

const FulfillmentConfirmPrefix = "fulfillment_confirm"

var (
	errNilFulfillmentAPI = errors.New("fulfillment workflow api is nil")
	errSessionRequired   = errors.New("session is required")
	errOrderRequired     = errors.New("order is required")
)

type SingleFulfillmentPreviewView struct {
	ActionKey        string
	ExpiresAt        string
	OrderID          uint
	OrderNo          string
	DeliveryKind     string
	PayloadPreview   string
	NotificationHint *NotificationHint
}

type SingleFulfillmentResultView struct {
	OrderID          uint
	OrderNo          string
	FulfillmentID    uint
	NotificationHint *NotificationHint
}

type singleFulfillmentPayload struct {
	OrderID      uint                   `json:"order_id"`
	OrderNo      string                 `json:"order_no"`
	Payload      string                 `json:"payload,omitempty"`
	DeliveryData map[string]interface{} `json:"delivery_data,omitempty"`
}

func (w *FulfillmentWorkflow) BuildSinglePreview(ctx context.Context, sessionView *session.SessionView, orderNo string, rawDelivery string) (*SingleFulfillmentPreviewView, error) {
	if w == nil || w.api == nil {
		return nil, errNilFulfillmentAPI
	}
	if w.confirmations == nil {
		return nil, errors.New("confirmation service is nil")
	}
	if sessionView == nil {
		return nil, errSessionRequired
	}

	order, err := w.findOrderByNo(ctx, sessionView, orderNo)
	if err != nil {
		return nil, err
	}
	if err := validateManualFulfillmentOrder(order); err != nil {
		return nil, err
	}

	payloadText, deliveryData, deliveryKind, err := parseDeliveryInput(rawDelivery)
	if err != nil {
		return nil, err
	}

	hint, err := w.BuildNotificationHint(ctx, sessionView, order)
	if err != nil {
		return nil, err
	}

	record, err := w.confirmations.Create(ctx, sessionView.TelegramUser, FulfillmentConfirmPrefix, singleFulfillmentPayload{
		OrderID:      order.ID,
		OrderNo:      order.OrderNo,
		Payload:      payloadText,
		DeliveryData: deliveryData,
	})
	if err != nil {
		return nil, err
	}

	return &SingleFulfillmentPreviewView{
		ActionKey:        record.ActionKey,
		ExpiresAt:        record.ExpiresAt.UTC().Format(timeRFC3339),
		OrderID:          order.ID,
		OrderNo:          order.OrderNo,
		DeliveryKind:     deliveryKind,
		PayloadPreview:   previewDelivery(payloadText, deliveryData),
		NotificationHint: hint,
	}, nil
}

func (w *FulfillmentWorkflow) ConfirmSingle(ctx context.Context, sessionView *session.SessionView, actionKey string) (*SingleFulfillmentResultView, error) {
	if w == nil || w.api == nil {
		return nil, errNilFulfillmentAPI
	}
	if w.confirmations == nil {
		return nil, errors.New("confirmation service is nil")
	}
	if sessionView == nil {
		return nil, errSessionRequired
	}

	var payload singleFulfillmentPayload
	action, err := w.confirmations.Consume(ctx, sessionView.TelegramUser, actionKey, &payload)
	if err != nil {
		return nil, err
	}

	target := fmt.Sprintf("order:%d", payload.OrderID)
	if w.audit != nil {
		_ = w.audit.LogActionStarted(ctx, sessionView.TelegramUser, sessionView.AdminID, "fulfillment_confirm", target, map[string]any{
			"action_key": action.Action,
			"order_no":   payload.OrderNo,
		})
	}

	resp, err := w.api.CreateFulfillment(ctx, strings.TrimSpace(sessionView.JWTToken), dujiao.CreateFulfillmentRequest{
		OrderID:      payload.OrderID,
		Payload:      payload.Payload,
		DeliveryData: payload.DeliveryData,
	})
	if err != nil {
		if w.audit != nil {
			_ = w.audit.LogActionFailed(ctx, sessionView.TelegramUser, sessionView.AdminID, "fulfillment_confirm", target, err.Error(), map[string]any{
				"action_key": action.Action,
			})
		}
		return nil, fmt.Errorf("create fulfillment: %w", err)
	}

	order, _ := w.api.GetOrder(ctx, strings.TrimSpace(sessionView.JWTToken), payload.OrderID)
	hint, _ := w.BuildNotificationHint(ctx, sessionView, order)

	result := &SingleFulfillmentResultView{
		OrderID:          payload.OrderID,
		OrderNo:          payload.OrderNo,
		NotificationHint: hint,
	}
	if resp != nil {
		result.FulfillmentID = resp.ID
	}

	if w.audit != nil {
		_ = w.audit.LogActionSucceeded(ctx, sessionView.TelegramUser, sessionView.AdminID, "fulfillment_confirm", target, "order_no="+payload.OrderNo, map[string]any{
			"action_key":     action.Action,
			"fulfillment_id": result.FulfillmentID,
		})
	}
	return result, nil
}

func (w *FulfillmentWorkflow) findOrderByNo(ctx context.Context, sessionView *session.SessionView, orderNo string) (*dujiao.OrderDetail, error) {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, errors.New("order number is required")
	}

	listResp, err := w.api.ListOrders(ctx, strings.TrimSpace(sessionView.JWTToken), dujiao.ListOrdersParams{
		OrderNo:  orderNo,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	if listResp == nil || len(listResp.Items) == 0 {
		return nil, fmt.Errorf("order %s not found", orderNo)
	}

	var targetID uint
	for _, item := range listResp.Items {
		if strings.TrimSpace(item.OrderNo) == orderNo {
			targetID = item.ID
			break
		}
	}
	if targetID == 0 {
		targetID = listResp.Items[0].ID
	}

	order, err := w.api.GetOrder(ctx, strings.TrimSpace(sessionView.JWTToken), targetID)
	if err != nil {
		return nil, fmt.Errorf("get order detail: %w", err)
	}
	if order == nil {
		return nil, fmt.Errorf("order %s detail missing", orderNo)
	}
	return order, nil
}

func validateManualFulfillmentOrder(order *dujiao.OrderDetail) error {
	if order == nil {
		return errOrderRequired
	}
	if order.Fulfillment != nil {
		return fmt.Errorf("order %s already fulfilled", order.OrderNo)
	}
	switch strings.TrimSpace(order.Status) {
	case "paid", "fulfilling":
	default:
		return fmt.Errorf("order %s status %s is not fulfillable", order.OrderNo, order.Status)
	}
	if len(order.Children) > 0 {
		return fmt.Errorf("order %s is a parent order and cannot be fulfilled directly", order.OrderNo)
	}
	if len(order.Items) == 0 {
		return fmt.Errorf("order %s has no order items", order.OrderNo)
	}
	for _, item := range order.Items {
		if strings.TrimSpace(item.FulfillmentType) != "manual" {
			return fmt.Errorf("order %s contains non-manual items", order.OrderNo)
		}
	}
	return nil
}

func parseDeliveryInput(raw string) (string, map[string]interface{}, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, "", errors.New("delivery content is required")
	}

	lines := strings.Split(raw, "\n")
	deliveryData := make(map[string]interface{})
	keyValueMode := true
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			keyValueMode = false
			break
		}
		deliveryData[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if keyValueMode && len(deliveryData) > 0 {
		return "", deliveryData, "structured", nil
	}
	return raw, nil, "payload", nil
}

func previewDelivery(payload string, deliveryData map[string]interface{}) string {
	if strings.TrimSpace(payload) != "" {
		payload = strings.TrimSpace(payload)
		if len(payload) > 120 {
			return payload[:120] + "..."
		}
		return payload
	}
	if len(deliveryData) == 0 {
		return ""
	}
	keys := make([]string, 0, len(deliveryData))
	for key := range deliveryData {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+fmt.Sprint(deliveryData[key]))
	}
	return strings.Join(parts, " | ")
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
