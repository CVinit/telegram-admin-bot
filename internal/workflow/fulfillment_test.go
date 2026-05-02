package workflow

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/CVinit/telegram-admin-bot/internal/dujiao"
	"github.com/CVinit/telegram-admin-bot/internal/session"
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

func TestBuildPendingListIncludesPaidAndFulfillingManualOrdersForProduct(t *testing.T) {
	deps := newFulfillmentTestDeps()
	deps.API.listOrdersRespByStatus = map[string]*dujiao.OrderListResponse{
		"paid": {
			Items: []dujiao.OrderListItem{
				{ID: 21, OrderNo: "DJ2001", Status: "paid", PaidAt: "2026-04-02T10:00:00Z", CreatedAt: "2026-04-02T09:55:00Z"},
			},
		},
		"fulfilling": {
			Items: []dujiao.OrderListItem{
				{ID: 22, OrderNo: "DJ2002", Status: "fulfilling", PaidAt: "2026-04-02T10:05:00Z", CreatedAt: "2026-04-02T09:58:00Z"},
				{ID: 23, OrderNo: "DJ2003", Status: "fulfilling", PaidAt: "2026-04-02T10:06:00Z", CreatedAt: "2026-04-02T09:59:00Z"},
			},
		},
	}
	deps.API.ordersByID[21] = &dujiao.OrderDetail{
		ID:        21,
		OrderNo:   "DJ2001",
		UserID:    5,
		Status:    "paid",
		PaidAt:    "2026-04-02T10:00:00Z",
		CreatedAt: "2026-04-02T09:55:00Z",
		Items: []dujiao.OrderItem{
			{ProductID: 9, SKUID: 3, Quantity: 1, FulfillmentType: "manual"},
		},
	}
	deps.API.ordersByID[22] = &dujiao.OrderDetail{
		ID:        22,
		OrderNo:   "DJ2002",
		UserID:    5,
		Status:    "fulfilling",
		PaidAt:    "2026-04-02T10:05:00Z",
		CreatedAt: "2026-04-02T09:58:00Z",
		Items: []dujiao.OrderItem{
			{ProductID: 9, SKUID: 3, Quantity: 2, FulfillmentType: "manual"},
		},
	}
	deps.API.ordersByID[23] = &dujiao.OrderDetail{
		ID:        23,
		OrderNo:   "DJ2003",
		UserID:    5,
		Status:    "fulfilling",
		PaidAt:    "2026-04-02T10:06:00Z",
		CreatedAt: "2026-04-02T09:59:00Z",
		Items: []dujiao.OrderItem{
			{ProductID: 8, SKUID: 3, Quantity: 1, FulfillmentType: "manual"},
		},
	}

	view, err := deps.Workflow.BuildPendingList(context.Background(), deps.Session, BatchFulfillmentFilter{
		ProductID: 9,
		SKUID:     3,
		Limit:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.OrderCount != 2 || view.TotalQuantity != 3 {
		t.Fatalf("unexpected pending summary: %#v", view)
	}
	if len(view.Items) != 2 || view.Items[0].OrderNo != "DJ2001" || view.Items[1].OrderNo != "DJ2002" {
		t.Fatalf("expected paid order sequence, got %#v", view.Items)
	}
	if len(deps.API.listOrderCalls) != 2 || deps.API.listOrderCalls[0].Status != "paid" || deps.API.listOrderCalls[1].Status != "fulfilling" {
		t.Fatalf("expected paid and fulfilling list calls, got %#v", deps.API.listOrderCalls)
	}
}

func TestBuildAndConfirmBatchPreviewAssignsSecretsByPaidOrder(t *testing.T) {
	deps := newFulfillmentTestDeps()
	deps.API.listOrdersResp = &dujiao.OrderListResponse{
		Items: []dujiao.OrderListItem{
			{ID: 31, OrderNo: "DJ3001", Status: "paid", PaidAt: "2026-04-02T10:05:00Z", CreatedAt: "2026-04-02T09:59:00Z"},
			{ID: 32, OrderNo: "DJ3002", Status: "paid", PaidAt: "2026-04-02T10:00:00Z", CreatedAt: "2026-04-02T09:55:00Z"},
		},
	}
	deps.API.ordersByID[31] = &dujiao.OrderDetail{
		ID:        31,
		OrderNo:   "DJ3001",
		UserID:    5,
		Status:    "paid",
		PaidAt:    "2026-04-02T10:05:00Z",
		CreatedAt: "2026-04-02T09:59:00Z",
		Items: []dujiao.OrderItem{
			{ProductID: 9, SKUID: 3, Quantity: 2, FulfillmentType: "manual"},
		},
		UserEmail: "buyer@example.com",
	}
	deps.API.ordersByID[32] = &dujiao.OrderDetail{
		ID:        32,
		OrderNo:   "DJ3002",
		UserID:    5,
		Status:    "paid",
		PaidAt:    "2026-04-02T10:00:00Z",
		CreatedAt: "2026-04-02T09:55:00Z",
		Items: []dujiao.OrderItem{
			{ProductID: 9, SKUID: 3, Quantity: 1, FulfillmentType: "manual"},
		},
		UserEmail: "buyer@example.com",
	}

	preview, err := deps.Workflow.BuildBatchPreview(context.Background(), deps.Session, BatchFulfillmentFilter{
		Status:    "paid",
		ProductID: 9,
		SKUID:     3,
		Limit:     10,
	}, "CARD-1\nCARD-2\nCARD-3")
	if err != nil {
		t.Fatal(err)
	}
	if preview.DeliveryKind != "card_secrets" {
		t.Fatalf("expected card secret assignment mode, got %#v", preview)
	}
	if len(preview.OrderNos) != 2 || preview.OrderNos[0] != "DJ3002" || preview.OrderNos[1] != "DJ3001" {
		t.Fatalf("expected paid order sequence, got %#v", preview.OrderNos)
	}

	result, err := deps.Workflow.ConfirmBatch(context.Background(), deps.Session, preview.ActionKey)
	if err != nil {
		t.Fatal(err)
	}
	if result.SuccessCount != 2 || result.FailedCount != 0 {
		t.Fatalf("unexpected batch result: %#v", result)
	}
	if len(deps.API.fulfillments) != 2 {
		t.Fatalf("expected two fulfillment calls, got %#v", deps.API.fulfillments)
	}
	if deps.API.fulfillments[0].OrderID != 32 || deps.API.fulfillments[0].Payload != "CARD-1" {
		t.Fatalf("unexpected first fulfillment: %#v", deps.API.fulfillments[0])
	}
	if deps.API.fulfillments[1].OrderID != 31 || deps.API.fulfillments[1].Payload != "CARD-2\nCARD-3" {
		t.Fatalf("unexpected second fulfillment: %#v", deps.API.fulfillments[1])
	}
}

func TestBuildBatchPreviewRejectsProductSecretCountMismatch(t *testing.T) {
	deps := newFulfillmentTestDeps()
	deps.API.listOrdersResp = &dujiao.OrderListResponse{
		Items: []dujiao.OrderListItem{
			{ID: 41, OrderNo: "DJ4001", Status: "paid", PaidAt: "2026-04-02T10:00:00Z"},
		},
	}
	deps.API.ordersByID[41] = &dujiao.OrderDetail{
		ID:      41,
		OrderNo: "DJ4001",
		UserID:  5,
		Status:  "paid",
		PaidAt:  "2026-04-02T10:00:00Z",
		Items: []dujiao.OrderItem{
			{ProductID: 9, SKUID: 3, Quantity: 2, FulfillmentType: "manual"},
		},
		UserEmail: "buyer@example.com",
	}

	_, err := deps.Workflow.BuildBatchPreview(context.Background(), deps.Session, BatchFulfillmentFilter{
		Status:    "paid",
		ProductID: 9,
		SKUID:     3,
		Limit:     10,
	}, "ONLY-ONE-CARD")
	if err == nil || !strings.Contains(err.Error(), "secret count") {
		t.Fatalf("expected secret count mismatch, got %v", err)
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
		smtp:    &dujiao.SMTPSettings{Enabled: true, OrderNotificationEnabled: true},
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
	listOrdersResp         *dujiao.OrderListResponse
	listOrdersRespByStatus map[string]*dujiao.OrderListResponse
	listOrdersErr          error
	ordersByID             map[uint]*dujiao.OrderDetail
	orderByNo              string
	lastListParams         dujiao.ListOrdersParams
	listOrderCalls         []dujiao.ListOrdersParams
	lastFulfillment        dujiao.CreateFulfillmentRequest
	fulfillments           []dujiao.CreateFulfillmentRequest
	fulfillmentResp        *dujiao.FulfillmentResponse
	fulfillmentErr         error
	usersByID              map[uint]*dujiao.AdminUserDetail
	smtp                   *dujiao.SMTPSettings
	runtime                *dujiao.TelegramBotRuntimeStatus
	channelClients         []dujiao.ChannelClient
}

func (s *stubFulfillmentAPI) ListOrders(_ context.Context, _ string, params dujiao.ListOrdersParams) (*dujiao.OrderListResponse, error) {
	s.lastListParams = params
	s.listOrderCalls = append(s.listOrderCalls, params)
	s.orderByNo = params.OrderNo
	if s.listOrdersRespByStatus != nil {
		if resp, ok := s.listOrdersRespByStatus[params.Status]; ok {
			return resp, s.listOrdersErr
		}
	}
	return s.listOrdersResp, s.listOrdersErr
}

func (s *stubFulfillmentAPI) GetOrder(_ context.Context, _ string, id uint) (*dujiao.OrderDetail, error) {
	return s.ordersByID[id], nil
}

func (s *stubFulfillmentAPI) CreateFulfillment(_ context.Context, _ string, req dujiao.CreateFulfillmentRequest) (*dujiao.FulfillmentResponse, error) {
	s.lastFulfillment = req
	s.fulfillments = append(s.fulfillments, req)
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
