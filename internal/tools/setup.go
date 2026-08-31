package tools

import (
	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/tools/local"
	"github.com/hwanyan/agently/pkg/types"
)

// SetupLocalToolGroup 按工具组名称为 Agent 注入本地工具
//
//	MCP 工具的发现与注入已统一收敛到 internal/agents 包的 InitAgents 中（按 server id 引用）
//	本函数只负责本地工具组的注入，命名也由原 SetupGeneralTools 改为更通用的 SetupLocalToolGroup
//	未知工具组会打 Error 但不阻断启动（业务可降级），如需 fail-fast 可在此改为返回 error
//	provider 支持配置热更新：工具内部持有 provider 而非固定配置值，每次 Execute 时
//	读取到的都是 config.yaml 变更后最新生效的 API Key / BaseURL 等配置
func SetupLocalToolGroup(aiAgent *agent.BasicAgent, provider types.ConfigProvider, group string) {
	if group == "" {
		return
	}
	logger.Info("开始注册本地工具组 [%s]...", group)

	tools := local.GetToolsByGroup(group, provider)
	if len(tools) == 0 {
		logger.Error("本地工具组 [%s] 未找到任何工具，请检查 internal/tools/local 中的注册表", group)
		return
	}
	for _, tool := range tools {
		aiAgent.AddTool(tool)
		logger.Info("工具 [%s] 已注册到 Agent [%s]", tool.Name(), aiAgent.GetName())
	}
	logger.Info("本地工具组 [%s] 注册完成，共 %d 个工具", group, len(tools))
}
