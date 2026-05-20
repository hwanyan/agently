// Package cli 提供命令行交互功能
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"agent-hwan/internal/agent"
	"agent-hwan/internal/logger"
	"agent-hwan/pkg/types"

	"github.com/chzyer/readline"
)

// CLIService 命令行服务
type CLIService struct {
	manager *agent.Manager
}

// NewCLIService 创建新的CLI服务
func NewCLIService(manager *agent.Manager) *CLIService {
	return &CLIService{
		manager: manager,
	}
}

// Start 启动CLI交互会话
func (c *CLIService) Start() {
	fmt.Println()
	fmt.Println("========== 🤖 AI Agent 助手（多Agent模式） ==========")
	fmt.Println()

	logger.Info("CLI服务启动")

	// 显示可用Agent列表
	c.showAgentList()

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
	rl, err := readline.New(c.getPromptPrefix(autoRoute))
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
		if c.handleSpecialCommands(userInput, rl, &autoRoute) {
			continue
		}

		// 检查输入是否为空
		if userInput == "" {
			fmt.Println("💭 请输入一些内容，或输入 'help' 查看帮助")
			continue
		}

		// 记录用户提问
		logger.Info("用户提问: %s", userInput)

		// 确定目标Agent
		var targetAgent *agent.BasicAgent
		var routeInfo string

		if autoRoute {
			// 自动路由模式：先记录用户输入，再通过路由Agent判断意图
			logger.Info("[auto] 用户输入: %s", userInput)
			routedID, reason, err := c.manager.Route(ctx, userInput)
			if err != nil {
				logger.Error("路由失败: %v", err)
				fmt.Printf("⚠️  路由失败，使用当前Agent: %v\n", err)
				targetAgent, _ = c.manager.Current()
				routeInfo = "路由失败"
			} else {
				targetAgent, _ = c.manager.Get(routedID)
				routeInfo = fmt.Sprintf("路由到 %s: %s", routedID, reason)
				// 如果路由到了不同的Agent，提示用户
				if routedID != c.manager.CurrentID() {
					fmt.Printf("🔀 [自动路由] → %s (%s)\n", targetAgent.GetName(), reason)
				}
			}
		} else {
			// 手动模式：使用当前Agent
			logger.Info("[%s] 用户输入: %s", c.manager.CurrentID(), userInput)
			targetAgent, err = c.manager.Current()
			if err != nil {
				fmt.Printf("❌ %v\n", err)
				continue
			}
			routeInfo = "手动模式"
		}

		logger.Info("路由信息: %s", routeInfo)

		// 处理用户消息
		fmt.Print("🤖 助手: ")
		fmt.Print("正在思考...")

		// 记录开始处理时间
		startTime := time.Now()

		// 流式响应
		responseStream, err := targetAgent.ProcessMessageStream(ctx, userInput)
		if err != nil {
			logger.Error("处理消息时出错: %v", err)
			fmt.Printf("\r🤖 助手: ❌ 处理消息时出错: %v\n", err)
			continue
		}

		// 清除"正在思考..."并开始流式输出
		fmt.Print("\r🤖 助手: ")

		// 收集完整响应和工具调用信息
		var fullResponse strings.Builder
		var toolCalls []string

		// 读取并处理流式事件
		for event := range responseStream {
			switch event.Type {
			case types.StreamEventContent:
				fmt.Print(event.Content)
				fullResponse.WriteString(event.Content)
			case types.StreamEventToolCall:
				toolCalls = append(toolCalls, event.ToolName)
			case types.StreamEventToolResult:
				// 工具结果已在agent层记录日志，此处无需额外处理
			case types.StreamEventError:
				fmt.Printf("\n❌ %s", event.Error)
			}
		}

		// 记录处理结果
		processingTime := time.Since(startTime)
		logger.Info("Agent响应: %s", fullResponse.String())
		logger.Info("处理耗时: %v", processingTime)

		// 记录工具调用情况
		if len(toolCalls) > 0 {
			logger.Info("本次对话使用工具: %v", toolCalls)
		} else {
			logger.Info("本次对话未使用工具")
		}

		fmt.Println() // 换行
	}

	logger.Info("CLI服务停止")
	fmt.Println("\n👋 感谢使用AI Agent系统！再见！")
}

// getPromptPrefix 获取终端提示符前缀
func (c *CLIService) getPromptPrefix(autoRoute bool) string {
	if autoRoute {
		return "👤 [auto] > "
	}
	currentID := c.manager.CurrentID()
	return fmt.Sprintf("👤 [%s] > ", currentID)
}

// handleSpecialCommands 处理特殊命令
func (c *CLIService) handleSpecialCommands(input string, rl *readline.Instance, autoRoute *bool) bool {
	lower := strings.ToLower(input)

	switch {
	case lower == "quit" || lower == "exit" || lower == "退出":
		fmt.Println("👋 再见！")
		os.Exit(0)
		return true

	case lower == "help" || lower == "帮助":
		c.showHelp()
		return true

	case lower == "history" || lower == "历史":
		currentAgent, err := c.manager.Current()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return true
		}
		c.showHistory(currentAgent)
		return true

	case lower == "clear" || lower == "清空":
		currentAgent, err := c.manager.Current()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return true
		}
		currentAgent.ClearHistory()
		logger.Info("用户清空对话历史")
		fmt.Println("🗑️  对话历史已清空")
		return true

	case lower == "tools" || lower == "工具":
		currentAgent, err := c.manager.Current()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return true
		}
		c.showTools(currentAgent)
		return true

	case lower == "agents" || lower == "助手列表":
		c.showAgentList()
		return true

	case lower == "auto" || lower == "自动":
		*autoRoute = true
		rl.SetPrompt(c.getPromptPrefix(*autoRoute))
		logger.Info("用户开启自动路由模式")
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
		if c.manager.IsSystemAgent(targetID) {
			fmt.Println("❌ 不能手动切换到系统Agent")
			return true
		}
		if err := c.manager.Switch(targetID); err != nil {
			logger.Error("切换Agent失败: %v", err)
			fmt.Printf("❌ 切换失败: %v\n", err)
		} else {
			// 关闭自动路由，进入手动模式
			*autoRoute = false
			rl.SetPrompt(c.getPromptPrefix(*autoRoute))
			currentAgent, _ := c.manager.Current()
			logger.Info("用户切换到Agent: %s", targetID)
			fmt.Printf("✅ 已切换到: %s (%s)，自动路由已关闭\n", currentAgent.GetName(), targetID)
			fmt.Println("💡 输入 'auto' 可重新开启自动路由模式")
		}
		return true

	default:
		return false
	}
}

// showHelp 显示帮助信息
func (c *CLIService) showHelp() {
	logger.Info("用户查看帮助信息")
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
func (c *CLIService) showAgentList() {
	logger.Info("用户查看Agent列表")
	agents := c.manager.List()

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
func (c *CLIService) showHistory(aiAgent *agent.BasicAgent) {
	logger.Info("用户查看对话历史")
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
			c.truncateMessage(msg.Content, 256))

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
func (c *CLIService) showTools(aiAgent *agent.BasicAgent) {
	logger.Info("用户查看可用工具")
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
func (c *CLIService) truncateMessage(content string, maxLen int) string {
	// 转换为rune切片以正确处理Unicode字符
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen-3]) + "..."
}
