package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/audit"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/config"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/dujiao"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/session"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/storage"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/telegram"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/workflow"
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
	pendingStore := storage.NewPendingActionStore(db)
	actionLogStore := storage.NewActionLogStore(db)
	sessionService := session.NewService(apiClient, sessionStore)
	auditService := audit.NewService(actionLogStore)
	confirmationService := workflow.NewConfirmationService(pendingStore, time.Duration(cfg.ActionConfirmTTLSeconds)*time.Second)
	salesWorkflow := workflow.NewSalesWorkflow(apiClient)
	restockWorkflow := workflow.NewRestockWorkflow(apiClient, confirmationService, auditService)
	fulfillmentWorkflow := workflow.NewFulfillmentWorkflow(apiClient, confirmationService, auditService)
	router := telegram.NewRouter(sessionService).
		WithSalesWorkflow(salesWorkflow).
		WithRestockWorkflow(restockWorkflow).
		WithFulfillmentWorkflow(fulfillmentWorkflow)
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
