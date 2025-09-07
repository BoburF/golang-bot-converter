package domain

import "time"

type User struct {
	ID                    int64
	Name                  string
	Phone                 string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	ConvertedImageCounter int64
}

type UserRepository interface {
	GetByPhone(phone string) (*User, error)
	Create(Name, Phone string) error
	Update(u *User) error
	IncrementConvertedImages(userID int64, incrBy int64) error
}
