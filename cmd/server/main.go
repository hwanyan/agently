// Package main AI Agent 的主程序入口
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"agent-hwan/internal/agent"
	"agent-hwan/internal/config"
	"agent-hwan/internal/llm"
	"agent-hwan/internal/logger"
	"agent-hwan/internal/prompt"
	"agent-hwan/internal/tools"
	"agent-hwan/internal/tools/mcp"
	"agent-hwan/pkg/types"

	"github.com/chzyer/readline"
)

func main() {
	fmt.Println()
	fmt.Println("========== 🤖 AI Agent 助手（多Agent模式） ==========")
	fmt.Println()

	// 第一步：加载配置
	fmt.Println("📋 步骤1: 加载配置...")
	cfg, err := config.LoadConfiguration()
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
		os.Exit(0)
	}
	fmt.Println("✅ 配置加载成功！")

	// 初始化日志系统
	if e := logger.Init(cfg.Log.FilePath); e != nil {
		log.Fatalf("❌ 初始化日志系统失败: %v", e)
		os.Exit(0)
	}
	defer logger.Close()
	logger.Info("日志系统初始化成功，日志文件: %s", cfg.Log.FilePath)

	// 第二步：创建LLM客户端
	logger.Info("步骤2: 创建LLM客户端...")
	llmClient := llm.NewOpenAIClient(
		cfg.LLM.APIKey,
		cfg.LLM.Model,
		cfg.LLM.BaseURL,
	)
	logger.Info("LLM客户端创建成功")

	// 第三步：获取全部的 Prompt 配置
	logger.Info("步骤3: 获取全部的 Prompt 配置...")
	prompts, err := prompt.GetPrompts(cfg.Prompt.Dir)
	if err != nil {
		logger.Error("获取 Prompt 配置失败: %v", err)
		log.Fatalf("❌ 获取 Prompt 配置失败: %v", err)
		os.Exit(0)
	}
	logger.Info("Prompt配置获取成功")

	// 第四步：注册工具信息（用于Prompt中的工具描述）
	logger.Info("步骤4: 注册工具信息...")
	toolInfos := tools.NewToolRegistry()
	logger.Info("工具信息注册成功")

	// 第五步：创建Agent管理器并注册全部Agent
	logger.Info("步骤5: 创建Agent管理器并注册全部Agent...")
	agentManager := initAllAgents(cfg, llmClient, prompts, toolInfos)

	// 校验系统Agent是否全部就绪（系统Agent必须成功注册，否则服务不启动）
	if err := agentManager.ValidateSystemAgents(); err != nil {
		logger.Error("系统Agent校验失败: %v", err)
		log.Fatalf("❌ %v", err)
		os.Exit(0)
	}
	logger.Info("Agent管理器创建成功，全部Agent已注册，系统Agent校验通过")

	// 第六步：开始交互
	logger.Info("步骤6: 开始交互")
	fmt.Println("================ END ================")
	startInteractiveSession(agentManager)
}

