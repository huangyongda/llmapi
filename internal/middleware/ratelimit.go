package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type RateLimiter struct {
	// 可以后续添加更复杂的限流逻辑
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{}
}

func (r *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 基础限流：简单检查
		// 实际的限流逻辑在认证时通过用户额度控制实现
		// 这里可以添加基于IP或其他维度的限流

		c.Next()
	}
}

// APIKeyRateLimit 基于API Key的限流
func (r *RateLimiter) APIKeyRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 限流逻辑已在认证中间件中通过用户额度实现
		c.Next()
	}
}

// CORS 中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
