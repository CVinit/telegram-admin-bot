package app

import (
	"context"

	"github.com/dujiao-next/telegram-admin-bot/internal/config"
)

type App struct {
	cfg config.Config
}

func New(cfg config.Config) (*App, error) {
	return &App{cfg: cfg}, nil
}

func (a *App) Run(ctx context.Context) error {
	_ = ctx
	return nil
}
