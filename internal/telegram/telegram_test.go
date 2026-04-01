package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
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
