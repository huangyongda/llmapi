package handlers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"llmapi/internal/config"
	"llmapi/internal/services"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)                  // 复制一份
	return w.ResponseWriter.Write(b) // 正常写回客户端
}

//{"id":"0609843beb940487f98bbf3b5141386f","choices":[{"finish_reason":"stop","index":0,"message":{"content":"我是 MiniMax-M2.5，是一个由 MiniMax 公司开发的 AI 助手。有什么我可以帮助您的吗？","role":"assistant","name":"MiniMax AI","audio_content":"","reasoning_content":"用户问我是什么模型。我应该如实回答我是MiniMax-M2.5。","reasoning_details":[{"type":"reasoning.text","id":"reasoning-text-1","format":"MiniMax-response-v1","index":0,"text":"用户问我是什么模型。我应该如实回答我是MiniMax-M2.5。"}]}}],"created":1773818171,"model":"MiniMax-M2.5","object":"chat.completion","usage":{"total_tokens":87,"total_characters":0,"prompt_tokens":44,"completion_tokens":43,"completion_tokens_details":{"reasoning_tokens":20}},"input_sensitive":false,"output_sensitive":false,"input_sensitive_type":0,"output_sensitive_type":0,"output_sensitive_int":0,"base_resp":{"status_code":0,"status_msg":""}}

type JsonResponse struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Type    string `json:"type"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
	}
	Usage struct {
		TotalTokens              int `json:"total_tokens"`
		TotalCharacters          int `json:"total_characters"`
		PromptTokens             int `json:"prompt_tokens"`
		CompletionTokens         int `json:"completion_tokens"`
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	}
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	}
}

func ResponseLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		blw := &bodyLogWriter{
			body:           bytes.NewBufferString(""),
			ResponseWriter: c.Writer,
		}

		userId := c.GetInt64("user_id")
		apiKeyId := c.GetInt64("apiKeyId")

		c.Writer = blw

		startTime := time.Now()

		c.Next() // 执行后续 handler（proxy）
		latencyMs := int(time.Since(startTime).Milliseconds())

		// 这里拿到返回内容
		responseBody := blw.body.String()
		bodyBytes := blw.body.Bytes()

		encoding := c.Writer.Header().Get("Content-Encoding")

		// fmt.Println("Encoding:", encoding)
		if encoding == "gzip" {
			reader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
			if err != nil {
				fmt.Println("gzip reader error:", err)
				return
			}
			defer reader.Close()
			var resultStr []byte
			resultStr, _ = io.ReadAll(reader)
			responseBody = string(resultStr)
		}
		if encoding == "br" {
			reader := brotli.NewReader(bytes.NewReader(bodyBytes))
			resultStr, err := io.ReadAll(reader)
			if err != nil {
				fmt.Println("brotli error:", err)
				return
			}
			responseBody = string(resultStr)
		}

		// 你可以写日志 / 存数据库 / 打印
		//获取最后一行数据
		var lastDataLine string

		if strings.Contains(responseBody, `"type":"message_delta"`) {
			lines := strings.Split(responseBody, "\n")

			for _, line := range lines {
				if strings.Contains(line, `"type":"message_delta"`) {
					// fmt.Println(line)
					lastDataLine = line
				}
			}
		} else {
			scanner := bufio.NewScanner(strings.NewReader(responseBody))
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					lastDataLine = line // 持续覆盖，最后就是最后一行
				}

			}
			// fmt.Println("=== Response ===")
			// fmt.Println(lastDataLine)

		}

		//如果捕获到数据，且是data: 开头，则去掉data: 前缀
		if lastDataLine != "" && strings.HasPrefix(lastDataLine, "data: ") {
			lastDataLine = strings.TrimPrefix(lastDataLine, "data: ")
		} else {
			lastDataLine = responseBody
		}

		// fmt.Println("=== Response ===")
		// fmt.Println("lastDataLine", lastDataLine)
		//把lastDataLine 转换成json
		var result JsonResponse
		err := json.Unmarshal([]byte(lastDataLine), &result)
		if err != nil {
			fmt.Println("Error:", err)
		}
		if result.BaseResp.StatusCode != 0 && result.Type == "" {
			fmt.Println("minimax返回错误:", result.BaseResp.StatusMsg, ",userid:", userId, "完整返回:", result)
		}
		// fmt.Println("json result:", result.Usage)
		go SaveResponseUsage(userId, apiKeyId, result, result.Model, latencyMs)

		curUseApiKey, _ := c.Get("cur_use_api_key")
		useNum := config.AppConfig.LLM.GetKeyUseInfo(curUseApiKey.(string))
		//时间 和 Response
		fmt.Println("Time:", time.Now().Format("2006-01-02 15:04:05"), "Current Usage:", useNum, "Response:", responseBody)
	}
}

func SaveResponseUsage(userid, apiKeyId int64, result JsonResponse, model string, latencyMs int) {

	// 计算费用 (简单估算)
	cost := float64(result.Usage.TotalTokens) * 0.00001

	// 记录用量
	usageService := services.NewUsageService()
	usageService.CreateUsageLog(
		apiKeyId,
		userid,
		model,
		result.Usage.PromptTokens+result.Usage.InputTokens,
		result.Usage.CompletionTokens+result.Usage.OutputTokens,
		result.Usage.TotalTokens+result.Usage.InputTokens+result.Usage.OutputTokens,
		cost,
		latencyMs,
	)
}
