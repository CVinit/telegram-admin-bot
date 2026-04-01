package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/config"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/dujiao"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/storage"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/telegram"
)

type App struct {
	cfg     config.Config
	runtime *telegram.Runtime
}

func New(cfg config.Config) (*App, error) {
	if err := ensureSQLiteDir(cfg.SQLitePath); err != nil {
		return nil, err
	}

	db, err := storage.OpenSQLite(cfg.SQLitePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := storage.Migrate(db); err != nil {
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}

	apiClient := dujiao.New(cfg.DujiaoBaseURL)
	sessionStore := storage.NewSessionStore(db)
	sessionService := session.NewService(apiClient, sessionStore)
	router := telegram.NewRouter(sessionService)
	runtime, err := telegram.NewRuntime(cfg.BotToken, router)
	if err != nil {
		return nil, fmt.Errorf("create telegram runtime: %w", err)
	}

	return &App{
		cfg:     cfg,
		runtime: runtime,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	if a == nil || a.runtime == nil {
		return fmt.Errorf("app runtime is not initialized")
	}
	return a.runtime.Run(ctx)
}

func ensureSQLiteDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite directory: %w", err)
	}
	return nil
}
