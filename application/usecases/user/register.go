package usecases_user

import "github.com/BoburF/golang-bot-converter/domain"

type RegisterUserUsecase struct {
	userRepo domain.UserRepository
}

func NewRegisterUser(repo domain.UserRepository) *RegisterUserUsecase {
	return &RegisterUserUsecase{userRepo: repo}
}

func (uc *RegisterUserUsecase) Execute(name, phone string) (*domain.User, error) {
	user, err := uc.userRepo.GetByPhone(phone)
	if err == nil {
		return user, nil
	}

	err = uc.userRepo.Create(name, phone)
	if err != nil {
		return &domain.User{}, err
	}

	return uc.userRepo.GetByPhone(phone)
}