// initAllAgents 初始化Agent管理器，注册全部Agent（不区分单/多Agent模式）
func initAllAgents(cfg *types.Config, llmClient types.LLMClient, prompts map[string]string, toolInfos *tools.ToolRegistry) *agent.Manager {
	manager := agent.NewManager(cfg.Agents)

	for _, agentCfg := range cfg.Agents {
		// 输出创建中状态
		roleTag := ""
		if agentCfg.Role == types.AgentRoleSystem {
			roleTag = " [系统级]"
		}
		fmt.Printf("  ⌛️ Agent [%s] (%s) 创建中%s\n", agentCfg.ID, agentCfg.Name, roleTag)
		logger.Info("正在创建Agent [%s]: %s", agentCfg.ID, agentCfg.Name)

		// 根据工具组确定模板变量
		vars := make(map[string]any)
		switch agentCfg.ToolGroup {
		case "general":
			vars["TOOLS_SECTION"] = prompt.GenerateToolsSection(toolInfos)
		case "mcp_gold":
			vars["TOOLS_SECTION"] = generateMCPToolsDescription(cfg)
		case "none":
			// 系统Agent（如路由Agent）不需要工具描述，但需要Agent列表描述
			vars["AGENTS_SECTION"] = generateAgentsDescription(cfg.Agents)
		default:
			vars["TOOLS_SECTION"] = "暂无可用工具"
		}

		// 获取对应场景的系统提示词
		systemPrompt, err := prompt.GetPromptByScene(prompts, agentCfg.PromptScene, vars)
		if err != nil {
			logger.Error("获取Agent [%s] 的系统提示词失败: %v", agentCfg.ID, err)
			// 系统Agent初始化失败直接报错退出，业务Agent允许跳过
			if agentCfg.Role == types.AgentRoleSystem {
				log.Fatalf("❌ 系统Agent [%s] 初始化失败: %v", agentCfg.ID, err)
			}
			fmt.Printf("  ❌ Agent [%s] (%s) 创建失败: %v\n", agentCfg.ID, agentCfg.Name, err)
			logger.Error("Agent [%s] (%s) 创建失败: %v", agentCfg.ID, agentCfg.Name, err)
			continue
		}

		// 创建Agent实例
		aiAgent := agent.NewBasicAgent(
			agentCfg.Name,
			agentCfg.Description,
			llmClient,
			systemPrompt,
		)

		// 根据工具组注册工具，并记录是否有工具注册异常
		var toolWarnings []string
		switch agentCfg.ToolGroup {
		case "general":
			tools.SetupGeneralTools(aiAgent, cfg)
		case "mcp_gold":
			toolWarnings = setupMCPToolsForAgent(aiAgent, cfg)
		case "none":
			// 系统Agent不需要工具
		}

		// 注册到管理器（Manager.Register 会根据 Role 自动处理系统/业务Agent）
		manager.Register(agentCfg.ID, agentCfg.Role, aiAgent)

		// 输出注册结果
		if len(toolWarnings) > 0 {
			// 有工具注册警告（如MCP连接失败），Agent本身创建成功但功能受限
			fmt.Printf("  ⚠️  Agent [%s] (%s) 创建成功，但连接 MCP 服务出错，可能有些功能无法实现\n", agentCfg.ID, agentCfg.Name)
			logger.Warning("Agent [%s] (%s) 创建成功，但存在工具注册警告: %v", agentCfg.ID, agentCfg.Name, toolWarnings)
		} else {
			fmt.Printf("  ✅ Agent [%s] (%s) 创建成功%s\n", agentCfg.ID, agentCfg.Name, roleTag)
			logger.Info("Agent [%s] (%s) 创建成功%s", agentCfg.ID, agentCfg.Name, roleTag)
		}
	}

	return manager
}

