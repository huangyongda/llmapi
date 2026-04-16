package tools

import (
	"regexp"
	"strconv"

	"github.com/pkoukk/tiktoken-go"
)

// UsageInfo Token 使用量信息
type UsageInfo struct {
	PromptTokens int  // 输入 token
	OutputTokens int  // 输出 token
	TotalTokens  int  // 总 token
	IsCalculated bool // 是否需要自己计算
}

// ExtractUsage 从 LLM 流式响应字符串中提取 token 使用量
// 支持以下格式：
//   - glm 格式: prompt_tokens/completion_tokens/total_tokens
//   - Anthropic 格式: input_tokens/output_tokens
//   - 无 usage: 需要自己计算，需要调用 CalculateUsage 计算
func ExtractUsage(data string) UsageInfo {
	usage := UsageInfo{}

	// 尝试提取 glm 格式: "prompt_tokens":41,"completion_tokens":85,"total_tokens":126
	// 取最后一个匹配的值，因为可能有多个 usage 块
	re := regexp.MustCompile(`"prompt_tokens"\s*:\s*(\d+)`)
	if matches := re.FindAllStringSubmatch(data, -1); len(matches) > 0 {
		if v, err := strconv.Atoi(matches[len(matches)-1][1]); err == nil {
			usage.PromptTokens = v
		}
	}
	re = regexp.MustCompile(`"completion_tokens"\s*:\s*(\d+)`)
	if matches := re.FindAllStringSubmatch(data, -1); len(matches) > 0 {
		if v, err := strconv.Atoi(matches[len(matches)-1][1]); err == nil {
			usage.OutputTokens = v
		}
	}
	re = regexp.MustCompile(`"total_tokens"\s*:\s*(\d+)`)
	if matches := re.FindAllStringSubmatch(data, -1); len(matches) > 0 {
		if v, err := strconv.Atoi(matches[len(matches)-1][1]); err == nil {
			usage.TotalTokens = v
		}
	}

	// 如果没有 total_tokens，但有其他两个，计算总和
	if usage.TotalTokens == 0 && usage.PromptTokens > 0 && usage.OutputTokens > 0 {
		usage.TotalTokens = usage.PromptTokens + usage.OutputTokens
	}

	// 如果没找到 glm 格式，尝试 Anthropic 格式: "input_tokens":121,"output_tokens":3354
	if usage.PromptTokens == 0 && usage.OutputTokens == 0 {
		re = regexp.MustCompile(`"input_tokens"\s*:\s*(\d+)`)
		if matches := re.FindAllStringSubmatch(data, -1); len(matches) > 0 {
			if v, err := strconv.Atoi(matches[len(matches)-1][1]); err == nil {
				usage.PromptTokens = v
			}
		}
		re = regexp.MustCompile(`"output_tokens"\s*:\s*(\d+)`)
		if matches := re.FindAllStringSubmatch(data, -1); len(matches) > 0 {
			if v, err := strconv.Atoi(matches[len(matches)-1][1]); err == nil {
				usage.OutputTokens = v
			}
		}
		cache_read_input_tokens := 0
		re = regexp.MustCompile(`"cache_read_input_tokens"\s*:\s*(\d+)`)
		if matches := re.FindAllStringSubmatch(data, -1); len(matches) > 0 {
			if v, err := strconv.Atoi(matches[len(matches)-1][1]); err == nil {
				cache_read_input_tokens = v
			}
		}
		if usage.PromptTokens > 0 && cache_read_input_tokens > 0 {
			usage.PromptTokens += cache_read_input_tokens
		}
		usage.TotalTokens = usage.PromptTokens + usage.OutputTokens
	}

	// 如果都没有，需要自己计算
	if usage.PromptTokens == 0 && usage.OutputTokens == 0 {
		usage.IsCalculated = true
	}

	return usage
}

// ExtractContent 从 SSE 流式响应字符串中提取并拼接所有 delta.content
// 输入格式: "data: {...} data: {...} data: {...}"
// 返回拼接后的完整字符串
func ExtractContent(data string) string {
	var result string

	// 匹配 "data: {...}" 块，支持跨行
	re := regexp.MustCompile(`data:\s*(\{[^}]*(?:\{[^}]*\}[^}]*)*\})`)
	matches := re.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		jsonStr := match[1]

		// 提取 delta.content
		contentRe := regexp.MustCompile(`"delta"\s*:\s*\{[^}]*"content"\s*:\s*"([^"]*)"`)
		contentMatch := contentRe.FindStringSubmatch(jsonStr)
		if len(contentMatch) >= 2 {
			result += contentMatch[1]
		}
	}

	return result
}

// CalculateUsage 使用 tiktoken 计算 token 使用量
// inputText: 输入的 prompt 文本
// outputText: 输出的 completion 文本
func CalculateUsage(inputText, outputText string) (promptTokens, outputTokens int) {
	encoding, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return 0, 0
	}

	if inputText != "" {
		promptTokens = len(encoding.Encode(inputText, nil, nil))
	}
	if outputText != "" {
		outputTokens = len(encoding.Encode(outputText, nil, nil))
	}

	return promptTokens, outputTokens
}
