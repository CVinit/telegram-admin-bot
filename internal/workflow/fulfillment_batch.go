package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/CVinit/telegram-admin-bot/internal/dujiao"
	"github.com/CVinit/telegram-admin-bot/internal/session"
)

const FulfillmentBatchConfirmPrefix = "fulfillment_batch_confirm"

const defaultBatchFulfillmentLimit = 20

type BatchFulfillmentFilter struct {
	Status         string
	CreatedFrom    string
	CreatedTo      string
	ProductKeyword string
	ProductID      uint
	SKUID          uint
	Limit          int
}

type BatchFulfillmentPreviewView struct {
	ActionKey      string
	ExpiresAt      string
	OrderCount     int
	TotalQuantity  int
	ProductID      uint
	SKUID          uint
	SecretCount    int
	OrderNos       []string
	DeliveryKind   string
	PayloadPreview string
}

type PendingFulfillmentOrderView struct {
	OrderID   uint
	OrderNo   string
	Status    string
	ProductID uint
	SKUID     uint
	Quantity  int
	PaidAt    string
	CreatedAt string
}

type PendingFulfillmentListView struct {
	OrderCount    int
	TotalQuantity int
	ProductID     uint
	SKUID         uint
	Statuses      []string
	Items         []PendingFulfillmentOrderView
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

type batchFulfillmentAssignment struct {
	OrderID  uint     `json:"order_id"`
	OrderNo  string   `json:"order_no"`
	Quantity int      `json:"quantity"`
	Secrets  []string `json:"secrets,omitempty"`
	Payload  string   `json:"payload,omitempty"`
}

type batchFulfillmentPayload struct {
	OrderIDs     []uint                       `json:"order_ids"`
	OrderNos     []string                     `json:"order_nos"`
	Payload      string                       `json:"payload,omitempty"`
	DeliveryData map[string]interface{}       `json:"delivery_data,omitempty"`
	Assignments  []batchFulfillmentAssignment `json:"assignments,omitempty"`
}

type manualFulfillmentCandidate struct {
	OrderID   uint
	OrderNo   string
	Status    string
	ProductID uint
	SKUID     uint
	Quantity  int
	PaidAt    string
	CreatedAt string
}

func (w *FulfillmentWorkflow) BuildPendingList(ctx context.Context, sessionView *session.SessionView, filter BatchFulfillmentFilter) (*PendingFulfillmentListView, error) {
	if w == nil || w.api == nil {
		return nil, errNilFulfillmentAPI
	}
	if sessionView == nil {
		return nil, errSessionRequired
	}

	candidates, err := w.collectManualFulfillmentCandidates(ctx, sessionView, filter)
	if err != nil {
		return nil, err
	}

	view := &PendingFulfillmentListView{
		ProductID: filter.ProductID,
		SKUID:     filter.SKUID,
		Statuses:  fulfillmentStatuses(filter.Status),
		Items:     make([]PendingFulfillmentOrderView, 0, len(candidates)),
	}
	for _, candidate := range candidates {
		view.Items = append(view.Items, PendingFulfillmentOrderView{
			OrderID:   candidate.OrderID,
			OrderNo:   candidate.OrderNo,
			Status:    candidate.Status,
			ProductID: candidate.ProductID,
			SKUID:     candidate.SKUID,
			Quantity:  candidate.Quantity,
			PaidAt:    candidate.PaidAt,
			CreatedAt: candidate.CreatedAt,
		})
		view.TotalQuantity += candidate.Quantity
	}
	view.OrderCount = len(view.Items)
	return view, nil
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

	candidates, err := w.collectManualFulfillmentCandidates(ctx, sessionView, filter)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no eligible manual orders matched the batch filter")
	}

	if filter.ProductID > 0 {
		return w.buildProductBatchPreview(ctx, sessionView, filter, candidates, rawDelivery)
	}

	orderIDs, orderNos := candidateIDsAndNos(candidates)
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
		TotalQuantity:  totalCandidateQuantity(candidates),
		OrderNos:       orderNos,
		DeliveryKind:   deliveryKind,
		PayloadPreview: previewDelivery(payloadText, deliveryData),
	}, nil
}

