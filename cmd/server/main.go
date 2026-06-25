// Package main AI Agent 的主程序入口
package main

import (
	"log"

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
	agentManager := initAgent(cfg)

	// 初始化数据库和认证服务
	db, authSvc, sessionStore, messageStore := initDatabase(cfg)
	defer db.Close()

	// 启动 HTTP 服务
	startHTTPServer(cfg, agentManager, authSvc, sessionStore, messageStore)
}

// initAgents 加载 agent
// 注意：InitAgents 内部会做"配置-Agent 引用"严格校验与 MCP 服务探活，任意一项失败即 fail-fast，
// 避免出现"启动看似成功，运行时才发现工具为空"的隐性故障
func initAgent(cfg *types.Config) *agent.Manager {
	agentManager, err := agents.InitAgents(cfg)
	if err != nil {
		logger.Error("Agent 初始化失败: %v", err)
		log.Fatalf("Agent 初始化失败: %v", err)
	}
	return agentManager
}

// initConfig 加载配置
func initConfig() *types.Config {
	cfg, err := config.LoadConfiguration()
	if err != nil {
		logger.Error("加载配置失败: %v", err)
		log.Fatalf("加载配置失败: %v", err)
	}
	return cfg
}

// initLogger 初始化日志系统
func initLogger(cfg *types.Config) {
	if err := logger.Init(cfg.Log.FilePath); err != nil {
		logger.Error("初始化日志系统失败: %v", err)
		log.Fatalf("初始化日志系统失败: %v", err)
	}
	logger.Info("Agently 系统启动")
}

// initDatabase 初始化数据库服务
//
// 注意：返回的 *store.DB 是连接池句柄，其生命周期必须与进程一致；
// 由调用方（main）负责在进程退出时调用 db.Close()，
// 切勿在本函数内使用 defer db.Close()，否则函数返回后连接池立刻被关闭，
// 业务接口在后续调用 SQL 时会得到 "sql: database is closed" 错误。
func initDatabase(cfg *types.Config) (db *store.DB, authSvc *auth.Service,
	sessionStore *store.SessionStore, messageStore *store.MessageStore) {
	var err error
	if cfg.Database.DSN == "" {
		logger.Error("未配置数据库 DSN，服务启动失败")
		log.Fatalf("未配置数据库 DSN，服务启动失败")
	}
	db, err = store.New(cfg.Database)
	if err != nil {
		logger.Error("数据库连接失败: %v", err)
		log.Fatalf("数据库连接失败: %v", err)
	}
	if cfg.SMTP.Host == "" {
		logger.Error("SMTP 未配置，邮箱验证服务无法启动，请在配置文件中填写 SMTP 配置")
		log.Fatalf("SMTP 未配置，邮箱验证服务无法启动，请在配置文件中填写 SMTP 配置")
	}

	userStore := store.NewUserStore(db)
	sessionStore = store.NewSessionStore(db)
	messageStore = store.NewMessageStore(db)

	emailVerifySvc := auth.NewEmailVerifyService(db, cfg.SMTP)
	logger.Info("邮箱验证服务已启用（SMTP: %s）", cfg.SMTP.Host)

	authSvc = auth.NewService(userStore, cfg.JWT, emailVerifySvc)
	logger.Info("数据库连接成功，认证服务已初始化")

	return db, authSvc, sessionStore, messageStore
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
