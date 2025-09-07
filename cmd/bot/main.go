package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/BoburF/golang-bot-converter/domain"
	infra_db_sql "github.com/BoburF/golang-bot-converter/infra/db/sql"
	infra_db_sql_repositories "github.com/BoburF/golang-bot-converter/infra/db/sql/repositories"
	"github.com/go-telegram/bot"
	"github.com/joho/godotenv"
)

func main() {
	db := infra_db_sql.NewSQLITE()
	userRepo := infra_db_sql_repositories.NewUserRepository(db)

	loadConfig()

	loadBot(userRepo)
}

func loadBot(userRepo domain.UserRepository) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	handler := NewBotHandler(userRepo)

	opts := []bot.Option{
		bot.WithDefaultHandler(handler.HandleMessage),
	}

	b, err := bot.New(os.Getenv("TELEGRAM_TOKEN"), opts...)
	if err != nil {
		panic(err)
	}

	b.Start(ctx)
}

func loadConfig() {
	godotenv.Load()

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN is not set")
	}
}
