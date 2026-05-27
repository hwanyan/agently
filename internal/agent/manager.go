// Package agent 提供AI Agent的核心实现
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/pkg/types"
	"strings"
	"sync"
)

// Manager Agent管理器
// 负责管理多个Agent实例，提供Agent的注册、选择和路由功能
// 通过 AgentItemConfig.Role 字段自动区分系统Agent和业务Agent：
//   - system: 系统级Agent（如意图路由），启动时必须成功注册，否则服务不启动
//   - business: 业务级Agent（如通用助手、黄金分析），允许注册失败（如依赖服务不可用）
type Manager struct {
	agents       map[string]*BasicAgent  // Agent实例映射表（key为Agent ID）
	agentConfigs []types.AgentItemConfig // Agent配置列表
	currentID    string                  // 当前激活的Agent ID（默认为第一个业务Agent）
	routerID     string                  // 路由Agent的ID（自动从 role=system 的Agent中识别）
	mutex        sync.RWMutex            // 读写锁
}

// NewManager 创建Agent管理器
func NewManager(configs []types.AgentItemConfig) *Manager {
	return &Manager{
		agents:       make(map[string]*BasicAgent),
		agentConfigs: configs,
	}
}

// Register 注册一个Agent实例
// 根据配置中的 Role 字段自动处理：
//   - system 角色的Agent自动设置为路由器
//   - business 角色的第一个Agent自动设置为默认激活Agent
//
// 参数：
//   - id: Agent唯一标识
//   - agentInstance: Agent实例
func (m *Manager) Register(id string, agentRole types.AgentRole, agentInstance *BasicAgent) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.agents[id] = agentInstance
	logger.Info("Agent [%s] 已注册到管理器", id)

	if agentRole == types.AgentRoleSystem {
		m.routerID = id
		logger.Info("系统Agent [%s] 已自动设置为路由器", id)
	} else if agentRole == types.AgentRoleBusiness && m.currentID == "" {
		m.currentID = id
		logger.Info("业务Agent [%s] 已设为默认激活Agent", id)
	}
}

// ValidateSystemAgents 校验所有系统级Agent是否已成功注册
// 此方法应在全部Agent注册完成后调用，如果有系统Agent未注册则返回错误
// 返回：
//   - error: 如果有系统Agent未注册则返回错误，否则返回nil
func (m *Manager) ValidateSystemAgents() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, cfg := range m.agentConfigs {
		if cfg.Role == types.AgentRoleSystem {
			if _, exists := m.agents[cfg.ID]; !exists {
				return fmt.Errorf("系统Agent [%s] (%s) 未成功注册，服务无法启动", cfg.ID, cfg.Name)
			}
		}
	}
	return nil
}

// Switch 切换当前激活的Agent
// 参数：
//   - id: 要切换到的Agent ID
//
// 返回：
//   - error: 如果Agent不存在则返回错误
func (m *Manager) Switch(id string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.agents[id]; !exists {
		return fmt.Errorf("Agent '%s' 不存在", id)
	}

	m.currentID = id
	logger.Info("已切换到 Agent [%s]", id)
	return nil
}

// Current 获取当前激活的Agent
// 返回：
//   - *BasicAgent: 当前Agent实例
//   - error: 如果没有激活的Agent则返回错误
func (m *Manager) Current() (*BasicAgent, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.currentID == "" {
		return nil, fmt.Errorf("没有激活的Agent")
	}

	agentInstance, exists := m.agents[m.currentID]
	if !exists {
		return nil, fmt.Errorf("当前Agent '%s' 不存在", m.currentID)
	}

	return agentInstance, nil
}

// CurrentID 获取当前激活的Agent ID
func (m *Manager) CurrentID() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.currentID
}

// Get 根据ID获取Agent实例
func (m *Manager) Get(id string) (*BasicAgent, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	agentInstance, exists := m.agents[id]
	return agentInstance, exists
}

// IsSystemAgent 判断指定ID的Agent是否为系统Agent
func (m *Manager) IsSystemAgent(id string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, cfg := range m.agentConfigs {
		if cfg.ID == id {
			return cfg.Role == types.AgentRoleSystem
		}
	}
	return false
}

