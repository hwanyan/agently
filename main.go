// Package main AI Agent 的主程序入口
package main

import (
	"log"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/agents"
	"github.com/hwanyan/agently/internal/auth"
	"github.com/hwanyan/agently/internal/config"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/server"
	"github.com/hwanyan/agently/internal/store"
	"github.com/hwanyan/agently/pkg/types"
)

func main() {
	// 加载配置（支持热更新：cm.Get 本身就是 ConfigProvider，统一以该接口向下游分发）
	cm := initConfigManager()
	defer cm.Close()

	// 初始化日志系统
	initLogger(cm.Get)

	// 注册全部的 agent（内部用 provider() 取快照做 MCP 启动期校验，并把 provider 传给 LLM 与本地工具）
	agentManager := initAgent(cm.Get)

	// 初始化数据库和认证服务
	db, authSvc, sessionStore, messageStore := initDatabase(cm.Get)
	defer db.Close()

	// 启动 gRPC 服务与 HTTP 网关
	startServer(cm.Get, agentManager, authSvc, sessionStore, messageStore)
}

// initConfigManager 创建配置管理器并启动热更新监听（watch 模式）
//
// 首次加载失败即 fail-fast；watch 启动失败也视为 fail-fast，避免"看似支持热更新、实际未生效"的隐性故障。
// config.yaml 后续内容发生变更时，会在内存中原子替换，无需重启进程即可生效（受限于部分启动期
// 一次性配置项，如监听端口/数据库连接池/日志路径/MCP服务器列表，详见 types.ConfigProvider 的说明）。
// cm.Get 满足 types.ConfigProvider 签名，可直接作为配置提供者传递给下游组件。
func initConfigManager() *config.Manager {
	cm, err := config.NewManager("./config.yaml")
	if err != nil {
		logger.Error("加载配置失败: %v", err)
		log.Fatalf("加载配置失败: %v", err)
	}
	if err := cm.Watch(); err != nil {
		logger.Error("启动配置热更新监听失败: %v", err)
		log.Fatalf("启动配置热更新监听失败: %v", err)
	}
	return cm
}

// initAgents 加载 agent
// 注意：InitAgents 内部会做"配置-Agent 引用"严格校验与 MCP 服务探活，任意一项失败即 fail-fast，
// 避免出现"启动看似成功，运行时才发现工具为空"的隐性故障
func initAgent(provider types.ConfigProvider) *agent.Manager {
	agentManager, err := agents.InitAgents(provider)
	if err != nil {
		logger.Error("Agent 初始化失败: %v", err)
		log.Fatalf("Agent 初始化失败: %v", err)
	}
	return agentManager
}

// initLogger 初始化日志系统
func initLogger(provider types.ConfigProvider) {
	if err := logger.Init(provider().Log.FilePath); err != nil {
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
func initDatabase(provider types.ConfigProvider) (db *store.DB, authSvc *auth.Service,
	sessionStore *store.SessionStore, messageStore *store.MessageStore) {
	cfg := provider()
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

// startServer 启动 gRPC 服务与 grpc-gateway HTTP 网关
func startServer(
	provider types.ConfigProvider,
	manager *agent.Manager,
	authSvc *auth.Service,
	sessionStore *store.SessionStore,
	messageStore *store.MessageStore,
) {
	srv := server.NewServer(
		manager,
		authSvc,
		sessionStore,
		messageStore,
		provider,
	)
	srv.Start()
	logger.Info("服务启动成功")
}
