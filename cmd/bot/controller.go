package main

import (
	"context"

	"github.com/BoburF/golang-bot-converter/domain"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type BotHandler struct {
	userRepo domain.UserRepository
}

func NewBotHandler(repo domain.UserRepository) *BotHandler {
	return &BotHandler{userRepo: repo}
}

func (h *BotHandler) HandleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	phone := update.Message.From.Username

	_, err := h.userRepo.GetByPhone(phone)
	if err != nil {
		_ = h.userRepo.Create(update.Message.From.FirstName, phone)
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Welcome " + update.Message.From.FirstName,
	})
}
