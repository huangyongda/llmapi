package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"llmapi/internal/database"
	"llmapi/internal/models"
)

type APIKeyService struct{}

func NewAPIKeyService() *APIKeyService {
	return &APIKeyService{}
}

func (s *APIKeyService) GenerateKeyValue() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	return "sk-cp-" + hex.EncodeToString(bytes), nil
}

func (s *APIKeyService) CreateAPIKey(userID int64, keyName string, expiresAt *time.Time) (*models.APIKey, error) {
	// 检查用户已有的API Key数量
	var count int64
	if err := database.DB.Model(&models.APIKey{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("failed to count API keys: %w", err)
	}
	if count >= 5 {
		return nil, fmt.Errorf("每个用户最多只能创建5个API Key")
	}

	keyValue, err := s.GenerateKeyValue()
	if err != nil {
		return nil, err
	}

	apiKey := &models.APIKey{
		UserID:    userID,
		KeyValue:  keyValue,
		KeyName:   keyName,
		IsActive:  true,
		ExpiresAt: expiresAt,
	}

	if err := database.DB.Create(apiKey).Error; err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return apiKey, nil
}

func (s *APIKeyService) GetAPIKeyByID(id int64) (*models.APIKey, error) {
	var apiKey models.APIKey
	if err := database.DB.Preload("User").First(&apiKey, id).Error; err != nil {
		return nil, fmt.Errorf("API key not found: %w", err)
	}
	return &apiKey, nil
}

func (s *APIKeyService) GetAPIKeyByValue(keyValue string) (*models.APIKey, error) {
	// 直接从数据库查询，获取完整的API Key信息
	var apiKey models.APIKey
	if err := database.DB.Preload("User").Where("key_value = ?", keyValue).First(&apiKey).Error; err != nil {
		return nil, fmt.Errorf("API key not found: %w", err)
	}

	// 检查是否过期
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API key has expired")
	}

	// 检查用户是否过期
	if apiKey.User.ExpiresAt != nil && apiKey.User.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("user has expired")
	}

	// 检查是否激活
	if !apiKey.IsActive {
		return nil, fmt.Errorf("API key is inactive")
	}

	return &apiKey, nil
}

func (s *APIKeyService) GetAPIKeysByUserID(userID int64) ([]models.APIKey, error) {
	var apiKeys []models.APIKey
	if err := database.DB.Where("user_id = ?", userID).Find(&apiKeys).Error; err != nil {
		return nil, fmt.Errorf("failed to get API keys: %w", err)
	}
	return apiKeys, nil
}

func (s *APIKeyService) GetAllAPIKeys(page, pageSize int, keySearch string, userID int64) ([]models.APIKey, int64, error) {
	var apiKeys []models.APIKey
	var total int64

	query := database.DB.Model(&models.APIKey{}).Preload("User")

	// 按 key 值搜索（模糊匹配）
	if keySearch != "" {
		query = query.Where("key_value LIKE ?", "%"+keySearch+"%")
	}

	// 按用户ID搜索
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&apiKeys).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get API keys: %w", err)
	}

	return apiKeys, total, nil
}

func (s *APIKeyService) DeleteAPIKey(id int64) error {
	return database.DB.Delete(&models.APIKey{}, id).Error
}

func (s *APIKeyService) ResetAPIKey(id int64) (*models.APIKey, error) {
	_, err := s.GetAPIKeyByID(id)
	if err != nil {
		return nil, err
	}

	// 生成新的Key值
	newKeyValue, err := s.GenerateKeyValue()
	if err != nil {
		return nil, err
	}

	// 更新数据库
	if err := database.DB.Model(&models.APIKey{}).Where("id = ?", id).Update("key_value", newKeyValue).Error; err != nil {
		return nil, fmt.Errorf("failed to reset API key: %w", err)
	}

	return s.GetAPIKeyByID(id)
}

func (s *APIKeyService) ToggleAPIKeyStatus(id int64) error {
	apiKey, err := s.GetAPIKeyByID(id)
	if err != nil {
		return err
	}

	newStatus := !apiKey.IsActive

	if err := database.DB.Model(&models.APIKey{}).Where("id = ?", id).Update("is_active", newStatus).Error; err != nil {
		return fmt.Errorf("failed to toggle API key status: %w", err)
	}

	return nil
}

// SyncAPIKeysStatusByUserID 根据用户过期状态同步所有 API Key
func (s *APIKeyService) SyncAPIKeysStatusByUserID(userID int64, isExpired bool) error {
	// isExpired=true 表示用户过期，禁用所有 API Key
	// isExpired=false 表示用户未过期，启用所有 API Key
	isActive := !isExpired
	return database.DB.Model(&models.APIKey{}).Where("user_id = ?", userID).Update("is_active", isActive).Error
}
