package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"llmapi/internal/config"
	"llmapi/internal/models"
	"llmapi/internal/services"
)

type AdminHandler struct {
	userService   *services.UserService
	apiKeyService *services.APIKeyService
	usageService  *services.UsageService
}

func NewAdminHandler() *AdminHandler {
	return &AdminHandler{
		userService:   services.NewUserService(),
		apiKeyService: services.NewAPIKeyService(),
		usageService:  services.NewUsageService(),
	}
}

func (h *AdminHandler) GetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	username := c.Query("username")
	sort := c.DefaultQuery("sort", "id")

	users, total, err := h.userService.GetAllUsers(page, pageSize, username, sort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var userResponses []models.UserResponse
	for _, user := range users {
		userResponses = append(userResponses, user.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      userResponses,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req struct {
		Username     string     `json:"username" binding:"required"`
		Password     string     `json:"password" binding:"required"`
		RequestLimit int        `json:"request_limit"`
		ExpiresAt    *time.Time `json:"expires_at"`
		Remark       string     `json:"remark"`
		Level        int        `json:"level"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 如果未指定过期时间，默认7天后过期
	if req.ExpiresAt == nil {
		defaultExpiresAt := time.Now().Add(7 * 24 * time.Hour)
		req.ExpiresAt = &defaultExpiresAt
	}

	// 默认 level 为 1
	if req.Level == 0 {
		req.Level = 1
	}

	user, err := h.userService.CreateUser(req.Username, req.Password, req.RequestLimit, false, req.ExpiresAt, req.Remark, req.Level)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user.ToResponse())
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		RequestLimit int        `json:"request_limit"`
		ExpiresAt    *time.Time `json:"expires_at"`
		Remark       string     `json:"remark"`
		Level        int        `json:"level"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.userService.UpdateUser(userID, req.RequestLimit, req.ExpiresAt, req.Remark, req.Level); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.userService.DeleteUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (h *AdminHandler) GetAPIKeys(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	keySearch := c.Query("key")
	userIDStr := c.Query("user_id")

	var userID int64
	if userIDStr != "" {
		userID, _ = strconv.ParseInt(userIDStr, 10, 64)
	}

	apiKeys, total, err := h.apiKeyService.GetAllAPIKeys(page, pageSize, keySearch, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []models.APIKeyResponse
	for _, k := range apiKeys {
		resp := k.ToResponse()
		if k.User.Username != "" {
			resp.Username = k.User.Username
		}
		responses = append(responses, resp)
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      responses,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *AdminHandler) CreateAPIKey(c *gin.Context) {
	var req struct {
		UserID    int64      `json:"user_id" binding:"required"`
		KeyName   string     `json:"key_name"`
		ExpiresAt *time.Time `json:"expires_at"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	apiKey, err := h.apiKeyService.CreateAPIKey(req.UserID, req.KeyName, req.ExpiresAt)
	if err != nil {
		if strings.Contains(err.Error(), "每个用户最多只能创建5个") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, apiKey.ToResponse())
}

func (h *AdminHandler) ResetAPIKey(c *gin.Context) {
	apiKeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	apiKey, err := h.apiKeyService.ResetAPIKey(apiKeyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiKey.ToResponse())
}

func (h *AdminHandler) DeleteAPIKey(c *gin.Context) {
	apiKeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	if err := h.apiKeyService.DeleteAPIKey(apiKeyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted successfully"})
}

func (h *AdminHandler) ToggleAPIKey(c *gin.Context) {
	apiKeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	if err := h.apiKeyService.ToggleAPIKeyStatus(apiKeyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key status toggled"})
}

func (h *AdminHandler) GetUsage(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	logs, total, err := h.usageService.GetAllUsage(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []models.UsageLogResponse
	for _, log := range logs {
		responses = append(responses, log.ToResponse())
	}

	// 获取总统计
	stats, _, _, _, _ := h.usageService.GetTotalStats()

	c.JSON(http.StatusOK, gin.H{
		"data":      responses,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"stats":     stats,
	})
}

func (h *AdminHandler) GetUserUsage(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	logs, total, err := h.usageService.GetUsageByUserID(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []models.UsageLogResponse
	for _, log := range logs {
		responses = append(responses, log.ToResponse())
	}

	// 获取用户统计
	stats, tokens, cost, _ := h.usageService.GetUserStats(userID)

	c.JSON(http.StatusOK, gin.H{
		"data":         responses,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
		"total_stats":  stats,
		"total_tokens": tokens,
		"total_cost":   cost,
	})
}

func (h *AdminHandler) GetStats(c *gin.Context) {
	totalRequests, totalTokens, totalCost, totalUsers, _ := h.usageService.GetTotalStats()

	c.JSON(http.StatusOK, gin.H{
		"total_requests": totalRequests,
		"total_tokens":   totalTokens,
		"total_cost":     totalCost,
		"total_users":    totalUsers,
	})
}

// 用户端API
func (h *AdminHandler) GetMyAPIKeys(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	apiKeys, err := h.apiKeyService.GetAPIKeysByUserID(userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []models.APIKeyResponse
	for _, k := range apiKeys {
		responses = append(responses, k.ToResponse())
	}

	c.JSON(http.StatusOK, responses)
}

func (h *AdminHandler) CreateMyAPIKey(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		KeyName string `json:"key_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		req.KeyName = "My API Key"
	}

	apiKey, err := h.apiKeyService.CreateAPIKey(userID.(int64), req.KeyName, nil)
	if err != nil {
		if strings.Contains(err.Error(), "每个用户最多只能创建5个") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, apiKey.ToResponse())
}

func (h *AdminHandler) DeleteMyAPIKey(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	apiKeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key ID"})
		return
	}

	// 验证API Key属于该用户
	apiKey, err := h.apiKeyService.GetAPIKeyByID(apiKeyID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
		return
	}

	if apiKey.UserID != userID.(int64) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	if err := h.apiKeyService.DeleteAPIKey(apiKeyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key deleted successfully"})
}

func (h *AdminHandler) GetMyUsage(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	logs, total, err := h.usageService.GetUsageByUserID(userID.(int64), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []models.UsageLogResponse
	for _, log := range logs {
		responses = append(responses, log.ToResponse())
	}

	// 获取用户统计
	stats, tokens, cost, _ := h.usageService.GetUserStats(userID.(int64))

	// 获取用户信息
	user, _ := h.userService.GetUserByID(userID.(int64))

	c.JSON(http.StatusOK, gin.H{
		"data":           responses,
		"total":          total,
		"page":           page,
		"page_size":      pageSize,
		"total_requests": stats,
		"total_tokens":   tokens,
		"total_cost":     cost,
		"request_limit":  user.RequestLimit,
		"request_used":   user.RequestCount,
	})
}

func (h *AdminHandler) GetUpstreamUsage(c *gin.Context) {
	apiKeys := config.AppConfig.LLM.APIKeys
	if len(apiKeys) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "LLM API keys not configured"})
		return
	}

	url := "https://www.minimaxi.com/v1/api/openplatform/coding_plan/remains"

	var results []map[string]interface{}

	for i, apiKey := range apiKeys {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}

		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		var upstreamResp map[string]interface{}
		if err := json.Unmarshal(body, &upstreamResp); err != nil {
			continue
		}

		// 添加 key 索引标识
		upstreamResp["key_index"] = i
		results = append(results, upstreamResp)
	}

	if len(results) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch upstream usage for all keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": results,
	})
}

// GetActivationUsers 获取待激活用户列表
func (h *AdminHandler) GetActivationUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	users, total, err := h.userService.GetAllActivationUsers(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []models.ActivationUserResponse
	for _, user := range users {
		responses = append(responses, user.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      responses,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CreateActivationUser 创建激活用户
func (h *AdminHandler) CreateActivationUser(c *gin.Context) {
	var req struct {
		Username     string `json:"username" binding:"required"`
		Password     string `json:"password" binding:"required"`
		ValidDays    int    `json:"valid_days" binding:"required"`
		RequestLimit int    `json:"request_limit" binding:"required"`
		Level        int    `json:"level"`
		Remarks      string `json:"remarks"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 默认 level 为 1
	if req.Level == 0 {
		req.Level = 1
	}

	user, err := h.userService.CreateActivationUser(req.Username, req.Password, req.ValidDays, req.RequestLimit, req.Remarks, req.Level)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user.ToResponse())
}

// DeleteActivationUser 删除激活用户
func (h *AdminHandler) DeleteActivationUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.userService.DeleteActivationUser(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Activation user deleted successfully"})
}

// BatchCreateActivationUsers 批量创建激活用户
func (h *AdminHandler) BatchCreateActivationUsers(c *gin.Context) {
	var req struct {
		Users []struct {
			Username     string `json:"username" binding:"required"`
			Password     string `json:"password" binding:"required"`
			ValidDays    int    `json:"valid_days" binding:"required"`
			RequestLimit int    `json:"request_limit" binding:"required"`
			Level        int    `json:"level"`
			Remarks      string `json:"remarks"`
		} `json:"users" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var activationUsers []models.ActivationUser
	for _, u := range req.Users {
		level := u.Level
		if level == 0 {
			level = 1
		}
		activationUsers = append(activationUsers, models.ActivationUser{
			Username:     u.Username,
			PasswordHash: u.Password, // 会在service中哈希
			ValidDays:    u.ValidDays,
			RequestLimit: u.RequestLimit,
			Level:        level,
			Remarks:      u.Remarks,
		})
	}

	users, err := h.userService.BatchCreateActivationUsers(activationUsers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responses []models.ActivationUserResponse
	for _, user := range users {
		responses = append(responses, user.ToResponse())
	}

	c.JSON(http.StatusCreated, responses)
}
