package workflow

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/dujiao"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
)

const RestockConfirmPrefix = "restock_confirm"

type restockAPI interface {
	GetProduct(ctx context.Context, token string, id uint) (*dujiao.ProductDetail, error)
	CreateCardSecretBatch(ctx context.Context, token string, req dujiao.CreateCardSecretBatchRequest) (*dujiao.CreateCardSecretBatchResponse, error)
}

type restockAuditLogger interface {
	LogActionStarted(ctx context.Context, telegramUser int64, adminID uint, action, target string, metadata map[string]any) error
	LogActionSucceeded(ctx context.Context, telegramUser int64, adminID uint, action, target, result string, metadata map[string]any) error
	LogActionFailed(ctx context.Context, telegramUser int64, adminID uint, action, target, result string, metadata map[string]any) error
}

type RestockWorkflow struct {
	api           restockAPI
	confirmations *ConfirmationService
	audit         restockAuditLogger
	now           func() time.Time
}

type RestockPreviewView struct {
	ActionKey   string
	ExpiresAt   time.Time
	ProductID   uint
	ProductName string
	SKUID       uint
	SKULabel    string
	SecretCount int
	Samples     []string
	BatchNo     string
	Source      string
}

type RestockResultView struct {
	ProductID   uint
	ProductName string
	SKUID       uint
	SKULabel    string
	Created     int
	BatchNo     string
	SecretCount int
}

type restockPayload struct {
	ProductID   uint     `json:"product_id"`
	ProductName string   `json:"product_name"`
	SKUID       uint     `json:"sku_id,omitempty"`
	SKULabel    string   `json:"sku_label,omitempty"`
	Secrets     []string `json:"secrets"`
	BatchNo     string   `json:"batch_no"`
	Source      string   `json:"source"`
}

func NewRestockWorkflow(api restockAPI, confirmations *ConfirmationService, audit restockAuditLogger) *RestockWorkflow {
	return &RestockWorkflow{
		api:           api,
		confirmations: confirmations,
		audit:         audit,
		now:           time.Now,
	}
}

func ParseRestockText(raw string) []string {
	return dedupeSecrets(strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r'
	}))
}

func ParseRestockFile(fileName string, content []byte) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(filepath.Ext(fileName))) {
	case "", ".txt":
		return ParseRestockText(string(content)), nil
	case ".csv":
		reader := csv.NewReader(bytes.NewReader(content))
		reader.TrimLeadingSpace = true

		rows, err := reader.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("parse csv restock file: %w", err)
		}

		values := make([]string, 0, len(rows))
		for _, row := range rows {
			for _, cell := range row {
				values = append(values, cell)
			}
		}
		return dedupeSecrets(values), nil
	default:
		return nil, fmt.Errorf("unsupported restock file type %q", filepath.Ext(fileName))
	}
}

func (w *RestockWorkflow) BuildPreviewFromText(ctx context.Context, sessionView *session.SessionView, productID, skuID uint, rawText string) (*RestockPreviewView, error) {
	return w.buildPreview(ctx, sessionView, productID, skuID, ParseRestockText(rawText), "text")
}

func (w *RestockWorkflow) BuildPreviewFromFile(ctx context.Context, sessionView *session.SessionView, productID, skuID uint, fileName string, content []byte) (*RestockPreviewView, error) {
	secrets, err := ParseRestockFile(fileName, content)
	if err != nil {
		return nil, err
	}
	return w.buildPreview(ctx, sessionView, productID, skuID, secrets, "file:"+strings.TrimSpace(fileName))
}

func (w *RestockWorkflow) Confirm(ctx context.Context, sessionView *session.SessionView, actionKey string) (*RestockResultView, error) {
	if w == nil || w.api == nil {
		return nil, errors.New("restock workflow api is nil")
	}
	if w.confirmations == nil {
		return nil, errors.New("confirmation service is nil")
	}
	if sessionView == nil {
		return nil, errors.New("session is required")
	}

	var payload restockPayload
	action, err := w.confirmations.Consume(ctx, sessionView.TelegramUser, actionKey, &payload)
	if err != nil {
		return nil, err
	}

	target := restockTarget(payload.ProductID, payload.SKUID)
	if w.audit != nil {
		_ = w.audit.LogActionStarted(ctx, sessionView.TelegramUser, sessionView.AdminID, "restock_confirm", target, map[string]any{
			"action_key":   action.Action,
			"secret_count": len(payload.Secrets),
			"batch_no":     payload.BatchNo,
		})
	}

	resp, err := w.api.CreateCardSecretBatch(ctx, strings.TrimSpace(sessionView.JWTToken), dujiao.CreateCardSecretBatchRequest{
		ProductID: payload.ProductID,
		SKUID:     payload.SKUID,
		Secrets:   payload.Secrets,
		BatchNo:   payload.BatchNo,
		Note:      "telegram-admin-bot restock",
	})
	if err != nil {
		if w.audit != nil {
			_ = w.audit.LogActionFailed(ctx, sessionView.TelegramUser, sessionView.AdminID, "restock_confirm", target, err.Error(), map[string]any{
				"action_key": action.Action,
			})
		}
		return nil, fmt.Errorf("create card secret batch: %w", err)
	}
	if resp == nil {
		return nil, errors.New("create card secret batch response is nil")
	}

	result := &RestockResultView{
		ProductID:   payload.ProductID,
		ProductName: payload.ProductName,
		SKUID:       payload.SKUID,
		SKULabel:    payload.SKULabel,
		Created:     resp.Created,
		BatchNo:     strings.TrimSpace(resp.BatchNo),
		SecretCount: len(payload.Secrets),
	}
	if result.BatchNo == "" {
		result.BatchNo = payload.BatchNo
	}

	if w.audit != nil {
		_ = w.audit.LogActionSucceeded(ctx, sessionView.TelegramUser, sessionView.AdminID, "restock_confirm", target, "created="+strconv.Itoa(result.Created), map[string]any{
			"action_key": action.Action,
			"batch_no":   result.BatchNo,
		})
	}

	return result, nil
}

