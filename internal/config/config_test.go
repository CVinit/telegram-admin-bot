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
