package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultActionConfirmTTLSeconds  = 300
	defaultSessionExpireSkewSeconds = 60
	defaultLogLevel                 = "info"
)

type Config struct {
	BotToken                 string
	DujiaoBaseURL            string
	SQLitePath               string
	ActionConfirmTTLSeconds  int
	SessionExpireSkewSeconds int
	LogLevel                 string
}

func Load() (Config, error) {
	env := map[string]string{
		"TELEGRAM_BOT_TOKEN":          os.Getenv("TELEGRAM_BOT_TOKEN"),
		"DUJIAO_BASE_URL":             os.Getenv("DUJIAO_BASE_URL"),
		"SQLITE_PATH":                 os.Getenv("SQLITE_PATH"),
		"ACTION_CONFIRM_TTL_SECONDS":  os.Getenv("ACTION_CONFIRM_TTL_SECONDS"),
		"SESSION_EXPIRE_SKEW_SECONDS": os.Getenv("SESSION_EXPIRE_SKEW_SECONDS"),
		"LOG_LEVEL":                   os.Getenv("LOG_LEVEL"),
	}
	return LoadFromEnv(env)
}

func LoadFromEnv(env map[string]string) (Config, error) {
	cfg := Config{
		BotToken:                 strings.TrimSpace(env["TELEGRAM_BOT_TOKEN"]),
		DujiaoBaseURL:            strings.TrimSpace(env["DUJIAO_BASE_URL"]),
		SQLitePath:               strings.TrimSpace(env["SQLITE_PATH"]),
		ActionConfirmTTLSeconds:  defaultActionConfirmTTLSeconds,
		SessionExpireSkewSeconds: defaultSessionExpireSkewSeconds,
		LogLevel:                 defaultLogLevel,
	}

	if cfg.BotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.DujiaoBaseURL == "" {
		return Config{}, fmt.Errorf("DUJIAO_BASE_URL is required")
	}
	if cfg.SQLitePath == "" {
		return Config{}, fmt.Errorf("SQLITE_PATH is required")
	}

	if raw := strings.TrimSpace(env["ACTION_CONFIRM_TTL_SECONDS"]); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("ACTION_CONFIRM_TTL_SECONDS must be a positive integer")
		}
		cfg.ActionConfirmTTLSeconds = value
	}

	if raw := strings.TrimSpace(env["SESSION_EXPIRE_SKEW_SECONDS"]); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return Config{}, fmt.Errorf("SESSION_EXPIRE_SKEW_SECONDS must be a non-negative integer")
		}
		cfg.SessionExpireSkewSeconds = value
	}

	if raw := strings.TrimSpace(env["LOG_LEVEL"]); raw != "" {
		cfg.LogLevel = raw
	}

	return cfg, nil
}
