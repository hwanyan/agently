// Package agents 负责全部 Agent 的初始化、注册与编排
// 约定：每个 Agent 子目录中包含 prompt.yaml，目录名 = Agent ID
package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/llm"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/tools"
	"github.com/hwanyan/agently/internal/tools/mcp"
	"github.com/hwanyan/agently/pkg/types"
	"gopkg.in/yaml.v2"
)

// promptFile 用于反序列化 prompt.yaml 的中间结构
//
// 之所以单独定义而不是复用 agent.AgentInfo，是因为 yaml 字段名与 Go 字段名映射规则不同，
// 且 prompt.yaml 中的 system_prompt 是块标量，与 agent.AgentInfo.Prompt 的语义并不完全一致。
type promptFile struct {
	Name         string           `yaml:"name"`
	Description  string           `yaml:"description"`
	Role         string           `yaml:"role"`
	Tools        agent.AgentTools `yaml:"tools"`
	SystemPrompt string           `yaml:"system_prompt"`
	Settings     map[string]any   `yaml:"settings"` // 可选，当前未使用，仅做兼容占位
}

// initPromptMapping 构建项目中全部 agent 与其对应的基本信息的映射关系
// 即 { key(agent_id) : value(AgentInfo) }
// 约定：agents/<agent_id>/prompt.yaml，目录名 = Agent ID
// 说明：从编译时嵌入的 EmbeddedPrompts (embed.FS) 中读取，无需依赖外部文件路径
func initPromptMapping() map[string]agent.AgentInfo {
	logger.Info("构建项目中全部 agent 与其对应的基本信息的映射关系...")

	prompts := make(map[string]agent.AgentInfo)
	entries, err := EmbeddedPrompts.ReadDir(".")
	if err != nil {
		logger.Error("读取嵌入的 agents 目录失败: %v", err)
		return prompts
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// 按约定，entry.Name 即是 agent_id
		agentID := entry.Name()
		promptPath := agentID + "/prompt.yaml"

		raw, e := EmbeddedPrompts.ReadFile(promptPath)
		if e != nil {
			logger.Warning("%s agent 的 Prompt 读取失败: %v", agentID, e)
			continue
		}
		agentInfo, ok := parseAgentPrompt(agentID, raw)
		if !ok {
			continue
		}
		logger.Info("构建 %s agent 与其基本信息的映射关系成功", agentID)
		prompts[agentID] = agentInfo
	}

	logger.Info("构建项目中全部 agent 与其对应的基本信息的映射关系结束")
	return prompts
}

// parseAgentPrompt 将 prompt.yaml 解析为 agent.AgentInfo
//
// 必填字段缺失时会跳过该 Agent 并打印 Warning。
func parseAgentPrompt(agentID string, raw []byte) (agent.AgentInfo, bool) {
	var pf promptFile
	if err := yaml.Unmarshal(raw, &pf); err != nil {
		logger.Warning("%s agent 的 Prompt 解析失败: %v", agentID, err)
		return agent.AgentInfo{}, false
	}

	if pf.Name == "" {
		logger.Warning("%s agent 的 Prompt 中未找到有效的 name 内容", agentID)
		return agent.AgentInfo{}, false
	}
	if pf.Description == "" {
		logger.Warning("%s agent 的 Prompt 中未找到有效的 description 内容", agentID)
		return agent.AgentInfo{}, false
	}
	if pf.Role == "" {
		logger.Warning("%s agent 的 Prompt 中未找到有效的 role 内容", agentID)
		return agent.AgentInfo{}, false
	}
	if strings.TrimSpace(pf.SystemPrompt) == "" {
		logger.Warning("%s agent 的 Prompt 中未找到有效的 system_prompt 内容", agentID)
		return agent.AgentInfo{}, false
	}

	return agent.AgentInfo{
		ID:          agentID,
		Name:        pf.Name,
		Description: pf.Description,
		Tools:       pf.Tools,
		Role:        agent.AgentRole(pf.Role),
		Prompt:      pf.SystemPrompt,
	}, true
}

// InitAgents 初始化并注册全部 Agent，返回 Agent 管理器
//
// 启动期会做如下校验（任意一项失败即返回 error，由调用方决定 fail-fast）：
//  1. prompt.yaml 中声明的 mcp_servers ID 必须在 cfg.MCP.Servers 中存在
//  2. cfg.MCP.Servers 中所有 ID 必须唯一且非空
//  3. 被引用的 MCP 服务必须能成功 ListTools（探活）
func InitAgents(cfg *types.Config) (*agent.Manager, error) {
	// 1. 配置自身校验：MCP server 的 ID 必须唯一且非空
	if err := validateMCPServersConfig(cfg); err != nil {
		return nil, err
	}

	// 2. 加载所有 prompt
	prompts := initPromptMapping()

	// 3. 校验 Agent 的工具引用是否都能在配置中找到
	if err := validateAgentToolsRef(prompts, cfg); err != nil {
		return nil, err
	}

	// 4. 探活：所有被引用的 MCP 服务必须能成功 ListTools
	mcpToolsByServerID, err := discoverAllReferencedMCPTools(prompts, cfg)
	if err != nil {
		return nil, err
	}

	// 5. 创建 Manager 与 LLM 客户端
	m := agent.NewManager()
	llmClient := llm.NewClientFromConfig(cfg)

	// 6. 生成业务 Agent 列表描述（用于路由 Agent 的 Prompt 模板变量替换）
	agentsDescription := generateAgentsDescription(prompts)

	// 7. 注册全部 Agent
	for _, agentInfo := range prompts {
		// 系统 Agent（路由 Agent）的 Prompt 中包含模板变量，进行替换
		if agentInfo.Role == agent.AgentRoleSystem {
			agentInfo.Prompt = replaceTemplateVars(agentInfo.Prompt, map[string]string{
				"AGENTS_SECTION": agentsDescription,
			})
		}
		// 创建 Agent 实例
		agentInstance := agent.NewBasicAgent(agentInfo.Name, agentInfo.Description, llmClient, agentInfo.Prompt)
		// 按声明的 tools 注入工具
		attachTools(agentInstance, agentInfo, cfg, mcpToolsByServerID)
		// 注册到管理器
		m.Register(agentInstance, &agentInfo)
	}

	return m, nil
}

