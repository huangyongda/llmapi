package services

import (
	"fmt"
	"time"

	"llmapi/internal/database"
	"llmapi/internal/models"
)

type UsageService struct{}

func NewUsageService() *UsageService {
	return &UsageService{}
}

func (s *UsageService) CreateUsageLog(apiKeyID, userID int64, model string, promptTokens, completionTokens, totalTokens int, cost float64, latencyMs int, RequestID string) (*models.UsageLog, error) {
	usageLog := &models.UsageLog{
		APIKeyID:         apiKeyID,
		UserID:           userID,
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		Cost:             cost,
		LatencyMs:        latencyMs,
		RequestID:        RequestID,
	}

	if err := database.DB.Create(usageLog).Error; err != nil {
		return nil, fmt.Errorf("failed to create usage log: %w", err)
	}

	return usageLog, nil
}

func (s *UsageService) GetUsageByUserID(userID int64, page, pageSize int) ([]models.UsageLog, int64, error) {
	var logs []models.UsageLog
	var total int64

	query := database.DB.Model(&models.UsageLog{}).Where("user_id = ?", userID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get usage logs: %w", err)
	}

	return logs, total, nil
}

func (s *UsageService) GetAllUsage(page, pageSize int) ([]models.UsageLog, int64, error) {
	var logs []models.UsageLog
	var total int64

	query := database.DB.Model(&models.UsageLog{})
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get usage logs: %w", err)
	}

	return logs, total, nil
}

func (s *UsageService) GetUserStats(userID int64) (int64, int64, int64, error) {
	var totalRequests, totalTokens, totalCost int64

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 今日统计
	result := database.DB.Model(&models.UsageLog{}).
		Where("user_id = ? AND created_at >= ?", userID, startOfDay).
		Select("COUNT(*)", "COALESCE(SUM(total_tokens), 0)", "COALESCE(SUM(cost), 0)").
		Row()

	if err := result.Scan(&totalRequests, &totalTokens, &totalCost); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get user stats: %w", err)
	}

	// 总计
	var totalPrompt, totalCompletion int64
	result2 := database.DB.Model(&models.UsageLog{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(prompt_tokens), 0)", "COALESCE(SUM(completion_tokens), 0)").
		Row()

	if err := result2.Scan(&totalPrompt, &totalCompletion); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get total tokens: %w", err)
	}

	return totalRequests, totalTokens + totalPrompt + totalCompletion, totalCost, nil
}

func (s *UsageService) GetTotalStats() (int64, int64, float64, int64, error) {
	var totalRequests, totalTokens int64
	var totalCost float64
	var totalUsers int64

	result := database.DB.Model(&models.UsageLog{}).
		Select("COUNT(*)", "COALESCE(SUM(total_tokens), 0)", "COALESCE(SUM(cost), 0)").
		Row()

	if err := result.Scan(&totalRequests, &totalTokens, &totalCost); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to get total stats: %w", err)
	}

	database.DB.Model(&models.User{}).Count(&totalUsers)

	return totalRequests, totalTokens, totalCost, totalUsers, nil
}

func (s *UsageService) GetUsageByAPIKey(apiKeyID int64, page, pageSize int) ([]models.UsageLog, int64, error) {
	var logs []models.UsageLog
	var total int64

	query := database.DB.Model(&models.UsageLog{}).Where("api_key_id = ?", apiKeyID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get usage logs: %w", err)
	}

	return logs, total, nil
}