func (w *FulfillmentWorkflow) buildProductBatchPreview(ctx context.Context, sessionView *session.SessionView, filter BatchFulfillmentFilter, candidates []manualFulfillmentCandidate, rawDelivery string) (*BatchFulfillmentPreviewView, error) {
	secrets, err := parseCardSecretLines(rawDelivery)
	if err != nil {
		return nil, err
	}
	totalQuantity := totalCandidateQuantity(candidates)
	if len(secrets) != totalQuantity {
		return nil, fmt.Errorf("card secret count mismatch: need %d, got %d", totalQuantity, len(secrets))
	}

	orderIDs := make([]uint, 0, len(candidates))
	orderNos := make([]string, 0, len(candidates))
	assignments := make([]batchFulfillmentAssignment, 0, len(candidates))
	cursor := 0
	for _, candidate := range candidates {
		end := cursor + candidate.Quantity
		assignedSecrets := append([]string(nil), secrets[cursor:end]...)
		assignments = append(assignments, batchFulfillmentAssignment{
			OrderID:  candidate.OrderID,
			OrderNo:  candidate.OrderNo,
			Quantity: candidate.Quantity,
			Secrets:  assignedSecrets,
			Payload:  strings.Join(assignedSecrets, "\n"),
		})
		orderIDs = append(orderIDs, candidate.OrderID)
		orderNos = append(orderNos, candidate.OrderNo)
		cursor = end
	}

	record, err := w.confirmations.Create(ctx, sessionView.TelegramUser, FulfillmentBatchConfirmPrefix, batchFulfillmentPayload{
		OrderIDs:    orderIDs,
		OrderNos:    orderNos,
		Assignments: assignments,
	})
	if err != nil {
		return nil, err
	}

	return &BatchFulfillmentPreviewView{
		ActionKey:      record.ActionKey,
		ExpiresAt:      record.ExpiresAt.UTC().Format(timeRFC3339),
		OrderCount:     len(orderNos),
		TotalQuantity:  totalQuantity,
		ProductID:      filter.ProductID,
		SKUID:          filter.SKUID,
		SecretCount:    len(secrets),
		OrderNos:       orderNos,
		DeliveryKind:   "card_secrets",
		PayloadPreview: fmt.Sprintf("%d orders / %d secrets", len(orderNos), len(secrets)),
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

	requests := payload.fulfillmentRequests()
	if w.audit != nil {
		_ = w.audit.LogActionStarted(ctx, sessionView.TelegramUser, sessionView.AdminID, "fulfillment_batch_confirm", "orders:"+strconv.Itoa(len(requests)), map[string]any{
			"action_key": action.Action,
		})
	}

	result := &BatchFulfillmentResultView{
		TotalCount: len(requests),
		Items:      make([]BatchFulfillmentItemResult, 0, len(requests)),
	}
	for _, request := range requests {
		item := BatchFulfillmentItemResult{
			OrderID: request.OrderID,
			OrderNo: request.OrderNo,
		}

		_, err := w.api.CreateFulfillment(ctx, strings.TrimSpace(sessionView.JWTToken), dujiao.CreateFulfillmentRequest{
			OrderID:      request.OrderID,
			Payload:      request.Payload,
			DeliveryData: request.DeliveryData,
		})
		if err != nil {
			item.Status = "failed"
			item.Error = err.Error()
			result.FailedCount++
			result.Items = append(result.Items, item)
			continue
		}

		order, _ := w.api.GetOrder(ctx, strings.TrimSpace(sessionView.JWTToken), request.OrderID)
		item.NotificationHint, _ = w.BuildNotificationHint(ctx, sessionView, order)
		item.Status = "delivered"
		if item.OrderNo == "" && order != nil {
			item.OrderNo = order.OrderNo
		}
		result.SuccessCount++
		result.Items = append(result.Items, item)
	}

	if w.audit != nil {
		_ = w.audit.LogActionSucceeded(ctx, sessionView.TelegramUser, sessionView.AdminID, "fulfillment_batch_confirm", "orders:"+strconv.Itoa(len(requests)), fmt.Sprintf("success=%d failed=%d", result.SuccessCount, result.FailedCount), map[string]any{
			"action_key": action.Action,
		})
	}
	return result, nil
}

type batchFulfillmentRequest struct {
	OrderID      uint
	OrderNo      string
	Payload      string
	DeliveryData map[string]interface{}
}

func (p batchFulfillmentPayload) fulfillmentRequests() []batchFulfillmentRequest {
	if len(p.Assignments) > 0 {
		requests := make([]batchFulfillmentRequest, 0, len(p.Assignments))
		for _, assignment := range p.Assignments {
			requests = append(requests, batchFulfillmentRequest{
				OrderID: assignment.OrderID,
				OrderNo: assignment.OrderNo,
				Payload: assignment.Payload,
			})
		}
		return requests
	}

	requests := make([]batchFulfillmentRequest, 0, len(p.OrderIDs))
	for idx, orderID := range p.OrderIDs {
		request := batchFulfillmentRequest{
			OrderID:      orderID,
			Payload:      p.Payload,
			DeliveryData: p.DeliveryData,
		}
		if idx < len(p.OrderNos) {
			request.OrderNo = p.OrderNos[idx]
		}
		requests = append(requests, request)
	}
	return requests
}

func (w *FulfillmentWorkflow) collectEligibleManualOrders(ctx context.Context, sessionView *session.SessionView, filter BatchFulfillmentFilter) ([]uint, []string, error) {
	candidates, err := w.collectManualFulfillmentCandidates(ctx, sessionView, filter)
	if err != nil {
		return nil, nil, err
	}
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("no eligible manual orders matched the batch filter")
	}
	orderIDs, orderNos := candidateIDsAndNos(candidates)
	return orderIDs, orderNos, nil
}

