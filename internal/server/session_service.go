package server

import (
	"context"
	"strings"
	"time"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/store"
	agentlypb "github.com/hwanyan/agently/pb/gen"
	"github.com/hwanyan/agently/pkg/types"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SessionServer 会话与对话服务的 gRPC 实现
type SessionServer struct {
	agentlypb.UnimplementedSessionServiceServer
	manager      *agent.Manager
	sessionStore *store.SessionStore
	messageStore *store.MessageStore
}

// NewSessionServer 创建会话服务实现
func NewSessionServer(manager *agent.Manager, sessionStore *store.SessionStore, messageStore *store.MessageStore) *SessionServer {
	return &SessionServer{
		manager:      manager,
		sessionStore: sessionStore,
		messageStore: messageStore,
	}
}

// ListSessions 查询当前用户的会话列表
func (s *SessionServer) ListSessions(ctx context.Context, req *agentlypb.ListSessionsRequest) (*agentlypb.ListSessionsResponse, error) {
	userID := UserIDFromContext(ctx)

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := int(req.GetOffset())
	if offset < 0 {
		offset = 0
	}

	logger.Info("gRPC请求: 查询会话列表 - 用户: %s, Agent: %s", userID, req.GetAgentId())
	sessions, err := s.sessionStore.ListByUser(userID, req.GetAgentId(), limit, offset)
	if err != nil {
		logger.Error("查询会话列表失败: %v", err)
		return nil, status.Error(codes.Internal, "查询会话列表失败: "+err.Error())
	}

	items := make([]*agentlypb.Session, 0, len(sessions))
	for _, sess := range sessions {
		items = append(items, toPBSession(sess))
	}
	logger.Info("返回会话列表，共 %d 个会话", len(items))
	return &agentlypb.ListSessionsResponse{Code: 0, Sessions: items}, nil
}

// CreateSession 创建新会话
func (s *SessionServer) CreateSession(ctx context.Context, req *agentlypb.CreateSessionRequest) (*agentlypb.CreateSessionResponse, error) {
	userID := UserIDFromContext(ctx)

	if req.GetAgentId() == "" {
		logger.Warning("创建会话时AgentID为空")
		return nil, status.Error(codes.InvalidArgument, "agent_id 不能为空")
	}
	if _, exists := s.manager.Get(req.GetAgentId()); !exists {
		logger.Warning("创建会话时Agent不存在: %s", req.GetAgentId())
		return nil, status.Error(codes.InvalidArgument, "Agent 不存在: "+req.GetAgentId())
	}

	logger.Info("gRPC请求: 创建新会话 - 用户: %s, Agent: %s", userID, req.GetAgentId())
	session, err := s.sessionStore.Create(userID, req.GetAgentId(), req.GetTitle())
	if err != nil {
		logger.Error("创建会话失败: %v", err)
		return nil, status.Error(codes.Internal, "创建会话失败: "+err.Error())
	}
	logger.Info("创建会话成功: %s", session.ID)
	return &agentlypb.CreateSessionResponse{Code: 0, Session: toPBSession(session)}, nil
}

// ListMessages 查询会话消息历史
func (s *SessionServer) ListMessages(ctx context.Context, req *agentlypb.ListMessagesRequest) (*agentlypb.ListMessagesResponse, error) {
	userID := UserIDFromContext(ctx)

	if req.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "缺少会话 ID")
	}

	// 校验会话归属
	session, err := s.sessionStore.GetByID(req.GetSessionId(), userID)
	if err != nil || session == nil {
		logger.Warning("会话不存在或无权限: %s, 用户: %s", req.GetSessionId(), userID)
		return nil, status.Error(codes.NotFound, "会话不存在或无权限")
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 100
	}
	offset := int(req.GetOffset())
	if offset < 0 {
		offset = 0
	}

	logger.Info("gRPC请求: 查询会话消息 - 会话: %s, 用户: %s", req.GetSessionId(), userID)
	msgs, err := s.messageStore.ListBySession(req.GetSessionId(), userID, limit, offset)
	if err != nil {
		logger.Error("查询消息失败: %v", err)
		return nil, status.Error(codes.Internal, "查询消息失败: "+err.Error())
	}

	items := make([]*agentlypb.StoredMessage, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, toPBMessage(m))
	}
	logger.Info("返回消息列表，共 %d 条消息", len(items))
	return &agentlypb.ListMessagesResponse{Code: 0, Messages: items}, nil
}

