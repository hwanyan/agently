// Package agent 提供AI Agent的核心实现
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"agent-hwan/pkg/types"
)

// BasicAgent 基础AI Agent实现
// 这是一个完整的AI Agent实现，包含对话管理、工具调用等功能
type BasicAgent struct {
	name         string                // Agent名称
	description  string                // Agent描述
	llmClient    types.LLMClient       // LLM客户端
	systemPrompt string                // 系统提示词
	tools        map[string]types.Tool // 工具映射表
	history      []types.Message       // 对话历史
	mutex        sync.Mutex            // 互斥锁，保护并发访问
	streamDone   chan struct{}         // 标记流式响应的 history 写入是否完成
}

// NewBasicAgent 创建一个新的基础 AI Agent
// 参数：
//   - name: Agent名称
//   - description: Agent描述
//   - llmClient: LLM客户端
//   - systemPrompt: 系统提示词（可选）
//
// 返回：
//   - *BasicAgent: Agent实例
func NewBasicAgent(name, description string, llmClient types.LLMClient, systemPrompt string) *BasicAgent {
	agent := &BasicAgent{
		name:         name,
		description:  description,
		llmClient:    llmClient,
		systemPrompt: systemPrompt,
		tools:        make(map[string]types.Tool),
		history:      make([]types.Message, 0),
		streamDone:   nil, // 无流式任务时为 nil
	}

	// 如果提供了系统提示词，添加到历史记录中
	if systemPrompt != "" {
		agent.addSystemMessage(systemPrompt)
	}

	return agent
}

// ProcessMessage 处理用户消息并返回响应
// 这是Agent的主要入口点，实现了完整的对话流程
// 参数：
//   - ctx: 上下文
//   - userMessage: 用户消息
//
// 返回：
//   - string: Agent的响应
//   - error: 可能的错误
func (a *BasicAgent) ProcessMessage(ctx context.Context, userMessage string) (string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 等待上一次流式响应的 history 写入完成
	a.waitStreamDone()

	// 添加用户消息到历史记录
	a.addUserMessage(userMessage)

	// 开始对话循环，处理可能的工具调用
	for {
		// 获取可用工具列表
		tools := a.getToolsList()

		// 发送消息到LLM
		response, err := a.llmClient.Chat(ctx, a.history, tools)
		if err != nil {
			return "", fmt.Errorf("LLM 调用失败: %w", err)
		}

		// 添加LLM响应到历史记录
		a.history = append(a.history, *response)

		// 检查是否有工具调用
		if len(response.ToolCalls) == 0 {
			// 没有工具调用，直接返回响应
			return response.Content, nil
		}

		// 处理工具调用
		if e := a.processToolCalls(ctx, response.ToolCalls); e != nil {
			return "", fmt.Errorf("处理工具调用失败: %w", e)
		}

		// 继续循环，让LLM基于工具结果生成最终响应
	}
}

// ProcessMessageStream 流式处理用户消息
// 参数：
//   - ctx: 上下文
//   - userMessage: 用户消息
//
// 返回：
//   - <-chan string: 流式响应通道
//   - error: 可能的错误
func (a *BasicAgent) ProcessMessageStream(ctx context.Context, userMessage string) (<-chan string, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 等待上一次流式响应的 history 写入完成
	a.waitStreamDone()

	// 添加用户消息到历史记录
	a.addUserMessage(userMessage)

	// 获取可用工具列表
	tools := a.getToolsList()

	// 发送流式请求到LLM
	rawStream, err := a.llmClient.ChatStream(ctx, a.history, tools)
	if err != nil {
		return nil, err
	}

	// 创建新的 streamDone channel，标记本次流式任务
	done := make(chan struct{})
	a.streamDone = done

	// 创建包装的流式响应，用于收集完整响应并输出
	wrappedStream := make(chan string, 100)

	go func() {
		defer close(wrappedStream)
		defer close(a.streamDone) // 标记流式 history 写入完成

		var fullResponse strings.Builder

		// 转发流式响应并收集完整内容
		for chunk := range rawStream {
			fullResponse.WriteString(chunk)
			wrappedStream <- chunk
		}

		// 将完整的AI响应保存到历史记录
		if fullResponse.Len() > 0 {
			aiMessage := types.Message{
				ID:        generateMessageID(),
				Role:      "assistant",
				Content:   fullResponse.String(),
				ToolCalls: []types.ToolCall{}, // 流式响应暂不支持工具调用
				Timestamp: time.Now(),
			}
			a.history = append(a.history, aiMessage)
		}
	}()

	return wrappedStream, nil
}

