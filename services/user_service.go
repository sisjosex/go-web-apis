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
	return s.userRepository.LoginUser(loginDTO)
}

func (s *userService) LoginExternal(loginDTO models.LoginExternalDto) (*models.SessionUser, error) {
	return s.userRepository.LoginExternal(loginDTO)
}

func (s *userService) UpdateProfile(userDTO models.UpdateProfileDto) (*models.User, error) {
	return s.userRepository.UpdateProfile(userDTO)
}

func (s *userService) LogoutUser(sessionUser models.LogoutSessionDto) (*bool, error) {
	return s.userRepository.LogoutUser(sessionUser)
}

func (s *userService) GetProfile(userDTO models.GetProfileDto) (*models.User, error) {
	return s.userRepository.GetProfile(userDTO)
}

func (s *userService) GenerateEmailVerificationToken(verifyEmailRequest models.VerifyEmailRequest) (*models.VerifyEmailToken, error) {
	return s.userRepository.GenerateEmailVerificationToken(verifyEmailRequest)
}

func (s *userService) ConfirmEmailAddress(verifyEmailRequest models.VerifyEmailToken) (*bool, error) {
	return s.userRepository.ConfirmEmailAddress(verifyEmailRequest)
}

func (s *userService) ChangePassword(changePasswordDto models.ChangePasswordDto) (*bool, error) {
	return s.userRepository.ChangePassword(changePasswordDto)
}

func (s *userService) GeneratePasswordResetToken(passwordResetDto models.PasswordResetRequestDto) (*models.PasswordResetTokenRequestDto, error) {
	return s.userRepository.GeneratePasswordResetToken(passwordResetDto)
}

func (s *userService) ResetPasswordWithToken(passwordResetWithTokenDto models.PasswordResetWithTokenDto) (*bool, error) {
	return s.userRepository.ResetPasswordWithToken(passwordResetWithTokenDto)
}
