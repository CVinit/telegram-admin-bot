package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/storage"
)

const (
	resultStarted   = "started"
	resultSucceeded = "succeeded"
	resultFailed    = "failed"
)

type actionLogStore interface {
	CreateActionLog(ctx context.Context, log *storage.ActionLog) error
	ListRecentActionLogs(ctx context.Context, limit int) ([]storage.ActionLog, error)
}

type Service struct {
	logs actionLogStore
	now  func() time.Time
}

func NewService(logs *storage.ActionLogStore) *Service {
	return newService(logs, time.Now)
}

func newService(logs actionLogStore, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		logs: logs,
		now:  now,
	}
}

func (s *Service) LogActionStarted(ctx context.Context, telegramUser int64, adminID uint, action, target string, metadata map[string]any) error {
	return s.log(ctx, telegramUser, adminID, action, target, resultStarted, metadata)
}

func (s *Service) LogActionSucceeded(ctx context.Context, telegramUser int64, adminID uint, action, target, result string, metadata map[string]any) error {
	return s.log(ctx, telegramUser, adminID, action, target, composeResult(resultSucceeded, result), metadata)
}

func (s *Service) LogActionFailed(ctx context.Context, telegramUser int64, adminID uint, action, target, result string, metadata map[string]any) error {
	return s.log(ctx, telegramUser, adminID, action, target, composeResult(resultFailed, result), metadata)
}

func (s *Service) RenderRecentActionSummary(ctx context.Context, telegramUser int64, limit int) (string, error) {
	if telegramUser <= 0 {
		return "", errors.New("telegram user must be positive")
	}
	if s.logs == nil {
		return "", errors.New("action log store is nil")
	}
	if limit <= 0 {
		limit = 5
	}

	fetchLimit := limit * 3
	if fetchLimit < 20 {
		fetchLimit = 20
	}
	recent, err := s.logs.ListRecentActionLogs(ctx, fetchLimit)
	if err != nil {
		return "", fmt.Errorf("list recent action logs: %w", err)
	}

	filtered := make([]storage.ActionLog, 0, limit)
	for _, entry := range recent {
		if entry.TelegramUser != telegramUser {
			continue
		}
		filtered = append(filtered, entry)
		if len(filtered) >= limit {
			break
		}
	}
	if len(filtered) == 0 {
		return "No recent actions.", nil
	}

	var builder strings.Builder
	builder.WriteString("Recent actions:\n")
	for i, entry := range filtered {
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(renderActionLine(entry))
	}
	return builder.String(), nil
}

func (s *Service) log(ctx context.Context, telegramUser int64, adminID uint, action, target, result string, metadata map[string]any) error {
	if telegramUser <= 0 {
		return errors.New("telegram user must be positive")
	}
	if adminID == 0 {
		return errors.New("admin ID must be positive")
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return errors.New("action is required")
	}
	if s.logs == nil {
		return errors.New("action log store is nil")
	}

	metadataJSON, err := encodeMetadata(metadata)
	if err != nil {
		return err
	}

	entry := &storage.ActionLog{
		TelegramUser: telegramUser,
		AdminID:      adminID,
		Action:       action,
		Target:       strings.TrimSpace(target),
		MetadataJSON: metadataJSON,
		Result:       result,
		CreatedAt:    s.now().UTC(),
	}
	if err := s.logs.CreateActionLog(ctx, entry); err != nil {
		return fmt.Errorf("create action log: %w", err)
	}
	return nil
}

func encodeMetadata(metadata map[string]any) (string, error) {
	if len(metadata) == 0 {
		return "{}", nil
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	return string(payload), nil
}

func composeResult(state, details string) string {
	details = strings.TrimSpace(details)
	if details == "" {
		return state
	}
	return state + ":" + details
}

func renderActionLine(entry storage.ActionLog) string {
	createdAt := entry.CreatedAt.UTC().Format("2006-01-02 15:04:05")
	if entry.Target == "" {
		return fmt.Sprintf("[%s] %s (%s)", createdAt, entry.Action, entry.Result)
	}
	return fmt.Sprintf("[%s] %s %s (%s)", createdAt, entry.Action, entry.Target, entry.Result)
}
