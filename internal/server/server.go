// Package server 负责 gRPC 服务与 grpc-gateway HTTP 网关的启动编排。
//
// 分层说明：
//   - internal/server            （本包）  仅承担"进程如何启动"这一件事：
//     监听地址计算、gRPC Server/HTTP Gateway 的创建与注册、SSE 网关处理器等
//     "为 main.go 的编排提供抽象"的引导层（bootstrap）逻辑。
//   - internal/server/rpc         承载具体 RPC 方法的业务实现（AuthServer / AgentServer / SessionServer）。
//   - internal/server/middleware  承载鉴权拦截器与请求上下文注入等横切关注点。
//
// 三者职责正交、依赖方向单一（server 依赖 rpc 与 middleware，反之不成立），
// 新增一个 RPC 服务或一个中间件时，只需在对应子包内扩展，无需触碰本文件。
package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/auth"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/server/middleware"
	"github.com/hwanyan/agently/internal/server/rpc"
	"github.com/hwanyan/agently/internal/store"
	agentlypb "github.com/hwanyan/agently/pb/gen"
	"github.com/hwanyan/agently/pkg/types"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
)

// Server 负责编排 gRPC 服务与 grpc-gateway HTTP 网关。
//
// 架构：
//   - 核心业务以 gRPC 服务对外提供（AuthService / AgentService / SessionService），
//     具体实现见 internal/server/rpc；
//   - grpc-gateway 在本机反向代理到 gRPC 端口，对外暴露与旧版完全一致的 REST 路径；
//   - 对话接口 /sessions/{id}/chat 走 gRPC 服务端流式，网关侧以自定义 SSE 处理器
//     （见 sse.go）把 gRPC 流事件转写为与旧版一致的 SSE（event/data）格式。
type Server struct {
	manager      *agent.Manager
	authSvc      *auth.Service
	sessionStore *store.SessionStore
	messageStore *store.MessageStore
	config       *types.Config
}

// NewServer 创建服务编排器
//
// 监听地址等属于启动期一次性生效的配置（config.yaml 变更后无法热更新监听端口），
// 因此本类在 Start() 启动时通过 provider() 取一份当前生效的配置快照并固化到 config 字段，
// 进程生命周期内监听地址保持一致。provider 由 config.Manager.Get 提供，统一 ConfigProvider 接口。
func NewServer(
	manager *agent.Manager,
	authSvc *auth.Service,
	sessionStore *store.SessionStore,
	messageStore *store.MessageStore,
	provider types.ConfigProvider,
) *Server {
	return &Server{
		manager:      manager,
		authSvc:      authSvc,
		sessionStore: sessionStore,
		messageStore: messageStore,
		config:       provider(),
	}
}

