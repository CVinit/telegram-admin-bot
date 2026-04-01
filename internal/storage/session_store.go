package storage

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SessionStore struct {
	db *gorm.DB
}

func NewSessionStore(db *gorm.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) UpsertSession(ctx context.Context, session *AdminSession) error {
	if session == nil {
		return errors.New("session is nil")
	}
	if session.TelegramUser <= 0 {
		return errors.New("telegram user must be positive")
	}
	if session.AdminID == 0 {
		return errors.New("admin ID is required")
	}

	now := time.Now().UTC()

	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "telegram_user"}},
		DoUpdates: clause.Assignments(map[string]any{
			"admin_id":       session.AdminID,
			"is_super":       session.IsSuper,
			"username":       session.Username,
			"jwt_token":      session.JWTToken,
			"jwt_expires_at": session.JWTExpiresAt,
			"roles_json":     session.RolesJSON,
			"policies_json":  session.PoliciesJSON,
			"last_login_at":  session.LastLoginAt,
			"last_used_at":   session.LastUsedAt,
			"updated_at":     now,
		}),
	}).Create(session).Error
}

func (s *SessionStore) GetSessionByTelegramUser(ctx context.Context, telegramUser int64) (*AdminSession, error) {
	var session AdminSession
	err := s.db.WithContext(ctx).
		Where("telegram_user = ?", telegramUser).
		Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *SessionStore) DeleteSessionByTelegramUser(ctx context.Context, telegramUser int64) error {
	return s.db.WithContext(ctx).
		Where("telegram_user = ?", telegramUser).
		Delete(&AdminSession{}).Error
}
