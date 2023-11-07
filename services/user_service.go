package services

import (
	"josex/web/config"
	"josex/web/models"
	"josex/web/validators/user"
	"strings"

	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db}
}

func (us *UserService) FindUsers(searchTerm string, page int, pageSize int) ([]models.User, *config.ValidationError) {
	var users []models.User

	query := us.db.Model(&models.User{})

	if searchTerm != "" {
		query = query.Where("email ILIKE ?", ""+searchTerm+"%").
			Or("first_name ILIKE ?", ""+searchTerm+"%").
			Or("last_name ILIKE ?", ""+searchTerm+"%")
	}

	// paginate
	query = query.Offset((page - 1) * pageSize).Limit(pageSize)

	if err := query.Find(&users).Error; err != nil {
		return nil, config.BuildErrorSingle(config.UserSearchFailed)
	}

	return users, nil
}

func (us *UserService) CreateUser(userDto *user.CreateUserDto) (int, *config.ValidationError) {

	user := &models.User{
		FirstName: userDto.FirstName,
		LastName:  userDto.LastName,
		Email:     strings.TrimSpace(strings.ToLower(userDto.Email)),
		Password:  strings.TrimSpace(userDto.Password),
		Birthday:  userDto.Birthday,
		Phone:     userDto.Phone,
	}

	result := us.db.Create(&user)

	if result.Error != nil {
		return 0, config.BuildErrorSingle(config.UserRegisterFailed)
	}

	return user.ID, nil
}

func (us *UserService) GetUserByID(userID int) (*models.User, *config.ValidationError) {
	var user *models.User

	if err := us.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, config.BuildErrorSingle(config.UserGetByIdNotFound)
	}

	return user, nil
}

func (us *UserService) GetUserByEmail(email string, id *int) (*models.User, *config.ValidationError) {
	var user *models.User

	query := us.db.Where("email = ?", strings.TrimSpace(strings.ToLower(email)))

	if id != nil {
		query = query.Where("id = ?", id)
	}

	if err := query.First(&user).Error; err != nil {
		return nil, config.BuildErrorSingle(config.UserGetByIdEmailFound)
	}

	return user, nil
}
