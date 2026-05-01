package session

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/CVinit/telegram-admin-bot/internal/dujiao"
	"github.com/CVinit/telegram-admin-bot/internal/storage"
	"gorm.io/gorm"
)

func TestLoginPersistsTelegramAdminSession(t *testing.T) {
	deps := newSessionTestDeps(t)
	_, err := deps.Service.Login(context.Background(), 123456789, "ops", "secret")
	if err != nil {
		t.Fatal(err)
	}
	session, err := deps.Store.GetSessionByTelegramUser(context.Background(), 123456789)
	if err != nil {
		t.Fatal(err)
	}
	if session.AdminID == 0 {
		t.Fatal("expected admin id to be stored")
	}
}

func TestRequireSessionRejectsExpiredSession(t *testing.T) {
	deps := newSessionTestDeps(t)

	err := deps.Store.UpsertSession(context.Background(), &storage.AdminSession{
		TelegramUser: 123456790,
		AdminID:      9,
		Username:     "ops",
		JWTToken:     "expired-token",
		JWTExpiresAt: deps.Now.Add(-1 * time.Minute),
		RolesJSON:    `["operator"]`,
		PoliciesJSON: `[{"subject":"admin","object":"orders","action":"write"}]`,
		LastLoginAt:  deps.Now.Add(-10 * time.Minute),
		LastUsedAt:   deps.Now.Add(-1 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = deps.Service.RequireSession(context.Background(), 123456790)
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}

func TestLogoutDeletesStoredSession(t *testing.T) {
	deps := newSessionTestDeps(t)
	_, err := deps.Service.Login(context.Background(), 123456791, "ops", "secret")
	if err != nil {
		t.Fatal(err)
	}

	if err := deps.Service.Logout(context.Background(), 123456791); err != nil {
		t.Fatal(err)
	}

	session, err := deps.Store.GetSessionByTelegramUser(context.Background(), 123456791)
	if err != nil {
		t.Fatal(err)
	}
	if session != nil {
		t.Fatalf("expected no session after logout, got %+v", session)
	}
}

func TestRequireSessionRestoresIsSuperFlag(t *testing.T) {
	deps := newSessionTestDeps(t)

	if _, err := deps.Service.Login(context.Background(), 123456792, "ops", "secret"); err != nil {
		t.Fatal(err)
	}

	view, err := deps.Service.RequireSession(context.Background(), 123456792)
	if err != nil {
		t.Fatal(err)
	}
	if !view.IsSuper {
		t.Fatalf("expected IsSuper to persist across session reload, got %#v", view)
	}
}

type sessionTestDeps struct {
	Service *Service
	Store   *storage.SessionStore
	Now     time.Time
}

func newSessionTestDeps(t *testing.T) *sessionTestDeps {
	t.Helper()

	db := openSessionServiceTestDB(t)
	store := storage.NewSessionStore(db)
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	api := &stubAuthAPI{
		loginResp: &dujiao.LoginResponse{
			Token:     "jwt-token-1",
			User:      dujiao.LoginAdminUser{ID: 3, Username: "ops"},
			ExpiresAt: now.Add(2 * time.Hour).Format(time.RFC3339),
		},
		authzResp: &dujiao.AuthzMeResponse{
			AdminID: 3,
			IsSuper: true,
			Roles:   []string{"operator"},
			Policies: []dujiao.AuthzPolicy{
				{
					Subject: "admin",
					Object:  "/api/v1/admin/products",
					Action:  "write",
				},
			},
		},
	}

	service := newService(api, store, func() time.Time { return now })
	return &sessionTestDeps{
		Service: service,
		Store:   store,
		Now:     now,
	}
}

func openSessionServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session_service_test.db")
	db, err := storage.OpenSQLite(path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := storage.Migrate(db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

type stubAuthAPI struct {
	loginResp *dujiao.LoginResponse
	loginErr  error
	authzResp *dujiao.AuthzMeResponse
	authzErr  error
}

func (s *stubAuthAPI) Login(_ context.Context, _, _ string) (*dujiao.LoginResponse, error) {
	if s.loginErr != nil {
		return nil, s.loginErr
	}
	return s.loginResp, nil
}

func (s *stubAuthAPI) GetAuthzMe(_ context.Context, _ string) (*dujiao.AuthzMeResponse, error) {
	if s.authzErr != nil {
		return nil, s.authzErr
	}
	return s.authzResp, nil
}