// generateAgentsDescription 生成Agent列表描述（用于路由Agent的Prompt）
func generateAgentsDescription(agents []types.AgentItemConfig) string {
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

// generateMCPToolsDescription 生成MCP工具的描述文本（用于Prompt）
func generateMCPToolsDescription(cfg *types.Config) string {
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

// setupMCPToolsForAgent 为Agent设置MCP工具
// 返回：
//   - []string: 工具注册过程中的警告信息列表（如MCP服务连接失败等）
func setupMCPToolsForAgent(aiAgent *agent.BasicAgent, cfg *types.Config) []string {
	var warnings []string

	if len(cfg.MCP.Servers) == 0 {
		logger.Warning("MCP服务器未配置，跳过MCP工具注册")
		warnings = append(warnings, "MCP服务器未配置")
		return warnings
	}

	ctx := context.Background()

	for _, serverCfg := range cfg.MCP.Servers {
		logger.Info("连接MCP服务器: %s (%s)", serverCfg.Name, serverCfg.BaseURL)

		// 创建MCP客户端
		client := mcp.NewClient(serverCfg)

		// 发现并创建工具
		mcpTools, err := mcp.DiscoverAndCreateTools(ctx, client)
		if err != nil {
			warnMsg := fmt.Sprintf("MCP服务器 [%s] 连接失败: %v", serverCfg.Name, err)
			logger.Warning("%s（Agent仍可使用，待服务启动后重试）", warnMsg)
			fmt.Printf("  ⚠️  %s\n", warnMsg)
			warnings = append(warnings, warnMsg)
			continue
		}

		// 将MCP工具注册到Agent
		var toolInterfaces []types.Tool
		for _, t := range mcpTools {
			toolInterfaces = append(toolInterfaces, t)
		}
		tools.SetupMCPTools(aiAgent, toolInterfaces)
		logger.Info("从MCP服务器 [%s] 加载了 %d 个工具", serverCfg.Name, len(mcpTools))
	}

	return warnings
}

// startInteractiveSession 开始交互会话
func startInteractiveSession(manager *agent.Manager) {
	fmt.Println()
	fmt.Println("🎉 AI Agent系统已准备就绪！")
	fmt.Println()

	// 显示可用Agent列表
	showAgentList(manager)

	fmt.Println()
	fmt.Println("💡 你可以尝试以下问题：")
	fmt.Println("   • 北京的天气怎么样？")
	fmt.Println("   • 查询 8.8.8.8 的IP信息")
	fmt.Println("   • 生成一个内容为 https://example.com 的二维码")
	fmt.Println("   • 请把\"今天天气不错\"翻译为英文")
	fmt.Println("   • 今日黄金行情如何？")
	fmt.Println()
	fmt.Println("🔧 可用命令：")
	fmt.Println("   • 'help'    - 显示帮助信息")
	fmt.Println("   • 'agents'  - 查看可用Agent列表")
	fmt.Println("   • 'switch <id>' - 手动切换Agent（如: switch gold_analysis）")
	fmt.Println("   • 'auto'    - 开启自动路由模式（默认）")
	fmt.Println("   • 'history' - 查看对话历史")
	fmt.Println("   • 'clear'   - 清空对话历史")
	fmt.Println("   • 'tools'   - 查看当前Agent的可用工具")
	fmt.Println("   • 'quit' 或 'exit' - 退出程序")
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))

	ctx := context.Background()

	// 自动路由模式标志（默认开启）
	autoRoute := true

	// 创建readline实例
	rl, err := readline.New(getPromptPrefix(manager, autoRoute))
	if err != nil {
		logger.Error("创建 readline 失败: %v", err)
		fmt.Printf("❌ 创建 readline 失败: %v\n", err)
		return
	}
	defer rl.Close()

	for {
		// 读取用户输入
		userInput, err := rl.Readline()
		if err != nil {
			break // EOF或其他错误
		}

		// 去除用户输入前后的空白字符
		userInput = strings.TrimSpace(userInput)

		// 处理特殊命令
		if handled := handleSpecialCommands(userInput, manager, rl, &autoRoute); handled {
			continue
		}

		// 检查输入是否为空
		if userInput == "" {
			fmt.Println("💭 请输入一些内容，或输入 'help' 查看帮助")
			continue
		}

		// 确定目标Agent
		var targetAgent *agent.BasicAgent

		if autoRoute {
			// 自动路由模式：先记录用户输入，再通过路由Agent判断意图
			logger.Info("[auto] 用户输入: %s", userInput)
			routedID, reason, err := manager.Route(ctx, userInput)
			if err != nil {
				logger.Error("路由失败: %v", err)
				fmt.Printf("⚠️  路由失败，使用当前Agent: %v\n", err)
				targetAgent, _ = manager.Current()
			} else {
				targetAgent, _ = manager.Get(routedID)
				// 如果路由到了不同的Agent，提示用户
				if routedID != manager.CurrentID() {
					fmt.Printf("🔀 [自动路由] → %s (%s)\n", targetAgent.GetName(), reason)
				}
			}
		} else {
			// 手动模式：使用当前Agent
			logger.Info("[%s] 用户输入: %s", manager.CurrentID(), userInput)
			targetAgent, err = manager.Current()
			if err != nil {
				fmt.Printf("❌ %v\n", err)
				continue
			}
		}

		// 处理用户消息
		fmt.Print("🤖 助手: ")
		fmt.Print("正在思考...")

		// 流式响应
		responseStream, err := targetAgent.ProcessMessageStream(ctx, userInput)
		if err != nil {
			fmt.Printf("\r🤖 助手: ❌ 处理消息时出错: %v\n", err)
			continue
		}
		// 清除"正在思考..."并开始流式输出
		fmt.Print("\r🤖 助手: ")
		// 读取并显示流式响应
		for chunk := range responseStream {
			fmt.Print(chunk)
		}

		fmt.Println() // 换行
	}

	fmt.Println("\n👋 感谢使用AI Agent系统！再见！")
}

