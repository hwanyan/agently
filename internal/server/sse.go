package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	agentlypb "github.com/hwanyan/agently/pb/gen"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// sseChatHandler 返回自定义的 SSE 对话处理器：内部以 gRPC 客户端调用流式 Chat，
// 并把 gRPC 事件流转写为与旧版一致的 SSE（event/data）格式。
//
// 之所以没有下沉到 internal/server/rpc（RPC 业务实现层），是因为它本质上是
// "HTTP 网关如何把一个 gRPC 服务端流转写为 SSE" 的传输协议适配逻辑，与
// SessionServer.Chat 的业务语义（鉴权、路由、持久化）无关，理应留在负责
// 网关编排的 server 根包中。
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
