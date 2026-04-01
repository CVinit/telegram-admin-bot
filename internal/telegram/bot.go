package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const maxTelegramDocumentBytes = 2 << 20

type Runtime struct {
	api    *tgbot.Bot
	router *Router
}

func NewRuntime(token string, router *Router) (*Runtime, error) {
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("telegram bot token is required")
	}
	if router == nil {
		return nil, errors.New("telegram router is nil")
	}

	runtime := &Runtime{router: router}
	api, err := tgbot.New(
		token,
		tgbot.WithSkipGetMe(),
		tgbot.WithDefaultHandler(runtime.handleUpdate),
	)
	if err != nil {
		return nil, err
	}
	runtime.api = api
	return runtime, nil
}

func (r *Runtime) Run(ctx context.Context) error {
	if r == nil || r.api == nil {
		return errors.New("telegram runtime is not initialized")
	}
	r.api.Start(ctx)
	return nil
}

func (r *Runtime) handleUpdate(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	incoming, err := r.fromTelegramUpdate(ctx, b, update)
	if incoming == nil {
		return
	}
	if err != nil {
		if incoming.ChatID != 0 {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: incoming.ChatID,
				Text:   "读取上传文件失败: " + err.Error(),
			})
		}
		return
	}

	response, err := r.router.Handle(ctx, *incoming)
	if err != nil {
		if incoming.ChatID != 0 {
			_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: incoming.ChatID,
				Text:   "处理请求失败: " + err.Error(),
			})
		}
		return
	}
	if response == nil {
		return
	}

	if incoming.CallbackID != "" {
		_, _ = b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
			CallbackQueryID: incoming.CallbackID,
			Text:            strings.TrimSpace(response.CallbackText),
		})
	}

	if incoming.ChatID == 0 || strings.TrimSpace(response.Text) == "" {
		return
	}

	params := &tgbot.SendMessageParams{
		ChatID: incoming.ChatID,
		Text:   response.Text,
	}
	if markup := buildReplyMarkup(response); markup != nil {
		params.ReplyMarkup = markup
	}
	_, _ = b.SendMessage(ctx, params)
}

func (r *Runtime) fromTelegramUpdate(ctx context.Context, b *tgbot.Bot, update *models.Update) (*IncomingUpdate, error) {
	if update == nil {
		return nil, nil
	}
	if update.Message != nil {
		incoming := &IncomingUpdate{
			TelegramUser: userIDFromMessage(update.Message),
			ChatID:       update.Message.Chat.ID,
			ChatType:     ChatType(update.Message.Chat.Type),
			Text:         update.Message.Text,
		}
		if strings.TrimSpace(update.Message.Caption) != "" {
			incoming.Text = update.Message.Caption
		}
		if update.Message.Document != nil {
			incoming.DocumentName = strings.TrimSpace(update.Message.Document.FileName)
			data, err := downloadTelegramDocument(ctx, b, update.Message.Document)
			if err != nil {
				return incoming, err
			}
			incoming.DocumentData = data
		}
		return incoming, nil
	}
	if update.CallbackQuery != nil {
		incoming := &IncomingUpdate{
			TelegramUser: update.CallbackQuery.From.ID,
			CallbackID:   update.CallbackQuery.ID,
			CallbackData: update.CallbackQuery.Data,
		}
		switch update.CallbackQuery.Message.Type {
		case models.MaybeInaccessibleMessageTypeMessage:
			if update.CallbackQuery.Message.Message != nil {
				incoming.ChatID = update.CallbackQuery.Message.Message.Chat.ID
				incoming.ChatType = ChatType(update.CallbackQuery.Message.Message.Chat.Type)
			}
		case models.MaybeInaccessibleMessageTypeInaccessibleMessage:
			if update.CallbackQuery.Message.InaccessibleMessage != nil {
				incoming.ChatID = update.CallbackQuery.Message.InaccessibleMessage.Chat.ID
				incoming.ChatType = ChatType(update.CallbackQuery.Message.InaccessibleMessage.Chat.Type)
			}
		}
		return incoming, nil
	}
	return nil, nil
}

func userIDFromMessage(message *models.Message) int64 {
	if message == nil || message.From == nil {
		return 0
	}
	return message.From.ID
}

func buildReplyMarkup(response *Response) models.ReplyMarkup {
	if response == nil {
		return nil
	}
	if response.RemoveKeyboard {
		return &models.ReplyKeyboardRemove{RemoveKeyboard: true}
	}
	if len(response.Keyboard) == 0 {
		return nil
	}

	switch response.KeyboardKind {
	case KeyboardKindReply:
		keyboard := make([][]models.KeyboardButton, 0, len(response.Keyboard))
		for _, row := range response.Keyboard {
			if len(row) == 0 {
				continue
			}
			buttons := make([]models.KeyboardButton, 0, len(row))
			for _, button := range row {
				buttons = append(buttons, models.KeyboardButton{Text: button.Text})
			}
			keyboard = append(keyboard, buttons)
		}
		if len(keyboard) == 0 {
			return nil
		}
		return &models.ReplyKeyboardMarkup{
			Keyboard:       keyboard,
			ResizeKeyboard: true,
			IsPersistent:   true,
		}
	case KeyboardKindInline:
		keyboard := make([][]models.InlineKeyboardButton, 0, len(response.Keyboard))
		for _, row := range response.Keyboard {
			if len(row) == 0 {
				continue
			}
			buttons := make([]models.InlineKeyboardButton, 0, len(row))
			for _, button := range row {
				buttons = append(buttons, models.InlineKeyboardButton{
					Text:         button.Text,
					CallbackData: button.CallbackData,
				})
			}
			keyboard = append(keyboard, buttons)
		}
		if len(keyboard) == 0 {
			return nil
		}
		return &models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	default:
		return nil
	}
}

func downloadTelegramDocument(ctx context.Context, b *tgbot.Bot, document *models.Document) ([]byte, error) {
	if b == nil || document == nil {
		return nil, errors.New("telegram document is nil")
	}

	file, err := b.GetFile(ctx, &tgbot.GetFileParams{FileID: document.FileID})
	if err != nil {
		return nil, fmt.Errorf("get telegram file: %w", err)
	}
	if file == nil || strings.TrimSpace(file.FilePath) == "" {
		return nil, errors.New("telegram file path is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.FileDownloadLink(file), nil)
	if err != nil {
		return nil, fmt.Errorf("create telegram download request: %w", err)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download telegram file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("telegram file download returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTelegramDocumentBytes))
	if err != nil {
		return nil, fmt.Errorf("read telegram file content: %w", err)
	}
	return data, nil
}
