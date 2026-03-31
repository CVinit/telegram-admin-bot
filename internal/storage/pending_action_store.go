package storage

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type PendingActionStore struct {
	db *gorm.DB
}

func NewPendingActionStore(db *gorm.DB) *PendingActionStore {
	return &PendingActionStore{db: db}
}

func (s *PendingActionStore) CreatePendingAction(ctx context.Context, action *PendingAction) error {
	if action == nil {
		return errors.New("action is nil")
	}
	if action.TelegramUser <= 0 {
		return errors.New("telegram user must be positive")
	}
	action.Action = strings.TrimSpace(action.Action)
	if action.Action == "" {
		return errors.New("action is required")
	}
	if action.ExpiresAt.IsZero() || !action.ExpiresAt.After(time.Now().UTC()) {
		return errors.New("expires at must be in the future")
	}
	if action.Status == "" {
		action.Status = pendingActionStatusPending
	}
	return s.db.WithContext(ctx).Create(action).Error
}

func (s *PendingActionStore) ConsumePendingAction(ctx context.Context, telegramUser int64, action string) (*PendingAction, error) {
	var result PendingAction
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Where(
			"telegram_user = ? AND action = ? AND status = ? AND expires_at > ?",
			telegramUser,
			action,
			pendingActionStatusPending,
			time.Now().UTC(),
		).
			Order("created_at ASC, id ASC").
			Take(&result).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		result.Status = pendingActionStatusConsumed
		result.ConsumedAt = &now

		update := tx.Model(&PendingAction{}).
			Where("id = ? AND status = ?", result.ID, pendingActionStatusPending).
			Updates(map[string]any{
				"status":      result.Status,
				"consumed_at": result.ConsumedAt,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			result = PendingAction{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.ID == 0 {
		return nil, nil
	}
	return &result, nil
}

func (s *PendingActionStore) MarkPendingActionExpired(ctx context.Context, now time.Time) (int64, error) {
	res := s.db.WithContext(ctx).Model(&PendingAction{}).
		Where("status = ? AND expires_at <= ?", pendingActionStatusPending, now.UTC()).
		Update("status", pendingActionStatusExpired)
	return res.RowsAffected, res.Error
}
