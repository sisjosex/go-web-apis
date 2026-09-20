package repositories

import (
	"context"
	"errors"
	"josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	coreModels "josex/web/modules/core/models"
	database "josex/web/modules/core/services"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type authRepository struct {
	dbService database.DatabaseService
}

func NewAuthRepository(dbService database.DatabaseService) interfaces.AuthRepository {
	return &authRepository{dbService: dbService}
}

func (r *authRepository) InsertUser(userDTO authModels.CreateUserDto) (*coreModels.User, error) {
	user := &coreModels.User{}
	query := `
        SELECT * FROM auth.sp_register_user(
			p_first_name := $1,
			p_last_name := $2,
			p_phone := $3,
			p_birthday := $4,
			p_email := $5,
			p_password := $6,
			p_profile_picture_url := $7,
			p_bio := $8,
			p_website_url := $9
		)
    `

	params := []interface{}{
		userDTO.FirstName,
		userDTO.LastName,
		userDTO.Phone,
		userDTO.Birthday,
		userDTO.Email,
		userDTO.Password,
		userDTO.ProfilePictureUrl,
		userDTO.Bio,
		userDTO.WebsiteUrl,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.Phone,
		&user.Birthday,
		&user.ProfilePictureUrl,
		&user.Bio,
		&user.WebsiteUrl,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return user, nil
}

func (r *authRepository) UpdateProfile(userDTO authModels.UpdateProfileDto) (*coreModels.User, error) {
	user := &coreModels.User{}
	query := `
        SELECT * FROM auth.sp_update_profile(
p_id := $1,
p_first_name := $2,
p_last_name := $3,
p_phone := $4,
p_birthday := $5,
p_current_password := $6,
p_new_password := $7,
p_profile_picture_url := $8,
p_bio := $9,
p_website_url := $10
)
    `

	params := []interface{}{
		userDTO.ID,
		userDTO.FirstName,
		userDTO.LastName,
		userDTO.Phone,
		userDTO.Birthday,
		userDTO.PasswordCurrent,
		userDTO.PasswordNew,
		userDTO.ProfilePictureUrl,
		userDTO.Bio,
		userDTO.WebsiteUrl,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Phone,
		&user.Birthday,
		&user.Email,
		&user.ProfilePictureUrl,
		&user.Bio,
		&user.WebsiteUrl,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return user, nil
}

func (r *authRepository) LoginUser(userDTO authModels.LoginUserDto) (*authModels.SessionUser, error) {
	token := &authModels.SessionUser{}
	query := `
        SELECT * FROM auth.sp_login_email(
p_email := $1,
p_password := $2,
p_ip_address := $3,
p_device_id := $4,
p_device_info := $5,
p_device_os := $6,
p_browser := $7,
p_user_agent := $8,
p_client_type := $9
)
    `

	params := []interface{}{
		userDTO.Email,
		userDTO.Password,
		userDTO.IpAddress,
		userDTO.DeviceId,
		userDTO.DeviceInfo,
		userDTO.DeviceOs,
		userDTO.Browser,
		userDTO.UserAgent,
		userDTO.ClientType,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&token.UserId,
		&token.SessionId,
		&token.SystemRole,
		&token.SubscriptionPlan,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return token, nil
}

func (r *authRepository) LoginExternal(userDTO authModels.LoginExternalDto) (*authModels.SessionUser, error) {
	token := &authModels.SessionUser{}
	query := `
        SELECT * FROM auth.sp_login_external(
p_auth_provider_name := $1,
p_auth_provider_id := $2,
p_device_id := $3,
p_first_name := $4,
p_last_name := $5,
p_email := $6,
p_phone := $7,
p_birthday := $8,
p_ip_address := $9,
p_device_info := $10,
p_device_os := $11,
p_browser := $12,
p_user_agent := $13,
p_client_type := $14
)
    `

	params := []interface{}{
		userDTO.AuthProviderName,
		userDTO.AuthProviderId,
		userDTO.DeviceId,
		userDTO.FirstName,
		userDTO.LastName,
		userDTO.Email,
		userDTO.Phone,
		userDTO.Birthday,
		userDTO.IpAddress,
		userDTO.DeviceInfo,
		userDTO.DeviceOs,
		userDTO.Browser,
		userDTO.UserAgent,
		userDTO.ClientType,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&token.SessionId,
		&token.UserId,
		&token.SystemRole,
		&token.SubscriptionPlan,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return token, nil
}

func (r *authRepository) LogoutUser(userDTO authModels.LogoutSessionDto) (*bool, error) {
	userSessionSuccess := false

	query := `
        SELECT * FROM auth.sp_logout(
p_user_id := $1,
p_session_id := $2
)
    `

	params := []interface{}{
		userDTO.UserId,
		userDTO.SessionId,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&userSessionSuccess,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &userSessionSuccess, nil
}

func (r *authRepository) GetProfile(getProfileDto authModels.GetProfileDto) (*coreModels.User, error) {
	user := &coreModels.User{}

	query := `
        SELECT * FROM auth.sp_get_profile(
p_user_id := $1
)
    `

	params := []interface{}{
		getProfileDto.ID,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Phone,
		&user.Birthday,
		&user.Email,
		&user.ProfilePictureUrl,
		&user.Bio,
		&user.WebsiteUrl,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return user, nil
}

func (r *authRepository) ValidateSession(userID uuid.UUID, sessionID uuid.UUID) error {
	query := `
        SELECT auth.sp_validate_session(
p_user_id := $1,
p_session_id := $2
)
    `

	row := r.dbService.QueryRow(context.Background(), query, userID, sessionID)

	var isValid bool
	err := row.Scan(&isValid)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}

	return nil
}

func (r *authRepository) GetUserSessions(userID uuid.UUID) ([]authModels.UserSession, error) {
	ctx := context.Background()
	query := `SELECT * FROM auth.sp_get_user_sessions(p_user_id := $1)`

	rows, err := r.dbService.Query(ctx, query, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	var sessions []authModels.UserSession
	for rows.Next() {
		var session authModels.UserSession
		err := rows.Scan(
			&session.SessionID,
			&session.UserID,
			&session.DeviceID,
			&session.DeviceOS,
			&session.Browser,
			&session.IpAddress,
			&session.LoginTime,
			&session.LastActive,
			&session.LogoutTime,
			&session.IsActive,
		)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}

func (r *authRepository) LogoutSession(userID uuid.UUID, sessionID uuid.UUID) error {
	ctx := context.Background()
	query := `SELECT auth.sp_logout_session(p_user_id := $1, p_session_id := $2)`

	var result bool
	err := r.dbService.QueryRow(ctx, query, userID, sessionID).Scan(&result)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}

	return nil
}

func (r *authRepository) LogoutAllSessions(userID uuid.UUID, currentSessionID *uuid.UUID) (int, error) {
	ctx := context.Background()
	query := `SELECT auth.sp_logout_all_sessions(p_user_id := $1, p_current_session_id := $2)`

	var count int
	err := r.dbService.QueryRow(ctx, query, userID, currentSessionID).Scan(&count)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return 0, pgErr
		}
		return 0, err
	}

	return count, nil
}

func (r *authRepository) GenerateEmailVerificationToken(verifyEmailRequest authModels.VerifyEmailRequest, tx pgx.Tx) (*authModels.VerifyEmailToken, error) {
	VerifyEmailToken := &authModels.VerifyEmailToken{}

	query := `
        SELECT * FROM auth.sp_generate_email_verification_token(
p_user_id := $1,
p_email := $2
)
    `

	params := []interface{}{
		verifyEmailRequest.UserId,
		verifyEmailRequest.Email,
	}

	row := tx.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&VerifyEmailToken.Token,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return VerifyEmailToken, nil
}

func (r *authRepository) ConfirmEmailAddress(verifyEmailRequest authModels.VerifyEmailToken) (*bool, error) {
	VerifyEmailToken := false

	query := `
        SELECT * FROM auth.sp_verify_email(
p_token := $1
)
    `

	params := []interface{}{
		verifyEmailRequest.Token,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&VerifyEmailToken,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &VerifyEmailToken, nil
}

func (r *authRepository) ChangePassword(changePasswordDto authModels.ChangePasswordDto) (*bool, error) {
	ChangePassword := false

	query := `
SELECT * FROM auth.sp_change_password(
p_user_id := $1,
p_password_current := $2,
p_password_new := $3
)
`

	params := []interface{}{
		changePasswordDto.UserId,
		changePasswordDto.PasswordCurrent,
		changePasswordDto.PasswordNew,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&ChangePassword,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &ChangePassword, nil
}

func (r *authRepository) GeneratePasswordResetToken(passwordResetRequestDto authModels.PasswordResetRequestDto, tx pgx.Tx) (*authModels.PasswordResetTokenRequestDto, error) {
	PasswordResetWithToken := &authModels.PasswordResetTokenRequestDto{}

	query := `
SELECT * FROM auth.sp_generate_password_reset_token(
p_email := $1
)
`

	params := []interface{}{
		passwordResetRequestDto.Email,
	}

	row := tx.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&PasswordResetWithToken.Token,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return PasswordResetWithToken, nil
}

func (r *authRepository) ValidateResetToken(dto authModels.ValidateResetTokenDto) (*bool, error) {
	valid := false

	query := `
SELECT * FROM auth.sp_validate_reset_token(
p_token := $1
)
`

	row := r.dbService.QueryRow(context.Background(), query, dto.Token)

	err := row.Scan(&valid)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &valid, nil
}

func (r *authRepository) ResetPasswordWithToken(passwordResetWithTokenDto authModels.PasswordResetWithTokenDto) (*bool, error) {
	ResetPassword := false

	query := `
SELECT * FROM auth.sp_reset_password_with_token(
p_token := $1,
p_new_password := $2
)
`

	params := []interface{}{
		passwordResetWithTokenDto.Token.String(),
		passwordResetWithTokenDto.PasswordNew,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&ResetPassword,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &ResetPassword, nil
}
