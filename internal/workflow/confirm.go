package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/CVinit/telegram-admin-bot/internal/storage"
)

var ErrConfirmationNotFound = errors.New("confirmation not found or expired")

type pendingActionStore interface {
	CreatePendingAction(ctx context.Context, action *storage.PendingAction) error
	ConsumePendingAction(ctx context.Context, telegramUser int64, action string) (*storage.PendingAction, error)
}

type ConfirmationService struct {
	store pendingActionStore
	ttl   time.Duration
	now   func() time.Time
}

type ConfirmationRecord struct {
	ActionKey string
	ExpiresAt time.Time
}

func NewConfirmationService(store pendingActionStore, ttl time.Duration) *ConfirmationService {
	return newConfirmationService(store, ttl, time.Now)
}

func newConfirmationService(store pendingActionStore, ttl time.Duration, now func() time.Time) *ConfirmationService {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if now == nil {
		now = time.Now
	}
	return &ConfirmationService{
		store: store,
		ttl:   ttl,
		now:   now,
	}
}

func (s *ConfirmationService) Create(ctx context.Context, telegramUser int64, prefix string, payload any) (*ConfirmationRecord, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("confirmation store is nil")
	}
	if telegramUser <= 0 {
		return nil, errors.New("telegram user must be positive")
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, errors.New("confirmation prefix is required")
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal confirmation payload: %w", err)
	}

	now := s.now().UTC()
	record := &ConfirmationRecord{
		ActionKey: prefix + ":" + strconv.FormatInt(telegramUser, 10) + ":" + strconv.FormatInt(now.UnixNano(), 10),
		ExpiresAt: now.Add(s.ttl),
	}
	if err := s.store.CreatePendingAction(ctx, &storage.PendingAction{
		TelegramUser: telegramUser,
		Action:       record.ActionKey,
		PayloadJSON:  string(rawPayload),
		ExpiresAt:    record.ExpiresAt,
	}); err != nil {
		return nil, fmt.Errorf("create pending action: %w", err)
	}

	return record, nil
}

func (s *ConfirmationService) Consume(ctx context.Context, telegramUser int64, actionKey string, out any) (*storage.PendingAction, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("confirmation store is nil")
	}
	if telegramUser <= 0 {
		return nil, errors.New("telegram user must be positive")
	}
	actionKey = strings.TrimSpace(actionKey)
	if actionKey == "" {
		return nil, errors.New("confirmation action key is required")
	}

	action, err := s.store.ConsumePendingAction(ctx, telegramUser, actionKey)
	if err != nil {
		return nil, fmt.Errorf("consume pending action: %w", err)
	}
	if action == nil {
		return nil, ErrConfirmationNotFound
	}
	if out != nil {
		if err := json.Unmarshal([]byte(action.PayloadJSON), out); err != nil {
			return nil, fmt.Errorf("decode confirmation payload: %w", err)
		}
	}
	return action, nil
}
