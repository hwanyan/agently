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
	"agent-hwan/internal/prompt"
	"agent-hwan/internal/tools"
	"agent-hwan/pkg/types"

	"github.com/chzyer/readline"
)

func main() {
	fmt.Println()
	fmt.Println("🤖 AI Agent 开发教程")
	fmt.Println("====================")
	fmt.Println("欢迎来到 AI Agent 开发教程！")
	fmt.Println("这个程序将演示如何构建一个完整的 AI Agent 系统。")
	fmt.Println()

	// 第一步：加载配置
	fmt.Println("📋 步骤1: 加载配置...")
	cfg, err := loadConfiguration()
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
		os.Exit(0)
	}
	fmt.Println("✅ 配置加载成功！")

	// 第二步：创建LLM客户端
	fmt.Println("\n🔗 步骤2: 创建LLM客户端...")
	llmClient := llm.NewOpenAIClient(
		cfg.LLM.APIKey,
		cfg.LLM.Model,
		cfg.LLM.BaseURL,
	)
	fmt.Println("✅ LLM客户端创建成功！")

	// 第三步：获取全部的 Prompt 配置
	fmt.Println("\n📋 步骤3: 获取全部的 Prompt 配置...")
	prompts, err := prompt.GetPrompts()
	if err != nil {
		log.Fatalf("❌ 获取 Prompt 配置失败: %v", err)
		os.Exit(0)
	}
	fmt.Println("✅ Prompt配置获取成功！")
	fmt.Printf("  配置信息：%v\n", prompts)

	// 第四步：创建AI Agent
	fmt.Println("\n🧠 步骤4: 创建AI Agent...")
	systemPrompt, err := prompt.GetSystemPrompt(prompts, &cfg.Tools)
	if err != nil {
		fmt.Printf("  ❌ 获取系统提示失败: \n%v\n", err)
		os.Exit(0)
	}
	fmt.Println("  ✅ 成功加载系统提示词！")
	aiAgent := agent.NewBasicAgent(
		cfg.Agent.Name,
		cfg.Agent.Description,
		llmClient,
		systemPrompt,
	)
	fmt.Printf("✅ AI Agent '%s' 创建成功！\n", cfg.Agent.Name)

	// 第五步：添加工具
	fmt.Println("\n🔧 步骤5: 添加工具...")
	setupTools(aiAgent, cfg)
	fmt.Println("✅ 所有工具添加完成！")

	// 第六步：开始交互
	fmt.Println("\n🎯 步骤6: 开始交互...")
	startInteractiveSession(aiAgent)
}

// loadConfiguration 加载配置
func loadConfiguration() (*types.Config, error) {
	// 首先尝试从配置文件加载
	configPath := "config.json"
	// 读取配置文件
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("  📄 从配置文件加载: %s\n", configPath)
		// 加载配置文件并验证
		cfg, err := config.LoadConfig(configPath)
		if err == nil {
			return cfg, nil
		}
		fmt.Println("  ⚠️  配置文件中的API密钥未设置，尝试环境变量...")
	} else {
		fmt.Println("  ⚠️  配置文件不存在，尝试环境变量...")
	}

	// 从环境变量加载
	fmt.Println("  🌍 从环境变量加载配置...")
	cfg, err := config.LoadConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("从环境变量加载配置失败: %w", err)
	}
	// fmt.Printf("  加载的配置：%v", cfg)

	return cfg, nil
}

// setupTools 设置工具
func setupTools(aiAgent *agent.BasicAgent, cfg *types.Config) {
	// 添加计算器工具
	fmt.Println("  ➕ 添加计算器工具...")
	calculatorTool := tools.NewCalculatorTool("", "")
	aiAgent.AddTool(calculatorTool)
	fmt.Println("     ✓ 计算器工具已添加")

	// 添加天气工具
	fmt.Println("  🌤️  添加天气工具...")
	weatherTool := tools.NewWeatherTool(cfg.Tools.Weather.Name,
		cfg.Tools.Weather.Desc,
		cfg.Tools.Weather.APIKey,
		cfg.Tools.Weather.BaseURL)
	aiAgent.AddTool(weatherTool)
	fmt.Println("     ✓ 天气工具已添加")

	// 显示工具统计
	tools := aiAgent.GetTools()
	fmt.Printf("  📊 总共添加了 %d 个工具\n", len(tools))
}