// RouteResult 路由结果
type RouteResult struct {
	AgentID string `json:"agent_id"` // 目标Agent ID
	Reason  string `json:"reason"`   // 路由原因
}

// Route 使用路由Agent分析用户意图并返回目标Agent ID
// 参数：
//   - ctx: 上下文
//   - userMessage: 用户输入
//
// 返回：
//   - string: 目标Agent ID
//   - string: 路由原因
//   - error: 可能的错误
func (m *Manager) Route(ctx context.Context, userMessage string) (string, string, error) {
	m.mutex.RLock()
	routerID := m.routerID
	m.mutex.RUnlock()

	if routerID == "" {
		// 没有路由Agent，使用当前Agent
		return m.CurrentID(), "无路由Agent，使用当前Agent", nil
	}

	router, exists := m.Get(routerID)
	if !exists {
		return m.CurrentID(), "路由Agent未注册，使用当前Agent", nil
	}

	// 调用路由Agent进行意图分析（非流式，快速返回）
	response, err := router.ProcessMessage(ctx, userMessage)
	if err != nil {
		logger.Error("路由Agent调用失败: %v，回退到当前Agent", err)
		return m.CurrentID(), "路由失败，使用当前Agent", nil
	}

	// 解析路由结果JSON
	result, err := parseRouteResult(response)
	if err != nil {
		logger.Warning("路由结果解析失败: %v，原始响应: %s，回退到当前Agent", err, response)
		return m.CurrentID(), "路由结果解析失败，使用当前Agent", nil
	}

	// 验证目标Agent是否存在
	if _, exists := m.Get(result.AgentID); !exists {
		logger.Warning("路由目标Agent [%s] 不存在，回退到当前Agent", result.AgentID)
		return m.CurrentID(), fmt.Sprintf("目标Agent [%s] 不存在", result.AgentID), nil
	}

	logger.Info("路由结果: 用户意图 → Agent [%s], 原因: %s", result.AgentID, result.Reason)
	return result.AgentID, result.Reason, nil
}

// parseRouteResult 解析路由Agent的JSON响应
func parseRouteResult(response string) (*RouteResult, error) {
	// 清理响应（去除可能的markdown代码块标记）
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result RouteResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w, 原始内容: %s", err, response)
	}

	if result.AgentID == "" {
		return nil, fmt.Errorf("路由结果中 agent_id 为空")
	}

	return &result, nil
}

// List 获取所有已注册的业务Agent信息（不包含系统Agent）
// 返回：
//   - []AgentInfo: Agent信息列表
func (m *Manager) List() []AgentInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var list []AgentInfo
	for _, cfg := range m.agentConfigs {
		// 跳过系统Agent，不在列表中展示
		if cfg.Role == types.AgentRoleSystem {
			continue
		}
		_, registered := m.agents[cfg.ID]
		list = append(list, AgentInfo{
			ID:          cfg.ID,
			Name:        cfg.Name,
			Description: cfg.Description,
			IsActive:    cfg.ID == m.currentID,
			Registered:  registered,
		})
	}
	return list
}

// ListAll 获取所有已注册的Agent信息（包含系统Agent）
// 返回：
//   - []AgentInfo: Agent信息列表
func (m *Manager) ListAll() []AgentInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var list []AgentInfo
	for _, cfg := range m.agentConfigs {
		_, registered := m.agents[cfg.ID]
		list = append(list, AgentInfo{
			ID:          cfg.ID,
			Name:        cfg.Name,
			Description: cfg.Description,
			IsActive:    cfg.ID == m.currentID,
			Registered:  registered,
			IsSystem:    cfg.Role == types.AgentRoleSystem,
		})
	}
	return list
}

// AgentInfo Agent信息（用于展示）
type AgentInfo struct {
	ID          string // Agent唯一标识
	Name        string // Agent显示名称
	Description string // Agent描述
	IsActive    bool   // 是否为当前激活的Agent
	Registered  bool   // 是否已注册
	IsSystem    bool   // 是否为系统Agent
}