// getPromptPrefix 获取终端提示符前缀
func getPromptPrefix(manager *agent.Manager, autoRoute bool) string {
	if autoRoute {
		return "👤 [auto] > "
	}
	currentID := manager.CurrentID()
	return fmt.Sprintf("👤 [%s] > ", currentID)
}

// handleSpecialCommands 处理特殊命令
func handleSpecialCommands(input string, manager *agent.Manager, rl *readline.Instance, autoRoute *bool) bool {
	lower := strings.ToLower(input)

	switch {
	case lower == "quit" || lower == "exit" || lower == "退出":
		fmt.Println("👋 再见！")
		os.Exit(0)
		return true

	case lower == "help" || lower == "帮助":
		showHelp()
		return true

	case lower == "history" || lower == "历史":
		currentAgent, err := manager.Current()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return true
		}
		showHistory(currentAgent)
		return true

	case lower == "clear" || lower == "清空":
		currentAgent, err := manager.Current()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return true
		}
		currentAgent.ClearHistory()
		fmt.Println("🗑️  对话历史已清空")
		return true

	case lower == "tools" || lower == "工具":
		currentAgent, err := manager.Current()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return true
		}
		showTools(currentAgent)
		return true

	case lower == "agents" || lower == "助手列表":
		showAgentList(manager)
		return true

	case lower == "auto" || lower == "自动":
		*autoRoute = true
		rl.SetPrompt(getPromptPrefix(manager, *autoRoute))
		fmt.Println("✅ 已开启自动路由模式，系统将自动判断并分配最合适的Agent")
		return true

	case strings.HasPrefix(lower, "switch ") || strings.HasPrefix(lower, "切换 "):
		parts := strings.Fields(input)
		if len(parts) < 2 {
			fmt.Println("❌ 请指定要切换的Agent ID，如: switch gold_analysis")
			return true
		}
		targetID := parts[1]
		// 不允许切换到系统Agent
		if manager.IsSystemAgent(targetID) {
			fmt.Println("❌ 不能手动切换到系统Agent")
			return true
		}
		if err := manager.Switch(targetID); err != nil {
			fmt.Printf("❌ 切换失败: %v\n", err)
		} else {
			// 关闭自动路由，进入手动模式
			*autoRoute = false
			rl.SetPrompt(getPromptPrefix(manager, *autoRoute))
			currentAgent, _ := manager.Current()
			fmt.Printf("✅ 已切换到: %s (%s)，自动路由已关闭\n", currentAgent.GetName(), targetID)
			fmt.Println("💡 输入 'auto' 可重新开启自动路由模式")
		}
		return true

	default:
		return false
	}
}

