package mcp

import (
	"context"
	"fmt"

	"agent-hwan/internal/logger"
)

// MCPTool MCP工具适配器
// 将MCP远程工具适配为本地 types.Tool 接口，使其可以被Agent直接使用
type MCPTool struct {
	client     *Client        // MCP客户端
	definition ToolDefinition // 工具定义
}

// NewMCPTool 创建MCP工具适配器
// 参数：
//   - client: MCP客户端
//   - definition: 从MCP服务器获取的工具定义
//
// 返回：
//   - *MCPTool: MCP工具适配器实例
func NewMCPTool(client *Client, definition ToolDefinition) *MCPTool {
	return &MCPTool{
		client:     client,
		definition: definition,
	}
}

// Name 返回工具名称
func (t *MCPTool) Name() string {
	return t.definition.Name
}

// Description 返回工具描述
func (t *MCPTool) Description() string {
	return t.definition.Description
}

// Parameters 返回工具参数定义（JSON Schema格式）
func (t *MCPTool) Parameters() map[string]interface{} {
	return t.definition.InputSchema
}

// Execute 执行MCP工具
// 通过MCP客户端调用远程工具服务
func (t *MCPTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	logger.Info("通过MCP协议调用远程工具 [%s] @ [%s]", t.definition.Name, t.client.GetServerName())

	result, err := t.client.CallTool(ctx, t.definition.Name, args)
	if err != nil {
		return "", fmt.Errorf("MCP工具 [%s] 执行失败: %w", t.definition.Name, err)
	}

	return result, nil
}

// DiscoverAndCreateTools 从MCP服务器发现工具并创建适配器
// 参数：
//   - ctx: 上下文
//   - client: MCP客户端
//
// 返回：
//   - []*MCPTool: MCP工具适配器列表
//   - error: 可能的错误
func DiscoverAndCreateTools(ctx context.Context, client *Client) ([]*MCPTool, error) {
	// 从MCP服务器获取工具列表
	definitions, err := client.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("发现MCP工具失败: %w", err)
	}

	// 创建工具适配器
	tools := make([]*MCPTool, 0, len(definitions))
	for _, def := range definitions {
		tool := NewMCPTool(client, def)
		tools = append(tools, tool)
		logger.Info("已发现MCP工具: [%s] - %s", def.Name, def.Description)
	}

	return tools, nil
}
