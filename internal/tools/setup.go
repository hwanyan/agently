package tools

import (
	"agent-hwan/internal/agent"
	"agent-hwan/internal/logger"
	"agent-hwan/pkg/types"
)

// SetupTools 设置通用工具（保留兼容性）
func SetupTools(aiAgent *agent.BasicAgent, cfg *types.Config) {
	SetupGeneralTools(aiAgent, cfg)
}

// SetupGeneralTools 设置通用助手的工具集
// 包含天气、IP查询、二维码、翻译、文本处理、短链接等工具
func SetupGeneralTools(aiAgent *agent.BasicAgent, cfg *types.Config) {
	logger.Info("开始注册通用工具组...")

	// 添加天气工具
	logger.Info("添加天气工具...")
	weatherTool := NewWeatherTool(cfg.Tools.Weather.APIKey, cfg.Tools.Weather.BaseURL)
	aiAgent.AddTool(weatherTool)
	logger.Info("天气工具已添加")

	// 添加IP查询工具
	logger.Info("添加IP查询工具...")
	ipTool := NewIPTool(cfg.Tools.IP.AccessKey)
	aiAgent.AddTool(ipTool)
	logger.Info("IP查询工具已添加")

	// 添加二维码生成工具
	logger.Info("添加二维码生成工具...")
	qrTool := NewQRCodeTool()
	aiAgent.AddTool(qrTool)
	logger.Info("二维码生成工具已添加")

	// 添加翻译工具
	logger.Info("添加翻译工具...")
	trTool := NewTranslationTool(cfg.Tools.Translate.APIKey, cfg.Tools.Translate.BaseURL)
	aiAgent.AddTool(trTool)
	logger.Info("翻译工具已添加")

	// 添加文本处理工具
	logger.Info("添加文本处理工具...")
	textTool := NewTextProcessorTool()
	aiAgent.AddTool(textTool)
	logger.Info("文本处理工具已添加")

	// 添加短链接生成工具
	logger.Info("添加短链接生成工具...")
	shortURLTool := NewShortURLTool(cfg.Tools.ShortURL.APIKey, cfg.Tools.ShortURL.BaseURL)
	aiAgent.AddTool(shortURLTool)
	logger.Info("短链接生成工具已添加")

	// 显示工具统计
	tools := aiAgent.GetTools()
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
