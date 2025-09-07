package main

import (
	"context"
	"log"

	usecases_user "github.com/BoburF/golang-bot-converter/application/usecases/user"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type BotHandler struct {
	registerUserUsecase *usecases_user.RegisterUserUsecase
}

func NewBotHandler(registerUserUsecase *usecases_user.RegisterUserUsecase) *BotHandler {
	return &BotHandler{registerUserUsecase: registerUserUsecase}
}

func (h *BotHandler) HandleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	if update.Message.Contact != nil {
		user, err := h.registerUserUsecase.Execute(update.Message.From.FirstName, update.Message.Contact.PhoneNumber)
		if err != nil {
			log.Print(err)
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Something went wrong!",
			})
			return
		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Welcome " + user.Name + "!",
		})
		return
	}

	switch update.Message.Text {
	case "/start":
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Please share your phone number to continue:",
			ReplyMarkup: &models.ReplyKeyboardMarkup{
				Keyboard: [][]models.KeyboardButton{
					{
						{
							Text:           "Share phone number 📱",
							RequestContact: true,
						},
					},
				},
				ResizeKeyboard:  true,
				OneTimeKeyboard: true,
			},
		})
	}
}
