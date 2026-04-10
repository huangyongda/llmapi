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

func (s *UserService) CreateUser(username, password string, requestLimit int, isAdmin bool, expiresAt *time.Time, remark string, level int, hasWeeklyLimit int, useGlm, useKimi int, weekRequestLimit int) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Username:         username,
		PasswordHash:     string(hash),
		RequestLimit:     requestLimit,
		IsAdmin:          isAdmin,
		Level:            level,
		ExpiresAt:        expiresAt,
		Remark:           remark,
		HasWeeklyLimit:   hasWeeklyLimit,
		UseGml:           useGlm,
		UseKimi:          useKimi,
		WeekRequestLimit: weekRequestLimit,
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

func (s *UserService) GetAllUsers(page, pageSize int, username string, sort string, level, userID, useGml, useKimi string) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := database.DB.Model(&models.User{})

	// 如果有用户名搜索条件，添加模糊匹配
	if username != "" {
		query = query.Where("username LIKE ?  or remark LIKE ?", "%"+username+"%", "%"+username+"%")
	}
	// 等级筛选
	if level != "" {
		query = query.Where("level = ?", level)
	}
	// 用户ID筛选
	if userID != "" {
		query = query.Where("id = ?", userID)
	}
	// Gml筛选
	if useGml != "" {
		query = query.Where("use_gml = ?", useGml)
	}
	// Kimi筛选
	if useKimi != "" {
		query = query.Where("use_kimi = ?", useKimi)
	}
	query.Count(&total)

	offset := (page - 1) * pageSize
	if sort == "" || sort == "id" {
		query = query.Order("id desc")
	}
	if sort == "usage" {
		query = query.Order("request_count desc")
	}

	if err := query.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get users: %w", err)
	}

	return users, total, nil
}

func (s *UserService) UpdateUser(id int64, requestLimit int, expiresAt *time.Time, remark string, level int, hasWeeklyLimit int, useGml int, useKimi int, weekRequestLimit int) error {
	// 先获取用户当前的信息
	user, err := s.GetUserByID(id)
	if err != nil {
		return err
	}

	// 更新用户信息
	updates := map[string]interface{}{
		"request_limit":      requestLimit,
		"remark":             remark,
		"level":              level,
		"has_weekly_limit":   hasWeeklyLimit,
		"use_gml":            useGml,
		"use_kimi":           useKimi,
		"week_request_limit": weekRequestLimit,
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
		newUser, err := s.CreateUser(username, password, activationUser.RequestLimit, false, &expiresAt, activationUser.Remarks, activationUser.Level, activationUser.HasWeeklyLimit, activationUser.UseGml, activationUser.UseKimi, activationUser.WeekRequestLimit)
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

func (s *UserService) IncrementRequestCount(userID int64, num string) error {
	return database.DB.Model(&models.User{}).Where("id = ?", userID).
		Update("request_count", database.DB.Raw("request_count + "+num)).Error
}

func (s *UserService) IncrementWeekRequestCount(userID int64, num string) error {
	return database.DB.Model(&models.User{}).Where("id = ?", userID).
		Update("week_request_count", database.DB.Raw("week_request_count + "+num)).Error
}

func (s *UserService) GetAvailableRequests(userID int64) (int, *models.User, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return 0, nil, err
	}

	// 判断是否启用周限额：HasWeeklyLimit == 1 且 WeekRequestLimit > 0
	if user.HasWeeklyLimit == 1 && user.WeekRequestLimit > 0 {
		// 周限额模式：返回 min(周限额剩余, 总限额剩余)
		weekAvailable := user.WeekRequestLimit - user.WeekRequestCount
		totalAvailable := user.RequestLimit - user.RequestCount
		if weekAvailable < totalAvailable {
			return weekAvailable, user, nil
		}
		return totalAvailable, user, nil
	}

	// 非周限额模式：返回总限额剩余
	return user.RequestLimit - user.RequestCount, user, nil
}

func (s *UserService) CheckAndDecrementLimit(userID int64, num string) (bool, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return false, err
	}

	// 判断是否启用周限额：HasWeeklyLimit == 1 且 WeekRequestLimit > 0
	if user.HasWeeklyLimit == 1 && user.WeekRequestLimit > 0 {
		// 周限额模式：先检查周限额，再检查总限额
		if user.WeekRequestCount >= user.WeekRequestLimit {
			return false, nil
		}
		if user.RequestCount >= user.RequestLimit {
			return false, nil
		}
	} else {
		// 非周限额模式：只检查总限额
		if user.RequestCount >= user.RequestLimit {
			return false, nil
		}
	}

	// 递增总使用次数
	if err := s.IncrementRequestCount(userID, num); err != nil {
		return false, err
	}

	// 如果启用周限额同时递增周使用次数
	if user.HasWeeklyLimit == 1 && user.WeekRequestLimit > 0 {
		if err := s.IncrementWeekRequestCount(userID, num); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (s *UserService) InitAdmin() error {
	adminConfig := config.AppConfig.Admin
	existingAdmin, _ := s.GetUserByUsername(adminConfig.Username)
	if existingAdmin != nil {
		return nil
	}

	_, err := s.CreateUser(adminConfig.Username, adminConfig.Password, 0, true, nil, "", 1, -1, -1, -1, 0)
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
func (s *UserService) CreateActivationUser(username, password string, validDays, requestLimit int, remarks string, level int, useGml int, useKimi int, hasWeeklyLimit int, weekRequestLimit int) (*models.ActivationUser, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	activationUser := &models.ActivationUser{
		Username:         username,
		PasswordHash:     string(hash),
		ValidDays:        validDays,
		RequestLimit:     requestLimit,
		Level:            level,
		UseGml:           useGml,
		UseKimi:          useKimi,
		HasWeeklyLimit:   hasWeeklyLimit,
		WeekRequestLimit: weekRequestLimit,
		Remarks:          remarks,
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
