package infra_db_sql_repositories

import (
	"database/sql"
	"time"

	"github.com/BoburF/golang-bot-converter/domain"
	_ "github.com/mattn/go-sqlite3"
)

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) GetByPhone(phone string) (*domain.User, error) {
	row := r.db.QueryRow(`
		SELECT id, name, phone, created_at, updated_at, converted_image_counter
		FROM users WHERE phone = ?`, phone)

	var u domain.User
	err := row.Scan(&u.ID, &u.Name, &u.Phone, &u.CreatedAt, &u.UpdatedAt, &u.ConvertedImageCounter)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) Create(Name, Phone string) error {
	user := domain.User{
		Name:  Name,
		Phone: Phone,
	}

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	res, err := r.db.Exec(`
		INSERT INTO users (name, phone, created_at, updated_at, converted_image_counter)
		VALUES (?, ?, ?, ?, ?)`,
		user.Name, user.Phone, user.CreatedAt, user.UpdatedAt, user.ConvertedImageCounter,
	)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err == nil {
		user.ID = id
	}
	return nil
}

func (r *userRepository) Update(u *domain.User) error {
	u.UpdatedAt = time.Now()
	_, err := r.db.Exec(`
		UPDATE users 
		SET name=?, phone=?, updated_at=?, converted_image_counter=?
		WHERE id=?`,
		u.Name, u.Phone, u.UpdatedAt, u.ConvertedImageCounter, u.ID,
	)
	return err
}

func (r *userRepository) IncrementConvertedImages(userID int64, incrBy int64) error {
	_, err := r.db.Exec(`
		UPDATE users 
		SET converted_image_counter = converted_image_counter + ?, updated_at=?
		WHERE id=?`,
		incrBy, time.Now(), userID,
	)
	return err
}
