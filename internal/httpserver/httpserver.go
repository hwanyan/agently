// Package httpserver 提供HTTP API服务功能
package httpserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/auth"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/middleware"
	"github.com/hwanyan/agently/internal/store"
	"github.com/hwanyan/agently/pkg/types"
)

// HTTPService HTTP服务
type HTTPService struct {
	manager      *agent.Manager
	authSvc      *auth.Service
	sessionStore *store.SessionStore
	messageStore *store.MessageStore
	config       *types.Config
}

// NewHTTPService 创建新的HTTP服务
func NewHTTPService(
	manager *agent.Manager,
	authSvc *auth.Service,
	sessionStore *store.SessionStore,
	messageStore *store.MessageStore,
	config *types.Config,
) *HTTPService {
	return &HTTPService{
		manager:      manager,
		authSvc:      authSvc,
		sessionStore: sessionStore,
		messageStore: messageStore,
		config:       config,
	}
}

// Start 启动HTTP服务
func (h *HTTPService) Start() {
	// 确定监听地址
	host := h.config.HTTP.Host
	if host == "" {
		host = "0.0.0.0"
	}
	port := h.config.HTTP.Port
	if port == 0 {
		port = 8088
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	// 注册路由
	mux := http.NewServeMux()

	// dbUnavailable 返回一个数据库不可用时的 503 兜底 Handler
	dbUnavailable := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.jsonError(w, http.StatusServiceUnavailable, "数据库服务不可用，请稍后重试")
	}

	// 认证相关路由（无需鉴权）
	if h.authSvc != nil {
		mux.HandleFunc("/api/auth/register", h.corsMiddleware(h.handleRegister()))
		mux.HandleFunc("/api/auth/login", h.corsMiddleware(h.handleLogin()))
	} else {
		mux.HandleFunc("/api/auth/register", h.corsMiddleware(dbUnavailable))
		mux.HandleFunc("/api/auth/login", h.corsMiddleware(dbUnavailable))
	}

	// 会话相关路由（需要鉴权）
	if h.authSvc != nil && h.sessionStore != nil && h.messageStore != nil {
		mux.HandleFunc("/api/sessions", h.corsMiddleware(middleware.AuthMiddleware(h.authSvc, h.handleSessions())))
		mux.HandleFunc("/api/sessions/", h.corsMiddleware(middleware.AuthMiddleware(h.authSvc, h.handleSessionDetail())))
		mux.HandleFunc("/api/agents", h.corsMiddleware(middleware.AuthMiddleware(h.authSvc, h.handleAgents())))
		mux.HandleFunc("/api/switch", h.corsMiddleware(middleware.AuthMiddleware(h.authSvc, h.handleSwitch())))
	} else {
		mux.HandleFunc("/api/sessions", h.corsMiddleware(dbUnavailable))
		mux.HandleFunc("/api/sessions/", h.corsMiddleware(dbUnavailable))
		mux.HandleFunc("/api/agents", h.corsMiddleware(dbUnavailable))
		mux.HandleFunc("/api/switch", h.corsMiddleware(dbUnavailable))
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // 流式响应需要较长的写超时
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("HTTP服务启动，监听地址: %s", addr)
	fmt.Printf("\n🌐 HTTP 服务已启动: http://%s\n", addr)

	if err := server.ListenAndServe(); err != nil {
		logger.Error("HTTP服务启动失败: %v", err)
		log.Printf("❌ HTTP服务启动失败: %v", err)
	}
}

// corsMiddleware 处理跨域请求的中间件
func (h *HTTPService) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// 处理预检请求
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

// jsonResponse 统一的 JSON 响应辅助函数
func (h *HTTPService) jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// jsonError 统一的 JSON 错误响应辅助函数
func (h *HTTPService) jsonError(w http.ResponseWriter, statusCode int, message string) {
	h.jsonResponse(w, statusCode, map[string]interface{}{
		"code":    statusCode,
		"message": message,
	})
}

// handleAgents 获取可用 Agent 列表
// GET /api/agents
func (h *HTTPService) handleAgents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			logger.Warning("非法请求方法: %s", r.Method)
			h.jsonError(w, http.StatusMethodNotAllowed, "仅支持 GET 请求")
			return
		}

		logger.Info("HTTP请求: 获取Agent列表")
		agents := h.manager.List()

		// 转换为 JSON 友好的结构
		type AgentItem struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			IsActive    bool   `json:"is_active"`
			Registered  bool   `json:"registered"`
		}

		var items []AgentItem
		for _, ag := range agents {
			items = append(items, AgentItem{
				ID:          ag.ID,
				Name:        ag.Name,
				Description: ag.Description,
				IsActive:    ag.IsActive,
				Registered:  ag.Registered,
			})
		}

		logger.Info("返回Agent列表，共 %d 个Agent", len(items))
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"code":   0,
			"agents": items,
		})
	}
}