// grpcAddr 计算 gRPC 监听地址
func (s *Server) grpcAddr() string {
	host := s.config.GRPC.Host
	// gRPC 仅作为 HTTP 网关的内部服务，默认只监听回环，不对外暴露。
	// 如需对外提供原生 gRPC 接入，可在配置中显式设置 grpc.host（如 "0.0.0.0"）。
	if host == "" {
		host = "127.0.0.1"
	}
	port := s.config.GRPC.Port
	// grpc.port 默认为 50050
	if port == 0 {
		port = 50050
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// grpcDialTarget 计算网关反向代理到 gRPC 的目标地址（始终走本机回环）
func (s *Server) grpcDialTarget() string {
	port := s.config.GRPC.Port
	if port == 0 {
		port = s.config.HTTP.Port + 1
	}
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// httpAddr 计算 HTTP 网关监听地址
func (s *Server) httpAddr() string {
	host := s.config.HTTP.Host
	if host == "" {
		host = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%d", host, s.config.HTTP.Port)
}

// Start 启动 gRPC 服务与 HTTP 网关（阻塞运行）
func (s *Server) Start() {
	// 防御性检查：核心依赖必须就绪
	if s.authSvc == nil || s.sessionStore == nil || s.messageStore == nil {
		logger.Error("服务启动失败: 关键依赖未初始化（authSvc/sessionStore/messageStore）")
		log.Fatalf("服务启动失败: 关键依赖未初始化（authSvc/sessionStore/messageStore）")
	}

	// 1. 启动 gRPC 服务
	go s.startGRPC()

	// 2. 启动 HTTP 网关（阻塞）
	s.startGateway()
}

// startGRPC 启动 gRPC 服务器，注册全部 RPC 服务实现（internal/server/rpc）
// 并挂载鉴权拦截器（internal/server/middleware）
func (s *Server) startGRPC() {
	addr := s.grpcAddr()
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("gRPC 监听失败: %v", err)
		log.Fatalf("gRPC 监听失败: %v", err)
	}

	// 声明鉴权拦截器
	interceptor := middleware.NewAuthInterceptor(s.authSvc)
	// 创建由 option 配置的 gRPC 服务器
	grpcServer := grpc.NewServer(
		// 给所有一元 RPC 装鉴权（可链式扩展更多中间件）
		grpc.ChainUnaryInterceptor(interceptor.Unary()),
		// 给所有流式 RPC（如 Chat）装鉴权
		grpc.ChainStreamInterceptor(interceptor.Stream()),
	)

	agentlypb.RegisterAuthServiceServer(grpcServer, rpc.NewAuthServer(s.authSvc))
	agentlypb.RegisterAgentServiceServer(grpcServer, rpc.NewAgentServer(s.manager))
	agentlypb.RegisterSessionServiceServer(grpcServer, rpc.NewSessionServer(s.manager, s.sessionStore, s.messageStore))

	logger.Info("gRPC 服务启动，监听地址: %s", addr)
	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("gRPC 服务运行失败: %v", err)
		log.Fatalf("gRPC 服务运行失败: %v", err)
	}
}

// startGateway 启动 grpc-gateway HTTP 网关
func (s *Server) startGateway() {
	ctx := context.Background()

	// 字段名使用 snake_case（UseProtoNames），并输出零值字段（EmitUnpopulated）
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true, // 用 proto 原始字段名（蛇形）
				EmitUnpopulated: true, // 零值字段也输出（如 false / "" / 0）
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
		// 把 HTTP Authorization 头透传到 gRPC metadata，供鉴权拦截器使用
		runtime.WithMetadata(func(_ context.Context, r *http.Request) metadata.MD {
			md := metadata.MD{}
			if a := r.Header.Get("Authorization"); a != "" {
				md.Set("authorization", a)
			}
			return md
		}),
		// 统一错误响应格式 {code, message}，code 使用 HTTP 状态码（与旧接口一致）
		runtime.WithErrorHandler(gatewayErrorHandler),
	)

	// 与 gRPC 服务建立本机连接（供网关反向代理与 SSE 处理器复用）
	// 因为目标是本机回环，明文通信无需 TLS
	conn, err := grpc.NewClient(s.grpcDialTarget(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("网关连接 gRPC 失败: %v", err)
		log.Fatalf("网关连接 gRPC 失败: %v", err)
	}

	// 注册生成的网关处理器
	if err := agentlypb.RegisterAuthServiceHandler(ctx, mux, conn); err != nil {
		log.Fatalf("注册 AuthService 网关失败: %v", err)
	}
	if err := agentlypb.RegisterAgentServiceHandler(ctx, mux, conn); err != nil {
		log.Fatalf("注册 AgentService 网关失败: %v", err)
	}
	if err := agentlypb.RegisterSessionServiceHandler(ctx, mux, conn); err != nil {
		log.Fatalf("注册 SessionService 网关失败: %v", err)
	}

	// 自定义 SSE 对话处理器必须在生成处理器【之后】注册：
	// runtime.ServeMux.Handle 采用“前插”策略、匹配时从前往后取首个命中，
	// 因此后注册者位于最前、优先命中，从而覆盖生成的流式 Chat 处理器。
	sessionClient := agentlypb.NewSessionServiceClient(conn)
	if err := mux.HandlePath("POST", "/v1/sessions/{session_id}:chat", s.sseChatHandler(sessionClient)); err != nil {
		logger.Error("注册 SSE 对话处理器失败: %v", err)
		log.Fatalf("注册 SSE 对话处理器失败: %v", err)
	}

	addr := s.httpAddr()
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second, // 流式响应需要较长的写超时
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("HTTP 网关启动，监听地址: %s", addr)
	if err := httpServer.ListenAndServe(); err != nil {
		logger.Error("HTTP 网关启动失败: %v", err)
		log.Fatalf("HTTP 网关启动失败: %v", err)
	}
}
