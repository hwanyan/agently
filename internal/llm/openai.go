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

	"github.com/hwanyan/agently/pkg/types"
	"github.com/hwanyan/agently/pkg/utils"
)

// OpenAIClient OpenAI客户端
// 实现了types.LLMClient接口，用于与OpenAI API通信
type OpenAIClient struct {
	provider types.ConfigProvider // 配置提供者：每次请求时读取最新的 API Key / Model / BaseURL（支持热更新）
	client   *http.Client         // HTTP客户端
}

// NewOpenAIClient 创建一个新的OpenAI客户端
// 参数：
//   - provider: 配置提供者，用于在每次调用时读取最新的 LLM 配置（API Key/Model/BaseURL），支持热更新
//
// 返回：
//   - *OpenAIClient: OpenAI客户端实例
func NewOpenAIClient(provider types.ConfigProvider) *OpenAIClient {
	return &OpenAIClient{
		provider: provider,
		client: &http.Client{
			Timeout: 600 * time.Second, // 设置 600 秒超时
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
	// 每次调用都读取最新的 LLM 配置，支持 config.yaml 热更新
	cfg := c.provider().LLM

	// 构建请求体
	reqBody := c.buildChatRequest(cfg.Model, messages, tools)

	// 发送请求
	respBody, err := c.sendRequest(ctx, cfg.BaseURL, cfg.APIKey, "/chat/completions", reqBody)
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
func (c *OpenAIClient) ChatStream(ctx context.Context, messages []types.Message, tools []types.Tool) (<-chan types.StreamChunk, error) {
	// 每次调用都读取最新的 LLM 配置，支持 config.yaml 热更新
	cfg := c.provider().LLM

	// 构建请求体（启用流式响应）
	reqBody := c.buildChatRequest(cfg.Model, messages, tools)
	reqBody["stream"] = true

	// 创建响应通道
	responseChan := make(chan types.StreamChunk, 100)

	// 启动goroutine处理流式响应
	go func() {
		defer close(responseChan)

		// 发送流式请求
		if err := c.sendStreamRequest(ctx, cfg.BaseURL, cfg.APIKey, "/chat/completions", reqBody, responseChan); err != nil {
			// 发送错误信息到通道
			responseChan <- types.StreamChunk{Content: fmt.Sprintf("错误: %v", err)}
		}
	}()

	return responseChan, nil
}

// buildChatRequest 构建聊天请求体
func (c *OpenAIClient) buildChatRequest(model string, messages []types.Message, tools []types.Tool) map[string]interface{} {
	// 转换消息格式
	apiMessages := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		apiMsg := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}

		// 如果有工具调用，添加到消息中
		if len(msg.ToolCallList) > 0 {
			toolCalls := make([]map[string]interface{}, 0, len(msg.ToolCallList))
			for _, tc := range msg.ToolCallList {
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
		"model":    model,
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
func (c *OpenAIClient) sendRequest(ctx context.Context, baseURL, apiKey, endpoint string, reqBody map[string]interface{}) ([]byte, error) {
	// 序列化请求体
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

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
func (c *OpenAIClient) sendStreamRequest(ctx context.Context, baseURL, apiKey, endpoint string, reqBody map[string]interface{}, responseChan chan<- types.StreamChunk) error {
	// 序列化请求体
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
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
// 支持解析文本内容和工具调用两种响应类型
func (c *OpenAIClient) processStreamResponse(body io.Reader, responseChan chan<- types.StreamChunk) error {
	scanner := &StreamScanner{}
	scanner.SetReader(body)

	// 用于累积工具调用片段（OpenAI 流式 API 中工具调用是分片传输的）
	toolCallMap := make(map[int]*StreamToolCallAccumulator)

	// 阻塞等待，有数据就读，直到遇到 [DONE] 或 EOF 为止
	// 前面的 sendStreamRequest 和 processStreamResponse 都只执行一次
	// 只要设置了 reqBody["stream"] = true，LLM 服务端就会返回流式响应
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

			// 使用 ChatCompletionStreamResponse 解析 JSON 数据
			// 该结构体定义在 schema.go 中，对应 OpenAI 流式 API 的 SSE chunk 格式
			var streamResp ChatCompletionStreamResponse
			// 跳过无法解析的数据
			if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
				continue
			}
			if len(streamResp.Choices) == 0 {
				continue
			}
			choice := streamResp.Choices[0]

			// 将内容转发到输出通道
			if choice.Delta.Content != "" {
				responseChan <- types.StreamChunk{Content: choice.Delta.Content}
			}

			// 处理工具调用片段（累积）
			for _, tc := range choice.Delta.ToolCalls {
				idx := tc.Index
				// processStreamResponse 收到的工具调用片段可能包含多个工具调用
				// 因此要用 map 来存储多个工具调用片段
				if _, exists := toolCallMap[idx]; !exists {
					toolCallMap[idx] = &StreamToolCallAccumulator{}
				}
				entry := toolCallMap[idx]

				// 对于一个工具调用，只有首次出现时才会传递 ID 和类型，如下：
				// chunk 1: {index: 0, id: "call_xxx", type: "function", function: {name: "get_weather", arguments: ""}}
				// chunk 2: {index: 0, function: {arguments: "{\"ci"}}
				// chunk 3: {index: 0, function: {arguments: "ty\":"}}
				// chunk 4: {index: 0, function: {arguments: "\"北京\"}"}}
				if tc.ID != "" {
					entry.ID = tc.ID
				}
				if tc.Type != "" {
					entry.Type = tc.Type
				}
				if tc.Function.Name != "" {
					entry.FuncName = tc.Function.Name
				}
				// 累积参数片段（注：Arguments 的拼接不会出错）
				// 因为：SSE 是基于 HTTP/1.1 的单连接顺序传输，它在单个 TCP 连接上进行顺序字节流传输。
				// 这意味着服务端按什么顺序写入 chunk，客户端就按什么顺序读到。这是 TCP 协议的基本保证，
				// 不受网络延迟影响（网络延迟只会影响"多久收到下一个 chunk"，不会影响"收到的顺序"）。
				if tc.Function.Arguments != "" {
					entry.FuncArgs.WriteString(tc.Function.Arguments)
				}
			}

			// 当 finish_reason 为 "tool_calls" 时，表示工具调用的全部信息都已发送，这是需要汇总这些工具调用信息
			if choice.FinishReason != nil && *choice.FinishReason == "tool_calls" {
				var toolCallList []types.ToolCall
				for i := 0; i < len(toolCallMap); i++ {
					entry := toolCallMap[i]
					toolCallList = append(toolCallList, types.ToolCall{
						ID:   entry.ID,
						Type: entry.Type,
						Function: types.FunctionCall{
							Name:      entry.FuncName,
							Arguments: entry.FuncArgs.String(),
						},
					})
				}
				responseChan <- types.StreamChunk{ToolCallList: toolCallList, Done: true}
			}
		}
	}

	return scanner.Err()
}

// parseChatResponse 解析聊天响应
func (c *OpenAIClient) parseChatResponse(respBody []byte) (*types.Message, error) {
	// 使用 ChatCompletionResponse 解析完整的 JSON 响应
	// 该结构体定义在 schema.go 中，对应 OpenAI Chat Completions API 的标准响应格式
	var apiResp ChatCompletionResponse

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
		message.ToolCallList = toolCalls
	}

	return message, nil
}