// showHelp 显示帮助信息
func showHelp() {
	fmt.Println("📖 AI Agent 帮助信息")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("🗣️  **对话**:")
	fmt.Println("   • 直接输入问题，系统自动路由到合适的Agent")
	fmt.Println("   • 也可手动切换到指定Agent进行对话")
	fmt.Println()
	fmt.Println("🤖 **多Agent管理**:")
	fmt.Println("   • 'agents'       - 查看所有可用Agent")
	fmt.Println("   • 'switch <id>'  - 手动切换到指定Agent（关闭自动路由）")
	fmt.Println("   • 'auto'         - 开启自动路由模式")
	fmt.Println("   • 每个Agent有独立的工具集和对话历史")
	fmt.Println()
	fmt.Println("⌨️  **命令**:")
	fmt.Println("   • 'help'    - 显示此帮助信息")
	fmt.Println("   • 'agents'  - 查看可用Agent列表")
	fmt.Println("   • 'switch <id>' - 手动切换Agent")
	fmt.Println("   • 'auto'    - 开启自动路由模式")
	fmt.Println("   • 'history' - 查看对话历史")
	fmt.Println("   • 'clear'   - 清空对话历史")
	fmt.Println("   • 'tools'   - 查看当前Agent的可用工具")
	fmt.Println("   • 'quit' 或 'exit' - 退出程序")
	fmt.Println()
	fmt.Println("💡 **可用Agent**:")
	fmt.Println("   • general        - 通用工具助手（天气、翻译、IP查询等）")
	fmt.Println("   • gold_analysis  - 黄金分析助手（专业黄金市场分析）")
	fmt.Println()
	fmt.Println("🔀 **路由模式说明**:")
	fmt.Println("   • 自动模式(auto): 系统自动分析意图，路由到合适的Agent")
	fmt.Println("   • 手动模式(switch): 固定使用指定Agent，不自动路由")
	fmt.Println(strings.Repeat("-", 40))
}

// showAgentList 显示可用Agent列表
func showAgentList(manager *agent.Manager) {
	agents := manager.List()

	fmt.Println("🤖 可用Agent列表")
	fmt.Println(strings.Repeat("-", 50))

	for i, info := range agents {
		activeMarker := "  "
		if info.IsActive {
			activeMarker = "▶ "
		}
		statusIcon := "✅"
		if !info.Registered {
			statusIcon = "⚠️"
		}
		fmt.Printf("%s%d. %s [%s] %s\n", activeMarker, i+1, statusIcon, info.ID, info.Name)
		fmt.Printf("      %s\n", info.Description)
	}

	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("💡 使用 'switch <id>' 手动切换Agent，或使用 'auto' 开启自动路由")
}

// showHistory 显示对话历史
func showHistory(aiAgent *agent.BasicAgent) {
	history := aiAgent.GetConversationHistory()

	if len(history) == 0 {
		fmt.Println("📝 对话历史为空")
		return
	}

	fmt.Println("📝 对话历史")
	fmt.Println(strings.Repeat("-", 40))

	for i, msg := range history {
		var roleIcon string
		var roleName string
		switch msg.Role {
		case "system":
			roleIcon = "⚙️"
			roleName = "系统"
		case "user":
			roleIcon = "👤"
			roleName = "用户"
		case "assistant":
			roleIcon = "🤖"
			roleName = "助手"
		case "tool":
			roleIcon = "🔧"
			roleName = "工具"
		default:
			roleIcon = "❓"
			roleName = "未知"
		}

		fmt.Printf("%d. %s %s: %s\n",
			i+1,
			roleIcon,
			roleName,
			truncateMessage(msg.Content, 256))

		// 如果有工具调用，显示工具调用信息
		if len(msg.ToolCallList) > 0 {
			for _, tc := range msg.ToolCallList {
				fmt.Printf("   🔧 调用工具: %s\n", tc.Function.Name)
			}
		}
	}
	fmt.Println(strings.Repeat("-", 40))
}

// showTools 显示可用工具
func showTools(aiAgent *agent.BasicAgent) {
	agentTools := aiAgent.GetTools()

	fmt.Println("🔧 当前Agent可用工具列表")
	fmt.Println(strings.Repeat("-", 40))

	if len(agentTools) == 0 {
		fmt.Println("暂无可用工具（MCP服务可能未启动）")
		return
	}

	i := 1
	for _, tool := range agentTools {
		fmt.Printf("%d. ** %s **\n", i, tool.Name())
		fmt.Printf("   描述: %s\n", tool.Description())
		fmt.Println()
		i++
	}

	fmt.Printf("总共 %d 个工具可用\n", len(agentTools))
	fmt.Println(strings.Repeat("-", 40))
}

// truncateMessage 截断消息内容
func truncateMessage(content string, maxLen int) string {
	// 转换为rune切片以正确处理Unicode字符
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen-3]) + "..."
}
