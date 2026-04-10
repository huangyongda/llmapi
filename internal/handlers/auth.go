package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"llmapi/internal/services"
)

// Session 结构
type Session struct {
	UserID    int64
	Username  string
	IsAdmin   bool
	Token     string
	ExpiresAt time.Time
}

var (
	sessions        = make(map[string]*Session)
	sessionsMu      sync.RWMutex
	userConcurrency = &sync.Map{} // map[int64]chan struct{}，每个用户一个容量为1的信号量

)

type AuthHandler struct {
	userService *services.UserService
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		userService: services.NewUserService(),
	}
}

// GenerateToken 生成会话token
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	user, err := h.userService.VerifyPassword(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// 生成会话token
	token, err := generateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// 存储会话
	sessionsMu.Lock()
	sessions[token] = &Session{
		UserID:    user.ID,
		Username:  user.Username,
		IsAdmin:   user.IsAdmin,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	sessionsMu.Unlock()

	// 设置session
	c.Set("user_id", user.ID)
	c.Set("username", user.Username)
	c.Set("is_admin", user.IsAdmin)

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"token":   token,
		"user":    user.ToResponse(),
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// 获取token
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			sessionsMu.Lock()
			delete(sessions, parts[1])
			sessionsMu.Unlock()
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	user, err := h.userService.GetUserByID(userID.(int64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user.ToResponse())
}

// SessionAuth 基于Token的会话认证中间件
func (h *AuthHandler) SessionAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		token := parts[1]
		sessionsMu.RLock()
		session, exists := sessions[token]
		sessionsMu.RUnlock()
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		if session.ExpiresAt.Before(time.Now()) {
			sessionsMu.Lock()
			delete(sessions, token)
			sessionsMu.Unlock()
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired"})
			c.Abort()
			return
		}

		// 设置用户信息到context
		c.Set("user_id", session.UserID)
		c.Set("username", session.Username)
		c.Set("is_admin", session.IsAdmin)

		c.Next()
	}
}

// GetSession 获取当前会话
func (h *AuthHandler) GetSession(c *gin.Context) (*Session, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return nil, false
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil, false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return nil, false
	}

	token := parts[1]
	sessionsMu.RLock()
	session, exists := sessions[token]
	sessionsMu.RUnlock()
	if !exists {
		return nil, false
	}

	if session.UserID != userID.(int64) {
		return nil, false
	}

	return session, true
}

// AdminRequired 检查是否为管理员
func (h *AuthHandler) AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("is_admin")
		if !exists || !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// getUserConcurrencyLimit 根据当前时间返回用户的并发限制
// 下午15:00-17:30：1个并发，其他时间段：2个并发
func getUserConcurrencyLimit() int {
	hour := time.Now().Hour()
	minute := time.Now().Minute()

	// 下午15:00-17:30（17:30即17点30分）限制为2个并发
	if (hour == 15 && minute >= 0) || (hour >= 16 && hour < 17) || (hour == 17 && minute <= 30) {
		return 2
	}
	// 其他时间段限制为5个并发
	return 5
}

// tryAcquireUserLock 尝试获取用户并发锁（非阻塞）
// 返回 true 表示获取成功，可以处理请求
// 返回 false 表示该用户已有请求在处理中
func tryAcquireUserLock(userID int64) bool {
	// 根据当前时间段获取对应的并发限制
	limit := getUserConcurrencyLimit()

	// 获取或创建该用户的信号量
	value, _ := userConcurrency.LoadOrStore(userID, make(chan struct{}, limit))
	sem := value.(chan struct{})

	// 非阻塞尝试获取
	select {
	case sem <- struct{}{}:
		return true
	default:
		return false
	}
}

// releaseUserLock 释放用户并发锁
func releaseUserLock(userID int64) {
	value, ok := userConcurrency.Load(userID)
	if !ok {
		return
	}
	sem := value.(chan struct{})
	// 非阻塞释放，避免重复释放导致panic
	select {
	case <-sem:
	default:
	}
}

// cleanupUserLock 用户登出或会话过期时清理并发锁资源（可选）
func cleanupUserLock(userID int64) {
	userConcurrency.Delete(userID)
}

// APIKeyAuth API Key认证中间件
func (h *AuthHandler) APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {

		// fmt.Println("=== Request Headers ===")
		// for key, values := range c.Request.Header {
		// 	fmt.Printf("%-20s: %s\n", key, strings.Join(values, ", "))
		// }
		// fmt.Println("=======================")

		authHeader := c.GetHeader("Authorization")
		xapiKey := c.GetHeader("x-api-key")

		if authHeader == "" && xapiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		apiKeyValue := ""
		if authHeader != "" {
			// 提取API Key
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
				c.Abort()
				return
			}

			apiKeyValue = parts[1]
		}
		if xapiKey != "" {
			apiKeyValue = xapiKey
		}

		blockedPaths := []string{
			// "v1/image_generation",
			// "v1/t2a_v2",
			// "v1/t2a_async_v2",
			"v1/files/upload",
			// "v1/voice_clone",
			// "v1/voice_design",
			"v1/video",
			"v1/music_generation",
			"v1/lyrics_generation",

			// 后续可以继续加
		}

		for _, p := range blockedPaths {
			if strings.Contains(c.Request.URL.Path, p) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "不支持这接口",
				})
				c.Abort()
				return
			}
		}

		// 验证API Key
		apiKeyService := services.NewAPIKeyService()
		fmt.Println("apiKeyValue:", apiKeyValue)
		apiKey, err := apiKeyService.GetAPIKeyByValue(apiKeyValue)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key:" + apiKeyValue})
			c.Abort()
			return
		}
		now := time.Now()
		fmt.Println("用户请求内容 ", c.Request.URL.Path, "用户id:", apiKey.UserID, ",", now.Format("2006-01-02 15:04:05"))
		// ============ 并发限制开始 ============
		// 尝试获取该用户的并发锁
		if !tryAcquireUserLock(apiKey.UserID) {

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "超过并发数量 " + now.Format("2006-01-02 15:04:05"),
			})

			fmt.Println("llmResponse: 用户已达到并发限制 ,userId:", apiKey.UserID, ",time:", now.Format("2006-01-02 15:04:05"))
			c.Abort()
			return
		}

		// 创建一个用于取消超时释放的 channel
		timeoutCancel := make(chan struct{})

		// 启动60秒超时自动释放锁的 goroutine
		go func() {
			select {
			case <-time.After(60 * time.Second):
				// 超时后自动释放锁
				releaseUserLock(apiKey.UserID)
				fmt.Println("llmResponse: 锁超时自动释放, userId:", apiKey.UserID, ",time:", time.Now().Format("2006-01-02 15:04:05"))
			case <-timeoutCancel:
				// 请求正常完成，取消超时释放
			}
		}()

		// 确保请求结束时释放锁并取消超时定时器
		defer func() {
			close(timeoutCancel)
			releaseUserLock(apiKey.UserID)
		}()

		// 检查用户额度
		userService := services.NewUserService()
		available, user, err := userService.GetAvailableRequests(apiKey.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user limit"})
			c.Abort()
			return
		}
		c.Set("level", user.Level)
		//保存用户id到context
		c.Set("user_id", apiKey.UserID)
		c.Set("apiKeyId", apiKey.ID)

		if available <= 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Request limit exceeded"})
			c.Abort()
			return
		}

		// 扣减额度
		_, err = userService.CheckAndDecrementLimit(apiKey.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decrement limit"})
			c.Abort()
			return
		}

		// 将apiKey信息存入context
		c.Set("api_key", apiKey)
		c.Next()
	}
}
