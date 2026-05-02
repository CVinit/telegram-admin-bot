package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/CVinit/telegram-admin-bot/internal/session"
	"github.com/CVinit/telegram-admin-bot/internal/workflow"
)

func TestRejectsSensitiveCommandOutsidePrivateChat(t *testing.T) {
	router := NewRouter(&stubSessionService{})

	result, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       -1001,
		ChatType:     ChatTypeGroup,
		Text:         "/login ops secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Text, "仅支持私聊") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestLoginReturnsHomeKeyboard(t *testing.T) {
	router := NewRouter(&stubSessionService{
		loginView: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			IsSuper:      true,
		},
	})

	result, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       123456789,
		ChatType:     ChatTypePrivate,
		Text:         "/login ops secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Text, "ops") {
		t.Fatalf("unexpected login response: %#v", result)
	}
	if len(result.Keyboard) == 0 {
		t.Fatalf("expected home keyboard, got %#v", result)
	}
}

func TestSessionCommandShowsCurrentSession(t *testing.T) {
	router := NewRouter(&stubSessionService{
		requireView: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			IsSuper:      true,
			Roles:        []string{"operator"},
		},
	})

	result, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       123456789,
		ChatType:     ChatTypePrivate,
		Text:         "/session",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Text, "ops") || !strings.Contains(result.Text, "operator") {
		t.Fatalf("unexpected session response: %#v", result)
	}
}

func TestMenuHomeCallbackReturnsKeyboard(t *testing.T) {
	router := NewRouter(&stubSessionService{
		requireView: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
		},
	})

	result, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       123456789,
		ChatType:     ChatTypePrivate,
		CallbackData: CallbackMenuHome,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Keyboard) == 0 {
		t.Fatalf("expected menu keyboard, got %#v", result)
	}
}

func TestSalesCommandRoutesRequestedRange(t *testing.T) {
	sales := &stubSalesWorkflow{
		response: &workflow.SalesOverviewView{
			RangeKey:   "week",
			Title:      "本周销售",
			GMVPaid:    "100.00",
			Currency:   "CNY",
			Timezone:   "Asia/Shanghai",
			PaidOrders: 5,
		},
	}

	router := NewRouter(&stubSessionService{
		requireView: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			JWTToken:     "jwt-demo",
		},
	}).WithSalesWorkflow(sales)

	result, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       123456789,
		ChatType:     ChatTypePrivate,
		Text:         "/sales week",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sales.lastRange != "week" {
		t.Fatalf("expected sales workflow range=week, got %q", sales.lastRange)
	}
	if !strings.Contains(result.Text, "本周销售") {
		t.Fatalf("unexpected sales response: %#v", result)
	}
}

func TestShipCommandRoutesRequestedOrder(t *testing.T) {
	flow := &stubFulfillmentWorkflow{
		singlePreview: &workflow.SingleFulfillmentPreviewView{
			ActionKey:      "fulfillment_confirm:1",
			OrderID:        11,
			OrderNo:        "DJ1001",
			DeliveryKind:   "payload",
			PayloadPreview: "card-1",
		},
	}

	router := NewRouter(&stubSessionService{
		requireView: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			JWTToken:     "jwt-demo",
		},
	}).WithFulfillmentWorkflow(flow)

	result, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       123456789,
		ChatType:     ChatTypePrivate,
		Text:         "/ship DJ1001\ncard-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if flow.lastOrderNo != "DJ1001" {
		t.Fatalf("expected order number DJ1001, got %q", flow.lastOrderNo)
	}
	if !strings.Contains(result.Text, "DJ1001") {
		t.Fatalf("unexpected ship response: %#v", result)
	}
}

func TestBatchShipCommandRoutesFilterAndDelivery(t *testing.T) {
	flow := &stubFulfillmentWorkflow{
		batchPreview: &workflow.BatchFulfillmentPreviewView{
			ActionKey:      "fulfillment_batch_confirm:1",
			OrderCount:     1,
			OrderNos:       []string{"DJ1001"},
			DeliveryKind:   "payload",
			PayloadPreview: "card-1",
		},
	}

	router := NewRouter(&stubSessionService{
		requireView: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			JWTToken:     "jwt-demo",
		},
	}).WithFulfillmentWorkflow(flow)

	result, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       123456789,
		ChatType:     ChatTypePrivate,
		Text:         "/batch_ship status=paid product=vip limit=5\ncard-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if flow.lastBatchFilter.ProductKeyword != "vip" || flow.lastBatchFilter.Limit != 5 {
		t.Fatalf("unexpected batch filter: %#v", flow.lastBatchFilter)
	}
	if !strings.Contains(result.Text, "批量发货预览") {
		t.Fatalf("unexpected batch ship response: %#v", result)
	}
}

