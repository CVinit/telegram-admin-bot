package workflow

import (
	"context"
	"testing"

	"github.com/CVinit/telegram-admin-bot/internal/dujiao"
	"github.com/CVinit/telegram-admin-bot/internal/session"
)

func TestSalesTodayUsesDashboardOverviewRangeToday(t *testing.T) {
	deps := newSalesTestDeps()

	_, err := deps.Workflow.BuildOverview(context.Background(), deps.Session, "today")
	if err != nil {
		t.Fatal(err)
	}
	if deps.API.LastQuery.Range != "today" {
		t.Fatalf("expected range=today, got %s", deps.API.LastQuery.Range)
	}
}

type salesTestDeps struct {
	Workflow *SalesWorkflow
	Session  *session.SessionView
	API      *stubDashboardAPI
}

func newSalesTestDeps() *salesTestDeps {
	api := &stubDashboardAPI{
		response: &dujiao.DashboardOverviewResponse{
			Range:    "today",
			Timezone: "Asia/Shanghai",
			Currency: "CNY",
			KPI: dujiao.DashboardKPI{
				GMVPaid:            "100.00",
				TotalProfit:        "40.00",
				PaidOrders:         5,
				CompletedOrders:    4,
				PaymentSuccessRate: "80%",
				TotalUserBalance:   "12.00",
			},
		},
	}

	return &salesTestDeps{
		Workflow: NewSalesWorkflow(api),
		Session: &session.SessionView{
			AdminID:  3,
			Username: "ops",
			JWTToken: "jwt-demo",
		},
		API: api,
	}
}

type stubDashboardAPI struct {
	LastToken string
	LastQuery dujiao.DashboardOverviewQuery
	response  *dujiao.DashboardOverviewResponse
	err       error
}

func (s *stubDashboardAPI) GetDashboardOverview(_ context.Context, token string, query dujiao.DashboardOverviewQuery) (*dujiao.DashboardOverviewResponse, error) {
	s.LastToken = token
	s.LastQuery = query
	return s.response, s.err
}
