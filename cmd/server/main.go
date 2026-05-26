// Package main AI Agent 的主程序入口
package main

import (
	"agent-hwan/internal/agent"
	"agent-hwan/internal/auth"
	"agent-hwan/internal/cli"
	"agent-hwan/internal/config"
	"agent-hwan/internal/httpserver"
	"agent-hwan/internal/llm"
	"agent-hwan/internal/logger"
	"agent-hwan/internal/prompt"
	"agent-hwan/internal/store"
	"agent-hwan/internal/tools"
	"agent-hwan/internal/tools/mcp"
	"agent-hwan/pkg/types"
	"context"
	"log"
	"os"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfiguration()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
		os.Exit(0)
	}

	// 初始化日志系统（按日期划分）
	if e := logger.Init(cfg.Log.FilePath); e != nil {
		log.Fatalf("初始化日志系统失败: %v", e)
		os.Exit(0)
	}
	defer logger.Close()
	logger.Info("AI Agent系统启动")

	// 第一步：创建LLM客户端
	llmClient := llm.NewOpenAIClient(
		cfg.LLM.APIKey,
		cfg.LLM.Model,
		cfg.LLM.BaseURL,
	)
	logger.Info("【系统步骤-1】LLM客户端创建成功")

	// 第二步：获取全部的 Prompt 配置
	prompts, err := prompt.GetPrompts(cfg.Prompt.Dir)
	if err != nil {
		log.Fatalf("获取 Prompt 配置失败: %v", err)
		os.Exit(0)
	}
	logger.Info("【系统步骤-2】Prompt配置获取成功")

	// 第三步：创建Agent管理器并注册全部Agent
	agentManager := initAllAgents(cfg, llmClient, prompts)
	if err := agentManager.ValidateSystemAgents(); err != nil {
		log.Fatalf("系统Agent校验失败: %v", err)
		os.Exit(0)
	}
	logger.Info("【系统步骤-3】Agent管理器创建成功，全部Agent已注册，系统Agent校验通过")

	// 第四步：为 HTTP 服务创建独立的 Agent 实例
	httpAgentManager := initAllAgents(cfg, llmClient, prompts)
	if err := httpAgentManager.ValidateSystemAgents(); err != nil {
		log.Fatalf("HTTP Agent管理器系统Agent校验失败: %v", err)
		os.Exit(0)
	}
	logger.Info("【系统步骤-4】HTTP Agent管理器创建成功")

	// 第五步：初始化数据库和认证服务
	var db *store.DB
	var authSvc *auth.Service
	var userStore *store.UserStore
	var sessionStore *store.SessionStore
	var messageStore *store.MessageStore
	if cfg.Database.DSN != "" {
		db, err = store.New(cfg.Database)
		if err != nil {
			logger.Error("数据库连接失败: %v（HTTP服务将以无持久化模式运行）", err)
		} else {
			defer db.Close()
			userStore = store.NewUserStore(db)
			sessionStore = store.NewSessionStore(db)
			messageStore = store.NewMessageStore(db)
			authSvc = auth.NewService(userStore, cfg.JWT)
			logger.Info("数据库连接成功，认证服务已初始化")
		}
	} else {
		logger.Info("未配置数据库 DSN，跳过数据库初始化")
	}
	logger.Info("【系统步骤-5】数据库初始化成功")

	// 启动 HTTP 服务
	go startHTTPServer(cfg, httpAgentManager, authSvc, sessionStore, messageStore)
	// 启动 CLI 交互
	startCLISession(agentManager)

}

// startCLISession 启动CLI交互会话
func startCLISession(manager *agent.Manager) {
	cliService := cli.NewCLIService(manager)
	cliService.Start()
	logger.Info("CLI交互服务启动成功")
}

// initAllAgents 初始化Agent管理器，注册全部Agent
func initAllAgents(cfg *types.Config, llmClient types.LLMClient, prompts map[string]string) *agent.Manager {
	manager := agent.NewManager(cfg.Agents)

	for _, agentCfg := range cfg.Agents {
		// 根据工具组确定模板变量
		vars := make(map[string]any)
		switch agentCfg.ToolGroup {
		case "none":
			// 系统Agent（如路由Agent）不需要工具，但需要Agent列表描述
			vars["AGENTS_SECTION"] = tools.GenerateAgentsDescription(cfg.Agents)
		}

		// 获取对应场景的系统提示词
		systemPrompt, err := prompt.GetPromptByScene(prompts, agentCfg.PromptScene, vars)
		if err != nil {
			// 系统Agent初始化失败直接报错退出，业务Agent允许跳过
			if agentCfg.Role == types.AgentRoleSystem {
				log.Fatalf("系统Agent [%s] 初始化失败: %v", agentCfg.ID, err)
			}
			continue
		}

		// 创建Agent实例
		aiAgent := agent.NewBasicAgent(
			agentCfg.Name,
			agentCfg.Description,
			llmClient,
			systemPrompt,
		)

		// 根据工具组注册工具
		switch agentCfg.ToolGroup {
		case "general":
			tools.SetupGeneralTools(aiAgent, cfg)
		case "mcp_gold":
			setupMCPToolsForAgent(aiAgent, cfg)
		case "none":
			// 系统Agent不需要工具
		}

		// 注册到管理器
		manager.Register(agentCfg.ID, agentCfg.Role, aiAgent)
	}

	return manager
}

// setupMCPToolsForAgent 为Agent设置MCP工具
func setupMCPToolsForAgent(aiAgent *agent.BasicAgent, cfg *types.Config) {
	if len(cfg.MCP.Servers) == 0 {
		return
	}

	for _, serverCfg := range cfg.MCP.Servers {
		// 创建MCP客户端
		client := mcp.NewClient(serverCfg)

		// 发现并创建工具
		mcpTools, err := mcp.DiscoverAndCreateTools(context.Background(), client)
		if err != nil {
			continue
		}

		// 将MCP工具注册到Agent
		var toolInterfaces []types.Tool
		for _, t := range mcpTools {
			toolInterfaces = append(toolInterfaces, t)
		}
		tools.SetupMCPTools(aiAgent, toolInterfaces)
	}
}

// startHTTPServer 启动 HTTP 服务
func startHTTPServer(
	cfg *types.Config,
	manager *agent.Manager,
	authSvc *auth.Service,
	sessionStore *store.SessionStore,
	messageStore *store.MessageStore,
) {
	httpService := httpserver.NewHTTPService(
		manager,
		authSvc,
		sessionStore,
		messageStore,
		cfg,
	)
	httpService.Start()
	logger.Info("HTTP服务启动成功")
}
