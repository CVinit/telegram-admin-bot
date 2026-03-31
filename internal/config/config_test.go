package config

import "testing"

func TestLoadConfigRequiresBotToken(t *testing.T) {
	_, err := LoadFromEnv(map[string]string{
		"DUJIAO_BASE_URL": "http://127.0.0.1:8080/api/v1",
		"SQLITE_PATH":     "./data/bot.db",
	})
	if err == nil {
		t.Fatal("expected bot token validation error")
	}
}

func TestLoadConfigRequiresBaseURL(t *testing.T) {
	_, err := LoadFromEnv(map[string]string{
		"TELEGRAM_BOT_TOKEN": "bot-token",
		"SQLITE_PATH":        "./data/bot.db",
	})
	if err == nil {
		t.Fatal("expected base URL validation error")
	}
}

func TestLoadConfigRequiresSQLitePath(t *testing.T) {
	_, err := LoadFromEnv(map[string]string{
		"TELEGRAM_BOT_TOKEN": "bot-token",
		"DUJIAO_BASE_URL":    "http://127.0.0.1:8080/api/v1",
	})
	if err == nil {
		t.Fatal("expected sqlite path validation error")
	}
}

func TestLoadConfigRejectsInvalidIntegerSettings(t *testing.T) {
	_, err := LoadFromEnv(map[string]string{
		"TELEGRAM_BOT_TOKEN":          "bot-token",
		"DUJIAO_BASE_URL":             "http://127.0.0.1:8080/api/v1",
		"SQLITE_PATH":                 "./data/bot.db",
		"ACTION_CONFIRM_TTL_SECONDS":  "0",
		"SESSION_EXPIRE_SKEW_SECONDS": "-1",
	})
	if err == nil {
		t.Fatal("expected integer validation error")
	}
}
