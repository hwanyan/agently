// Package llm 提供与大语言模型交互的功能
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

	"agent-hwan/pkg/types"
	"agent-hwan/pkg/utils"
)

// OpenAIClient OpenAI客户端
// 实现了types.LLMClient接口，用于与OpenAI API通信
type OpenAIClient struct {
	apiKey  string       // API密钥
	baseURL string       // API基础URL
	model   string       // 使用的模型名称
	client  *http.Client // HTTP客户端
}

// NewOpenAIClient 创建一个新的OpenAI客户端
// 参数：
//   - apiKey: OpenAI API密钥
//   - model: 要使用的模型名称（如gpt-3.5-turbo、gpt-4等）
//   - baseURL: API基础URL（可选，默认使用OpenAI官方API）
//
// 返回：
//   - *OpenAIClient: OpenAI客户端实例
func NewOpenAIClient(apiKey, model, baseURL string) *OpenAIClient {
	return &OpenAIClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 60 * time.Second, // 设置60秒超时
		},
	}
}

// Chat 发送消息到LLM并获取响应
// 参数：
//   - ctx: 上下文
//   - messages: 对话消息列表
//   - tools: 可用的工具列表
//
// 返回：
//   - *types.Message: LLM的响应消息
//   - error: 可能的错误
func (c *OpenAIClient) Chat(ctx context.Context, messages []types.Message, tools []types.Tool) (*types.Message, error) {
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

// ChatStream 流式发送消息到LLM
// 参数：
//   - ctx: 上下文
//   - messages: 对话消息列表
//   - tools: 可用的工具列表
//
// 返回：
//   - <-chan string: 流式响应的通道
//   - error: 可能的错误
func (c *OpenAIClient) ChatStream(ctx context.Context, messages []types.Message, tools []types.Tool) (<-chan string, error) {
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
func (c *OpenAIClient) buildChatRequest(messages []types.Message, tools []types.Tool) map[string]interface{} {
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
func (c *OpenAIClient) sendRequest(ctx context.Context, endpoint string, reqBody map[string]interface{}) ([]byte, error) {
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
	resp, err := c.client.Do(req)
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
func (c *OpenAIClient) sendStreamRequest(ctx context.Context, endpoint string, reqBody map[string]interface{}, responseChan chan<- string) error {
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
	resp, err := c.client.Do(req)
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
func (c *OpenAIClient) processStreamResponse(body io.Reader, responseChan chan<- string) error {
	scanner := &StreamScanner{}
	scanner.SetReader(body)

	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行和注释行
		// 空行：SSE 协议中可能用于分隔不同事件
		// 以 `:` 开头的行：SSE 协议中的注释行（例如 : This is a comment），无实际数据，跳过
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 处理数据行
		// 识别以 data: 开头的行（SSE 协议标准格式）
		// 提取有效数据部分（例如 data: {"choices": [...]} → 截取 {"choices": [...]}）
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// 检查是否是结束标记
			// 当收到 [DONE] 标记时，表示流式传输结束，退出循环
			// 这是 OpenAI 流式 API 的约定结束信号
			if data == "[DONE]" {
				break
			}

			// 解析JSON数据
			// 定义匿名结构体，匹配 OpenAI 流式响应格式：
			// choices 数组中的第一个元素
			// delta 对象中的 content 字段（增量内容）
			// 解析失败时跳过当前数据块（例如网络传输中的不完整 JSON）
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

	// 1. 当以下情况发生时返回 nil：
	//   - 数据正常读取完毕（遇到 io.EOF）
	//   - 收到 [DONE] 标记正常结束
	//   - 没有发生任何读取错误
	// 2. 当 scanner.Scan() 中发生错误时返回具体错误：
	//   - 网络连接错误：net.OpError
	//   - 超时错误：context.DeadlineExceeded
	//   - 系统I/O错误：syscall.ECONNRESET 等
	return scanner.Err()
}

// parseChatResponse 解析聊天响应
func (c *OpenAIClient) parseChatResponse(respBody []byte) (*types.Message, error) {
	var apiResp struct {
		Choices []struct {
			Message struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name        string `json:"name"`
						Description string `json:"description"`
						Arguments   string `json:"arguments"`
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
					Name:        tc.Function.Name,
					Description: tc.Function.Description,
					Arguments:   tc.Function.Arguments,
				},
			})
		}
		message.ToolCalls = toolCalls
	}

	return message, nil
}
