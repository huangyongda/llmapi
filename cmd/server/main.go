package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"llmapi/internal/config"
	"llmapi/internal/database"
	"llmapi/internal/handlers"
	"llmapi/internal/middleware"
	"llmapi/internal/services"
)

func main() {
	// 加载配置
	if err := config.LoadConfig("configs/config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化MySQL
	if err := database.InitMySQL(&config.AppConfig.Database); err != nil {
		log.Fatalf("Failed to initialize MySQL: %v", err)
	}

	// 执行数据库迁移
	if err := database.Migrate(); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	userService := services.NewUserService()
	if err := userService.InitAdmin(); err != nil {
		log.Printf("Warning: Failed to init admin: %v", err)
	}

	// 初始化处理器
	authHandler := handlers.NewAuthHandler()
	proxyHandler := handlers.NewProxyHandler()
	adminHandler := handlers.NewAdminHandler()

	// 设置路由
	r := gin.Default()

	// 提供静态文件
	r.Static("/static", "web/static")
	r.LoadHTMLGlob("web/views/*.html")

	// 首页
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	// HTML页面路由
	r.GET("/index.html", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})
	r.GET("/login.html", func(c *gin.Context) {
		c.HTML(200, "login.html", nil)
	})
	r.GET("/dashboard.html", func(c *gin.Context) {
		c.HTML(200, "dashboard.html", nil)
	})
	r.GET("/admin.html", func(c *gin.Context) {
		c.HTML(200, "admin.html", nil)
	})

	// 添加中间件
	r.Use(middleware.CORS())

	// 健康检查
	r.GET("/health", proxyHandler.HealthCheck)

	//anthropic
	anthropic := r.Group("/anthropic")
	{
		anthropic.Use(authHandler.APIKeyAuth())
		anthropic.Any("*path", proxyHandler.ProxyHandler)
	}

	// API路由
	api := r.Group("/v1")
	{
		// 需要API Key认证的路由
		api.Use(authHandler.APIKeyAuth())
		api.Any("*path", proxyHandler.ProxyHandler)

	}

	// API路由
	web := r.Group("/web")
	{
		// 认证路由
		auth := web.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
		}

		// 用户自己的管理路由 (需要登录)
		user := web.Group("/user")
		user.Use(authHandler.SessionAuth())
		{
			user.GET("/apikeys", adminHandler.GetMyAPIKeys)
			user.POST("/apikeys", adminHandler.CreateMyAPIKey)
			user.DELETE("/apikeys/:id", adminHandler.DeleteMyAPIKey)
			user.GET("/usage", adminHandler.GetMyUsage)
			user.GET("/me", authHandler.GetCurrentUser)
		}
	}

	// 管理后台路由
	admin := r.Group("/admin")
	admin.Use(authHandler.SessionAuth())
	admin.Use(func(c *gin.Context) {
		// 检查是否是管理员
		if isAdmin, exists := c.Get("is_admin"); !exists || !isAdmin.(bool) {
			c.JSON(403, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	})
	{
		admin.GET("/login", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "Admin login endpoint"})
		})
		admin.POST("/login", authHandler.Login)

		// 用户管理
		admin.GET("/users", adminHandler.GetUsers)
		admin.POST("/users", adminHandler.CreateUser)
		admin.PUT("/users/:id", adminHandler.UpdateUser)
		admin.DELETE("/users/:id", adminHandler.DeleteUser)

		// API Key管理
		admin.GET("/apikeys", adminHandler.GetAPIKeys)
		admin.POST("/apikeys", adminHandler.CreateAPIKey)
		admin.POST("/apikeys/:id/reset", adminHandler.ResetAPIKey)
		admin.DELETE("/apikeys/:id", adminHandler.DeleteAPIKey)
		admin.POST("/apikeys/:id/toggle", adminHandler.ToggleAPIKey)

		// 用量统计
		admin.GET("/usage", adminHandler.GetUsage)
		admin.GET("/users/:user_id/usage", adminHandler.GetUserUsage)
		admin.GET("/stats", adminHandler.GetStats)
		admin.GET("/upstream-usage", adminHandler.GetUpstreamUsage)
	}

	// 启动服务器
	addr := config.AppConfig.GetServerAddr()
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	<-make(chan struct{})
	fmt.Println("Server stopped")
}
