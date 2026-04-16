package tools

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestExtractUsage(t *testing.T) {
	// 测试 glm 格式
	glmData := `data: {"id":"20260414023612262985ad09bc433c","created":1776105372,"object":"chat.completion.chunk","model":"glm-5.1","choices":[{"index":0,"finish_reason":"stop","delta":{"role":"assistant","content":""}}],"usage":{"prompt_tokens":41,"completion_tokens":85,"total_tokens":126,"prompt_tokens_details":{"cached_tokens":0},"completion_tokens_details":{"reasoning_tokens":43}}} data: [DONE]`

	usage := ExtractUsage(glmData)
	if usage.PromptTokens != 41 {
		t.Errorf("glm: expected prompt_tokens=41, got %d", usage.PromptTokens)
	}
	if usage.OutputTokens != 85 {
		t.Errorf("glm: expected completion_tokens=85, got %d", usage.OutputTokens)
	}
	if usage.TotalTokens != 126 {
		t.Errorf("glm: expected total_tokens=126, got %d", usage.TotalTokens)
	}
	if usage.IsCalculated {
		t.Error("glm: expected IsCalculated=false")
	}

	// 测试 Anthropic 格式
	anthropicData := `event: content_block_stop data: {"type": "content_block_stop", "index": 2} event: message_delta data: {"type": "message_delta", "delta": {"stop_reason": "tool_use", "stop_sequence": null}, "usage": {"input_tokens": 121, "output_tokens": 3354, "cache_read_input_tokens": 47872, "server_tool_use": {"web_search_requests": 0}, "service_tier": "standard"}} event: message_stop data: {"type": "message_stop"}`

	usage = ExtractUsage(anthropicData)
	if usage.PromptTokens != 121 {
		t.Errorf("anthropic: expected input_tokens=121, got %d", usage.PromptTokens)
	}
	if usage.OutputTokens != 3354 {
		t.Errorf("anthropic: expected output_tokens=3354, got %d", usage.OutputTokens)
	}
	if usage.TotalTokens != 3475 {
		t.Errorf("anthropic: expected total_tokens=3475, got %d", usage.TotalTokens)
	}
	if usage.IsCalculated {
		t.Error("anthropic: expected IsCalculated=false")
	}

	// 测试无 usage 格式
	noUsageData := `data: {"id":"202604140236037711f3f35cd34e0b","created":1776105363,"object":"chat.completion.chunk","model":"glm-5.1","choices":[{"index":0,"delta":{"role":"assistant","content":"吗"}}]} data: {"id":"202604140236037711f3f35cd34e0b","created":1776105363,"object":"chat.completion.chunk","model":"glm-5.1","choices":[{"index":0,"delta":{"role":"assistant","content":"？"}}]} data: [DONE]`

	usage = ExtractUsage(noUsageData)
	if usage.PromptTokens != 0 {
		t.Errorf("no usage: expected prompt_tokens=0, got %d", usage.PromptTokens)
	}
	if usage.OutputTokens != 0 {
		t.Errorf("no usage: expected output_tokens=0, got %d", usage.OutputTokens)
	}
	if !usage.IsCalculated {
		t.Error("no usage: expected IsCalculated=true")
	}
}

func TestExtractUsageFromFile(t *testing.T) {
	content, err := os.ReadFile("/Users/huangyongda/code/llmapi/test4-16-09.txt")
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	var total, success, fail int
	totalToken := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		total++

		// 提取 llmResponse: 后的内容
		idx := strings.Index(line, "llmResponse:")
		if idx == -1 {
			fail++
			continue
		}
		llmResponse := line[idx+len("llmResponse:"):]

		usage := ExtractUsage(llmResponse)
		if usage.PromptTokens > 0 || usage.OutputTokens > 0 || usage.TotalTokens > 0 {
			fmt.Println("匹配成功: ", usage)
			totalToken += usage.TotalTokens
			success++
		} else {
			t.Logf("未匹配到 usage: %s", llmResponse)
			fail++
		}
		// if usage.IsCalculated {
		// 	outputTxt := ExtractContent(llmResponse)
		// 	fmt.Println("outputTxt:", outputTxt)
		// 	promptTokens, outputTokens := CalculateUsage("", outputTxt)
		// 	usage.PromptTokens = promptTokens
		// 	usage.OutputTokens = outputTokens
		// 	usage.TotalTokens = promptTokens + outputTokens
		// 	usage.IsCalculated = false
		// 	t.Logf("重新计算 usage: %v", usage.TotalTokens)
		// }
	}
	t.Logf("总消耗: %d", totalToken)

	t.Logf("总数量: %d, 匹配成功: %d, 匹配失败: %d", total, success, fail)
}
