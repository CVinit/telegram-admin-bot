package telegram

import (
	"context"
	"errors"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

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
	incoming := fromTelegramUpdate(update)
	if incoming == nil {
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

func fromTelegramUpdate(update *models.Update) *IncomingUpdate {
	if update == nil {
		return nil
	}
	if update.Message != nil {
		return &IncomingUpdate{
			TelegramUser: userIDFromMessage(update.Message),
			ChatID:       update.Message.Chat.ID,
			ChatType:     ChatType(update.Message.Chat.Type),
			Text:         update.Message.Text,
		}
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
		return incoming
	}
	return nil
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
	if response.KeyboardKind != KeyboardKindReply || len(response.Keyboard) == 0 {
		return nil
	}

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
}
