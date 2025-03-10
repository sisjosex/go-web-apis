package repositories

import (
	"context"
	"errors"
	"josex/web/interfaces"
	"josex/web/models"

	"github.com/jackc/pgx/v5/pgconn"
)

type userRepository struct {
	dbService interfaces.DatabaseService
}

func NewUserRepository(dbService interfaces.DatabaseService) *userRepository {
	return &userRepository{dbService: dbService}
}

func (r *userRepository) InsertUser(userDTO models.CreateUserDto) (*models.User, error) {
	user := &models.User{}
	query := `
        SELECT * FROM auth.sp_create_user(
			p_first_name := $1,
			p_last_name := $2,
			p_phone := $3,
			p_birthday := $4,
			p_email := $5,  -- No se requiere email en este caso
			p_password := $6,  -- No se requiere password en este caso
			p_profile_picture_url := $7,
			p_bio := $8,
			p_website_url := $9
		)
    `

	// Construir la lista de parámetros en el mismo orden que la consulta
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

func (r *userRepository) UpdateUser(userDTO models.UpdateUserDto) (*models.User, error) {
	user := &models.User{}
	query := `
        SELECT * FROM auth.sp_update_user(
			p_id := $1,
			p_first_name := $2,
			p_last_name := $3,
			p_phone := $4,
			p_birthday := $5,
			p_email := $6,
			p_current_password := $7, -- Contraseña actual
			p_new_password := $8, -- Nueva contraseña
			p_is_active := $9,
			p_is_verified := $10,
			p_expiration_date := $11,
			p_profile_picture_url := $12,
			p_bio := $13,
			p_website_url := $14
		)
    `

	// Construir la lista de parámetros en el mismo orden que la consulta
	params := []interface{}{
		userDTO.ID,
		userDTO.FirstName,
		userDTO.LastName,
		userDTO.Phone,
		userDTO.Birthday,
		userDTO.Email,
		userDTO.PasswordCurrent,
		userDTO.PasswordNew,
		userDTO.IsActive,
		userDTO.IsVerified,
		userDTO.ExpirationDate,
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

func (r *userRepository) UpdateProfile(userDTO models.UpdateProfileDto) (*models.User, error) {
	user := &models.User{}
	query := `
        SELECT * FROM auth.sp_update_profile(
			p_id := $1,
			p_first_name := $2,
			p_last_name := $3,
			p_phone := $4,
			p_birthday := $5,
			p_email := $6,
			p_current_password := $7, -- Contraseña actual
			p_new_password := $8, -- Nueva contraseña
			p_profile_picture_url := $9,
			p_bio := $10,
			p_website_url := $11
		)
    `

	// Construir la lista de parámetros en el mismo orden que la consulta
	params := []interface{}{
		userDTO.ID,
		userDTO.FirstName,
		userDTO.LastName,
		userDTO.Phone,
		userDTO.Birthday,
		userDTO.Email,
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

func (r *userRepository) LoginUser(userDTO models.LoginUserDto) (*models.SessionUser, error) {
	token := &models.SessionUser{}
	query := `
        SELECT * FROM auth.sp_login_email(
			p_email := $1,
			p_password := $2,
			p_ip_address := $3,
			p_device_id := $4,
			p_device_info := $5,
			p_device_os := $6,
			p_browser := $7,
			p_user_agent := $8
		)
    `

	// Construir la lista de parámetros en el mismo orden que la consulta
	params := []interface{}{
		userDTO.Email,
		userDTO.Password,
		userDTO.IpAddress,
		userDTO.DeviceId,
		userDTO.DeviceInfo,
		userDTO.DeviceOs,
		userDTO.Browser,
		userDTO.UserAgent,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&token.UserId,
		&token.SessionId,
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

func (r *userRepository) LoginExternal(userDTO models.LoginExternalDto) (*models.SessionUser, error) {
	token := &models.SessionUser{}
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
			p_user_agent := $13
		)
    `

	// Construir la lista de parámetros en el mismo orden que la consulta
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
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

	err := row.Scan(
		&token.UserId,
		&token.SessionId,
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

func (r *userRepository) LogoutUser(userDTO models.LogoutSessionDto) (*bool, error) {
	userSessionSuccess := false

	query := `
        SELECT * FROM auth.sp_logout(
			p_user_id := $1,
			p_session_id := $2
		)
    `

	// Construir la lista de parámetros en el mismo orden que la consulta
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

func (r *userRepository) GetProfile(getProfileDto models.GetProfileDto) (*models.User, error) {
	user := &models.User{}

	query := `
        SELECT * FROM auth.sp_get_profile(
			p_id := $1
		)
    `

	// Construir la lista de parámetros en el mismo orden que la consulta
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

func (r *userRepository) GenerateEmailVerificationToken(verifyEmailRequest models.VerifyEmailRequest) (*models.VerifyEmailToken, error) {
	VerifyEmailToken := &models.VerifyEmailToken{}

	query := `
        SELECT * FROM auth.sp_generate_email_verification_token(
			p_user_id := $1,
			p_email := $2
		)
    `

	// Construir la lista de parámetros en el mismo orden que la consulta
	params := []interface{}{
		verifyEmailRequest.UserId,
		verifyEmailRequest.Email,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

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

func (r *userRepository) ConfirmEmailAddress(verifyEmailRequest models.VerifyEmailToken) (*bool, error) {
	VerifyEmailToken := false

	query := `
        SELECT * FROM auth.sp_verify_email(
			p_token := $1
		)
    `

	// Construir la lista de parámetros en el mismo orden que la consulta
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

func (r *userRepository) ChangePassword(changePasswordDto models.ChangePasswordDto) (*bool, error) {
	ChangePassword := false

	query := `
		SELECT * FROM auth.sp_change_password(
			p_user_id := $1,
			p_password_current := $2,
			p_password_new := $3
		)
	`

	// Construir la lista de parámetros en el mismo orden que la consulta
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

func (r *userRepository) GeneratePasswordResetToken(passwordResetRequestDto models.PasswordResetRequestDto) (*models.PasswordResetTokenRequestDto, error) {
	PasswordResetWithToken := &models.PasswordResetTokenRequestDto{}

	query := `
		SELECT * FROM auth.sp_generate_password_reset_token(
			p_email := $1
		)
	`

	// Construir la lista de parámetros en el mismo orden que la consulta
	params := []interface{}{
		passwordResetRequestDto.Email,
	}

	row := r.dbService.QueryRow(context.Background(), query, params...)

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

func (r *userRepository) ResetPasswordWithToken(passwordResetWithTokenDto models.PasswordResetWithTokenDto) (*bool, error) {
	ResetPassword := false

	query := `
		SELECT * FROM auth.sp_reset_password_with_token(
			p_token := $1,
			p_new_password := $2
		)
	`

	// Construir la lista de parámetros en el mismo orden que la consulta
	params := []interface{}{
		passwordResetWithTokenDto.Token,
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
