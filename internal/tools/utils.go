package tools

import (
	"agent-hwan/pkg/types"
	"fmt"
	"strings"
)

// GenerateToolsSection 生成工具描述部分
func GenerateToolsSection(registry *ToolRegistry) string {
	if registry == nil {
		return "暂无可用工具"
	}

	tools := registry.GetAvailableTools()
	if len(tools) == 0 {
		return "暂无可用工具"
	}

	var builder strings.Builder
	builder.WriteString("可用工具列表：\n")

	for index, tool := range tools {
		builder.WriteString(fmt.Sprintf("%d. **%s**\n", index+1, tool.Name))
		builder.WriteString(fmt.Sprintf("  %s\n", tool.Description))
		builder.WriteString(fmt.Sprintf("  %s\n", tool.Arguments))
	}

	return strings.TrimSpace(builder.String())
}

// GenerateAgentsDescription 生成Agent列表描述（用于路由Agent的Prompt）
func GenerateAgentsDescription(agents []types.AgentItemConfig) string {
	var builder strings.Builder
	for _, ag := range agents {
		// 跳过系统Agent自身
		if ag.Role == types.AgentRoleSystem {
			continue
		}
		builder.WriteString(fmt.Sprintf("- **%s** (id: `%s`): %s\n", ag.Name, ag.ID, ag.Description))
	}
	return strings.TrimSpace(builder.String())
}

// GenerateMCPToolsDescription 生成MCP工具的描述文本（用于Prompt）
func GenerateMCPToolsDescription(cfg *types.Config) string {
	if len(cfg.MCP.Servers) == 0 {
		return "MCP工具服务未配置，工具将在运行时动态发现"
	}

	var builder strings.Builder
	builder.WriteString("以下工具通过MCP协议从远程服务器动态加载：\n")
	for i, server := range cfg.MCP.Servers {
		builder.WriteString(fmt.Sprintf("%d. MCP服务: %s (%s)\n", i+1, server.Name, server.BaseURL))
	}
	builder.WriteString("\n工具列表将在启动时自动发现并注册。")
	return builder.String()
}
