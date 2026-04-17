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
	"llmapi/tools"
	"strconv"
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
		post_model, _ := c.Get("post_model")
		post_model_name, ok := post_model.(string)
		if !ok {
			post_model_name = "none"
		}
		llmResponse := NormalizeLogLine(responseBody)
		if result.Usage.TotalTokens == 0 {
			usage := tools.ExtractUsage(llmResponse)
			if usage.PromptTokens > 0 || usage.OutputTokens > 0 || usage.TotalTokens > 0 {
				result.Usage.TotalTokens = usage.TotalTokens
				result.Usage.PromptTokens = usage.PromptTokens
				result.Usage.CompletionTokens = usage.OutputTokens
			}
		}

		// fmt.Println("json result:", result.Usage)
		go SaveResponseUsage(userId, apiKeyId, result, post_model_name, latencyMs)

		curUseApiKey, _ := c.Get("cur_use_api_key")
		//key后缀（7个字符）
		keySuffix := ""
		if curUseApiKey != nil {
			keyStr := curUseApiKey.(string)
			if len(keyStr) > 7 {
				keySuffix = keyStr[len(keyStr)-7:]
			} else {
				keySuffix = keyStr
			}
		}
		useNum := 0
		if key, ok := curUseApiKey.(string); ok {
			useNum = config.AppConfig.LLM.GetKeyUseInfo(key)
		}
		useNum += 1 // 因为在proxy里是请求前获取的key，所以这里+1更接近当前使用量
		//时间 和 Response
		// 获取当前返回的http状态码
		httpStatusCode := c.Writer.Status()
		retry_num, _ := c.Get("retry_num")
		requestID, _ := c.Get("RequestID")
		gmlModel := c.GetBool("gmlModel")
		userService := services.NewUserService()
		useGlm, _ := c.Get("UseGlm")

		beilv := 1.0
		if gmlModel && ok && (post_model_name == "GLM-5.1" || post_model_name == "GLM-5-Turbo") {
			//GLM-5.1和GLM-5-Turbo 14:00–18:00 (UTC+8) 扣除2倍 其他时间1倍
			hour := time.Now().Hour()
			beilv = 2.0
			if hour >= 14 && hour <= 18 {
				beilv = 3.0
			}
			// fmt.Println("GLM-5.1和GLM-5-Turbo 14:00–18:00 (UTC+8) 扣除", beilv, "倍", time.Now().Format("2006-01-02 15:04:05"))
		}
		if gmlModel {
			beilv = beilv * 1.5
		}

		if useGlm == 1 && result.Usage.TotalTokens > 0 {
			tokenUsage := result.Usage.TotalTokens
			tokenUsage = int(float64(tokenUsage) * beilv)
			// result.Usage.TotalTokens = tokenUsage
			_, _ = userService.CheckAndDecrementLimit(userId, strconv.Itoa(tokenUsage))
		}

		fmt.Println("requestID:", requestID, ",model:", post_model_name, ",retry_num:", retry_num, ",httpStatusCode:", httpStatusCode, ",userId:", userId, ",keySuffix:", keySuffix, ",Time:", time.Now().Format("2006-01-02 15:04:05"), beilv, "倍", ",Current Usage:", useNum, ",llmResponse:", llmResponse)
	}
}

// 综合处理函数
func NormalizeLogLine(log string) string {
	if log == "" {
		return ""
	}

	// 1. 替换所有换行符为空格
	result := strings.ReplaceAll(log, "\r\n", " ")
	result = strings.ReplaceAll(result, "\n", " ")
	result = strings.ReplaceAll(result, "\r", " ")

	// 2. 合并连续的空格
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}

	// 3. 去除首尾空格
	result = strings.TrimSpace(result)

	return result
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
