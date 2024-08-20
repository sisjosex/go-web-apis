package services

import (
	"josex/web/common"
	"josex/web/models"
	"josex/web/validators/user"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db}
}

func (us *UserService) FindUsers(searchTerm string, page int, pageSize int) ([]models.User, *common.ValidationError) {
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
		if err == gorm.ErrRecordNotFound {
			return users, nil
		}
		return nil, common.BuildErrorSingle(common.UserSearchFailed)
	}

	return users, nil
}

func (us *UserService) CreateUser(userDto *user.CreateUserDto) (uuid.UUID, *common.ValidationError) {

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
		return uuid.Nil, common.BuildErrorSingle(common.UserRegisterFailed)
	}

	return user.ID, nil
}

func (us *UserService) GetUserByID(userID uuid.UUID) (*models.User, *common.ValidationError) {
	var user *models.User

	if err := us.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, common.BuildErrorSingle(common.UserGetByIdNotFound)
	}

	return user, nil
}

func (us *UserService) GetUserByEmail(email string, id *string) (*models.User, *common.ValidationError) {
	var user models.User

	query := us.db.Where("email = ?", strings.TrimSpace(strings.ToLower(email))).Limit(1)

	if id != nil {
		query = query.Where("id = ?", id)
	}

	query.Find(&user)

	if user.ID != uuid.Nil {
		return &user, common.BuildErrorSingle(common.UserGetByIdNotFound)
	}

	return nil, nil
}
