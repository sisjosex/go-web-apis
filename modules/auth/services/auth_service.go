package services

import (
	"josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	coreModels "josex/web/modules/core/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type authService struct {
	authRepository interfaces.AuthRepository
}

func NewAuthService(authRepository interfaces.AuthRepository) interfaces.AuthService {
	return &authService{authRepository: authRepository}
}

func (s *authService) InsertUser(userDTO authModels.CreateUserDto) (*coreModels.User, error) {
	return s.authRepository.InsertUser(userDTO)
}

func (s *authService) LoginUser(userDTO authModels.LoginUserDto) (*authModels.SessionUser, error) {
	return s.authRepository.LoginUser(userDTO)
}

func (s *authService) LoginExternal(userDTO authModels.LoginExternalDto) (*authModels.SessionUser, error) {
	return s.authRepository.LoginExternal(userDTO)
}

func (s *authService) LogoutUser(userDTO authModels.LogoutSessionDto) (*bool, error) {
	return s.authRepository.LogoutUser(userDTO)
}

func (s *authService) GetProfile(userDTO authModels.GetProfileDto) (*coreModels.User, error) {
	return s.authRepository.GetProfile(userDTO)
}

func (s *authService) UpdateProfile(userDTO authModels.UpdateProfileDto) (*coreModels.User, error) {
	return s.authRepository.UpdateProfile(userDTO)
}

func (s *authService) ValidateSession(userID uuid.UUID, sessionID uuid.UUID) error {
	return s.authRepository.ValidateSession(userID, sessionID)
}

func (s *authService) GetUserSessions(userID uuid.UUID) ([]authModels.UserSession, error) {
	return s.authRepository.GetUserSessions(userID)
}

func (s *authService) LogoutSession(userID uuid.UUID, sessionID uuid.UUID) error {
	return s.authRepository.LogoutSession(userID, sessionID)
}

func (s *authService) LogoutAllSessions(userID uuid.UUID, currentSessionID *uuid.UUID) (int, error) {
	return s.authRepository.LogoutAllSessions(userID, currentSessionID)
}

func (s *authService) GenerateEmailVerificationToken(verifyEmailRequest authModels.VerifyEmailRequest, tx pgx.Tx) (*authModels.VerifyEmailToken, error) {
	return s.authRepository.GenerateEmailVerificationToken(verifyEmailRequest, tx)
}

func (s *authService) ConfirmEmailAddress(verifyEmailRequest authModels.VerifyEmailToken) (*bool, error) {
	return s.authRepository.ConfirmEmailAddress(verifyEmailRequest)
}

func (s *authService) ChangePassword(changePasswordDto authModels.ChangePasswordDto) (*bool, error) {
	return s.authRepository.ChangePassword(changePasswordDto)
}

func (s *authService) GeneratePasswordResetToken(passwordResetDto authModels.PasswordResetRequestDto, tx pgx.Tx) (*authModels.PasswordResetTokenRequestDto, error) {
	return s.authRepository.GeneratePasswordResetToken(passwordResetDto, tx)
}

func (s *authService) ValidateResetToken(dto authModels.ValidateResetTokenDto) (*bool, error) {
	return s.authRepository.ValidateResetToken(dto)
}

func (s *authService) ResetPasswordWithToken(passwordResetWithTokenDto authModels.PasswordResetWithTokenDto) (*bool, error) {
	return s.authRepository.ResetPasswordWithToken(passwordResetWithTokenDto)
}
