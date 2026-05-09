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

	"github.com/chzyer/readline"
)

func main() {
	fmt.Println()
	fmt.Println("========== 🤖 AI Agent 助手 ==========")
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
	if err := logger.Init(cfg.Log.FilePath); err != nil {
		log.Fatalf("❌ 初始化日志系统失败: %v", err)
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

	// 第三步：注册工具信息
	logger.Info("步骤3: 注册工具信息...")
	toolInfos := tools.NewToolRegistry()
	logger.Info("工具信息注册成功")

	// 第四步：获取全部的 Prompt 配置
	logger.Info("步骤4: 获取全部的 Prompt 配置...")
	prompts, err := prompt.GetPrompts(cfg.Prompt.Dir)
	if err != nil {
		logger.Error("获取 Prompt 配置失败: %v", err)
		log.Fatalf("❌ 获取 Prompt 配置失败: %v", err)
		os.Exit(0)
	}
	logger.Info("Prompt配置获取成功")

	// 第五步：创建AI Agent
	logger.Info("步骤5: 创建AI Agent...")
	systemPrompt, err := prompt.GetSystemPrompt(prompts, toolInfos)
	if err != nil {
		logger.Error("获取系统提示失败: %v", err)
		fmt.Printf("  ❌ 获取系统提示失败: \n%v\n", err)
		os.Exit(0)
	}
	logger.Info("成功加载系统提示词")
	aiAgent := agent.NewBasicAgent(
		cfg.Agent.Name,
		cfg.Agent.Description,
		llmClient,
		systemPrompt,
	)
	logger.Info("AI Agent '%s' 创建成功", cfg.Agent.Name)

	// 第六步：添加工具
	logger.Info("步骤6: 添加工具...")
	tools.SetupTools(aiAgent, cfg)
	logger.Info("所有工具添加完成")

	// 第七步：开始交互
	logger.Info("步骤7: 开始交互")
	fmt.Println("================ END ================")
	startInteractiveSession(aiAgent)
}

// startInteractiveSession 开始交互会话
func startInteractiveSession(aiAgent *agent.BasicAgent) {
	fmt.Println()
	fmt.Println("🎉 AI Agent已准备就绪！")
	fmt.Println()
	fmt.Println("💡 你可以尝试以下问题：")
	fmt.Println("   • 北京的天气怎么样？")
	fmt.Println("   • 上海今天天气如何？")
	fmt.Println("   • 查询 8.8.8.8 的IP信息")
	fmt.Println("   • 查询 google.com 的位置")
	fmt.Println("   • 生成一个内容为 https://example.com 的二维码（默认 dataurl）")
	fmt.Println("   • 请在终端展示这个二维码（使用 ascii 输出）")
	fmt.Println("   • 请将这个二维码保存为 png 图片，文件名设为 my_qr.png（使用 file 输出）")
	fmt.Println("   • 请把\"今天天气不错，适合散步。\"翻译为英文")
	fmt.Println("   • 把 \"Bonjour tout le monde\" 翻成中文")
	fmt.Println("   • 统计这段文字的字符数：\"Hello World\"")
	fmt.Println("   • 把 \"hello world\" 转换为大写")
	fmt.Println("   • 从这段文字中提取邮箱地址：\"联系我：john@example.com\"")
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

		logger.Info("用户输入: %s", userInput)

		// // 方式一：非流式响应接口的处理逻辑
		// 调用Agent处理消息
		//response, err := aiAgent.ProcessMessage(ctx, userInput)
		//if err != nil {
		//	// 清除"正在思考..."并显示错误（注：/r 的作用是将光标移动到当前行的行首，但不会换行，即实现同一行的覆盖式更新）
		//	logger.Error("处理消息时出错: %v", err)
		//	fmt.Printf("\r🤖 助手: ❌ 处理消息时出错: %v\n", err)
		//	continue
		//}
		//// 清除"正在思考..."并显示响应
		//logger.Info("AI响应: %s", response)
		//fmt.Print("\r🤖 助手: ")
		//fmt.Println(response)

		// 方式二：流式响应接口的处理逻辑
		// 调用Agent处理消息（流式）
		responseStream, err := aiAgent.ProcessMessageStream(ctx, userInput)
		if err != nil {
			// 清除"正在思考..."并显示错误（注：/r 的作用是将光标移动到当前行的行首，但不会换行，即实现同一行的覆盖式更新）
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
	fmt.Println("   • 天气查询 - 查询城市天气信息")
	fmt.Println("   • IP查询 - 查询IP地址地理位置信息")
	fmt.Println("   • 二维码生成 - 生成二维码（dataurl/ascii/file）")
	fmt.Println("   • 翻译工具 - 多语言文本翻译")
	fmt.Println("   • 文本处理 - 文本统计、格式转换、内容提取等")
	fmt.Println("   • 短链接生成 - 将长URL转换为短链接")
	fmt.Println()
	fmt.Println("⌨️  **命令**:")
	fmt.Println("   • 'help' - 显示此帮助信息")
	fmt.Println("   • 'history' - 查看对话历史")
	fmt.Println("   • 'clear' - 清空对话历史")
	fmt.Println("   • 'tools' - 查看可用工具详情")
	fmt.Println("   • 'quit' 或 'exit' - 退出程序")
	fmt.Println()
	fmt.Println("💡 **示例问题**:")
	fmt.Println("   • 北京今天天气怎么样？")
	fmt.Println("   • 查询 8.8.8.8 的IP信息")
	fmt.Println("   • 查询 google.com 的位置")
	fmt.Println("   • 生成一个内容为 https://example.com 的二维码（默认 dataurl）")
	fmt.Println("   • 请在终端展示这个二维码（使用 ascii 输出）")
	fmt.Println("   • 请将这个二维码保存为 png 图片，文件名设为 my_qr.png（使用 file 输出）")
	fmt.Println("   • 请把\"今天天气不错，适合散步。\"翻译为英文")
	fmt.Println("   • 把 \"Bonjour tout le monde\" 翻成中文")
	fmt.Println("   • 统计这段文字的字符数：\"Hello World\"")
	fmt.Println("   • 把 \"hello world\" 转换为大写")
	fmt.Println("   • 从这段文字中提取邮箱地址：\"联系我：john@example.com\"")
	fmt.Println("   • 帮我缩短这个链接：https://github.com/example/repository")
	fmt.Println("   • 使用 is.gd 服务生成短链接：https://www.example.com")
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
