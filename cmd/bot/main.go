package main

import (
	"log"

	infra_db_sql "github.com/BoburF/golang-bot-converter/infra/db/sql"
	infra_db_sql_repositories "github.com/BoburF/golang-bot-converter/infra/db/sql/repositories"
)

func main() {
	db := infra_db_sql.NewSQLITE()
	defer db.Close()

	userRepo := infra_db_sql_repositories.NewUserRepository(db)

	userPhone := "+998939752577"

	err := userRepo.Create("Bobur", userPhone)
	if err != nil {
		log.Fatal(err)
	}

	userGot, err := userRepo.GetByPhone(userPhone)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Got the user:", userGot)
}
