package repositories

import (
	"context"
	"errors"
	coreModels "josex/web/modules/core/models"
	database "josex/web/modules/core/services"
	"josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type userRepository struct {
	dbService database.DatabaseService
}

func NewUserRepository(dbService database.DatabaseService) interfaces.UserRepository {
	return &userRepository{dbService: dbService}
}

func (r *userRepository) InsertUser(userDTO userModels.CreateUserDto) (*coreModels.User, error) {
	user := &coreModels.User{}
	query := `
        SELECT * FROM users.sp_create_user(
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

	params := []any{
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

func (r *userRepository) UpdateUser(userDTO userModels.UpdateUserDto) (*coreModels.User, error) {
	ctx := context.Background()

	tx, err := r.dbService.BeginTransaction(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Inject tenant and performer into the transaction so the AFTER UPDATE trigger
	// can read them via current_setting('app.tenant_id', true) / ('app.performed_by', true).
	if userDTO.TenantID != (uuid.UUID{}) {
		performedByStr := ""
		if userDTO.PerformedBy != nil {
			performedByStr = userDTO.PerformedBy.String()
		}
		_, err = tx.Exec(ctx,
			"SELECT set_config('app.tenant_id',$1,true), set_config('app.performed_by',$2,true)",
			userDTO.TenantID.String(),
			performedByStr,
		)
		if err != nil {
			return nil, err
		}
	}

	query := `
        SELECT * FROM users.sp_update_user(
p_id := $1,
p_first_name := $2,
p_last_name := $3,
p_phone := $4,
p_birthday := $5,
p_email := $6,
p_current_password := $7,
p_new_password := $8,
p_is_active := $9,
p_expiration_date := $10,
p_profile_picture_url := $11,
p_bio := $12,
p_website_url := $13
)
    `

	params := []any{
		userDTO.ID,
		userDTO.FirstName,
		userDTO.LastName,
		userDTO.Phone,
		userDTO.Birthday,
		userDTO.Email,
		userDTO.PasswordCurrent,
		userDTO.PasswordNew,
		userDTO.IsActive,
		userDTO.ExpirationDate,
		userDTO.ProfilePictureUrl,
		userDTO.Bio,
		userDTO.WebsiteUrl,
	}

	user := &coreModels.User{}
	row := tx.QueryRow(ctx, query, params...)

	err = row.Scan(
		&user.ID,
		&user.FirstName,
		&user.LastName,
		&user.Phone,
		&user.Birthday,
		&user.Email,
		&user.ProfilePictureUrl,
		&user.Bio,
		&user.WebsiteUrl,
		&user.IsActive,
		&user.ExpirationDate,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepository) ListUsers(query userModels.UserListQuery) (*userModels.UserListResponse, error) {
	ctx := context.Background()

	page := max(1, query.Page)
	limit := query.Limit
	if limit < 1 {
		limit = 10
	}

	var searchVal, statusVal, sortVal, orderVal *string
	if query.Search != "" {
		searchVal = &query.Search
	}
	if query.Status != "" {
		statusVal = &query.Status
	}
	if query.Sort != "" {
		sortVal = &query.Sort
	}
	if query.Order != "" {
		orderVal = &query.Order
	}
	var levelVal *string
	if query.Level != "" {
		levelVal = &query.Level
	}

	sqlQuery := `
        SELECT * FROM users.sp_list_users(
            p_tenant_id       := $1,
            p_page            := $2,
            p_limit           := $3,
            p_search          := $4,
            p_status          := $5,
            p_sort            := $6,
            p_order           := $7,
            p_exclude_user_id := $8,
            p_level           := $9
        )
    `

	rows, err := r.dbService.Query(ctx, sqlQuery, query.TenantID, page, limit, searchVal, statusVal, sortVal, orderVal, query.ExcludeUserID, levelVal)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	defer rows.Close()

	var users []coreModels.User
	var totalCount int64
	for rows.Next() {
		var user coreModels.User
		err := rows.Scan(
			&user.ID,
			&user.FirstName,
			&user.LastName,
			&user.Phone,
			&user.Birthday,
			&user.Email,
			&user.ProfilePictureUrl,
			&user.Bio,
			&user.WebsiteUrl,
			&user.IsActive,
			&user.CreatedAt,
			&user.ExpirationDate,
			&totalCount,
			&user.TenantRole,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if users == nil {
		users = []coreModels.User{}
	}

	totalPages := 0
	if limit > 0 && totalCount > 0 {
		totalPages = int((totalCount + int64(limit) - 1) / int64(limit))
	}

	return &userModels.UserListResponse{
		Users:      users,
		Total:      totalCount,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

func (r *userRepository) GetUserById(userID uuid.UUID) (*coreModels.User, error) {
	ctx := context.Background()
	query := `SELECT * FROM users.sp_get_user_by_id(p_user_id := $1)`

	var user coreModels.User
	err := r.dbService.QueryRow(ctx, query, userID).Scan(
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

	return &user, nil
}

func (r *userRepository) GetStats(tenantID uuid.UUID, excludeUserID *uuid.UUID) (*userModels.UserStatsResponse, error) {
	ctx := context.Background()
	query := `SELECT * FROM users.sp_get_user_stats(p_tenant_id := $1, p_exclude_user_id := $2)`

	stats := &userModels.UserStatsResponse{}
	err := r.dbService.QueryRow(ctx, query, tenantID, excludeUserID).Scan(
		&stats.Total,
		&stats.Active,
		&stats.Inactive,
		&stats.Expired,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}
	return stats, nil
}

func (r *userRepository) AssignToTenant(tenantID, requesterID, userID uuid.UUID, role string) error {
	ctx := context.Background()
	query := `SELECT * FROM tenancy.sp_add_user_to_tenant($1, $2, $3, $4)`

	rows, err := r.dbService.Query(ctx, query, tenantID, requesterID, userID, role)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}
	rows.Close()
	return nil
}

func (r *userRepository) SoftDeleteUser(userID uuid.UUID) error {
	ctx := context.Background()
	query := `SELECT users.sp_soft_delete_user(p_user_id := $1)`

	var result bool
	err := r.dbService.QueryRow(ctx, query, userID).Scan(&result)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return pgErr
		}
		return err
	}

	return nil
}

func (r *userRepository) ResetPasswordToken(userID uuid.UUID) (*userModels.ResetPasswordResult, error) {
	user, err := r.GetUserById(userID)
	if err != nil {
		return nil, err
	}

	email := ""
	if user.Email != nil {
		email = *user.Email
	}

	var token string
	row := r.dbService.QueryRow(context.Background(), `
		SELECT auth.sp_generate_password_reset_token(
			p_email := $1
		)
	`, email)

	if err := row.Scan(&token); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return &userModels.ResetPasswordResult{Email: email, Token: token}, nil
}
