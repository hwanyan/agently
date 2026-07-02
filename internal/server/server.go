package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/auth"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/store"
	agentlypb "github.com/hwanyan/agently/pb/gen"
	"github.com/hwanyan/agently/pkg/types"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// Server 负责编排 gRPC 服务与 grpc-gateway HTTP 网关。
//
// 架构：
//   - 核心业务以 gRPC 服务对外提供（AuthService / AgentService / SessionService）；
//   - grpc-gateway 在本机反向代理到 gRPC 端口，对外暴露与旧版完全一致的 REST 路径；
//   - 对话接口 /sessions/{id}/chat 走 gRPC 服务端流式，网关侧以自定义 SSE 处理器
//     把 gRPC 流事件转写为与旧版一致的 SSE（event/data）格式。
type Server struct {
	manager      *agent.Manager
	authSvc      *auth.Service
	sessionStore *store.SessionStore
	messageStore *store.MessageStore
	config       *types.Config
}

// NewServer 创建服务编排器
func NewServer(
	manager *agent.Manager,
	authSvc *auth.Service,
	sessionStore *store.SessionStore,
	messageStore *store.MessageStore,
	config *types.Config,
) *Server {
	return &Server{
		manager:      manager,
		authSvc:      authSvc,
		sessionStore: sessionStore,
		messageStore: messageStore,
		config:       config,
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

// startGRPC 启动 gRPC 服务器
func (s *Server) startGRPC() {
	addr := s.grpcAddr()
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("gRPC 监听失败: %v", err)
		log.Fatalf("gRPC 监听失败: %v", err)
	}

	// 声明鉴权拦截器
	interceptor := NewAuthInterceptor(s.authSvc)
	// 创建由 option 配置的 gRPC 服务器
	grpcServer := grpc.NewServer(
		// 给所有一元 RPC 装鉴权（可链式扩展更多中间件）
		grpc.ChainUnaryInterceptor(interceptor.Unary()),
		// 给所有流式 RPC（如 Chat）装鉴权
		grpc.ChainStreamInterceptor(interceptor.Stream()),
	)

	agentlypb.RegisterAuthServiceServer(grpcServer, NewAuthServer(s.authSvc))
	agentlypb.RegisterAgentServiceServer(grpcServer, NewAgentServer(s.manager))
	agentlypb.RegisterSessionServiceServer(grpcServer, NewSessionServer(s.manager, s.sessionStore, s.messageStore))

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
	server := &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second, // 流式响应需要较长的写超时
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("HTTP 网关启动，监听地址: %s", addr)
	if err := server.ListenAndServe(); err != nil {
		logger.Error("HTTP 网关启动失败: %v", err)
		log.Fatalf("HTTP 网关启动失败: %v", err)
	}
}

// sseChatHandler 返回自定义的 SSE 对话处理器：内部以 gRPC 客户端调用流式 Chat，
// 并把 gRPC 事件流转写为与旧版一致的 SSE（event/data）格式。
func (s *Server) sseChatHandler(client agentlypb.SessionServiceClient) runtime.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		sessionID := pathParams["session_id"]

		var body struct {
			Message   string `json:"message"`
			AutoRoute *bool  `json:"auto_route"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
			return
		}

		req := &agentlypb.ChatRequest{
			SessionId: sessionID,
			Message:   body.Message,
			AutoRoute: body.AutoRoute,
		}

		// 透传 Authorization 到 gRPC metadata
		ctx := r.Context()
		if a := r.Header.Get("Authorization"); a != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", a)
		}

		stream, err := client.Chat(ctx, req)
		if err != nil {
			st := status.Convert(err)
			writeJSONError(w, runtime.HTTPStatusFromCode(st.Code()), st.Message())
			return
		}

		// 预读第一个事件：若为鉴权/校验类错误（如未登录、会话不存在、消息为空），
		// 以标准 JSON 错误响应返回，保持与旧接口一致的 4xx 语义；否则进入 SSE 模式。
		first, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				w.WriteHeader(http.StatusOK)
				return
			}
			st := status.Convert(err)
			writeJSONError(w, runtime.HTTPStatusFromCode(st.Code()), st.Message())
			return
		}

		// 进入 SSE 模式
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSONError(w, http.StatusInternalServerError, "服务器不支持流式响应")
			return
		}

		writeSSEEvent(w, flusher, first)
		for {
			ev, rErr := stream.Recv()
			if rErr == io.EOF {
				return
			}
			if rErr != nil {
				st := status.Convert(rErr)
				data, _ := json.Marshal(map[string]string{"error": st.Message()})
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(data))
				flusher.Flush()
				return
			}
			writeSSEEvent(w, flusher, ev)
		}
	}
}

// writeSSEEvent 将一个 ChatEvent 按事件类型转写为旧版 SSE 格式
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, ev *agentlypb.ChatEvent) {
	var data []byte
	switch ev.GetType() {
	case "route":
		data, _ = json.Marshal(map[string]string{"agent_name": ev.GetAgentName(), "reason": ev.GetReason()})
	case "message":
		data, _ = json.Marshal(map[string]string{"content": ev.GetContent()})
	case "tool_call":
		data, _ = json.Marshal(map[string]string{"tool_name": ev.GetToolName(), "tool_args": ev.GetToolArgs()})
	case "tool_result":
		data, _ = json.Marshal(map[string]string{"tool_name": ev.GetToolName(), "tool_result": ev.GetToolResult()})
	case "tool_error":
		data, _ = json.Marshal(map[string]string{"tool_name": ev.GetToolName(), "error": ev.GetError()})
	case "error":
		data, _ = json.Marshal(map[string]string{"error": ev.GetError()})
	case "done":
		data = []byte("{}")
	default:
		data = []byte("{}")
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.GetType(), string(data))
	flusher.Flush()
}

// gatewayErrorHandler 统一网关错误响应为 {code, message}，code 为 HTTP 状态码
func gatewayErrorHandler(_ context.Context, _ *runtime.ServeMux, _ runtime.Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
	st := status.Convert(err)
	httpStatus := runtime.HTTPStatusFromCode(st.Code())
	writeJSONError(w, httpStatus, st.Message())
}

// writeJSONError 写入统一的 JSON 错误响应
func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    statusCode,
		"message": message,
	})
}

// corsMiddleware 处理跨域请求
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