func TestBatchShipCommandParsesProductIDAndSKUID(t *testing.T) {
	flow := &stubFulfillmentWorkflow{
		batchPreview: &workflow.BatchFulfillmentPreviewView{
			ActionKey:      "fulfillment_batch_confirm:1",
			OrderCount:     2,
			OrderNos:       []string{"DJ1001", "DJ1002"},
			DeliveryKind:   "card_secrets",
			PayloadPreview: "2 orders / 3 secrets",
		},
	}

	router := NewRouter(&stubSessionService{
		requireView: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			JWTToken:     "jwt-demo",
		},
	}).WithFulfillmentWorkflow(flow)

	_, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       123456789,
		ChatType:     ChatTypePrivate,
		Text:         "/batch_ship product_id=9 sku_id=3 limit=5\nCARD-1\nCARD-2\nCARD-3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if flow.lastBatchFilter.ProductID != 9 || flow.lastBatchFilter.SKUID != 3 || flow.lastBatchFilter.Limit != 5 {
		t.Fatalf("unexpected product batch filter: %#v", flow.lastBatchFilter)
	}
	if flow.lastBatchBody != "CARD-1\nCARD-2\nCARD-3" {
		t.Fatalf("unexpected batch body: %q", flow.lastBatchBody)
	}
}

func TestPendingShipCommandRoutesFilter(t *testing.T) {
	flow := &stubFulfillmentWorkflow{
		pendingList: &workflow.PendingFulfillmentListView{
			OrderCount:    1,
			TotalQuantity: 2,
			Items: []workflow.PendingFulfillmentOrderView{
				{OrderID: 11, OrderNo: "DJ1001", Status: "paid", ProductID: 9, SKUID: 3, Quantity: 2},
			},
		},
	}

	router := NewRouter(&stubSessionService{
		requireView: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			JWTToken:     "jwt-demo",
		},
	}).WithFulfillmentWorkflow(flow)

	result, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       123456789,
		ChatType:     ChatTypePrivate,
		Text:         "/pending_ship product_id=9 sku_id=3 limit=5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if flow.lastPendingFilter.ProductID != 9 || flow.lastPendingFilter.SKUID != 3 || flow.lastPendingFilter.Limit != 5 {
		t.Fatalf("unexpected pending filter: %#v", flow.lastPendingFilter)
	}
	if !strings.Contains(result.Text, "待发货订单") || !strings.Contains(result.Text, "DJ1001") {
		t.Fatalf("unexpected pending response: %#v", result)
	}
}

func TestRestockCommandBuildsPreviewWithConfirmButton(t *testing.T) {
	restock := &stubRestockWorkflow{
		preview: &workflow.RestockPreviewView{
			ActionKey:   "restock_confirm:123:1",
			ProductID:   9,
			ProductName: "会员卡",
			SecretCount: 2,
			BatchNo:     "batch-1",
			Source:      "text",
		},
	}

	router := NewRouter(&stubSessionService{
		requireView: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			JWTToken:     "jwt-demo",
		},
	}).WithRestockWorkflow(restock)

	result, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       123456789,
		ChatType:     ChatTypePrivate,
		Text:         "/restock 9\nA\nB\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if restock.lastTextProductID != 9 {
		t.Fatalf("expected product 9, got %#v", restock)
	}
	if result.KeyboardKind != KeyboardKindInline || len(result.Keyboard) == 0 || result.Keyboard[0][0].CallbackData == "" {
		t.Fatalf("expected inline confirm button, got %#v", result)
	}
}

func TestRestockConfirmCallbackExecutesWorkflow(t *testing.T) {
	restock := &stubRestockWorkflow{
		result: &workflow.RestockResultView{
			ProductID:   9,
			ProductName: "会员卡",
			Created:     2,
			BatchNo:     "batch-1",
			SecretCount: 2,
		},
	}

	router := NewRouter(&stubSessionService{
		requireView: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			JWTToken:     "jwt-demo",
		},
	}).WithRestockWorkflow(restock)

	result, err := router.Handle(context.Background(), IncomingUpdate{
		TelegramUser: 123456789,
		ChatID:       123456789,
		ChatType:     ChatTypePrivate,
		CallbackID:   "cb-1",
		CallbackData: "restock_confirm:123:1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if restock.lastConfirmAction != "restock_confirm:123:1" {
		t.Fatalf("expected confirm action key, got %#v", restock)
	}
	if !strings.Contains(result.Text, "补库存已完成") {
		t.Fatalf("unexpected confirm response: %#v", result)
	}
}

type stubSessionService struct {
	loginView   *session.SessionView
	loginErr    error
	requireView *session.SessionView
	requireErr  error
	logoutErr   error
}

func (s *stubSessionService) Login(_ context.Context, _ int64, _, _ string) (*session.SessionView, error) {
	if s.loginErr != nil {
		return nil, s.loginErr
	}
	if s.loginView == nil {
		return nil, errors.New("missing login view")
	}
	return s.loginView, nil
}

