package services

import (
	coreModels "josex/web/modules/core/models"
	"josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"

	"github.com/google/uuid"
)

type userService struct {
	userRepository interfaces.UserRepository
}

func NewUserService(userRepository interfaces.UserRepository) interfaces.UserService {
	return &userService{userRepository: userRepository}
}

func (s *userService) InsertUser(userDTO userModels.CreateUserDto) (*coreModels.User, error) {
	return s.userRepository.InsertUser(userDTO)
}

func (s *userService) UpdateUser(userDTO userModels.UpdateUserDto) (*coreModels.User, error) {
	return s.userRepository.UpdateUser(userDTO)
}

func (s *userService) ListUsers() ([]coreModels.User, error) {
	return s.userRepository.ListUsers()
}

func (s *userService) GetUserById(userID uuid.UUID) (*coreModels.User, error) {
	return s.userRepository.GetUserById(userID)
}

func (s *userService) SoftDeleteUser(userID uuid.UUID) error {
	return s.userRepository.SoftDeleteUser(userID)
}
