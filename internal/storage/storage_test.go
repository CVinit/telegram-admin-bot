package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestAutoMigrateCreatesCoreTables(t *testing.T) {
	db := openTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	assertTableExists(t, db, "admin_sessions")
	assertTableExists(t, db, "pending_actions")
	assertTableExists(t, db, "action_logs")
}

func TestSessionStoreRoundTrip(t *testing.T) {
	db := openTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	store := NewSessionStore(db)
	ctx := context.Background()

	created := &AdminSession{
		TelegramUser: 1001,
		AdminID:      42,
		Username:     "alice",
		JWTToken:     "token-1",
		JWTExpiresAt: time.Now().UTC().Add(30 * time.Minute),
		RolesJSON:    `["owner"]`,
		PoliciesJSON: `["*"]`,
		LastLoginAt:  time.Now().UTC(),
		LastUsedAt:   time.Now().UTC(),
	}

	if err := store.UpsertSession(ctx, created); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetSessionByTelegramUser(ctx, created.TelegramUser)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected session, got nil")
	}
	if got.Username != "alice" || got.JWTToken != "token-1" {
		t.Fatalf("unexpected session values: %+v", got)
	}

	created.JWTToken = "token-2"
	created.Username = "alice-updated"
	created.RolesJSON = `["owner","operator"]`
	if err := store.UpsertSession(ctx, created); err != nil {
		t.Fatal(err)
	}

	got, err = store.GetSessionByTelegramUser(ctx, created.TelegramUser)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected updated session, got nil")
	}
	if got.Username != "alice-updated" || got.JWTToken != "token-2" {
		t.Fatalf("session was not updated: %+v", got)
	}

	if err := store.DeleteSessionByTelegramUser(ctx, created.TelegramUser); err != nil {
		t.Fatal(err)
	}
	got, err = store.GetSessionByTelegramUser(ctx, created.TelegramUser)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil session after delete, got %+v", got)
	}
}

func TestSessionStoreRejectsInvalidSession(t *testing.T) {
	db := openTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	store := NewSessionStore(db)
	ctx := context.Background()

	err := store.UpsertSession(ctx, &AdminSession{
		TelegramUser: 0,
		AdminID:      42,
		Username:     "invalid",
	})
	if err == nil {
		t.Fatal("expected invalid session error")
	}
}

func TestPendingActionStoreLifecycle(t *testing.T) {
	db := openTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	store := NewPendingActionStore(db)
	ctx := context.Background()

	first := &PendingAction{
		TelegramUser: 2002,
		Action:       "unbind_node",
		PayloadJSON:  `{"node_id":"n1"}`,
		ExpiresAt:    time.Now().UTC().Add(5 * time.Minute),
	}
	if err := store.CreatePendingAction(ctx, first); err != nil {
		t.Fatal(err)
	}

	consumed, err := store.ConsumePendingAction(ctx, 2002, "unbind_node")
	if err != nil {
		t.Fatal(err)
	}
	if consumed == nil {
		t.Fatal("expected consumed action, got nil")
	}
	if consumed.Status != pendingActionStatusConsumed || consumed.ConsumedAt == nil {
		t.Fatalf("expected consumed action state, got %+v", consumed)
	}

	consumed, err = store.ConsumePendingAction(ctx, 2002, "unbind_node")
	if err != nil {
		t.Fatal(err)
	}
	if consumed != nil {
		t.Fatalf("expected no pending action left, got %+v", consumed)
	}

	expiredCandidate := &PendingAction{
		TelegramUser: 2003,
		Action:       "bind_node",
		PayloadJSON:  `{"node_id":"n2"}`,
		ExpiresAt:    time.Now().UTC().Add(-5 * time.Minute),
		Status:       pendingActionStatusPending,
	}
	activeCandidate := &PendingAction{
		TelegramUser: 2004,
		Action:       "bind_node",
		PayloadJSON:  `{"node_id":"n3"}`,
		ExpiresAt:    time.Now().UTC().Add(5 * time.Minute),
	}
	if err := db.Create(expiredCandidate).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.CreatePendingAction(ctx, activeCandidate); err != nil {
		t.Fatal(err)
	}

	updated, err := store.MarkPendingActionExpired(ctx, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("expected one expired action, got %d", updated)
	}

	consumed, err = store.ConsumePendingAction(ctx, 2004, "bind_node")
	if err != nil {
		t.Fatal(err)
	}
	if consumed == nil {
		t.Fatal("expected active action to be consumable")
	}
}

func TestPendingActionStoreRejectsInvalidAction(t *testing.T) {
	db := openTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	store := NewPendingActionStore(db)
	ctx := context.Background()

	err := store.CreatePendingAction(ctx, &PendingAction{
		TelegramUser: 2005,
		ExpiresAt:    time.Now().UTC().Add(5 * time.Minute),
	})
	if err == nil {
		t.Fatal("expected empty action validation error")
	}

	err = store.CreatePendingAction(ctx, &PendingAction{
		TelegramUser: 2005,
		Action:       "bind_node",
	})
	if err == nil {
		t.Fatal("expected expires at validation error")
	}
}

func TestActionLogStoreListRecent(t *testing.T) {
	db := openTestDB(t)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	store := NewActionLogStore(db)
	ctx := context.Background()
	base := time.Now().UTC()

	logs := []*ActionLog{
		{
			TelegramUser: 3001,
			AdminID:      1,
			Action:       "create",
			Target:       "node-a",
			MetadataJSON: `{"k":"v1"}`,
			Result:       "ok",
			CreatedAt:    base.Add(-3 * time.Minute),
		},
		{
			TelegramUser: 3001,
			AdminID:      1,
			Action:       "update",
			Target:       "node-b",
			MetadataJSON: `{"k":"v2"}`,
			Result:       "ok",
			CreatedAt:    base.Add(-2 * time.Minute),
		},
		{
			TelegramUser: 3001,
			AdminID:      1,
			Action:       "delete",
			Target:       "node-c",
			MetadataJSON: `{"k":"v3"}`,
			Result:       "ok",
			CreatedAt:    base.Add(-1 * time.Minute),
		},
	}

	for _, entry := range logs {
		if err := store.CreateActionLog(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}

	recent, err := store.ListRecentActionLogs(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(recent))
	}
	if recent[0].Action != "delete" || recent[1].Action != "update" {
		t.Fatalf("unexpected order: %+v", recent)
	}
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "storage_test.db")
	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func assertTableExists(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	if !db.Migrator().HasTable(table) {
		t.Fatalf("expected table %q to exist", table)
	}
}
