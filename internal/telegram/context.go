package telegram

import "strings"

type ChatType string

const (
	ChatTypePrivate    ChatType = "private"
	ChatTypeGroup      ChatType = "group"
	ChatTypeSupergroup ChatType = "supergroup"
	ChatTypeChannel    ChatType = "channel"
)

const (
	CallbackMenuHome     = "menu:home"
	privateChatOnlyText  = "敏感命令仅支持私聊。"
	notImplementedSuffix = "功能尚未接入，后续任务会补上。"
)

type KeyboardKind string

const (
	KeyboardKindNone   KeyboardKind = ""
	KeyboardKindReply  KeyboardKind = "reply"
	KeyboardKindInline KeyboardKind = "inline"
)

type IncomingUpdate struct {
	TelegramUser int64
	ChatID       int64
	ChatType     ChatType
	Text         string
	DocumentName string
	DocumentData []byte
	CallbackID   string
	CallbackData string
}

func (u IncomingUpdate) IsPrivateChat() bool {
	return u.ChatType == ChatTypePrivate
}

type Button struct {
	Text         string
	CallbackData string
}

type Response struct {
	Text           string
	Keyboard       [][]Button
	KeyboardKind   KeyboardKind
	RemoveKeyboard bool
	CallbackID     string
	CallbackText   string
}

func trimCommandToken(raw string) string {
	command := strings.TrimSpace(raw)
	if command == "" {
		return ""
	}
	if idx := strings.IndexByte(command, '@'); idx >= 0 {
		command = command[:idx]
	}
	return strings.ToLower(command)
}
