package audit

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CVinit/telegram-admin-bot/internal/storage"
	"gorm.io/gorm"
)

func TestRenderRecentActionSummaryIncludesLatestAction(t *testing.T) {
	deps := newAuditTestDeps(t)
	if err := deps.Service.LogActionSucceeded(context.Background(), 123456789, 3, "restock", "product:9", "ok", map[string]any{"created": 5}); err != nil {
		t.Fatal(err)
	}
	summary, err := deps.Service.RenderRecentActionSummary(context.Background(), 123456789, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary, "restock") {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func TestLogActionLifecycleWritesEntries(t *testing.T) {
	deps := newAuditTestDeps(t)
	ctx := context.Background()

	if err := deps.Service.LogActionStarted(ctx, 123456789, 3, "restock", "product:9", map[string]any{"step": "start"}); err != nil {
		t.Fatal(err)
	}
	if err := deps.Service.LogActionSucceeded(ctx, 123456789, 3, "restock", "product:9", "ok", map[string]any{"created": 5}); err != nil {
		t.Fatal(err)
	}
	if err := deps.Service.LogActionFailed(ctx, 123456789, 3, "restock", "product:9", "backend timeout", map[string]any{"retry": false}); err != nil {
		t.Fatal(err)
	}

	recent, err := deps.Store.ListRecentActionLogs(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 action logs, got %d", len(recent))
	}

	if recent[0].Result != "failed:backend timeout" {
		t.Fatalf("expected latest failed result, got %q", recent[0].Result)
	}
	if recent[1].Result != "succeeded:ok" {
		t.Fatalf("expected middle succeeded result, got %q", recent[1].Result)
	}
	if recent[2].Result != "started" {
		t.Fatalf("expected oldest started result, got %q", recent[2].Result)
	}

	var metadata map[string]any
	if err := json.Unmarshal([]byte(recent[1].MetadataJSON), &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata["created"] != float64(5) {
		t.Fatalf("expected created metadata field, got %#v", metadata)
	}
}

func TestRenderRecentActionSummaryFiltersTelegramUser(t *testing.T) {
	deps := newAuditTestDeps(t)
	ctx := context.Background()

	if err := deps.Service.LogActionSucceeded(ctx, 111, 3, "bind", "node:a", "ok", map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	if err := deps.Service.LogActionSucceeded(ctx, 222, 3, "unbind", "node:b", "ok", map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}

	summary, err := deps.Service.RenderRecentActionSummary(ctx, 111, 10)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(summary, "unbind") {
		t.Fatalf("expected summary to exclude other user actions, got %q", summary)
	}
	if !strings.Contains(summary, "bind") {
		t.Fatalf("expected summary to include user actions, got %q", summary)
	}
}

type auditTestDeps struct {
	Service *Service
	Store   *storage.ActionLogStore
}

func newAuditTestDeps(t *testing.T) *auditTestDeps {
	t.Helper()
	db := openAuditServiceTestDB(t)
	store := storage.NewActionLogStore(db)
	now := time.Date(2026, 4, 1, 15, 0, 0, 0, time.UTC)
	service := newService(store, func() time.Time { return now })

	return &auditTestDeps{
		Service: service,
		Store:   store,
	}
}

func openAuditServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit_service_test.db")
	db, err := storage.OpenSQLite(path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := storage.Migrate(db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}