// ArchiveSession 归档会话（软删除）
func (s *SessionServer) ArchiveSession(ctx context.Context, req *agentlypb.ArchiveSessionRequest) (*agentlypb.ArchiveSessionResponse, error) {
	userID := UserIDFromContext(ctx)
	if req.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "缺少会话 ID")
	}

	logger.Info("gRPC请求: 归档会话 - 会话: %s, 用户: %s", req.GetSessionId(), userID)
	if err := s.sessionStore.Archive(req.GetSessionId(), userID); err != nil {
		logger.Error("归档会话失败: %v", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	logger.Info("归档会话成功: %s", req.GetSessionId())
	return &agentlypb.ArchiveSessionResponse{Code: 0, Message: "会话已归档"}, nil
}

// Chat 在指定会话中发送消息，服务端流式返回对话事件（对应旧 SSE 流）
func (s *SessionServer) Chat(req *agentlypb.ChatRequest, stream agentlypb.SessionService_ChatServer) error {
	ctx := stream.Context()
	userID := UserIDFromContext(ctx)

	sessionID := req.GetSessionId()
	if sessionID == "" {
		return status.Error(codes.InvalidArgument, "缺少会话 ID")
	}
	if strings.TrimSpace(req.GetMessage()) == "" {
		logger.Warning("聊天消息为空")
		return status.Error(codes.InvalidArgument, "message 不能为空")
	}

	// 校验会话归属
	session, err := s.sessionStore.GetByID(sessionID, userID)
	if err != nil || session == nil {
		logger.Warning("会话不存在或无权限: %s, 用户: %s", sessionID, userID)
		return status.Error(codes.NotFound, "会话不存在或无权限")
	}

	logger.Info("gRPC请求: 聊天消息 - 会话: %s, 用户: %s, 消息: %s", sessionID, userID, req.GetMessage())

	// 自动路由（缺省视为 true）
	autoRoute := true
	if req.AutoRoute != nil {
		autoRoute = req.GetAutoRoute()
	}

	var targetAgent *agent.BasicAgent
	var routeInfo string
	var routedAgentID string

	if autoRoute {
		routedID, reason, rErr := s.manager.Route(ctx, req.GetMessage())
		if rErr != nil {
			targetAgent, _ = s.manager.Current()
			routedAgentID = session.AgentID
			routeInfo = "路由失败，使用会话默认Agent"
		} else {
			targetAgent, _ = s.manager.Get(routedID)
			routedAgentID = routedID
			routeInfo = reason
		}
	} else {
		var exists bool
		targetAgent, exists = s.manager.Get(session.AgentID)
		if !exists {
			logger.Error("Agent不可用: %s", session.AgentID)
			return status.Error(codes.Internal, "Agent 不可用: "+session.AgentID)
		}
		routedAgentID = session.AgentID
		routeInfo = "手动模式"
	}
	logger.Info("路由信息: %s", routeInfo)

	// 若路由结果与会话当前 agent_id 不一致，同步更新会话的主 Agent
	if routedAgentID != "" && routedAgentID != session.AgentID {
		if uErr := s.sessionStore.UpdateAgentID(sessionID, userID, routedAgentID); uErr != nil {
			logger.Warning("同步会话Agent失败: %v", uErr)
		} else {
			logger.Info("会话Agent已同步: %s → %s", session.AgentID, routedAgentID)
			session.AgentID = routedAgentID
		}
	}

	// 发送路由事件
	if sErr := stream.Send(&agentlypb.ChatEvent{
		Type:      "route",
		AgentName: targetAgent.GetName(),
		Reason:    routeInfo,
	}); sErr != nil {
		return sErr
	}

	// 持久化用户消息
	userMsg := types.Message{Role: "user", Content: req.GetMessage()}
	if _, pErr := s.messageStore.SaveMessage(sessionID, userID, routedAgentID, userMsg); pErr != nil {
		logger.Error("持久化用户消息失败: %v", pErr)
	}
	// 若会话标题为空，用首条消息前 50 字作为标题
	if session.Title == "" {
		title := []rune(req.GetMessage())
		if len(title) > 50 {
			title = title[:50]
		}
		_ = s.sessionStore.UpdateTitle(sessionID, userID, string(title))
	}

	startTime := time.Now()

	// 流式处理消息
	responseStream, err := targetAgent.ProcessMessageStream(ctx, req.GetMessage())
	if err != nil {
		logger.Error("处理消息失败: %v", err)
		_ = stream.Send(&agentlypb.ChatEvent{Type: "error", Error: err.Error()})
		return nil
	}

	var fullResponse strings.Builder
	for event := range responseStream {
		switch event.Type {
		case types.StreamEventContent:
			fullResponse.WriteString(event.Content)
			if sErr := stream.Send(&agentlypb.ChatEvent{Type: "message", Content: event.Content}); sErr != nil {
				return sErr
			}
		case types.StreamEventToolCall:
			if sErr := stream.Send(&agentlypb.ChatEvent{Type: "tool_call", ToolName: event.ToolName, ToolArgs: event.ToolArgs}); sErr != nil {
				return sErr
			}
		case types.StreamEventToolResult:
			if sErr := stream.Send(&agentlypb.ChatEvent{Type: "tool_result", ToolName: event.ToolName, ToolResult: event.ToolResult}); sErr != nil {
				return sErr
			}
		case types.StreamEventToolError:
			// 工具单次调用失败：非致命，作为警告事件下发
			errMsg := event.Error
			if errMsg == "" {
				errMsg = event.ToolResult
			}
			if errMsg == "" {
				errMsg = "工具调用失败"
			}
			if sErr := stream.Send(&agentlypb.ChatEvent{Type: "tool_error", ToolName: event.ToolName, Error: errMsg}); sErr != nil {
				return sErr
			}
		case types.StreamEventError:
			errMsg := event.Error
			if errMsg == "" {
				errMsg = "未知错误"
			}
			if sErr := stream.Send(&agentlypb.ChatEvent{Type: "error", Error: errMsg}); sErr != nil {
				return sErr
			}
		}
	}

	processingTime := time.Since(startTime)
	logger.Info("Agent响应: %s", fullResponse.String())
	logger.Info("处理耗时: %v", processingTime)

	// 持久化 AI 回复
	if fullResponse.Len() > 0 {
		aiMsg := types.Message{Role: "assistant", Content: fullResponse.String()}
		if _, pErr := s.messageStore.SaveMessage(sessionID, userID, routedAgentID, aiMsg); pErr != nil {
			logger.Error("持久化AI回复失败: %v", pErr)
		}
	}

	// 结束事件
	return stream.Send(&agentlypb.ChatEvent{Type: "done"})
}