// startInteractiveSession 开始交互会话
func startInteractiveSession(aiAgent *agent.BasicAgent) {
	fmt.Println()
	fmt.Println("🎉 AI Agent已准备就绪！")
	fmt.Println()
	fmt.Println("💡 你可以尝试以下问题：")
	fmt.Println("   • 计算 123 + 456")
	fmt.Println("   • 北京的天气怎么样？")
	fmt.Println("   • 帮我计算 15 * 8 - 20")
	fmt.Println("   • 上海今天天气如何？")
	fmt.Println("   • 什么是AI Agent？")
	fmt.Println()
	fmt.Println("🔧 可用命令：")
	fmt.Println("   • 'help' - 显示帮助信息")
	fmt.Println("   • 'history' - 查看对话历史")
	fmt.Println("   • 'clear' - 清空对话历史")
	fmt.Println("   • 'tools' - 查看可用工具")
	fmt.Println("   • 'quit' 或 'exit' - 退出程序")
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))

	ctx := context.Background()

	// 创建readline实例
	rl, err := readline.New("👤 用户: ")
	if err != nil {
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
		if handled := handleSpecialCommands(userInput, aiAgent); handled {
			continue
		}

		// 检查输入是否为空
		if userInput == "" {
			fmt.Println("💭 请输入一些内容，或输入 'help' 查看帮助")
			continue
		}

		// 处理用户消息
		fmt.Print("🤖 助手: ")
		fmt.Print("正在思考...")

		// 调用Agent处理消息
		response, err := aiAgent.ProcessMessage(ctx, userInput)
		if err != nil {
			// 清除"正在思考..."并显示错误（注：/r 的作用是将光标移动到当前行的行首，但不会换行，即实现同一行的覆盖式更新）
			fmt.Printf("\r🤖 助手: ❌ 处理消息时出错: %v\n", err)
			continue
		}

		// 清除"正在思考..."并显示响应
		fmt.Print("\r🤖 助手: ")
		fmt.Println(response)
	}

	fmt.Println("\n👋 感谢使用AI Agent教程！再见！")
}

// handleSpecialCommands 处理特殊命令
func handleSpecialCommands(input string, aiAgent *agent.BasicAgent) bool {
	switch strings.ToLower(input) {
	case "quit", "exit", "退出":
		fmt.Println("👋 再见！")
		os.Exit(0)
		return true

	case "help", "帮助":
		showHelp()
		return true

	case "history", "历史":
		showHistory(aiAgent)
		return true

	case "clear", "清空":
		aiAgent.ClearHistory()
		fmt.Println("🗑️  对话历史已清空")
		return true

	case "tools", "工具":
		showTools(aiAgent)
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
	fmt.Println("   • 直接输入问题与AI助手对话")
	fmt.Println("   • AI助手会根据需要自动调用工具")
	fmt.Println()
	fmt.Println("🔧 **可用工具**:")
	fmt.Println("   • 计算器 - 执行数学运算")
	fmt.Println("   • 天气查询 - 查询城市天气信息")
	fmt.Println()
	fmt.Println("⌨️  **命令**:")
	fmt.Println("   • 'help' - 显示此帮助信息")
	fmt.Println("   • 'history' - 查看对话历史")
	fmt.Println("   • 'clear' - 清空对话历史")
	fmt.Println("   • 'tools' - 查看可用工具详情")
	fmt.Println("   • 'quit' 或 'exit' - 退出程序")
	fmt.Println()
	fmt.Println("💡 **示例问题**:")
	fmt.Println("   • 计算 25 * 4 + 10")
	fmt.Println("   • 北京今天天气怎么样？")
	fmt.Println("   • 帮我算一下 100 除以 8")
	fmt.Println("   • 什么是人工智能？")
	fmt.Println(strings.Repeat("-", 40))
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
			truncateMessage(msg.Content, 128))

		// 如果有工具调用，显示工具调用信息
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				fmt.Printf("   🔧 调用工具: %s\n", tc.Function.Name)
			}
		}
	}
	fmt.Println(strings.Repeat("-", 40))
}

// showTools 显示可用工具
func showTools(aiAgent *agent.BasicAgent) {
	tools := aiAgent.GetTools()

	fmt.Println("🔧 可用工具列表")
	fmt.Println(strings.Repeat("-", 40))

	if len(tools) == 0 {
		fmt.Println("暂无可用工具")
		return
	}

	i := 1
	for _, tool := range tools {
		fmt.Printf("%d. ** %s **\n", i, tool.Name())
		fmt.Printf("   描述: %s\n", tool.Description())
		fmt.Println()
		i++
	}

	fmt.Printf("总共 %d 个工具可用\n", len(tools))
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