// handleSwitch 切换当前 Agent
// POST /api/switch
// 请求体: {"agent_id": "general"}
func (h *HTTPService) handleSwitch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			logger.Warning("非法请求方法: %s", r.Method)
			h.jsonError(w, http.StatusMethodNotAllowed, "仅支持 POST 请求")
			return
		}

		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("请求体解析失败: %v", err)
			h.jsonError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
			return
		}
		if req.AgentID == "" {
			logger.Warning("AgentID为空")
			h.jsonError(w, http.StatusBadRequest, "agent_id 不能为空")
			return
		}

		logger.Info("HTTP请求: 切换Agent到 %s", req.AgentID)

		// 不允许切换到系统Agent
		if h.manager.IsSystemAgent(req.AgentID) {
			logger.Warning("尝试切换到系统Agent: %s", req.AgentID)
			h.jsonError(w, http.StatusBadRequest, "不能切换到系统Agent")
			return
		}

		if err := h.manager.Switch(req.AgentID); err != nil {
			logger.Error("切换Agent失败: %v", err)
			h.jsonError(w, http.StatusBadRequest, "切换失败: "+err.Error())
			return
		}

		currentAgent, _ := h.manager.Current()
		logger.Info("切换Agent成功: %s", req.AgentID)
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"code":    0,
			"message": fmt.Sprintf("已切换到: %s", currentAgent.GetName()),
			"agent": map[string]string{
				"id":   req.AgentID,
				"name": currentAgent.GetName(),
			},
		})
	}
}

// handleRegister 用户注册
// POST /api/auth/register
func (h *HTTPService) handleRegister() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			logger.Warning("非法请求方法: %s", r.Method)
			h.jsonError(w, http.StatusMethodNotAllowed, "仅支持 POST 请求")
			return
		}

		var req struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
			Nickname string `json:"nickname"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("注册请求体解析失败: %v", err)
			h.jsonError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
			return
		}

		logger.Info("HTTP请求: 用户注册 - 用户名: %s", req.Username)
		user, token, err := h.authSvc.Register(req.Username, req.Email, req.Password, req.Nickname)
		if err != nil {
			logger.Error("用户注册失败: %v", err)
			h.jsonError(w, http.StatusBadRequest, err.Error())
			return
		}

		logger.Info("用户注册成功: %s", req.Username)
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"code":  0,
			"token": token,
			"user":  user,
		})
	}
}

// handleLogin 用户登录
// POST /api/auth/login
func (h *HTTPService) handleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			logger.Warning("非法请求方法: %s", r.Method)
			h.jsonError(w, http.StatusMethodNotAllowed, "仅支持 POST 请求")
			return
		}

		var req struct {
			Username string `json:"username"` // 支持用户名或邮箱
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("登录请求体解析失败: %v", err)
			h.jsonError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
			return
		}

		logger.Info("HTTP请求: 用户登录 - 用户名: %s", req.Username)
		user, token, err := h.authSvc.Login(req.Username, req.Password)
		if err != nil {
			logger.Error("用户登录失败: %v", err)
			h.jsonError(w, http.StatusUnauthorized, err.Error())
			return
		}

		logger.Info("用户登录成功: %s", req.Username)
		h.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"code":  0,
			"token": token,
			"user":  user,
		})
	}
}

// handleSessions 会话列表 & 创建会话
func (h *HTTPService) handleSessions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)

		switch r.Method {
		case http.MethodGet:
			// 查询会话列表
			agentID := r.URL.Query().Get("agent_id")
			limit := 20
			offset := 0
			if v := r.URL.Query().Get("limit"); v != "" {
				fmt.Sscanf(v, "%d", &limit)
			}
			if v := r.URL.Query().Get("offset"); v != "" {
				fmt.Sscanf(v, "%d", &offset)
			}
			if limit > 100 {
				limit = 100
			}

			logger.Info("HTTP请求: 查询会话列表 - 用户: %s, Agent: %s", userID, agentID)
			sessions, err := h.sessionStore.ListByUser(userID, agentID, limit, offset)
			if err != nil {
				logger.Error("查询会话列表失败: %v", err)
				h.jsonError(w, http.StatusInternalServerError, "查询会话列表失败: "+err.Error())
				return
			}

			logger.Info("返回会话列表，共 %d 个会话", len(sessions))
			h.jsonResponse(w, http.StatusOK, map[string]interface{}{
				"code":     0,
				"sessions": sessions,
			})

		case http.MethodPost:
			// 创建新会话
			var req struct {
				AgentID string `json:"agent_id"`
				Title   string `json:"title"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				logger.Error("创建会话请求体解析失败: %v", err)
				h.jsonError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
				return
			}
			if req.AgentID == "" {
				logger.Warning("创建会话时AgentID为空")
				h.jsonError(w, http.StatusBadRequest, "agent_id 不能为空")
				return
			}
			// 校验 agent_id 是否存在
			if _, exists := h.manager.Get(req.AgentID); !exists {
				logger.Warning("创建会话时Agent不存在: %s", req.AgentID)
				h.jsonError(w, http.StatusBadRequest, "Agent 不存在: "+req.AgentID)
				return
			}

			logger.Info("HTTP请求: 创建新会话 - 用户: %s, Agent: %s", userID, req.AgentID)
			session, err := h.sessionStore.Create(userID, req.AgentID, req.Title)
			if err != nil {
				logger.Error("创建会话失败: %v", err)
				h.jsonError(w, http.StatusInternalServerError, "创建会话失败: "+err.Error())
				return
			}

			logger.Info("创建会话成功: %s", session.ID)
			h.jsonResponse(w, http.StatusOK, map[string]interface{}{
				"code":    0,
				"session": session,
			})

		default:
			logger.Warning("非法请求方法: %s", r.Method)
			h.jsonError(w, http.StatusMethodNotAllowed, "仅支持 GET 和 POST 请求")
		}
	}
}

