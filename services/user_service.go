package services

import (
	"josex/web/config"
	"josex/web/interfaces"
	"josex/web/models"
	"josex/web/utils"
	"time"
	//"josex/web/utils"
)

type userService struct {
	userRepository interfaces.UserRepository
}

func NewUserService(userRepository interfaces.UserRepository) interfaces.UserService {
	return &userService{userRepository: userRepository}
}

func (s *userService) InsertUser(userDTO models.CreateUserDto) (*models.User, error) {
	return s.userRepository.InsertUser(userDTO)
}

func (s *userService) UpdateUser(userDTO models.UpdateUserDto) (*models.User, error) {
	return s.userRepository.UpdateUser(userDTO)
}

func (s *userService) LoginUser(loginDTO models.LoginUserDto) (*models.SessionToken, error) {
	sessionUser, err := s.userRepository.LoginUser(loginDTO)

	if err != nil {
		return nil, err
	}

	tokenExpiration := time.Now().Add(time.Millisecond * time.Duration(config.JwtExpiration))

	tokenSigned, err := utils.GenerateAccessToken(
		sessionUser.UserId,
		sessionUser.SessionId,
		config.JwtSecret,
		tokenExpiration,
	)

	if err != nil {
		return nil, err
	}

	return &models.SessionToken{
		AccessToken: *tokenSigned,
	}, nil
}
