package main

import (
	"context"
	"log"

	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/app"
	"github.com/dujiao-next/dujiao-next/telegram-admin-bot/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	appInstance, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := appInstance.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
