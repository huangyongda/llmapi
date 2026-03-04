package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"llmapi/internal/config"
	"llmapi/internal/models"
	"llmapi/internal/services"
)

type ProxyHandler struct {
	proxyService *services.ProxyService
}

func NewProxyHandler() *ProxyHandler {
	return &ProxyHandler{
		proxyService: services.NewProxyService(),
	}
}

func (h *ProxyHandler) HandleChatCompletions(c *gin.Context) {
	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key not found"})
		return
	}

	ak := apiKey.(*models.APIKey)

	// 检查是否为流式请求
	if c.GetHeader("Accept") == "text/event-stream" || c.GetHeader("Content-Type") == "application/json" {
		// 简单检查stream字段
		// 实际应该解析body，但为了性能在这里不做处理
		// 让代理服务处理
	}

	h.proxyService.ProxyRequest(c.Writer, c.Request, ak)
}

func (h *ProxyHandler) ProxyHandler(c *gin.Context) {

	var targetHost string

	// ===== 判断类型 =====
	hasXApiKey := c.Request.Header.Get("x-api-key") != ""
	authHeader := c.Request.Header.Get("Authorization")

	if hasXApiKey {
		targetHost = config.AppConfig.LLM.APIURL
	} else if strings.HasPrefix(authHeader, "Bearer ") {
		targetHost = config.AppConfig.LLM.APIURL
	} else {
		c.String(http.StatusBadRequest, "Missing API key header")
		return
	}

	target, err := url.Parse(targetHost)
	if err != nil {
		return
	}

	// 去掉默认端口
	if target.Port() == "443" || target.Port() == "80" {
		target.Host = target.Hostname()
	}

	// 拼接目标 URL
	targetURL, err := url.Parse(targetHost)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	targetURL.Path = c.Request.URL.Path
	targetURL.RawQuery = c.Request.URL.RawQuery

	fmt.Println("Target URL:", targetURL.String())

	// ===== 先读取请求体 =====
	requestBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer c.Request.Body.Close()

	// 创建请求（使用读取后的 body）
	req, err := http.NewRequest(
		c.Request.Method,
		targetURL.String(),
		bytes.NewReader(requestBody),
	)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// 复制 header
	req.Header = c.Request.Header.Clone()

	// ===== 替换 API Key =====
	if hasXApiKey {
		// Anthropic
		req.Header.Set("x-api-key", config.AppConfig.LLM.APIKey)
		req.Header.Del("Authorization")
	} else {
		// OpenAI
		req.Header.Set("Authorization", "Bearer "+config.AppConfig.LLM.APIKey)
		req.Header.Del("x-api-key")
	}
	req.Host = target.Hostname()

	// HTTP Client（支持流式）
	client := &http.Client{

		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		c.String(http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for k, vv := range resp.Header {
		for _, v := range vv {
			c.Writer.Header().Add(k, v)
		}
	}

	c.Status(resp.StatusCode)

	// 流式透传
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		log.Println("Stream error:", err)
	}

	latencyMs := int(time.Since(startTime).Milliseconds())

	// 处理用量统计
	apikey, exists := c.Get("api_key")
	if exists {
		ak := apikey.(*models.APIKey)
		h.proxyService.HandleResponseUsage(requestBody, "--", ak, latencyMs)
	}
}

func (h *ProxyHandler) HandleCompletions(c *gin.Context) {
	apiKey, exists := c.Get("api_key")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key not found"})
		return
	}

	ak := apiKey.(*models.APIKey)
	h.proxyService.ProxyRequest(c.Writer, c.Request, ak)
}

func (h *ProxyHandler) HandleModels(c *gin.Context) {
	// 返回支持的模型列表
	models := []string{
		"gpt-4",
		"gpt-3.5-turbo",
		"gpt-3.5",
		"claude-3-opus",
		"claude-3-sonnet",
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   models,
	})
}

func (h *ProxyHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