// attachTools 按 AgentInfo.Tools 声明，给 Agent 注入本地工具组与 MCP 工具
//
// 注意：
//   - 本地工具组通过 internal/tools 包中的注册分发函数完成
//   - MCP 工具直接复用 discoverAllReferencedMCPTools 的探活结果，避免重复发现
func attachTools(
	aiAgent *agent.BasicAgent,
	info agent.AgentInfo,
	cfg *types.Config,
	mcpToolsByServerID map[string][]types.Tool,
) {
	// 1. 本地工具组
	for _, group := range info.Tools.LocalGroups {
		tools.SetupLocalToolGroup(aiAgent, cfg, group)
	}
	// 2. MCP 工具（按 ID 引用）
	for _, serverID := range info.Tools.MCPServers {
		toolsList, ok := mcpToolsByServerID[serverID]
		if !ok {
			// 理论上不会进入：前面已经做过引用-存在性校验与探活
			logger.Error("[%s] 引用的 MCP 服务 ID [%s] 未在探活结果中，跳过工具注入", info.ID, serverID)
			continue
		}
		for _, t := range toolsList {
			aiAgent.AddTool(t)
			logger.Info("Agent [%s] 注入 MCP 工具 [%s]（来自服务 ID=%s）", info.ID, t.Name(), serverID)
		}
	}
}

// validateMCPServersConfig 校验 cfg.MCP.Servers 自身：ID 必须唯一且非空
func validateMCPServersConfig(cfg *types.Config) error {
	seen := map[string]struct{}{}
	for i, s := range cfg.MCP.Servers {
		if strings.TrimSpace(s.ID) == "" {
			return fmt.Errorf("配置错误：mcp.servers[%d] 缺少必填字段 id（base_url=%s）", i, s.BaseURL)
		}
		if _, dup := seen[s.ID]; dup {
			return fmt.Errorf("配置错误：mcp.servers 中存在重复的 id [%s]", s.ID)
		}
		seen[s.ID] = struct{}{}
	}
	return nil
}

// validateAgentToolsRef 校验所有 Agent 在 prompt.yaml 中声明的 mcp_servers 是否都能在 cfg 中找到
func validateAgentToolsRef(prompts map[string]agent.AgentInfo, cfg *types.Config) error {
	available := map[string]struct{}{}
	for _, s := range cfg.MCP.Servers {
		available[s.ID] = struct{}{}
	}
	for agentID, info := range prompts {
		for _, ref := range info.Tools.MCPServers {
			if _, ok := available[ref]; !ok {
				return fmt.Errorf("Agent [%s] 引用的 MCP 服务 id=[%s] 在配置 mcp.servers 中不存在，请检查 prompt.yaml 与 config.yaml 是否一致", agentID, ref)
			}
		}
	}
	return nil
}

// discoverAllReferencedMCPTools 对所有被 Agent 引用过的 MCP 服务执行探活与工具发现
//
// 任意一个被引用的 MCP 服务探活失败 → 直接返回 error（fail-fast）。
// 未被任何 Agent 引用的 MCP 服务不会被探活，避免无意义的网络调用。
func discoverAllReferencedMCPTools(
	prompts map[string]agent.AgentInfo,
	cfg *types.Config,
) (map[string][]types.Tool, error) {
	// 收集所有被引用过的 server id
	referenced := map[string]struct{}{}
	for _, info := range prompts {
		for _, id := range info.Tools.MCPServers {
			referenced[id] = struct{}{}
		}
	}

	// 通过 ID 反查 server 配置
	cfgByID := map[string]types.MCPServerConfig{}
	for _, s := range cfg.MCP.Servers {
		cfgByID[s.ID] = s
	}

	result := map[string][]types.Tool{}
	for serverID := range referenced {
		serverCfg := cfgByID[serverID]
		client := mcp.NewClient(serverCfg)
		discovered, err := mcp.DiscoverAndCreateTools(context.Background(), client)
		if err != nil {
			return nil, fmt.Errorf("MCP 服务探活失败：id=%s, name=%s, base_url=%s, err=%w",
				serverCfg.ID, serverCfg.Name, serverCfg.BaseURL, err)
		}
		// 适配为 types.Tool
		ts := make([]types.Tool, 0, len(discovered))
		for _, t := range discovered {
			ts = append(ts, t)
		}
		result[serverID] = ts
		logger.Info("MCP 服务 [%s] 探活成功，发现 %d 个工具", serverID, len(ts))
	}
	return result, nil
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