// AddTool 添加工具到Agent
// 参数：
//   - tool: 要添加的工具
func (a *BasicAgent) AddTool(tool types.Tool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.tools[tool.Name()] = tool
}

// waitStreamDone 等待上一次流式响应的 history 写入完成
// 注意：调用此方法前必须已持有 a.mutex 锁
func (a *BasicAgent) waitStreamDone() {
	if a.streamDone == nil {
		return
	}
	// 等待流式 history 写入完成
	<-a.streamDone
	a.streamDone = nil
}

// GetConversationHistory 获取对话历史
// 返回：
//   - []types.Message: 对话历史消息列表
func (a *BasicAgent) GetConversationHistory() []types.Message {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 等待流式响应完成，确保 history 完整
	a.waitStreamDone()

	// 返回历史记录的副本，避免外部修改
	history := make([]types.Message, len(a.history))
	copy(history, a.history)
	return history
}

// ClearHistory 清空对话历史
func (a *BasicAgent) ClearHistory() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 等待上一次流式响应的 history 写入完成
	a.waitStreamDone()

	// 清空对话
	a.history = make([]types.Message, 0)

	// 如果有系统提示词，重新添加
	if a.systemPrompt != "" {
		a.addSystemMessage(a.systemPrompt)
	}
}

// GetName 获取Agent名称
func (a *BasicAgent) GetName() string {
	return a.name
}

// GetDescription 获取Agent描述
func (a *BasicAgent) GetDescription() string {
	return a.description
}

// GetTools 获取所有工具
func (a *BasicAgent) GetTools() map[string]types.Tool {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 返回工具映射的副本
	tools := make(map[string]types.Tool)
	for name, tool := range a.tools {
		tools[name] = tool
	}
	return tools
}

// addSystemMessage 添加系统消息
func (a *BasicAgent) addSystemMessage(content string) {
	message := types.Message{
		ID:        generateMessageID(),
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	}
	a.history = append(a.history, message)
}

// addUserMessage 添加用户消息
func (a *BasicAgent) addUserMessage(content string) {
	message := types.Message{
		ID:        generateMessageID(),
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	}
	a.history = append(a.history, message)
}

// addToolMessage 添加工具响应消息
func (a *BasicAgent) addToolMessage(toolCallID, content string) {
	message := types.Message{
		ID:         generateMessageID(),
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
		Timestamp:  time.Now(),
	}
	a.history = append(a.history, message)
}

// getToolsList 获取工具列表
func (a *BasicAgent) getToolsList() []types.Tool {
	tools := make([]types.Tool, 0, len(a.tools))
	for _, tool := range a.tools {
		tools = append(tools, tool)
	}
	return tools
}

// processToolCalls 处理工具调用
func (a *BasicAgent) processToolCalls(ctx context.Context, toolCalls []types.ToolCall) error {
	// fmt.Println("\n  🎯 进入工具调用函数...")
	for _, toolCall := range toolCalls {
		// 查找对应的工具
		tool, exists := a.tools[toolCall.Function.Name]
		if !exists {
			// 工具不存在，添加错误消息
			errorMsg := fmt.Sprintf("工具 '%s' 不存在", toolCall.Function.Name)
			a.addToolMessage(toolCall.ID, errorMsg)
			continue
		}

		// 解析工具参数
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			// 参数解析失败，添加错误消息
			errorMsg := fmt.Sprintf("解析工具参数失败: %v", err)
			a.addToolMessage(toolCall.ID, errorMsg)
			continue
		}

		// 执行工具
		result, err := tool.Execute(ctx, args)
		if err != nil {
			// 工具执行失败，添加错误消息
			errorMsg := fmt.Sprintf("工具执行失败: %v", err)
			a.addToolMessage(toolCall.ID, errorMsg)
			continue
		}

		// 成功执行需添加工具执行结果到历史记录
		a.addToolMessage(toolCall.ID, result)
	}

	return nil
}

// generateMessageID 生成消息ID
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
