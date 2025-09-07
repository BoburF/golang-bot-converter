package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"

	botcommands "github.com/BoburF/golang-bot-converter/cmd/bot/bot-commands"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	botcommands.SetCommands(bot)

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if len(update.Message.Photo) > 0 {
			photo := update.Message.Photo[len(update.Message.Photo)-1]
			file, err := bot.GetFile(tgbotapi.FileConfig{FileID: photo.FileID})
			if err != nil {
				log.Println("Error getting file:", err)
				continue
			}

			url := file.Link(token)
			inputFile := "input.png"
			outputFile := "output.jpg"

			if err := downloadFile(inputFile, url); err != nil {
				log.Println("Error downloading file:", err)
				continue
			}

			cmd := exec.Command("ffmpeg", "-y", "-i", inputFile, outputFile)
			if err := cmd.Run(); err != nil {
				log.Println("ffmpeg error:", err)
				continue
			}

			photoMsg := tgbotapi.NewPhoto(update.Message.Chat.ID, tgbotapi.FilePath(outputFile))
			bot.Send(photoMsg)

			os.Remove(inputFile)
			os.Remove(outputFile)
		} else {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Привет, "+update.Message.From.FirstName+"! Пришли фото, и я сконвертирую его в JPG.")
			bot.Send(msg)
		}
	}
}

func downloadFile(filepath string, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = out.ReadFrom(resp.Body)
	return err
}
