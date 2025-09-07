/*
Package botcommands provides a helper for registering bot commands
with the Telegram API. These commands will be visible in the bot's
command menu in the Telegram client.
*/
package botcommands

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func SetCommands(bot *tgbotapi.BotAPI) []tgbotapi.BotCommand {
	botCommands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Start the bot"},
	}

	cfg := tgbotapi.NewSetMyCommands(botCommands...)
	if _, err := bot.Request(cfg); err != nil {
		log.Fatal(err)
	}

	return botCommands
}