func (w *FulfillmentWorkflow) collectManualFulfillmentCandidates(ctx context.Context, sessionView *session.SessionView, filter BatchFulfillmentFilter) ([]manualFulfillmentCandidate, error) {
	if filter.ProductID == 0 && filter.SKUID > 0 {
		return nil, errors.New("sku_id requires product_id")
	}
	limit := normalizeBatchFulfillmentLimit(filter.Limit)
	statuses := fulfillmentStatuses(filter.Status)
	token := strings.TrimSpace(sessionView.JWTToken)
	seen := make(map[uint]struct{})
	candidates := make([]manualFulfillmentCandidate, 0, limit)

	for _, status := range statuses {
		listResp, err := w.api.ListOrders(ctx, token, dujiao.ListOrdersParams{
			Status:         status,
			CreatedFrom:    strings.TrimSpace(filter.CreatedFrom),
			CreatedTo:      strings.TrimSpace(filter.CreatedTo),
			ProductKeyword: strings.TrimSpace(filter.ProductKeyword),
			Page:           1,
			PageSize:       limit,
			SortBy:         "created_at",
			SortOrder:      "asc",
		})
		if err != nil {
			return nil, fmt.Errorf("list orders for fulfillment: %w", err)
		}
		if listResp == nil {
			continue
		}

		for _, item := range listResp.Items {
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = struct{}{}

			order, err := w.api.GetOrder(ctx, token, item.ID)
			if err != nil || order == nil {
				continue
			}
			if err := validateManualFulfillmentOrder(order); err != nil {
				continue
			}
			candidate, ok, err := manualCandidateForOrder(order, filter)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if candidate.PaidAt == "" {
				candidate.PaidAt = strings.TrimSpace(item.PaidAt)
			}
			if candidate.CreatedAt == "" {
				candidate.CreatedAt = strings.TrimSpace(item.CreatedAt)
			}
			candidates = append(candidates, candidate)
		}
	}

	sortManualCandidates(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

func manualCandidateForOrder(order *dujiao.OrderDetail, filter BatchFulfillmentFilter) (manualFulfillmentCandidate, bool, error) {
	candidate := manualFulfillmentCandidate{
		OrderID:   order.ID,
		OrderNo:   strings.TrimSpace(order.OrderNo),
		Status:    strings.TrimSpace(order.Status),
		PaidAt:    strings.TrimSpace(order.PaidAt),
		CreatedAt: strings.TrimSpace(order.CreatedAt),
	}

	for _, item := range order.Items {
		quantity := item.Quantity
		if quantity <= 0 {
			if filter.ProductID > 0 {
				return manualFulfillmentCandidate{}, false, fmt.Errorf("order %s item %d has invalid quantity %d", order.OrderNo, item.ID, item.Quantity)
			}
			quantity = 1
		}
		if filter.ProductID == 0 {
			candidate.Quantity += quantity
			continue
		}
		if item.ProductID != filter.ProductID || (filter.SKUID > 0 && item.SKUID != filter.SKUID) {
			return manualFulfillmentCandidate{}, false, nil
		}
		candidate.Quantity += quantity
		candidate.ProductID = filter.ProductID
		if filter.SKUID > 0 {
			candidate.SKUID = filter.SKUID
		}
	}

	if candidate.Quantity <= 0 {
		return manualFulfillmentCandidate{}, false, nil
	}
	return candidate, true, nil
}

func candidateIDsAndNos(candidates []manualFulfillmentCandidate) ([]uint, []string) {
	orderIDs := make([]uint, 0, len(candidates))
	orderNos := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		orderIDs = append(orderIDs, candidate.OrderID)
		orderNos = append(orderNos, candidate.OrderNo)
	}
	return orderIDs, orderNos
}

func totalCandidateQuantity(candidates []manualFulfillmentCandidate) int {
	total := 0
	for _, candidate := range candidates {
		total += candidate.Quantity
	}
	return total
}

func normalizeBatchFulfillmentLimit(limit int) int {
	if limit <= 0 {
		return defaultBatchFulfillmentLimit
	}
	return limit
}

func fulfillmentStatuses(status string) []string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "pending", "all":
		return []string{"paid", "fulfilling"}
	case "fulfilling":
		return []string{"fulfilling"}
	default:
		return []string{strings.TrimSpace(status)}
	}
}

