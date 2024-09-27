package repositories

import (
	"context"
	"errors"
	"fmt"
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
        SELECT * FROM sp_create_user(
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
			return nil, fmt.Errorf(pgErr.Message)
		}

		return nil, err
	}

	return user, nil
}

/*func (r *userRepository) GetUserByEmail(email string, id *string) (*models.User, *common.ValidationError) {
	var user models.User

	query := r.db.Where("email = ?", strings.TrimSpace(strings.ToLower(email))).Limit(1)

	if id != nil {
		query = query.Where("id = ?", id)
	}

	query.Find(&user)

	if user.ID != uuid.Nil {
		return &user, common.BuildErrorSingle(common.UserGetByIdNotFound)
	}

	return nil, nil
}*/
