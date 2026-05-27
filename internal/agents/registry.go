// Package agents 负责全部 Agent 的初始化、注册与编排
// 约定：每个 Agent 子目录中包含 prompt.yaml，目录名 = Agent ID
package agents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/llm"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/tools"
	"github.com/hwanyan/agently/internal/tools/mcp"
	"github.com/hwanyan/agently/pkg/types"
)

// initLLM 初始化 LLM 客户端
func initLLM(cfg *types.Config) types.LLMClient {
	llmClient := llm.NewOpenAIClient(
		cfg.LLM.APIKey,
		cfg.LLM.Model,
		cfg.LLM.BaseURL,
	)
	logger.Info("LLM 客户端创建成功")
	return llmClient
}

// initPromptMapping 构建项目中全部 agent 与其对应的基本信息的映射关系
// 即 { key(agent_id) : value(AgentInfo[name, description, prompt]) }
// 约定：agents/<agent_id>/prompt.yaml，目录名 = Agent ID
//
// 参数：
//   - agentsDir: agents 目录路径（如 ../../internal/agents）
func initPromptMapping(agentsDir string) map[string]agent.AgentInfo {
	logger.Info("构建项目中全部 agent 与其对应的基本信息的映射关系...")

	prompts := make(map[string]agent.AgentInfo)
	// 遍历 agents 目录下的全部子目录，如果该目录下存在名为 prompt.yaml 的文件，则构建由该目录名到这个 prompt 的映射关系
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		logger.Error("读取 agents 目录 [%s] 失败: %v", agentsDir, err)
		return prompts
	}
	for _, entry := range entries {
		if entry.IsDir() {
			promptPath := filepath.Join(agentsDir, entry.Name(), "prompt.yaml")
			// 按约定，entry.Name 即是 agent_id
			agentID := entry.Name()
			if _, e := os.Stat(promptPath); e != nil {
				continue
			}
			prompt, e := os.ReadFile(promptPath)
			if e != nil {
				logger.Warning("%s agent 的 Prompt 读取失败: %v", agentID, e)
				continue
			}
			// 从 prompt 中提取该 agent 的全部信息
			agentInfo, flag := extractAgentInfo(agentID, string(prompt))
			if flag == false {
				logger.Warning("%s agent 的 Prompt 中未找到有效的基本信息，跳过该 agent", agentID)
				continue
			}
			logger.Info("构建 %s agent 与其基本信息的映射关系成功", agentID)
			prompts[agentID] = agentInfo
		}
	}

	logger.Info("构建项目中全部 agent 与其对应的基本信息的映射关系结束")
	return prompts
}

// extractAgentInfo 提取 prompt 中为当前 agent 定义的基本信息
func extractAgentInfo(agentID, prompt string) (agent.AgentInfo, bool) {
	// 提取 name
	name := extractField(prompt, "name")
	if name == "" {
		logger.Warning("%s agent 的 Prompt 中未找到有效的 name 内容", agentID)
		return agent.AgentInfo{}, false
	}
	// 提取 description
	description := extractField(prompt, "description")
	if description == "" {
		logger.Warning("%s agent 的 Prompt 中未找到有效的 description 内容", agentID)
		return agent.AgentInfo{}, false
	}
	// 提取 tool_group
	toolGroup := extractField(prompt, "tool_group")
	if toolGroup == "" {
		logger.Warning("%s agent 的 Prompt 中未找到有效的 tool_group 内容", agentID)
		return agent.AgentInfo{}, false
	}
	// 提取 role
	role := extractField(prompt, "role")
	if role == "" {
		logger.Warning("%s agent 的 Prompt 中未找到有效的 role 内容", agentID)
		return agent.AgentInfo{}, false
	}
	// 提取 system_prompt
	content := extractSystemPrompt(prompt)
	if content == "" {
		logger.Warning("%s agent 的 Prompt 中未找到有效的 system_prompt 内容", agentID)
		return agent.AgentInfo{}, false
	}
	return agent.AgentInfo{
		ID:          agentID,
		Name:        name,
		Description: description,
		ToolGroup:   toolGroup,
		Role:        agent.AgentRole(role),
		Prompt:      content,
	}, true
}

