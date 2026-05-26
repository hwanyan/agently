package tools

import (
	"agent-hwan/internal/agent"
	"agent-hwan/internal/logger"
	"agent-hwan/internal/tools/local"
	"agent-hwan/pkg/types"
	"fmt"
	"strings"
)

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

// SetupGeneralTools 设置通用助手的工具集
// 通过插件化注册机制自动发现并加载 "general" 工具组的所有工具
func SetupGeneralTools(aiAgent *agent.BasicAgent, cfg *types.Config) {
	logger.Info("开始注册通用工具组...")

	// 通过插件化注册表自动获取该组所有工具
	tools := local.GetToolsByGroup("general", cfg)
	for _, tool := range tools {
		aiAgent.AddTool(tool)
		logger.Info("工具 [%s] 已注册", tool.Name())
	}

	logger.Info("通用工具组注册完成，共 %d 个工具", len(tools))
}

// SetupMCPTools 设置MCP工具集
// 通过MCP协议从远程服务器发现并注册工具
// 参数：
//   - aiAgent: Agent实例
//   - mcpTools: 已发现的MCP工具列表（实现了 types.Tool 接口）
func SetupMCPTools(aiAgent *agent.BasicAgent, mcpTools []types.Tool) {
	logger.Info("开始注册MCP工具组...")

	for _, tool := range mcpTools {
		aiAgent.AddTool(tool)
		logger.Info("MCP工具 [%s] 已添加", tool.Name())
	}

	logger.Info("MCP工具组注册完成，共 %d 个工具", len(mcpTools))
}