// handleSessionDetail 会话详情操作
func (h *HTTPService) handleSessionDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.GetUserID(r)

		// 解析路径：/api/sessions/{id} 或 /api/sessions/{id}/messages 或 /api/sessions/{id}/chat
		path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
		parts := strings.SplitN(path, "/", 2)
		sessionID := parts[0]
		subPath := ""
		if len(parts) == 2 {
			subPath = parts[1]
		}

		if sessionID == "" {
			logger.Warning("会话ID为空")
			h.jsonError(w, http.StatusBadRequest, "缺少会话 ID")
			return
		}

		switch {
		case subPath == "messages" && r.Method == http.MethodGet:
			// 查询会话消息历史
			// 先校验会话归属
			session, err := h.sessionStore.GetByID(sessionID, userID)
			if err != nil || session == nil {
				logger.Warning("会话不存在或无权限: %s, 用户: %s", sessionID, userID)
				h.jsonError(w, http.StatusNotFound, "会话不存在或无权限")
				return
			}

			limit := 100
			offset := 0
			if v := r.URL.Query().Get("limit"); v != "" {
				fmt.Sscanf(v, "%d", &limit)
			}
			if v := r.URL.Query().Get("offset"); v != "" {
				fmt.Sscanf(v, "%d", &offset)
			}

			logger.Info("HTTP请求: 查询会话消息 - 会话: %s, 用户: %s", sessionID, userID)
			msgs, err := h.messageStore.ListBySession(sessionID, userID, limit, offset)
			if err != nil {
				logger.Error("查询消息失败: %v", err)
				h.jsonError(w, http.StatusInternalServerError, "查询消息失败: "+err.Error())
				return
			}

			logger.Info("返回消息列表，共 %d 条消息", len(msgs))
			h.jsonResponse(w, http.StatusOK, map[string]interface{}{
				"code":     0,
				"messages": msgs,
			})

		case subPath == "chat" && r.Method == http.MethodPost:
			// 在指定会话中发送消息（SSE 流式）
			session, err := h.sessionStore.GetByID(sessionID, userID)
			if err != nil || session == nil {
				logger.Warning("会话不存在或无权限: %s, 用户: %s", sessionID, userID)
				h.jsonError(w, http.StatusNotFound, "会话不存在或无权限")
				return
			}

			var req struct {
				Message   string `json:"message"`
				AutoRoute *bool  `json:"auto_route"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				logger.Error("聊天请求体解析失败: %v", err)
				h.jsonError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
				return
			}
			if strings.TrimSpace(req.Message) == "" {
				logger.Warning("聊天消息为空")
				h.jsonError(w, http.StatusBadRequest, "message 不能为空")
				return
			}

			logger.Info("HTTP请求: 聊天消息 - 会话: %s, 用户: %s, 消息: %s", sessionID, userID, req.Message)

			// 自动路由
			autoRoute := true
			if req.AutoRoute != nil {
				autoRoute = *req.AutoRoute
			}

			ctx := r.Context()
			var targetAgent *agent.BasicAgent
			var routeInfo string
			var routedAgentID string

			if autoRoute {
				routedID, reason, err := h.manager.Route(ctx, req.Message)
				if err != nil {
					targetAgent, _ = h.manager.Current()
					routedAgentID = session.AgentID
					routeInfo = "路由失败，使用会话默认Agent"
				} else {
					targetAgent, _ = h.manager.Get(routedID)
					routedAgentID = routedID
					routeInfo = reason
				}
			} else {
				var exists bool
				targetAgent, exists = h.manager.Get(session.AgentID)
				if !exists {
					logger.Error("Agent不可用: %s", session.AgentID)
					h.jsonError(w, http.StatusInternalServerError, "Agent 不可用: "+session.AgentID)
					return
				}
				routedAgentID = session.AgentID
				routeInfo = "手动模式"
			}

			logger.Info("路由信息: %s", routeInfo)

			// 设置 SSE 响应头
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")

			flusher, ok := w.(http.Flusher)
			if !ok {
				logger.Error("服务器不支持流式响应")
				h.jsonError(w, http.StatusInternalServerError, "服务器不支持流式响应")
				return
			}

			// 发送路由信息
			routeData, _ := json.Marshal(map[string]string{
				"agent_name": targetAgent.GetName(),
				"reason":     routeInfo,
			})
			fmt.Fprintf(w, "event: route\ndata: %s\n\n", string(routeData))
			flusher.Flush()

			// 持久化用户消息
			userMsg := types.Message{
				Role:    "user",
				Content: req.Message,
			}
			if _, err := h.messageStore.SaveMessage(sessionID, userID, routedAgentID, userMsg); err != nil {
				logger.Error("持久化用户消息失败: %v", err)
			}
			// 如果会话标题为空，用首条消息的前 50 字作为标题
			if session.Title == "" {
				title := []rune(req.Message)
				if len(title) > 50 {
					title = title[:50]
				}
				_ = h.sessionStore.UpdateTitle(sessionID, userID, string(title))
			}

			// 记录开始处理时间
			startTime := time.Now()

			// 流式处理消息
			responseStream, err := targetAgent.ProcessMessageStream(ctx, req.Message)
			if err != nil {
				logger.Error("处理消息失败: %v", err)
				errData, _ := json.Marshal(map[string]string{"error": err.Error()})
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(errData))
				flusher.Flush()
				return
			}

			// 收集完整响应并流式输出
			var fullResponse strings.Builder
			for event := range responseStream {
				switch event.Type {
				case types.StreamEventContent:
					fullResponse.WriteString(event.Content)
					chunkData, _ := json.Marshal(map[string]string{"content": event.Content})
					fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(chunkData))
					flusher.Flush()
				case types.StreamEventToolCall:
					toolData, _ := json.Marshal(map[string]string{"tool_name": event.ToolName, "tool_args": event.ToolArgs})
					fmt.Fprintf(w, "event: tool_call\ndata: %s\n\n", string(toolData))
					flusher.Flush()
				case types.StreamEventToolResult:
					resultData, _ := json.Marshal(map[string]string{"tool_name": event.ToolName, "tool_result": event.ToolResult})
					fmt.Fprintf(w, "event: tool_result\ndata: %s\n\n", string(resultData))
					flusher.Flush()
				case types.StreamEventError:
					errData, _ := json.Marshal(map[string]string{"error": event.Error})
					fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(errData))
					flusher.Flush()
				}
			}

			// 记录处理结果
			processingTime := time.Since(startTime)
			logger.Info("Agent响应: %s", fullResponse.String())
			logger.Info("处理耗时: %v", processingTime)

			// 持久化 AI 回复
			if fullResponse.Len() > 0 {
				aiMsg := types.Message{
					Role:    "assistant",
					Content: fullResponse.String(),
				}
				if _, err := h.messageStore.SaveMessage(sessionID, userID, routedAgentID, aiMsg); err != nil {
					logger.Error("持久化AI回复失败: %v", err)
				}
			}

			fmt.Fprintf(w, "event: done\ndata: {}\n\n")
			flusher.Flush()

		case subPath == "" && r.Method == http.MethodDelete:
			// 归档会话
			logger.Info("HTTP请求: 归档会话 - 会话: %s, 用户: %s", sessionID, userID)
			if err := h.sessionStore.Archive(sessionID, userID); err != nil {
				logger.Error("归档会话失败: %v", err)
				h.jsonError(w, http.StatusBadRequest, err.Error())
				return
			}
			logger.Info("归档会话成功: %s", sessionID)
			h.jsonResponse(w, http.StatusOK, map[string]interface{}{
				"code":    0,
				"message": "会话已归档",
			})

		default:
			logger.Warning("接口不存在: %s", r.URL.Path)
			h.jsonError(w, http.StatusNotFound, "接口不存在")
		}
	}
}
