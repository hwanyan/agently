// Package main AI Agent 的主程序入口
package main

import (
	"log"
	"os"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/agents"
	"github.com/hwanyan/agently/internal/auth"
	"github.com/hwanyan/agently/internal/config"
	"github.com/hwanyan/agently/internal/http"
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
	agentManager := agents.InitAgents(cfg)

	// 初始化数据库和认证服务
	authSvc, sessionStore, messageStore := initDatabase(cfg)

	// 启动 HTTP 服务
	startHTTPServer(cfg, agentManager, authSvc, sessionStore, messageStore)
}

// initConfig 加载配置
func initConfig() *types.Config {
	cfg, err := config.LoadConfiguration()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
		logger.Error("加载配置失败: %v", err)
		os.Exit(0)
	}
	return cfg
}

// initLogger 初始化日志系统
func initLogger(cfg *types.Config) {
	if err := logger.Init(cfg.Log.FilePath); err != nil {
		log.Fatalf("初始化日志系统失败: %v", err)
		logger.Error("初始化日志系统失败: %v", err)
		os.Exit(0)
	}
	logger.Info("Agently 系统启动")
}

// initDatabase 初始化数据库服务
func initDatabase(cfg *types.Config) (authSvc *auth.Service, sessionStore *store.SessionStore,
	messageStore *store.MessageStore) {
	var db *store.DB
	var err error
	if cfg.Database.DSN == "" {
		log.Fatalf("未配置数据库 DSN，服务启动失败")
		logger.Error("未配置数据库 DSN，服务启动失败")
		os.Exit(0)
	}
	db, err = store.New(cfg.Database)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
		logger.Error("数据库连接失败: %v", err)
		os.Exit(0)
	}
	if cfg.SMTP.Host == "" {
		log.Fatalf("SMTP 未配置，邮箱验证服务无法启动，请在配置文件中填写 SMTP 配置")
		logger.Error("SMTP 未配置，邮箱验证服务无法启动，请在配置文件中填写 SMTP 配置")
		os.Exit(0)
	}

	defer db.Close()
	userStore := store.NewUserStore(db)
	sessionStore = store.NewSessionStore(db)
	messageStore = store.NewMessageStore(db)

	emailVerifySvc := auth.NewEmailVerifyService(db, cfg.SMTP)
	logger.Info("邮箱验证服务已启用（SMTP: %s）", cfg.SMTP.Host)

	authSvc = auth.NewService(userStore, cfg.JWT, emailVerifySvc)
	logger.Info("数据库连接成功，认证服务已初始化")

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
	httpService := http.NewHTTPService(
		manager,
		authSvc,
		sessionStore,
		messageStore,
		cfg,
	)
	httpService.Start()
	logger.Info("HTTP服务启动成功")
}
