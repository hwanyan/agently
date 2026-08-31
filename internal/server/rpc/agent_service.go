package rpc

import (
	"context"
	"fmt"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/logger"
	agentlypb "github.com/hwanyan/agently/pb/gen"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AgentServer Agent 管理服务的 gRPC 实现（列表 / 工具 / 切换）
type AgentServer struct {
	agentlypb.UnimplementedAgentServiceServer
	manager *agent.Manager
}

// NewAgentServer 创建 Agent 管理服务实现
func NewAgentServer(manager *agent.Manager) *AgentServer {
	return &AgentServer{manager: manager}
}

// ListAgents 获取可用 Agent 列表（不含系统 Agent）
func (s *AgentServer) ListAgents(ctx context.Context, _ *agentlypb.ListAgentsRequest) (*agentlypb.ListAgentsResponse, error) {
	logger.Info("gRPC请求: 获取Agent列表")
	agents := s.manager.List()

	items := make([]*agentlypb.AgentItem, 0, len(agents))
	for _, ag := range agents {
		items = append(items, &agentlypb.AgentItem{
			Id:          ag.ID,
			Name:        ag.Name,
			Description: ag.Description,
			IsActive:    ag.IsActive,
			Registered:  ag.Registered,
		})
	}
	logger.Info("返回Agent列表，共 %d 个Agent", len(items))
	return &agentlypb.ListAgentsResponse{Code: 0, Agents: items}, nil
}

// ListTools 汇总所有非系统 Agent 的工具（按工具名去重）
func (s *AgentServer) ListTools(ctx context.Context, _ *agentlypb.ListToolsRequest) (*agentlypb.ListToolsResponse, error) {
	logger.Info("gRPC请求: 获取工具列表")

	toolMap := make(map[string]*agentlypb.ToolItem)
	for _, agentInfo := range s.manager.List() { // List() 已过滤系统 Agent
		agentInstance, exists := s.manager.Get(agentInfo.ID)
		if !exists {
			continue
		}
		for _, tool := range agentInstance.GetTools() {
			name := tool.Name()
			if item, ok := toolMap[name]; ok {
				item.AgentIds = append(item.AgentIds, agentInfo.ID)
			} else {
				toolMap[name] = &agentlypb.ToolItem{
					Name:        name,
					Description: tool.Description(),
					AgentIds:    []string{agentInfo.ID},
				}
			}
		}
	}

	items := make([]*agentlypb.ToolItem, 0, len(toolMap))
	for _, item := range toolMap {
		items = append(items, item)
	}
	logger.Info("返回工具列表，共 %d 个工具", len(items))
	return &agentlypb.ListToolsResponse{Code: 0, Tools: items}, nil
}

// SwitchAgent 切换当前激活的 Agent
func (s *AgentServer) SwitchAgent(ctx context.Context, req *agentlypb.SwitchAgentRequest) (*agentlypb.SwitchAgentResponse, error) {
	if req.GetAgentId() == "" {
		logger.Warning("AgentID为空")
		return nil, status.Error(codes.InvalidArgument, "agent_id 不能为空")
	}
	logger.Info("gRPC请求: 切换Agent到 %s", req.GetAgentId())

	if s.manager.IsSystemAgent(req.GetAgentId()) {
		logger.Warning("尝试切换到系统Agent: %s", req.GetAgentId())
		return nil, status.Error(codes.InvalidArgument, "不能切换到系统Agent")
	}

	if err := s.manager.Switch(req.GetAgentId()); err != nil {
		logger.Error("切换Agent失败: %v", err)
		return nil, status.Error(codes.InvalidArgument, "切换失败: "+err.Error())
	}

	currentAgent, _ := s.manager.Current()
	logger.Info("切换Agent成功: %s", req.GetAgentId())
	return &agentlypb.SwitchAgentResponse{
		Code:    0,
		Message: fmt.Sprintf("已切换到: %s", currentAgent.GetName()),
		Agent: &agentlypb.AgentBrief{
			Id:   req.GetAgentId(),
			Name: currentAgent.GetName(),
		},
	}, nil
}
