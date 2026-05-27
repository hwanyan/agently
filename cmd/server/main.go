// Package main AI Agent 的主程序入口
package main

import (
	"log"
	"os"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/agents"
	"github.com/hwanyan/agently/internal/auth"
	"github.com/hwanyan/agently/internal/cli"
	"github.com/hwanyan/agently/internal/config"
	"github.com/hwanyan/agently/internal/httpserver"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/store"
	"github.com/hwanyan/agently/pkg/types"
)

func main() {
	// 加载配置
	cfg := initConfig()

	// 初始化日志系统
	initLogger(cfg)

	// 注册全部的 agent
	cliAgentManager := agents.InitAgents(cfg)
	httpAgentManager := agents.InitAgents(cfg)

	// 初始化数据库和认证服务
	authSvc, sessionStore, messageStore := initDatabase(cfg)

	// 启动 HTTP 服务
	go startHTTPServer(cfg, httpAgentManager, authSvc, sessionStore, messageStore)

	// 启动 CLI 交互
	startCLISession(cliAgentManager)
}

// initConfig 加载配置
func initConfig() *types.Config {
	cfg, err := config.LoadConfiguration()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
		os.Exit(0)
	}
	return cfg
}

// initLogger 初始化日志系统
func initLogger(cfg *types.Config) {
	if e := logger.Init(cfg.Log.FilePath); e != nil {
		log.Fatalf("初始化日志系统失败: %v", e)
		os.Exit(0)
	}
	logger.Info("Agently 系统启动")
}

// initDatabase 初始化数据库服务
func initDatabase(cfg *types.Config) (authSvc *auth.Service, sessionStore *store.SessionStore,
	messageStore *store.MessageStore) {
	var db *store.DB
	var err error
	if cfg.Database.DSN != "" {
		db, err = store.New(cfg.Database)
		if err != nil {
			logger.Error("数据库连接失败: %v（HTTP服务将以无持久化模式运行）", err)
		} else {
			defer db.Close()
			userStore := store.NewUserStore(db)
			sessionStore = store.NewSessionStore(db)
			messageStore = store.NewMessageStore(db)
			authSvc = auth.NewService(userStore, cfg.JWT)
			logger.Info("数据库连接成功，认证服务已初始化")
		}
	} else {
		logger.Info("未配置数据库 DSN，跳过数据库初始化")
	}
	logger.Info("数据库初始化完成")
	return authSvc, sessionStore, messageStore
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

// startCLISession 启动CLI交互会话
func startCLISession(manager *agent.Manager) {
	cliService := cli.NewCLIService(manager)
	cliService.Start()
	logger.Info("CLI交互服务启动成功")
}