func (w *RestockWorkflow) buildPreview(ctx context.Context, sessionView *session.SessionView, productID, skuID uint, secrets []string, source string) (*RestockPreviewView, error) {
	if w == nil || w.api == nil {
		return nil, errors.New("restock workflow api is nil")
	}
	if w.confirmations == nil {
		return nil, errors.New("confirmation service is nil")
	}
	if sessionView == nil {
		return nil, errors.New("session is required")
	}
	if productID == 0 {
		return nil, errors.New("product ID is required")
	}
	if len(secrets) == 0 {
		return nil, errors.New("restock secrets are empty")
	}

	product, err := w.api.GetProduct(ctx, strings.TrimSpace(sessionView.JWTToken), productID)
	if err != nil {
		return nil, fmt.Errorf("get product detail: %w", err)
	}
	if product == nil {
		return nil, errors.New("product detail is nil")
	}

	productName := displayTextFromMap(product.Title)
	if productName == "" {
		productName = fmt.Sprintf("product:%d", productID)
	}

	skuLabel := ""
	effectiveFulfillmentType := strings.TrimSpace(product.FulfillmentType)
	if skuID > 0 {
		sku, err := findProductSKU(product.SKUs, skuID)
		if err != nil {
			return nil, err
		}
		skuLabel = renderSKULabel(sku)
		if strings.TrimSpace(sku.FulfillmentType) != "" {
			effectiveFulfillmentType = strings.TrimSpace(sku.FulfillmentType)
		}
	}
	if effectiveFulfillmentType != "auto" {
		return nil, fmt.Errorf("product %d is not auto fulfillment", productID)
	}

	payload := restockPayload{
		ProductID:   productID,
		ProductName: productName,
		SKUID:       skuID,
		SKULabel:    skuLabel,
		Secrets:     secrets,
		BatchNo:     buildRestockBatchNo(w.now().UTC(), sessionView.TelegramUser),
		Source:      source,
	}
	record, err := w.confirmations.Create(ctx, sessionView.TelegramUser, RestockConfirmPrefix, payload)
	if err != nil {
		return nil, err
	}

	return &RestockPreviewView{
		ActionKey:   record.ActionKey,
		ExpiresAt:   record.ExpiresAt,
		ProductID:   productID,
		ProductName: productName,
		SKUID:       skuID,
		SKULabel:    skuLabel,
		SecretCount: len(secrets),
		Samples:     previewSamples(secrets, 3),
		BatchNo:     payload.BatchNo,
		Source:      source,
	}, nil
}

func dedupeSecrets(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func previewSamples(values []string, limit int) []string {
	if limit <= 0 || len(values) == 0 {
		return nil
	}
	if len(values) < limit {
		limit = len(values)
	}
	out := make([]string, 0, limit)
	for _, value := range values[:limit] {
		out = append(out, value)
	}
	return out
}

func buildRestockBatchNo(now time.Time, telegramUser int64) string {
	return fmt.Sprintf("tg-%s-%d", now.UTC().Format("20060102150405"), telegramUser)
}

func findProductSKU(skus []dujiao.ProductSKUSummary, skuID uint) (*dujiao.ProductSKUSummary, error) {
	for i := range skus {
		if skus[i].ID == skuID {
			return &skus[i], nil
		}
	}
	return nil, fmt.Errorf("sku %d not found", skuID)
}

func renderSKULabel(sku *dujiao.ProductSKUSummary) string {
	if sku == nil {
		return ""
	}
	if code := strings.TrimSpace(sku.SKUCode); code != "" {
		return code
	}
	label := displayTextFromMap(sku.SpecValues)
	if label != "" {
		return label
	}
	return fmt.Sprintf("sku:%d", sku.ID)
}

func displayTextFromMap(values map[string]any) string {
	if len(values) == 0 {
		return ""
	}
	for _, key := range []string{"zh-CN", "zh_CN", "zh", "name", "title", "value"} {
		if raw, ok := values[key].(string); ok && strings.TrimSpace(raw) != "" {
			return strings.TrimSpace(raw)
		}
	}
	for _, raw := range values {
		if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func restockTarget(productID, skuID uint) string {
	if skuID > 0 {
		return fmt.Sprintf("product:%d:sku:%d", productID, skuID)
	}
	return fmt.Sprintf("product:%d", productID)
}
