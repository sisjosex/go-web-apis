package services

import (
	"josex/web/interfaces"
	"josex/web/models"
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

func (s *userService) LoginUser(loginDTO models.LoginUserDto) (*models.SessionUser, error) {
	sessionUser, err := s.userRepository.LoginUser(loginDTO)

	if err != nil {
		return nil, err
	}

	return sessionUser, nil
}

func (s *userService) LoginExternal(loginDTO models.LoginExternalDto) (*models.SessionUser, error) {
	sessionUser, err := s.userRepository.LoginExternal(loginDTO)

	if err != nil {
		return nil, err
	}

	return sessionUser, nil
}

func (s *userService) UpdateProfile(userDTO models.UpdateProfileDto) (*models.User, error) {
	return s.userRepository.UpdateProfile(userDTO)
}

func (s *userService) LogoutUser(sessionUser models.SessionUser) (*models.SessionUser, error) {
	logoutResponse, err := s.userRepository.LogoutUser(sessionUser)

	if err != nil {
		return nil, err
	}

	return logoutResponse, nil
}
