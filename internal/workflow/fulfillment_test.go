package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/dujiao"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
)

func TestNotificationHintSkipsTelegramWhenUserHasNoTelegramIdentity(t *testing.T) {
	deps := newFulfillmentTestDeps()

	hint, err := deps.Workflow.BuildNotificationHint(context.Background(), deps.Session, deps.OrderWithoutTelegram)
	if err != nil {
		t.Fatal(err)
	}
	if hint.Telegram.Status != "expected_skip" {
		t.Fatalf("unexpected telegram hint: %#v", hint.Telegram)
	}
}

func TestConfirmSingleFulfillmentCallsCreateFulfillment(t *testing.T) {
	deps := newFulfillmentTestDeps()

	preview, err := deps.Workflow.BuildSinglePreview(context.Background(), deps.Session, "DJ1001", "account=demo\npassword=secret")
	if err != nil {
		t.Fatal(err)
	}

	result, err := deps.Workflow.ConfirmSingle(context.Background(), deps.Session, preview.ActionKey)
	if err != nil {
		t.Fatal(err)
	}
	if deps.API.lastFulfillment.OrderID != 11 {
		t.Fatalf("expected order id 11, got %+v", deps.API.lastFulfillment)
	}
	if result.OrderNo != "DJ1001" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestBuildBatchPreviewFiltersEligibleManualOrders(t *testing.T) {
	deps := newFulfillmentTestDeps()

	preview, err := deps.Workflow.BuildBatchPreview(context.Background(), deps.Session, BatchFulfillmentFilter{
		Status:         "paid",
		ProductKeyword: "vip",
		Limit:          10,
	}, "payload text")
	if err != nil {
		t.Fatal(err)
	}
	if preview.OrderCount != 1 {
		t.Fatalf("expected one eligible order, got %#v", preview)
	}
	if len(preview.OrderNos) != 1 || preview.OrderNos[0] != "DJ1001" {
		t.Fatalf("unexpected batch preview orders: %#v", preview.OrderNos)
	}
}

type fulfillmentTestDeps struct {
	Workflow             *FulfillmentWorkflow
	Session              *session.SessionView
	API                  *stubFulfillmentAPI
	Pending              *stubPendingActionStore
	OrderWithoutTelegram *dujiao.OrderDetail
}

func newFulfillmentTestDeps() *fulfillmentTestDeps {
	now := time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	orderWithTelegram := &dujiao.OrderDetail{
		ID:      11,
		OrderNo: "DJ1001",
		UserID:  5,
		Status:  "paid",
		Items: []dujiao.OrderItem{
			{FulfillmentType: "manual"},
		},
		UserEmail: "buyer@example.com",
	}
	orderWithoutTelegram := &dujiao.OrderDetail{
		ID:      12,
		OrderNo: "DJ1002",
		UserID:  6,
		Status:  "paid",
		Items: []dujiao.OrderItem{
			{FulfillmentType: "manual"},
		},
		UserEmail: "buyer2@example.com",
	}

	api := &stubFulfillmentAPI{
		listOrdersResp: &dujiao.OrderListResponse{
			Items: []dujiao.OrderListItem{
				{ID: 11, OrderNo: "DJ1001", Status: "paid"},
				{ID: 13, OrderNo: "DJ1003", Status: "paid"},
			},
		},
		ordersByID: map[uint]*dujiao.OrderDetail{
			11: orderWithTelegram,
			12: orderWithoutTelegram,
			13: {
				ID:      13,
				OrderNo: "DJ1003",
				UserID:  7,
				Status:  "paid",
				Items: []dujiao.OrderItem{
					{FulfillmentType: "auto"},
				},
			},
		},
		usersByID: map[uint]*dujiao.AdminUserDetail{
			5: {
				ID:    5,
				Email: "buyer@example.com",
				OAuthIdentities: []dujiao.AdminUserOAuthIdentity{
					{Provider: "telegram", ProviderUserID: "tg-5"},
				},
			},
			6: {
				ID:    6,
				Email: "buyer2@example.com",
			},
		},
		smtp:    &dujiao.SMTPSettings{Enabled: true},
		runtime: &dujiao.TelegramBotRuntimeStatus{Connected: true},
		channelClients: []dujiao.ChannelClient{
			{ChannelType: "telegram_bot", Status: 1, CallbackURL: "http://bot.internal"},
		},
		fulfillmentResp: &dujiao.FulfillmentResponse{ID: 99, OrderID: 11, Status: "delivered"},
	}
	pending := &stubPendingActionStore{}
	audit := &stubRestockAudit{}
	confirmations := newConfirmationService(pending, 10*time.Minute, func() time.Time { return now })
	workflow := NewFulfillmentWorkflow(api, confirmations, audit)

	return &fulfillmentTestDeps{
		Workflow: workflow,
		Session: &session.SessionView{
			TelegramUser: 123456789,
			AdminID:      3,
			Username:     "ops",
			JWTToken:     "jwt-demo",
		},
		API:                  api,
		Pending:              pending,
		OrderWithoutTelegram: orderWithoutTelegram,
	}
}

type stubFulfillmentAPI struct {
	listOrdersResp  *dujiao.OrderListResponse
	listOrdersErr   error
	ordersByID      map[uint]*dujiao.OrderDetail
	orderByNo       string
	lastListParams  dujiao.ListOrdersParams
	lastFulfillment dujiao.CreateFulfillmentRequest
	fulfillmentResp *dujiao.FulfillmentResponse
	fulfillmentErr  error
	usersByID       map[uint]*dujiao.AdminUserDetail
	smtp            *dujiao.SMTPSettings
	runtime         *dujiao.TelegramBotRuntimeStatus
	channelClients  []dujiao.ChannelClient
}

func (s *stubFulfillmentAPI) ListOrders(_ context.Context, _ string, params dujiao.ListOrdersParams) (*dujiao.OrderListResponse, error) {
	s.lastListParams = params
	s.orderByNo = params.OrderNo
	return s.listOrdersResp, s.listOrdersErr
}

func (s *stubFulfillmentAPI) GetOrder(_ context.Context, _ string, id uint) (*dujiao.OrderDetail, error) {
	return s.ordersByID[id], nil
}

func (s *stubFulfillmentAPI) CreateFulfillment(_ context.Context, _ string, req dujiao.CreateFulfillmentRequest) (*dujiao.FulfillmentResponse, error) {
	s.lastFulfillment = req
	return s.fulfillmentResp, s.fulfillmentErr
}

func (s *stubFulfillmentAPI) GetUser(_ context.Context, _ string, id uint) (*dujiao.AdminUserDetail, error) {
	return s.usersByID[id], nil
}

func (s *stubFulfillmentAPI) GetSMTPSettings(context.Context, string) (*dujiao.SMTPSettings, error) {
	return s.smtp, nil
}

func (s *stubFulfillmentAPI) GetTelegramBotRuntimeStatus(context.Context, string) (*dujiao.TelegramBotRuntimeStatus, error) {
	return s.runtime, nil
}

func (s *stubFulfillmentAPI) ListChannelClients(context.Context, string) ([]dujiao.ChannelClient, error) {
	return s.channelClients, nil
}
