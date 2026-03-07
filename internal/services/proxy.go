package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"llmapi/internal/config"
	"llmapi/internal/models"
)

type ProxyService struct {
	httpClient *http.Client
}

func NewProxyService() *ProxyService {
	log.Printf("Creating ProxyService with config: URL=%s, Timeout=%d", config.AppConfig.LLM.APIURL, config.AppConfig.LLM.Timeout)

	var transport *http.Transport
	if config.AppConfig.LLM.ProxyURL != "" {
		log.Printf("Using proxy: %s", config.AppConfig.LLM.ProxyURL)
		proxyURL, err := url.Parse(config.AppConfig.LLM.ProxyURL)
		if err == nil {
			transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}

	client := &http.Client{
		Timeout:   time.Duration(config.AppConfig.LLM.Timeout) * time.Second,
		Transport: transport,
	}

	log.Printf("ProxyService created, httpClient: %p", client)

	return &ProxyService{
		httpClient: client,
	}
}

type ChatCompletionRequest struct {
	Model       string                   `json:"model"`
	Messages    []map[string]interface{} `json:"messages"`
	Stream      bool                     `json:"stream,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	TopP        float64                  `json:"top_p,omitempty"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []AnthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

func (s *ProxyService) MapModel(model string) string {
	if mapping, ok := config.AppConfig.LLM.ModelMapping[model]; ok {
		return mapping
	}
	return model
}

func (s *ProxyService) ForwardChatCompletion(r *http.Request, reqBody []byte, apiKey *models.APIKey) ([]byte, error) {
	// 创建新的HTTP客户端避免并发问题
	httpClient := &http.Client{
		Timeout: time.Duration(config.AppConfig.LLM.Timeout) * time.Second,
	}
	//打印reqBody
	// log.Printf("ForwardChatCompletion: reqBody=%s", reqBody)
	// 解析请求获取model
	var chatReq ChatCompletionRequest
	if err := json.Unmarshal(reqBody, &chatReq); err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	// 模型映射
	mappedModel := s.MapModel(chatReq.Model)

	var targetURL string
	var targetReq *http.Request
	var err error

	authHeader := r.Header.Get("Authorization")

	Provider := "anthropic"
	if authHeader == "" {
		Provider = "anthropic"
	} else {
		Provider = "openai"
	}

	switch Provider {
	case "anthropic":
		targetURL = config.AppConfig.LLM.APIURL + r.RequestURI

		// 转换OpenAI格式到Anthropic格式
		anthropicReq := AnthropicRequest{
			Model:       mappedModel,
			MaxTokens:   chatReq.MaxTokens,
			Temperature: chatReq.Temperature,
			Stream:      chatReq.Stream,
		}

		for _, msg := range chatReq.Messages {
			role, _ := msg["role"].(string)
			content, _ := msg["content"].(string)
			anthropicReq.Messages = append(anthropicReq.Messages, AnthropicMessage{
				Role:    role,
				Content: content,
			})
		}

		anthropicReqJSON, _ := json.Marshal(anthropicReq)
		targetReq, err = http.NewRequest("POST", targetURL, bytes.NewBuffer(anthropicReqJSON))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		// 循环 r.Header.Clone()
		for k, v := range r.Header.Clone() {
			for _, vv := range v {
				if k == "Content-Length" {
					continue
				}
				targetReq.Header.Add(k, vv)
			}
		}
		targetReq.Header.Set("x-api-key", config.AppConfig.LLM.GetNextAPIKey())

		//打印header
		log.Printf("ForwardChatCompletion: targetReq.Header=%v", targetReq.Header)

	default: // openai or custom
		targetURL = config.AppConfig.LLM.APIURL + r.RequestURI
		targetReq, err = http.NewRequest("POST", targetURL, bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// 循环 r.Header.Clone()
		for k, v := range r.Header.Clone() {
			for _, vv := range v {
				if k == "Content-Length" {
					continue
				}
				targetReq.Header.Add(k, vv)
			}
		}
		targetReq.Header.Del("Authorization")
		targetReq.Header.Set("Authorization", "Bearer "+config.AppConfig.LLM.GetNextAPIKey())
		//打印header
		log.Printf("ForwardChatCompletion: targetReq.Header=%v", targetReq.Header)
	}

	// 发送请求
	log.Printf("ForwardChatCompletion: sending request to %s", targetURL)
	log.Printf("ForwardChatCompletion: httpClient=%p, targetReq=%p", s.httpClient, targetReq)

	startTime := time.Now()
	resp, err := httpClient.Do(targetReq)
	if err != nil {
		log.Printf("ForwardChatCompletion: Do error: %v", err)
		return nil, fmt.Errorf("failed to forward request: %w", err)
	}
	log.Printf("ForwardChatCompletion: response received, status=%d", resp.StatusCode)
	latencyMs := int(time.Since(startTime).Milliseconds())

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 如果响应是错误，返回错误信息
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upstream API error: %d - %s", resp.StatusCode, string(respBody))
	}

	// 解析响应获取用量信息
	s.HandleResponseUsage(respBody, chatReq.Model, apiKey, latencyMs)

	return respBody, nil
}

func (s *ProxyService) HandleResponseUsage(respBody []byte, model string, apiKey *models.APIKey, latencyMs int) {
	var resp map[string]interface{}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return
	}

	usage := map[string]int{
		"prompt_tokens":     0,
		"completion_tokens": 0,
		"total_tokens":      0,
	}

	if usageData, ok := resp["usage"].(map[string]interface{}); ok {
		if v, ok := usageData["prompt_tokens"].(float64); ok {
			usage["prompt_tokens"] = int(v)
		}
		if v, ok := usageData["completion_tokens"].(float64); ok {
			usage["completion_tokens"] = int(v)
		}
		if v, ok := usageData["total_tokens"].(float64); ok {
			usage["total_tokens"] = int(v)
		}
	}

	// 计算费用 (简单估算)
	cost := float64(usage["total_tokens"]) * 0.00001

	// 记录用量
	usageService := NewUsageService()
	usageService.CreateUsageLog(
		apiKey.ID,
		apiKey.UserID,
		model,
		usage["prompt_tokens"],
		usage["completion_tokens"],
		usage["total_tokens"],
		cost,
		latencyMs,
	)
}

func (s *ProxyService) ProxyRequest(w http.ResponseWriter, r *http.Request, apiKey *models.APIKey) {
	log.Printf("ProxyRequest: API Key ID=%d, UserID=%d", apiKey.ID, apiKey.UserID)

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// 转发请求
	respBody, err := s.ForwardChatCompletion(r, body, apiKey)
	if err != nil {
		log.Printf("ForwardChatCompletion error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("ForwardChatCompletion: response body: %s", string(respBody))

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBody)
}

func (s *ProxyService) HandleSSE(w http.ResponseWriter, r *http.Request, apiKey *models.APIKey) {
	// 创建新的HTTP客户端
	httpClient := &http.Client{
		Timeout: time.Duration(config.AppConfig.LLM.Timeout) * time.Second,
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// 解析请求获取model
	var chatReq ChatCompletionRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	// 模型映射
	mappedModel := s.MapModel(chatReq.Model)

	// 构建目标URL
	targetURL := config.AppConfig.LLM.APIURL + "/chat/completions"

	// 更新请求体中的模型
	var updatedReq map[string]interface{}
	if err := json.Unmarshal(body, &updatedReq); err != nil {
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}
	updatedReq["model"] = mappedModel
	updatedReq["stream"] = true

	reqBody, _ := json.Marshal(updatedReq)

	// 创建转发请求
	targetReq, err := http.NewRequest("POST", targetURL, bytes.NewBuffer(reqBody))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	targetReq.Header.Set("Content-Type", "application/json")
	targetReq.Header.Set("Authorization", "Bearer "+config.AppConfig.LLM.GetNextAPIKey())

	// 发送请求
	startTime := time.Now()
	resp, err := httpClient.Do(targetReq)
	if err != nil {
		http.Error(w, "Failed to forward request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// 设置SSE响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	// 复制响应头
	for k, v := range resp.Header {
		if strings.ToLower(k) == "content-type" {
			continue
		}
		w.Header()[k] = v
	}

	// 计算token使用量
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	var totalTokens int
	decoder := json.NewDecoder(resp.Body)
	var buffer bytes.Buffer

	for {
		select {
		case <-r.Context().Done():
			return
		default:
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}

			if token == "usage" {
				var usage struct {
					PromptTokens     int `json:"prompt_tokens"`
					CompletionTokens int `json:"completion_tokens"`
					TotalTokens      int `json:"total_tokens"`
				}
				if err := decoder.Decode(&usage); err == nil {
					totalTokens = usage.TotalTokens

					// 异步记录用量
					go func() {
						cost := float64(totalTokens) * 0.00001
						usageService := NewUsageService()
						usageService.CreateUsageLog(
							apiKey.ID,
							apiKey.UserID,
							chatReq.Model,
							usage.PromptTokens,
							usage.CompletionTokens,
							usage.TotalTokens,
							cost,
							int(time.Since(startTime).Milliseconds()),
						)
					}()
				}
				continue
			}

			buffer.Reset()
			if err := json.NewEncoder(&buffer).Encode(token); err == nil {
				w.Write(buffer.Bytes())
				flusher.Flush()
			}
		}
	}
}
