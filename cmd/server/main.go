package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"llmapi/internal/config"
	"llmapi/internal/database"
	"llmapi/internal/handlers"
	"llmapi/internal/middleware"
	"llmapi/internal/services"
	"llmapi/tools"
)

func InitDynamicWeightedSelector() {
	// 创建动态权重选择器
	//循环config.llm.api_keys
	keys := []tools.WeightedKey{}
	for i := 0; i < len(config.AppConfig.LLM.APIKeys); i++ {
		key := tools.WeightedKey{
			Key:    config.AppConfig.LLM.APIKeys[i],
			Weight: 1,
		}
		keys = append(keys, key)
	}
	Selector := tools.NewDynamicWeightedSelector(keys)
	tools.Selector = Selector
}

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
	InitDynamicWeightedSelector()

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动定时任务协程
	go runTask(ctx)

	// 初始化处理器
	authHandler := handlers.NewAuthHandler()
	proxyHandler := handlers.NewProxyHandler()
	adminHandler := handlers.NewAdminHandler()

	go executeTask()

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
	r.GET("/help.html", func(c *gin.Context) {
		c.HTML(200, "help.html", nil)
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
		anthropic.Use(authHandler.APIKeyAuth(), handlers.ResponseLogger())
		anthropic.Any("*path", proxyHandler.ProxyHandler)
	}

	// API路由
	api := r.Group("/v1")
	{
		// 需要API Key认证的路由
		api.Use(authHandler.APIKeyAuth(), handlers.ResponseLogger())
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

		// 激活用户管理
		admin.GET("/activation-users", adminHandler.GetActivationUsers)
		admin.POST("/activation-users", adminHandler.CreateActivationUser)
		admin.DELETE("/activation-users/:id", adminHandler.DeleteActivationUser)
		admin.POST("/activation-users/batch", adminHandler.BatchCreateActivationUsers)
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

// 独立协程：每 60 秒执行一次
func runTask(ctx context.Context) {
	ticker := time.NewTicker(120 * time.Second)
	defer ticker.Stop()

	fmt.Println("🕒 定时任务协程已启动")

	for {
		select {
		case <-ticker.C:
			executeTask()
		case <-ctx.Done():
			fmt.Println("🔚 定时任务协程已停止")
			return
		}
	}
}

func executeTask() {
	fmt.Println("🚀 开始执行定时任务")
	apiKeys := config.AppConfig.LLM.APIKeys
	if len(apiKeys) == 0 {
		fmt.Print("LLM API keys not configured")
		return
	}
	apiWeights := config.AppConfig.LLM.APIWeights

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
		//current_interval_usage_count
		current_interval_usage_count := 0
		for _, modelRemain := range upstreamResp["model_remains"].([]interface{}) {
			current_interval_usage_count = int(modelRemain.(map[string]interface{})["current_interval_usage_count"].(float64))
		}

		// apiWeights[i] 如果存在 则用 否则默认为1(float32)
		weight := float32(1.0)
		if i >= 0 && i < len(apiWeights) {
			if apiWeights[i] != 0 {
				weight = float32(apiWeights[i]) // 显式转换，注意精度丢失
			}
		}

		curWeight := int(weight * float32(current_interval_usage_count))

		tools.Selector.SetWeight(apiKey, curWeight)
		fmt.Println(apiKey, ":set curWeight ", curWeight)

		// 添加 key 索引标识
		upstreamResp["key_index"] = i
		results = append(results, upstreamResp)
	}
	for _, result := range results {
		//输出 current_interval_usage_count
		current_interval_usage_count := 0
		for _, modelRemain := range result["model_remains"].([]interface{}) {
			current_interval_usage_count = int(modelRemain.(map[string]interface{})["current_interval_usage_count"].(float64))
		}
		fmt.Println("key_index:", result["key_index"], "current_interval_usage_count:", current_interval_usage_count)
	}
	// fmt.Println(results)
}