func (s *stubSessionService) RequireSession(_ context.Context, _ int64) (*session.SessionView, error) {
	if s.requireErr != nil {
		return nil, s.requireErr
	}
	if s.requireView == nil {
		return nil, session.ErrSessionRequired
	}
	return s.requireView, nil
}

func (s *stubSessionService) Logout(_ context.Context, _ int64) error {
	return s.logoutErr
}

type stubSalesWorkflow struct {
	lastRange string
	response  *workflow.SalesOverviewView
	err       error
}

func (s *stubSalesWorkflow) BuildOverview(_ context.Context, _ *session.SessionView, rangeKey string) (*workflow.SalesOverviewView, error) {
	s.lastRange = rangeKey
	return s.response, s.err
}

type stubFulfillmentWorkflow struct {
	lastOrderNo       string
	lastDelivery      string
	singlePreview     *workflow.SingleFulfillmentPreviewView
	singleResult      *workflow.SingleFulfillmentResultView
	lastBatchFilter   workflow.BatchFulfillmentFilter
	lastBatchBody     string
	batchPreview      *workflow.BatchFulfillmentPreviewView
	batchResult       *workflow.BatchFulfillmentResultView
	lastPendingFilter workflow.BatchFulfillmentFilter
	pendingList       *workflow.PendingFulfillmentListView
	err               error
}

func (s *stubFulfillmentWorkflow) BuildSinglePreview(_ context.Context, _ *session.SessionView, orderNo string, rawDelivery string) (*workflow.SingleFulfillmentPreviewView, error) {
	s.lastOrderNo = orderNo
	s.lastDelivery = rawDelivery
	return s.singlePreview, s.err
}

func (s *stubFulfillmentWorkflow) ConfirmSingle(_ context.Context, _ *session.SessionView, _ string) (*workflow.SingleFulfillmentResultView, error) {
	return s.singleResult, s.err
}

func (s *stubFulfillmentWorkflow) BuildBatchPreview(_ context.Context, _ *session.SessionView, filter workflow.BatchFulfillmentFilter, rawDelivery string) (*workflow.BatchFulfillmentPreviewView, error) {
	s.lastBatchFilter = filter
	s.lastBatchBody = rawDelivery
	return s.batchPreview, s.err
}

func (s *stubFulfillmentWorkflow) ConfirmBatch(_ context.Context, _ *session.SessionView, _ string) (*workflow.BatchFulfillmentResultView, error) {
	return s.batchResult, s.err
}

func (s *stubFulfillmentWorkflow) BuildPendingList(_ context.Context, _ *session.SessionView, filter workflow.BatchFulfillmentFilter) (*workflow.PendingFulfillmentListView, error) {
	s.lastPendingFilter = filter
	return s.pendingList, s.err
}

func (s *stubFulfillmentWorkflow) ListProductsForFulfillment(_ context.Context, _ *session.SessionView, _ workflow.BatchFulfillmentFilter) (*workflow.ProductFulfillmentListView, error) {
	return &workflow.ProductFulfillmentListView{Items: nil}, s.err
}

func (s *stubFulfillmentWorkflow) BuildProductShipPreview(_ context.Context, _ *session.SessionView, _ uint, _ uint, _ string) (*workflow.ProductShipPreviewView, error) {
	return nil, s.err
}

func (s *stubFulfillmentWorkflow) ConfirmProductShip(_ context.Context, _ *session.SessionView, _ string) (*workflow.BatchFulfillmentResultView, error) {
	return s.batchResult, s.err
}

type stubRestockWorkflow struct {
	lastTextProductID uint
	lastTextSKUID     uint
	lastTextBody      string
	lastFileName      string
	lastFileContent   []byte
	lastConfirmAction string
	preview           *workflow.RestockPreviewView
	result            *workflow.RestockResultView
	err               error
}

func (s *stubRestockWorkflow) BuildPreviewFromText(_ context.Context, _ *session.SessionView, productID, skuID uint, rawText string) (*workflow.RestockPreviewView, error) {
	s.lastTextProductID = productID
	s.lastTextSKUID = skuID
	s.lastTextBody = rawText
	return s.preview, s.err
}

func (s *stubRestockWorkflow) BuildPreviewFromFile(_ context.Context, _ *session.SessionView, productID, skuID uint, fileName string, content []byte) (*workflow.RestockPreviewView, error) {
	s.lastTextProductID = productID
	s.lastTextSKUID = skuID
	s.lastFileName = fileName
	s.lastFileContent = content
	return s.preview, s.err
}

func (s *stubRestockWorkflow) Confirm(_ context.Context, _ *session.SessionView, actionKey string) (*workflow.RestockResultView, error) {
	s.lastConfirmAction = actionKey
	return s.result, s.err
}
