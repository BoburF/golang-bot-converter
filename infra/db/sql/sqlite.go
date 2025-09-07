package infra_db_sql

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

func NewSQLITE() *sql.DB {
	db, err := sql.Open("sqlite3", "app.db")
	if err != nil {
		log.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	migrationPath := filepath.Join(cwd, "migrations", "001.sql")

	migrationSQL, err := os.ReadFile(migrationPath)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(string(migrationSQL))
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Database initialized successfully 🚀")
	return db
}
