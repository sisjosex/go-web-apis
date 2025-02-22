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

	tokenExpiration := time.Now().Add(time.Second * time.Duration(config.AppConfig.JwtExpirationSeconds))

	tokenSigned, err := utils.GenerateAccessToken(
		sessionUser.UserId,
		sessionUser.SessionId,
		config.AppConfig.JwtSecretKey,
		tokenExpiration,
	)

	if err != nil {
		return nil, err
	}

	return &models.SessionToken{
		AccessToken: *tokenSigned,
	}, nil
}

func (s *userService) LogoutUser(sessionUser models.SessionUser) (*models.SessionUser, error) {
	logoutResponse, err := s.userRepository.LogoutUser(sessionUser)

	if err != nil {
		return nil, err
	}

	return logoutResponse, nil
}
