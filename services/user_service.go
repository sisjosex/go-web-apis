package services

import (
	"josex/web/interfaces"
	"josex/web/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (s *userService) ValidateSession(userID uuid.UUID, sessionID uuid.UUID) error {
	return s.userRepository.ValidateSession(userID, sessionID)
}

// GetUserSessions retrieves all sessions for a user
func (s *userService) GetUserSessions(userID uuid.UUID) ([]models.UserSession, error) {
	return s.userRepository.GetUserSessions(userID)
}

// LogoutSession logs out a specific session
func (s *userService) LogoutSession(userID uuid.UUID, sessionID uuid.UUID) error {
	return s.userRepository.LogoutSession(userID, sessionID)
}

// LogoutAllSessions logs out all sessions except optionally the current one
func (s *userService) LogoutAllSessions(userID uuid.UUID, currentSessionID *uuid.UUID) (int, error) {
	return s.userRepository.LogoutAllSessions(userID, currentSessionID)
}

// ListUsers retrieves paginated list of users with filters
func (s *userService) ListUsers(query models.UserListQuery) (*models.UserListResponse, error) {
	return s.userRepository.ListUsers(query)
}

// GetUserById retrieves a single user by ID
func (s *userService) GetUserById(userID uuid.UUID) (*models.User, error) {
	return s.userRepository.GetUserById(userID)
}

// SoftDeleteUser soft deletes a user
func (s *userService) SoftDeleteUser(userID uuid.UUID, reason string) error {
	return s.userRepository.SoftDeleteUser(userID, reason)
}

func (s *userService) GenerateEmailVerificationToken(verifyEmailRequest models.VerifyEmailRequest, tx pgx.Tx) (*models.VerifyEmailToken, error) {
	return s.userRepository.GenerateEmailVerificationToken(verifyEmailRequest, tx)
}

func (s *userService) ConfirmEmailAddress(verifyEmailRequest models.VerifyEmailToken) (*bool, error) {
	return s.userRepository.ConfirmEmailAddress(verifyEmailRequest)
}

func (s *userService) ChangePassword(changePasswordDto models.ChangePasswordDto) (*bool, error) {
	return s.userRepository.ChangePassword(changePasswordDto)
}

func (s *userService) GeneratePasswordResetToken(passwordResetDto models.PasswordResetRequestDto, tx pgx.Tx) (*models.PasswordResetTokenRequestDto, error) {
	return s.userRepository.GeneratePasswordResetToken(passwordResetDto, tx)
}

func (s *userService) ResetPasswordWithToken(passwordResetWithTokenDto models.PasswordResetWithTokenDto) (*bool, error) {
	return s.userRepository.ResetPasswordWithToken(passwordResetWithTokenDto)
}
