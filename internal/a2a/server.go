package a2a

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2acompat/a2av0"
	"github.com/a2aproject/a2a-go/v2/a2asrv"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/pkg/types"
)

// Server A2A 服务编排：将 manager 中的全部业务 Agent 暴露为独立的 A2A 端点。
//
// 路由约定（以 business agent [news_analysis] 为例）：
//   - POST /a2a/news_analysis                              → 该 Agent 的 A2A JSON-RPC 端点
//   - GET  /a2a/news_analysis/.well-known/agent-card.json  → 该 Agent 的 AgentCard（自描述清单）
//
// AgentCard 中 SupportedInterfaces[].url 指向 JSON-RPC 端点，供其他 Agent 发现后直接调用。
type Server struct {
	manager *agent.Manager
	cfg     *types.A2AConfig
	tenants map[string]string // tenant_id -> token，已解析并校验过；为空表示不启用鉴权
}

// NewServer 创建 A2A 服务编排器。
//
// 租户凭证在此处一次性解析并校验（tenant_id/token 均非空、互不重复等），
// 校验失败直接返回 error，遵循本项目"启动期 fail-fast"的惯例，避免带着错误配置起服务。
func NewServer(manager *agent.Manager, cfg *types.A2AConfig) (*Server, error) {
	tenants, err := resolveTenants(cfg)
	if err != nil {
		return nil, err
	}
	return &Server{manager: manager, cfg: cfg, tenants: tenants}, nil
}

// Start 启动 A2A HTTP 服务（阻塞运行，通常以 goroutine 方式调用）
func (s *Server) Start() {
	mux := http.NewServeMux()
	s.register(mux)

	httpServer := &http.Server{
		Addr:         s.addr(),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second, // 流式响应需要较长的写超时
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("A2A 服务启动，监听地址: %s", s.addr())
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("A2A 服务运行失败: %v", err)
	}
}

// register 为每个业务 Agent 挂载 JSON-RPC 端点与 AgentCard 端点
func (s *Server) register(mux *http.ServeMux) {
	// 未配置 public_url 时，AgentCard 的对外地址只能靠 host:port 本机推导（0.0.0.0 → localhost），
	// 这对第三方客户端不可用（localhost 指向客户端自己）。打印 warning 提示部署者显式配置 public_url。
	if s.cfg.PublicURL == "" {
		logger.Warning("未配置 a2a.public_url，AgentCard 将使用本机推导地址（%s），仅供本地/内网直连测试；给第三方使用请配置 a2a.public_url", s.publicBaseURL())
	}

	// 租户凭证已在 NewServer 中解析并校验完毕；为空表示未启用鉴权。
	authEnabled := len(s.tenants) > 0
	if !authEnabled {
		logger.Warning("未配置 A2A 鉴权租户（config.yaml 的 a2a.tenants），" +
			"JSON-RPC 端点不做鉴权，任何人都可直接调用；给第三方/公网使用前务必配置")
	} else {
		tenantIDs := make([]string, 0, len(s.tenants))
		for id := range s.tenants {
			tenantIDs = append(tenantIDs, id)
		}
		logger.Info("A2A 鉴权已启用，已注册租户: %s", strings.Join(tenantIDs, ", "))
	}

	for _, info := range s.manager.List() { // List() 已过滤系统 Agent，仅暴露业务 Agent
		agentID := info.ID

		// JSON-RPC 端点：仅当配置了鉴权租户时才挂载 tenantAuthInterceptor
		executor := newAgentExecutor(s.manager, agentID)
		var opts []a2asrv.RequestHandlerOption
		if authEnabled {
			opts = append(opts, a2asrv.WithCallInterceptors(newTenantAuthInterceptor(s.tenants)))
		}
		handler := a2asrv.NewHandler(executor, opts...)
		// 使用 a2acompat/a2av0 的 JSON-RPC handler（而非 a2asrv.NewJSONRPCHandler），
		// 因为该 SDK 的 a2asrv.NewJSONRPCHandler 内部使用的方法名是新版驼峰风格
		// （如 "SendMessage"），与 A2A 协议规范文档及绝大多数第三方客户端使用的
		// "message/send" 风格不一致，会导致标准调用报 -32601 method not found。
		// a2av0 是 SDK 官方维护的协议兼容层，认的是 "message/send"/"message/stream" 等
		// 规范风格方法名，内部仍然委托给同一个 handler（含鉴权拦截器），行为完全等价，
		// 仅方法名映射不同。
		mux.Handle(fmt.Sprintf("/a2a/%s", agentID), a2av0.NewJSONRPCHandler(handler))

		// AgentCard 端点：AgentCard 本身按 A2A 惯例保持公开可读（供发现），不做鉴权；
		// 但当 JSON-RPC 端点启用了鉴权时，需要在 Card 里如实声明 securitySchemes，
		// 让客户端在真正调用前就知道要带 X-A2A-Tenant-Id + Bearer token。
		endpointURL := fmt.Sprintf("%s/a2a/%s", s.publicBaseURL(), agentID)
		card := buildAgentCard(info, endpointURL, authEnabled)
		mux.Handle(
			fmt.Sprintf("/a2a/%s/.well-known/agent-card.json", agentID),
			a2asrv.NewStaticAgentCardHandler(card),
		)

		logger.Info("A2A 已暴露业务 Agent [%s]：%s（鉴权：%t）", agentID, endpointURL, authEnabled)
	}
}

// addr 返回 A2A 服务的监听地址
func (s *Server) addr() string {
	host := s.cfg.Host
	// 默认监听全部的网卡
	if host == "" {
		host = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%d", host, s.cfg.Port)
}

// publicBaseURL 返回对外可达的基础 URL（用于 AgentCard 中 SupportedInterfaces[].url）。
//
// 优先使用显式配置的 public_url——给第三方使用时必须配置此项（如经 nginx/ingress 反代后的
// 公网域名，或服务器的对外 IP）。未配置 public_url 时，默认使用本地地址（localhost）进行本地推导
// 这种情况下将只对本机/内网直连自测成立，对第三方不可用。
func (s *Server) publicBaseURL() string {
	if s.cfg.PublicURL != "" {
		return strings.TrimSuffix(s.cfg.PublicURL, "/")
	}
	logger.Warning("未配置 A2A 的public_url，将使用本机地址（仅限本地调用）")
	return fmt.Sprintf("http://localhost:%d", s.cfg.Port)
}
