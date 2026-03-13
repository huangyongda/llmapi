package services

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"llmapi/internal/config"
	"llmapi/internal/database"
	"llmapi/internal/models"
)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

func (s *UserService) CreateUser(username, password string, requestLimit int, isAdmin bool, expiresAt *time.Time) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Username:     username,
		PasswordHash: string(hash),
		RequestLimit: requestLimit,
		IsAdmin:      isAdmin,
		ExpiresAt:    expiresAt,
	}

	if err := database.DB.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 自动创建 API Key，key_name 与用户名相同
	apiKeyService := NewAPIKeyService()
	if _, err := apiKeyService.CreateAPIKey(user.ID, username, nil); err != nil {
		// API Key 创建失败不影响用户创建，但会记录错误
		fmt.Printf("failed to create API key for user %s: %v\n", username, err)
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

func (s *UserService) GetAllUsers(page, pageSize int, username string) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := database.DB.Model(&models.User{})

	// 如果有用户名搜索条件，添加模糊匹配
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get users: %w", err)
	}

	return users, total, nil
}

func (s *UserService) UpdateUser(id int64, requestLimit int, expiresAt *time.Time) error {
	// 先获取用户当前的信息
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 更新用户信息
	updates := map[string]interface{}{
		"request_limit": requestLimit,
	}
	if expiresAt != nil || user.ExpiresAt != nil {
		updates["expires_at"] = expiresAt
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}

	// 检查用户过期状态是否改变，同步更新 API Key 状态
	// 用户过期：expiresAt 不为 nil 且早于当前时间
	// 用户未过期：expiresAt 为 nil 或晚于当前时间
	isExpired := expiresAt != nil && expiresAt.Before(time.Now())

	// 如果用户之前的状态和现在的状态不同，才需要同步
	wasExpired := user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now())
	if wasExpired != isExpired {
		apiKeyService := NewAPIKeyService()
		if err := apiKeyService.SyncAPIKeysStatusByUserID(id, isExpired); err != nil {
			return err
		}
	}

	return nil
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
		// 用户表找不到，尝试在激活表中查找
		activationUser, err := s.GetActivationUserByUsername(username)
		if err != nil {
			return nil, errors.New("invalid username or password")
		}

		// 验证激活用户密码
		if err := bcrypt.CompareHashAndPassword([]byte(activationUser.PasswordHash), []byte(password)); err != nil {
			return nil, errors.New("invalid username or password")
		}

		// 自动创建用户
		expiresAt := time.Now().AddDate(0, 0, activationUser.ValidDays)
		newUser, err := s.CreateUser(username, password, activationUser.RequestLimit, false, &expiresAt)
		if err != nil {
			return nil, fmt.Errorf("failed to create user from activation: %w", err)
		}

		// 激活成功后删除激活用户记录
		s.DeleteActivationUser(activationUser.ID)

		return newUser, nil
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

	_, err := s.CreateUser(adminConfig.Username, adminConfig.Password, 0, true, nil)
	if err != nil {
		return fmt.Errorf("failed to create admin: %w", err)
	}

	fmt.Println("Admin user created successfully")
	return nil
}

// GetActivationUserByUsername 根据用户名获取激活用户
func (s *UserService) GetActivationUserByUsername(username string) (*models.ActivationUser, error) {
	var activationUser models.ActivationUser
	if err := database.DB.Where("username = ?", username).First(&activationUser).Error; err != nil {
		return nil, fmt.Errorf("activation user not found: %w", err)
	}
	return &activationUser, nil
}

// GetAllActivationUsers 获取所有激活用户
func (s *UserService) GetAllActivationUsers(page, pageSize int) ([]models.ActivationUser, int64, error) {
	var users []models.ActivationUser
	var total int64

	query := database.DB.Model(&models.ActivationUser{})
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get activation users: %w", err)
	}

	return users, total, nil
}

// CreateActivationUser 创建激活用户
func (s *UserService) CreateActivationUser(username, password string, validDays, requestLimit int) (*models.ActivationUser, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	activationUser := &models.ActivationUser{
		Username:     username,
		PasswordHash: string(hash),
		ValidDays:    validDays,
		RequestLimit: requestLimit,
	}

	if err := database.DB.Create(activationUser).Error; err != nil {
		return nil, fmt.Errorf("failed to create activation user: %w", err)
	}

	return activationUser, nil
}

// DeleteActivationUser 删除激活用户
func (s *UserService) DeleteActivationUser(id int64) error {
	return database.DB.Delete(&models.ActivationUser{}, id).Error
}

// BatchCreateActivationUsers 批量创建激活用户
func (s *UserService) BatchCreateActivationUsers(users []models.ActivationUser) ([]models.ActivationUser, error) {
	var activationUsers []models.ActivationUser

	for i := range users {
		hash, err := bcrypt.GenerateFromPassword([]byte(users[i].PasswordHash), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password for %s: %w", users[i].Username, err)
		}
		users[i].PasswordHash = string(hash)
		activationUsers = append(activationUsers, users[i])
	}

	if err := database.DB.Create(&activationUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to batch create activation users: %w", err)
	}

	return activationUsers, nil
}
