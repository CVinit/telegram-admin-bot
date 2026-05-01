package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/CVinit/telegram-admin-bot/internal/dujiao"
	"github.com/CVinit/telegram-admin-bot/internal/session"
	"github.com/CVinit/telegram-admin-bot/internal/storage"
)

func TestParseRestockTextDropsBlankLinesAndDuplicates(t *testing.T) {
	items := ParseRestockText("A\n\nB\nA\n")
	if len(items) != 2 {
		t.Fatalf("expected 2 unique items, got %d", len(items))
	}
}

func TestBuildRestockPreviewStoresPendingAction(t *testing.T) {
	deps := newRestockTestDeps()

	preview, err := deps.Workflow.BuildPreviewFromText(context.Background(), deps.Session, 9, 0, "A\nB\nA\n")
	if err != nil {
		t.Fatal(err)
	}
	if preview.SecretCount != 2 {
		t.Fatalf("expected 2 secrets, got %d", preview.SecretCount)
	}
	if preview.ActionKey == "" {
		t.Fatal("expected preview action key")
	}
	if len(deps.Pending.created) != 1 {
		t.Fatalf("expected one pending action, got %d", len(deps.Pending.created))
	}
}

func TestConfirmRestockConsumesPendingActionAndCallsBatch(t *testing.T) {
	deps := newRestockTestDeps()

	preview, err := deps.Workflow.BuildPreviewFromText(context.Background(), deps.Session, 9, 0, "A\nB\n")
	if err != nil {
		t.Fatal(err)
	}

	result, err := deps.Workflow.Confirm(context.Background(), deps.Session, preview.ActionKey)
	if err != nil {
		t.Fatal(err)
	}
	if deps.API.lastBatch.ProductID != 9 {
		t.Fatalf("expected product 9, got %+v", deps.API.lastBatch)
	}
	if len(deps.API.lastBatch.Secrets) != 2 {
		t.Fatalf("expected 2 secrets submitted, got %+v", deps.API.lastBatch)
	}
	if result.Created != 2 {
		t.Fatalf("expected created=2, got %+v", result)
	}
}

type restockTestDeps struct {
	Workflow *RestockWorkflow
	Session  *session.SessionView
	API      *stubRestockAPI
	Pending  *stubPendingActionStore
	Audit    *stubRestockAudit
}

func newRestockTestDeps() *restockTestDeps {
	now := time.Date(2026, 4, 1, 18, 0, 0, 0, time.UTC)
	api := &stubRestockAPI{
		product: &dujiao.ProductDetail{
			ID:              9,
			Title:           map[string]any{"zh-CN": "会员卡"},
			FulfillmentType: "auto",
		},
		batchResp: &dujiao.CreateCardSecretBatchResponse{Created: 2, BatchNo: "batch-1"},
	}
	pending := &stubPendingActionStore{}
	audit := &stubRestockAudit{}
	confirmations := newConfirmationService(pending, 10*time.Minute, func() time.Time { return now })
	workflow := NewRestockWorkflow(api, confirmations, audit)
	workflow.now = func() time.Time { return now }

	return &restockTestDeps{
		Workflow: workflow,
		Session: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			JWTToken:     "jwt-demo",
		},
		API:     api,
		Pending: pending,
		Audit:   audit,
	}
}

type stubRestockAPI struct {
	product    *dujiao.ProductDetail
	productErr error
	lastBatch  dujiao.CreateCardSecretBatchRequest
	batchResp  *dujiao.CreateCardSecretBatchResponse
	batchErr   error
}

func (s *stubRestockAPI) GetProduct(_ context.Context, _ string, _ uint) (*dujiao.ProductDetail, error) {
	return s.product, s.productErr
}

func (s *stubRestockAPI) CreateCardSecretBatch(_ context.Context, _ string, req dujiao.CreateCardSecretBatchRequest) (*dujiao.CreateCardSecretBatchResponse, error) {
	s.lastBatch = req
	return s.batchResp, s.batchErr
}

type stubPendingActionStore struct {
	created []*storage.PendingAction
}

func (s *stubPendingActionStore) CreatePendingAction(_ context.Context, action *storage.PendingAction) error {
	cp := *action
	s.created = append(s.created, &cp)
	return nil
}

func (s *stubPendingActionStore) ConsumePendingAction(_ context.Context, telegramUser int64, action string) (*storage.PendingAction, error) {
	for i, item := range s.created {
		if item.TelegramUser == telegramUser && item.Action == action {
			s.created = append(s.created[:i], s.created[i+1:]...)
			cp := *item
			return &cp, nil
		}
	}
	return nil, nil
}

type stubRestockAudit struct{}

func (s *stubRestockAudit) LogActionStarted(context.Context, int64, uint, string, string, map[string]any) error {
	return nil
}

func (s *stubRestockAudit) LogActionSucceeded(context.Context, int64, uint, string, string, string, map[string]any) error {
	return nil
}

func (s *stubRestockAudit) LogActionFailed(context.Context, int64, uint, string, string, string, map[string]any) error {
	return nil
}
