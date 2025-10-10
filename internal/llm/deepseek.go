package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agent-hwan/internal/llm/utils"
	"agent-hwan/pkg/types"
)

// DeepSeekClient DeepSeek API客户端
type DeepSeekClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewDeepSeekClient 创建新的DeepSeek客户端
func NewDeepSeekClient(apiKey, model, baseURL string) *DeepSeekClient {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}

	if model == "" {
		model = "deepseek-chat"
	}

	return &DeepSeekClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Chat 发送消息到DeepSeek并获取响应
func (c *DeepSeekClient) Chat(ctx context.Context, messages []types.Message, tools []types.Tool) (*types.Message, error) {
	// 构建请求体
	reqBody := c.buildChatRequest(messages, tools)

	// 发送请求
	respBody, err := c.sendRequest(ctx, "/chat/completions", reqBody)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	// 解析响应
	return c.parseChatResponse(respBody)
}

// ChatStream 流式发送消息到DeepSeek
func (c *DeepSeekClient) ChatStream(ctx context.Context, messages []types.Message, tools []types.Tool) (<-chan string, error) {
	// 构建请求体（启用流式响应）
	reqBody := c.buildChatRequest(messages, tools)
	reqBody["stream"] = true

	// 创建响应通道
	responseChan := make(chan string, 100)

	// 启动goroutine处理流式响应
	go func() {
		defer close(responseChan)

		// 发送流式请求
		if err := c.sendStreamRequest(ctx, "/chat/completions", reqBody, responseChan); err != nil {
			// 发送错误信息到通道
			responseChan <- fmt.Sprintf("错误: %v", err)
		}
	}()

	return responseChan, nil
}

// buildChatRequest 构建聊天请求体
func (c *DeepSeekClient) buildChatRequest(messages []types.Message, tools []types.Tool) map[string]interface{} {
	// 转换消息格式
	apiMessages := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		apiMsg := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}

		// 如果有工具调用，添加到消息中
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]map[string]interface{}, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, map[string]interface{}{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			apiMsg["tool_calls"] = toolCalls
		}

		// 如果是工具响应消息，添加tool_call_id
		if msg.ToolCallID != "" {
			apiMsg["tool_call_id"] = msg.ToolCallID
		}

		apiMessages = append(apiMessages, apiMsg)
	}

	// 构建基础请求体
	reqBody := map[string]interface{}{
		"model":    c.model,
		"messages": apiMessages,
	}

	// 如果有工具，添加工具定义
	if len(tools) > 0 {
		apiTools := make([]map[string]interface{}, 0, len(tools))
		for _, tool := range tools {
			apiTools = append(apiTools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        tool.Name(),
					"description": tool.Description(),
					"parameters":  tool.Parameters(),
				},
			})
		}
		reqBody["tools"] = apiTools
		reqBody["tool_choice"] = "auto" // 让模型自动决定是否使用工具
	}

	return reqBody
}

// sendRequest 发送HTTP请求
func (c *DeepSeekClient) sendRequest(ctx context.Context, endpoint string, reqBody map[string]interface{}) ([]byte, error) {
	// 序列化请求体
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// sendStreamRequest 发送流式HTTP请求
func (c *DeepSeekClient) sendStreamRequest(ctx context.Context, endpoint string, reqBody map[string]interface{}, responseChan chan<- string) error {
	// 序列化请求体
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	// 处理流式响应
	return c.processStreamResponse(resp.Body, responseChan)
}

// processStreamResponse 处理流式响应
func (c *DeepSeekClient) processStreamResponse(body io.Reader, responseChan chan<- string) error {
	scanner := &utils.StreamScanner{}
	scanner.SetReader(body)

	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 处理数据行
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// 检查是否是结束标记
			if data == "[DONE]" {
				break
			}

			// 解析JSON数据
			var streamResp struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue // 跳过无法解析的数据
			}

			// 发送内容到通道
			if len(streamResp.Choices) > 0 && streamResp.Choices[0].Delta.Content != "" {
				responseChan <- streamResp.Choices[0].Delta.Content
			}
		}
	}

	return scanner.Err()
}

// parseChatResponse 解析聊天响应
func (c *DeepSeekClient) parseChatResponse(respBody []byte) (*types.Message, error) {
	var apiResp struct {
		Choices []struct {
			Message struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("响应中没有选择项")
	}

	choice := apiResp.Choices[0]
	message := &types.Message{
		ID:        utils.GenerateMessageID(),
		Role:      choice.Message.Role,
		Content:   choice.Message.Content,
		Timestamp: time.Now(),
	}

	// 处理工具调用
	if len(choice.Message.ToolCalls) > 0 {
		toolCalls := make([]types.ToolCall, 0, len(choice.Message.ToolCalls))
		for _, tc := range choice.Message.ToolCalls {
			toolCalls = append(toolCalls, types.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: types.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		message.ToolCalls = toolCalls
	}

	return message, nil
}

// GetModel 获取当前模型
func (c *DeepSeekClient) GetModel() string {
	return c.model
}

// SetModel 设置模型
func (c *DeepSeekClient) SetModel(model string) {
	c.model = model
}

// GetBaseURL 获取基础URL
func (c *DeepSeekClient) GetBaseURL() string {
	return c.baseURL
}

// SetBaseURL 设置基础URL
func (c *DeepSeekClient) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

// ValidateConfig 验证配置
func (c *DeepSeekClient) ValidateConfig() error {
	if c.apiKey == "" {
		return fmt.Errorf("DeepSeek API密钥不能为空")
	}

	if c.baseURL == "" {
		return fmt.Errorf("DeepSeek基础URL不能为空")
	}

	if c.model == "" {
		return fmt.Errorf("DeepSeek模型不能为空")
	}

	return nil
}

// TestConnection 测试连接
func (c *DeepSeekClient) TestConnection(ctx context.Context) error {
	messages := []types.Message{
		{
			Role:    "user",
			Content: "Hello",
		},
	}

	_, err := c.Chat(ctx, messages, nil)
	return err
}

// GetSupportedModels 获取支持的模型列表
func (c *DeepSeekClient) GetSupportedModels() []string {
	return []string{
		"deepseek-chat",
		"deepseek-coder",
		"deepseek-math",
		"deepseek-reasoner",
	}
}

// EstimateTokens 估算token数量（简单实现）
func (c *DeepSeekClient) EstimateTokens(text string) int {
	// 简单的token估算：大约4个字符=1个token（对于中文和英文的混合文本）
	return len([]rune(text)) / 3
}

// CalculateCost 计算成本（需要根据DeepSeek的实际定价）
func (c *DeepSeekClient) CalculateCost(promptTokens, completionTokens int) float64 {
	// DeepSeek的定价（示例，需要根据实际定价调整）
	// 输入: $0.14 / 1M tokens
	// 输出: $0.28 / 1M tokens
	inputCost := float64(promptTokens) * 0.14 / 1000000
	outputCost := float64(completionTokens) * 0.28 / 1000000
	return inputCost + outputCost
}
