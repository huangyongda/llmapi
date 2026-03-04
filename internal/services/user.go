package services

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"llmapi/internal/config"
	"llmapi/internal/database"
	"llmapi/internal/models"
)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) CreateUser(username, password string, requestLimit int, isAdmin bool) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Username:     username,
		PasswordHash: string(hash),
		RequestLimit: requestLimit,
		IsAdmin:      isAdmin,
	}

	if err := database.DB.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (s *UserService) GetUserByID(id int64) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (s *UserService) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

func (s *UserService) GetAllUsers(page, pageSize int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := database.DB.Model(&models.User{})
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get users: %w", err)
	}

	return users, total, nil
}

func (s *UserService) UpdateUser(id int64, requestLimit int) error {
	return database.DB.Model(&models.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"request_limit": requestLimit,
	}).Error
}

func (s *UserService) DeleteUser(id int64) error {
	return database.DB.Delete(&models.User{}, id).Error
}

func (s *UserService) VerifyPassword(username, password string) (*models.User, error) {
	//如果是管理员登录 用config文件的密码判断
	if username == config.AppConfig.Admin.Username {
		if password != config.AppConfig.Admin.Password {
			return nil, errors.New("invalid username or password")
		}
		return &models.User{
			Username: username,
			IsAdmin:  true,
		}, nil
	}
	fmt.Println("Verifying password for user:", username)
	user, err := s.GetUserByUsername(username)
	if err != nil {
		return nil, errors.New("invalid username or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	return user, nil
}

func (s *UserService) IncrementRequestCount(userID int64) error {
	return database.DB.Model(&models.User{}).Where("id = ?", userID).
		Update("request_count", database.DB.Raw("request_count + 1")).Error
}

func (s *UserService) GetAvailableRequests(userID int64) (int, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return 0, err
	}
	return user.RequestLimit - user.RequestCount, nil
}

func (s *UserService) CheckAndDecrementLimit(userID int64) (bool, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return false, err
	}

	available := user.RequestLimit - user.RequestCount
	if available <= 0 {
		return false, nil
	}

	if err := s.IncrementRequestCount(userID); err != nil {
		return false, err
	}

	return true, nil
}

func (s *UserService) InitAdmin() error {
	adminConfig := config.AppConfig.Admin
	existingAdmin, _ := s.GetUserByUsername(adminConfig.Username)
	if existingAdmin != nil {
		return nil
	}

	_, err := s.CreateUser(adminConfig.Username, adminConfig.Password, 0, true)
	if err != nil {
		return fmt.Errorf("failed to create admin: %w", err)
	}

	fmt.Println("Admin user created successfully")
	return nil
}