// InitAgents 初始化并注册全部 Agent，返回 Agent 管理器
// 约定：Agent ID 直接对应 agents/<id>/prompt.yaml，无需额外映射
func InitAgents(cfg *types.Config) *agent.Manager {
	// 创建 Agent Manager
	m := agent.NewManager()

	// 创建 LLM 客户端
	llmClient := initLLM(cfg)

	// 构建 agent 的 prompt 映射（从 cfg.Prompt.Dir 指定的 agents 目录加载）
	prompts := initPromptMapping(cfg.Prompt.Dir)

	// 生成业务 Agent 列表描述（用于路由 Agent 的 Prompt 模板变量替换）
	agentsDescription := generateAgentsDescription(prompts)

	// 注册全部的 agent
	for _, agentInfo := range prompts {
		// 对系统 Agent（路由 Agent）的 Prompt 进行模板变量替换
		if agentInfo.Role == agent.AgentRoleSystem {
			agentInfo.Prompt = replaceTemplateVars(agentInfo.Prompt, map[string]string{
				"AGENTS_SECTION": agentsDescription,
			})
		}

		// 创建 Agent 实例（具有 llm 的 Agent）
		agentInstance := agent.NewBasicAgent(agentInfo.Name, agentInfo.Description, llmClient, agentInfo.Prompt)
		// 根据工具组注册工具
		registerTools(agentInstance, agentInfo.ToolGroup, cfg)
		// 注册到管理器
		m.Register(agentInstance, &agentInfo)
	}

	return m
}

// registerTools 根据工具组标识为 Agent 注册对应的工具集
func registerTools(aiAgent *agent.BasicAgent, toolGroup string, cfg *types.Config) {
	switch toolGroup {
	case "general":
		tools.SetupGeneralTools(aiAgent, cfg)
	case "mcp_gold":
		setupMCPToolsForAgent(aiAgent, cfg)
	case "none":
		// 系统Agent不需要工具
	default:
		logger.Warning("未知的工具组 [%s]，跳过工具注册", toolGroup)
	}
}

// setupMCPToolsForAgent 为Agent设置MCP工具
func setupMCPToolsForAgent(aiAgent *agent.BasicAgent, cfg *types.Config) {
	if len(cfg.MCP.Servers) == 0 {
		return
	}

	for _, serverCfg := range cfg.MCP.Servers {
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
		tools.SetupMCPTools(aiAgent, toolInterfaces)
	}
}

// generateAgentsDescription 生成业务 Agent 列表描述（用于路由 Agent 的 Prompt）
func generateAgentsDescription(prompts map[string]agent.AgentInfo) string {
	var builder strings.Builder
	for _, info := range prompts {
		// 跳过系统 Agent 自身
		if info.Role == agent.AgentRoleSystem {
			continue
		}
		builder.WriteString(fmt.Sprintf("- **%s** (id: `%s`): %s\n", info.Name, info.ID, info.Description))
	}
	return strings.TrimSpace(builder.String())
}

// replaceTemplateVars 替换模板中的 {{var}} 格式变量
func replaceTemplateVars(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// extractField 从 YAML 文件内容中提取顶层简单字段的值
// 支持提取格式为 `field: "value"` 或 `field: value` 的字段
func extractField(content, fieldName string) string {
	lines := strings.Split(content, "\n")
	prefix := fieldName + ":"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			// 提取冒号后面的值
			value := strings.TrimSpace(trimmed[len(prefix):])
			// 去除引号
			value = strings.Trim(value, "\"")
			value = strings.Trim(value, "'")
			return value
		}
	}
	return ""
}

// extractSystemPrompt 从 YAML 文件内容中提取 system_prompt 字段的值
// 简单解析：找到 "system_prompt: |" 行后，提取后续缩进内容
func extractSystemPrompt(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inSystemPrompt := false
	baseIndent := 0

	for _, line := range lines {
		if !inSystemPrompt {
			// 查找 system_prompt 字段开始
			trimmed := strings.TrimSpace(line)
			if trimmed == "system_prompt: |" || trimmed == "system_prompt: |+" || trimmed == "system_prompt: |-" {
				inSystemPrompt = true
				continue
			}
			continue
		}

		// 已进入 system_prompt 块
		if line == "" {
			// 空行保留
			result = append(result, "")
			continue
		}

		// 计算当前行缩进
		indent := len(line) - len(strings.TrimLeft(line, " "))

		// 第一行确定基础缩进
		if len(result) == 0 && strings.TrimSpace(line) != "" {
			baseIndent = indent
		}

		// 如果缩进小于等于0（回到顶层），说明 system_prompt 块结束
		if indent == 0 && strings.TrimSpace(line) != "" {
			break
		}

		// 去掉基础缩进后保留内容
		if indent >= baseIndent {
			result = append(result, line[baseIndent:])
		} else {
			result = append(result, line)
		}
	}

	// 去掉末尾空行
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	return strings.Join(result, "\n")
}
