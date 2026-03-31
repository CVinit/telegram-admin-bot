package storage

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type ActionLogStore struct {
	db *gorm.DB
}

func NewActionLogStore(db *gorm.DB) *ActionLogStore {
	return &ActionLogStore{db: db}
}

func (s *ActionLogStore) CreateActionLog(ctx context.Context, log *ActionLog) error {
	if log == nil {
		return errors.New("log is nil")
	}
	return s.db.WithContext(ctx).Create(log).Error
}

func (s *ActionLogStore) ListRecentActionLogs(ctx context.Context, limit int) ([]ActionLog, error) {
	if limit <= 0 {
		limit = 20
	}

	var logs []ActionLog
	err := s.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