func sortManualCandidates(candidates []manualFulfillmentCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		leftTime, leftOK := candidateSortTime(candidates[i])
		rightTime, rightOK := candidateSortTime(candidates[j])
		if leftOK && rightOK && !leftTime.Equal(rightTime) {
			return leftTime.Before(rightTime)
		}
		if leftOK != rightOK {
			return leftOK
		}
		if candidates[i].CreatedAt != candidates[j].CreatedAt {
			return candidates[i].CreatedAt < candidates[j].CreatedAt
		}
		if candidates[i].OrderID != candidates[j].OrderID {
			return candidates[i].OrderID < candidates[j].OrderID
		}
		return candidates[i].OrderNo < candidates[j].OrderNo
	})
}

func candidateSortTime(candidate manualFulfillmentCandidate) (time.Time, bool) {
	if parsed, ok := parseOrderTime(candidate.PaidAt); ok {
		return parsed, true
	}
	return parseOrderTime(candidate.CreatedAt)
}

func parseOrderTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func parseCardSecretLines(raw string) ([]string, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	secrets := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		secret := strings.TrimSpace(line)
		if secret == "" {
			continue
		}
		if _, ok := seen[secret]; ok {
			return nil, fmt.Errorf("duplicate card secret %q", secret)
		}
		seen[secret] = struct{}{}
		secrets = append(secrets, secret)
	}
	if len(secrets) == 0 {
		return nil, errors.New("delivery content is required")
	}
	return secrets, nil
}
