// Package mcp 提供 MCP（Model Context Protocol）工具调用的基础框架
// MCP 是一种标准化的工具调用协议，允许 Agent 通过统一接口调用远程工具服务
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"agent-hwan/internal/logger"
	"agent-hwan/pkg/types"
)

// Client MCP客户端
// 负责与MCP服务器通信，发现和调用远程工具
type Client struct {
	serverName string        // MCP服务名称
	baseURL    string        // MCP服务地址
	apiKey     string        // API密钥（可选）
	timeout    time.Duration // 请求超时时间
	httpClient *http.Client  // HTTP客户端
}

// NewClient 创建MCP客户端
// 参数：
//   - config: MCP服务器配置
//
// 返回：
//   - *Client: MCP客户端实例
func NewClient(config types.MCPServerConfig) *Client {
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second // 默认30秒超时
	}

	return &Client{
		serverName: config.Name,
		baseURL:    config.BaseURL,
		apiKey:     config.APIKey,
		timeout:    timeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// ToolDefinition MCP工具定义（从MCP服务器获取）
type ToolDefinition struct {
	Name        string                 `json:"name"`        // 工具名称
	Description string                 `json:"description"` // 工具描述
	InputSchema map[string]interface{} `json:"inputSchema"` // 输入参数的JSON Schema
}

// CallToolRequest MCP工具调用请求
type CallToolRequest struct {
	Name      string                 `json:"name"`      // 工具名称
	Arguments map[string]interface{} `json:"arguments"` // 调用参数
}

// CallToolResponse MCP工具调用响应
type CallToolResponse struct {
	Content []ContentBlock `json:"content"` // 响应内容块列表
	IsError bool           `json:"isError"` // 是否为错误响应
}

// ContentBlock MCP响应内容块
type ContentBlock struct {
	Type string `json:"type"` // 内容类型（text, image, resource等）
	Text string `json:"text"` // 文本内容
}

// ListToolsResponse MCP列出工具响应
type ListToolsResponse struct {
	Tools []ToolDefinition `json:"tools"` // 工具定义列表
}

// ListTools 从MCP服务器获取可用工具列表
// 返回：
//   - []ToolDefinition: 工具定义列表
//   - error: 可能的错误
func (c *Client) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	logger.Info("从MCP服务器 [%s] 获取工具列表...", c.serverName)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/list",
		"id":      1,
	}

	respBody, err := c.doRequest(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("获取MCP工具列表失败: %w", err)
	}

	// 解析响应
	var result struct {
		Result ListToolsResponse `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析MCP工具列表响应失败: %w", err)
	}

	logger.Info("从MCP服务器 [%s] 获取到 %d 个工具", c.serverName, len(result.Result.Tools))
	return result.Result.Tools, nil
}

// CallTool 调用MCP工具
// 参数：
//   - ctx: 上下文
//   - toolName: 工具名称
//   - arguments: 调用参数
//
// 返回：
//   - string: 工具执行结果
//   - error: 可能的错误
func (c *Client) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error) {
	logger.Info("调用MCP工具 [%s] @ [%s], 参数: %v", toolName, c.serverName, arguments)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"id":      1,
		"params": CallToolRequest{
			Name:      toolName,
			Arguments: arguments,
		},
	}

	respBody, err := c.doRequest(ctx, reqBody)
	if err != nil {
		return "", fmt.Errorf("调用MCP工具 [%s] 失败: %w", toolName, err)
	}

	// 解析响应
	var result struct {
		Result CallToolResponse `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析MCP工具响应失败: %w", err)
	}

	if result.Result.IsError {
		// 拼接错误内容
		var errMsg string
		for _, block := range result.Result.Content {
			if block.Type == "text" {
				errMsg += block.Text
			}
		}
		return "", fmt.Errorf("MCP工具执行错误: %s", errMsg)
	}

	// 拼接结果内容
	var resultText string
	for _, block := range result.Result.Content {
		if block.Type == "text" {
			resultText += block.Text
		}
	}

	logger.Info("MCP工具 [%s] 执行成功", toolName)
	return resultText, nil
}

// doRequest 执行HTTP请求
func (c *Client) doRequest(ctx context.Context, body interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP状态码异常: %d, 响应: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetServerName 获取MCP服务器名称
func (c *Client) GetServerName() string {
	return c.serverName
}
