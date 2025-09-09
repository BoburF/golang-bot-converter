package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"

	usecases_user "github.com/BoburF/golang-bot-converter/application/usecases/user"
	"github.com/BoburF/golang-bot-converter/domain"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type BotHandler struct {
	registerUserUsecase *usecases_user.RegisterUserUsecase
	lastPhoto           map[int64]string
}

func NewBotHandler(registerUserUsecase *usecases_user.RegisterUserUsecase) *BotHandler {
	return &BotHandler{registerUserUsecase: registerUserUsecase, lastPhoto: make(map[int64]string)}
}

func (h *BotHandler) HandleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery != nil {
		h.handleCallback(ctx, b, update.CallbackQuery)
		return
	}

	if update.Message == nil {
		return
	}

	if update.Message.Photo != nil {
		if _, exists := h.lastPhoto[update.Message.Chat.ID]; exists {
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "You already sent a photo. Please finish the conversion first.",
			})
			return
		}

		fileID := update.Message.Photo[len(update.Message.Photo)-1].FileID
		h.lastPhoto[update.Message.Chat.ID] = fileID

		var buttons [][]models.InlineKeyboardButton
		for _, format := range domain.ConvertToFormats {
			buttons = append(buttons, []models.InlineKeyboardButton{
				{
					Text:         "Convert to " + format,
					CallbackData: "convert_" + format,
				},
			})
		}

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Choose a format to convert your image:",
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: buttons,
			},
		})
		return
	}

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

		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "You can send any image and I will convert it to another type!",
		})
		return
	}

	switch update.Message.Text {
	case "/start":
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
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

func (h *BotHandler) handleCallback(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery) {
	chatID := cq.Message.Message.Chat.ID

	fileID, ok := h.lastPhoto[chatID]
	if !ok {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Please send a photo first.",
		})
		return
	}
	defer delete(h.lastPhoto, chatID)

	var format string
	switch cq.Data {
	case "convert_png":
		format = "png"
	case "convert_jpg":
		format = "jpg"
	case "convert_webp":
		format = "webp"
	default:
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "Unknown option.",
		})
		return
	}

	fileResp, err := b.GetFile(ctx, &bot.GetFileParams{FileID: fileID})
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Failed to fetch file"})
		return
	}

	downloadURL := "https://api.telegram.org/file/bot" + b.Token() + "/" + fileResp.FilePath

	resp, err := http.Get(downloadURL)
	if err != nil {
		b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Download failed"})
		return
	}
	defer resp.Body.Close()

	inputFile, err := os.CreateTemp("", "input-*.jpg")
	if err != nil {
		log.Println("Failed to create temp input file:", err)
		return
	}
	defer os.Remove(inputFile.Name())

	_, err = io.Copy(inputFile, resp.Body)
	if err != nil {
		log.Println("Failed to write input file:", err)
		return
	}
	inputFile.Close()

	outputFile, err := os.CreateTemp("", "output-*."+format)
	if err != nil {
		log.Println("Failed to create temp output file:", err)
		return
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)

	go func() {
		cmd := exec.Command("ffmpeg", "-y", "-i", inputFile.Name(), outputPath)
		if err := cmd.Run(); err != nil {
			log.Println("ffmpeg failed:", err)
			b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Conversion failed"})
			return
		}

		data, err := os.ReadFile(outputPath)
		if err != nil {
			log.Println("Failed to read output file:", err)
			return
		}

		b.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID: chatID,
			Document: &models.InputFileUpload{
				Filename: "converted." + format,
				Data:     bytes.NewReader(data),
			},
		})
	}()

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   "Converting your photo to " + format + "…",
	})

	b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            "Processing...",
		ShowAlert:       false,
	})
}
