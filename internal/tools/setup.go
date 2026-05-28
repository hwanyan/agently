package tools

import (
	"context"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/tools/local"
	"github.com/hwanyan/agently/internal/tools/mcp"
	"github.com/hwanyan/agently/pkg/types"
)

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

// SetupMCPTools 设置 MCP 工具
// 通过工具组为 agent 设置对应的工具集
// 参数：
//   - aiAgent: Agent实例
//   - cfg: 配置文件
//   - toolGroup: 工具组名称
func SetupMCPTools(aiAgent *agent.BasicAgent, cfg *types.Config, toolGroup string) {
	if len(cfg.MCP.Servers) == 0 {
		logger.Info("MCP服务列表为空，跳过MCP工具注册")
		return
	}

	for _, serverCfg := range cfg.MCP.Servers {
		// 跳过非当前 agent 工具组的 MCP 工具
		if serverCfg.Name != toolGroup {
			continue
		}
		client := mcp.NewClient(serverCfg)
		mcpTools, err := mcp.DiscoverAndCreateTools(context.Background(), client)
		if err != nil {
			logger.Warning("MCP服务 [%s] 工具发现失败: %v", serverCfg.Name, err)
			continue
		}

		var toolInterfaces []types.Tool
		for _, t := range mcpTools {
			toolInterfaces = append(toolInterfaces, t)
		}

		// 设置MCP工具集
		for _, tool := range mcpTools {
			aiAgent.AddTool(tool)
			logger.Info("MCP 服务[%s] 的工具 [%s] 已添加", serverCfg.Name, tool.Name())
		}
	}
}
