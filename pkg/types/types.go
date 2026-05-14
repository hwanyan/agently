// Package types 定义AI Agent系统中使用的基础数据类型
package types

import (
	"context"
	"time"
)

// Message 表示对话中的一条消息
// 这是Agent系统中消息传递的基本单位
type Message struct {
	ID           string     `json:"id"`             // 消息唯一标识
	Role         string     `json:"role"`           // 消息角色：system, user, assistant, tool
	Content      string     `json:"content"`        // 消息内容
	ToolCallList []ToolCall `json:"tool_call_list"` // 工具调用列表（表明当前 Message 是请求调用工具）
	ToolCallID   string     `json:"tool_call_id"`   // 工具调用ID（当这个值不为空时，Content 存放的是此次调用工具得到的结果）
	Timestamp    time.Time  `json:"timestamp"`      // 消息时间戳
}

// ToolCall 表示一个工具调用，包含了这次调用工具的详细信息
// 当LLM决定使用工具时，会生成这样的结构
type ToolCall struct {
	ID       string       `json:"id"`       // 工具调用的唯一标识
	Type     string       `json:"type"`     // 调用类型，通常是"function"
	Function FunctionCall `json:"function"` // 函数调用详情
}

// FunctionCall 表示函数调用的详细信息
type FunctionCall struct {
	Name        string `json:"name"`        // 函数名称
	Description string `json:"description"` // 函数描述
	Arguments   string `json:"arguments"`   // 函数参数（JSON格式字符串）
}

// Tool 接口定义了工具必须实现的方法
// 所有的工具都需要实现这个接口
type Tool interface {
	// Name 返回工具的名称
	Name() string

	// Description 返回工具的描述
	Description() string

	// Parameters 返回工具的参数定义（JSON Schema格式）
	Parameters() map[string]interface{}

	// Execute 执行工具并返回结果
	// ctx: 上下文
	// args: 工具参数
	// 返回: 执行结果和可能的错误
	Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

// StreamChunk 流式响应的数据块
// 每个 chunk 可能包含文本内容或工具调用片段
type StreamChunk struct {
	Content      string     // 文本内容片段
	ToolCallList []ToolCall // 工具调用列表（流式结束时汇总）
	Done         bool       // 是否是最后一个 chunk（汇总工具调用信息）
}

// LLMClient 接口定义了与大语言模型交互的方法
// 不同的 LLM 提供商需要实现这个接口
type LLMClient interface {
	// Chat 发送消息并获取响应
	// ctx: 上下文
	// messages: 对话历史
	// tools: 可用工具列表
	// 返回: LLM响应和可能的错误
	Chat(ctx context.Context, messages []Message, tools []Tool) (*Message, error)

	// ChatStream 流式发送消息并获取响应
	// ctx: 上下文
	// messages: 对话历史
	// tools: 可用工具列表
	// 返回: 流式响应通道和可能的错误
	ChatStream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamChunk, error)
}

// Agent 接口定义了AI Agent必须实现的方法
type Agent interface {
	// ProcessMessage 处理用户消息并返回响应
	ProcessMessage(ctx context.Context, userMessage string) (string, error)

	// ProcessMessageStream 流式处理用户消息
	ProcessMessageStream(ctx context.Context, userMessage string) (<-chan string, error)

	// AddTool 添加工具
	AddTool(tool Tool)

	// GetConversationHistory 获取对话历史
	GetConversationHistory() []Message

	// ClearHistory 清空对话历史
	ClearHistory()

	// GetName 获取Agent名称
	GetName() string

	// GetDescription 获取Agent描述
	GetDescription() string
}

// LLMConfig LLM配置
type LLMConfig struct {
	Provider string `yaml:"provider"` // LLM提供商（如openai、deepseek、claude等）
	APIKey   string `yaml:"api_key"`  // API密钥
	Model    string `yaml:"model"`    // 模型名称
	BaseURL  string `yaml:"base_url"` // API基础URL（可选）
}

// AgentRole Agent角色类型
// 用于区分系统级Agent和业务级Agent，不同角色在启动时有不同的容错策略
type AgentRole string

const (
	// AgentRoleSystem 系统Agent（如意图路由），启动时必须成功注册，否则服务不启动
	AgentRoleSystem AgentRole = "system"
	// AgentRoleBusiness 业务Agent（如通用助手、黄金分析），启动时允许注册失败
	AgentRoleBusiness AgentRole = "business"
)

// AgentItemConfig 多Agent配置中的单个Agent配置项
type AgentItemConfig struct {
	ID          string    `yaml:"id"`           // Agent唯一标识（如 general, gold_analysis）
	Name        string    `yaml:"name"`         // Agent显示名称
	Description string    `yaml:"description"`  // Agent描述
	PromptScene string    `yaml:"prompt_scene"` // 对应的 Prompt 场景名
	ToolGroup   string    `yaml:"tool_group"`   // 工具组标识（如 general, mcp_gold, none）
	Role        AgentRole `yaml:"role"`         // Agent角色：system（系统级）或 business（业务级）
}

// MCPServerConfig MCP服务器配置
type MCPServerConfig struct {
	Name    string `yaml:"name"`     // MCP服务名称
	BaseURL string `yaml:"base_url"` // MCP服务地址
	APIKey  string `yaml:"api_key"`  // MCP服务API密钥（可选）
	Timeout int    `yaml:"timeout"`  // 请求超时时间（秒）
}

// MCPConfig MCP工具配置
type MCPConfig struct {
	Servers []MCPServerConfig `yaml:"servers"` // MCP服务器列表
}

// WeatherConfig 天气工具配置
type WeatherConfig struct {
	APIKey  string `yaml:"api_key"`  // 和风天气API密钥
	BaseURL string `yaml:"base_url"` // 和风天气API基础URL
}

// IPToolConfig IP工具配置
type IPToolConfig struct {
	AccessKey string `yaml:"access_key"` // IP地址查询 access key
}

// TranslateConfig 翻译工具配置
type TranslateConfig struct {
	APIKey  string `yaml:"api_key"`  // 翻译API密钥
	BaseURL string `yaml:"base_url"` // 翻译API基础URL
}

// ShortURLConfig 短链接工具配置
type ShortURLConfig struct {
	APIKey  string `yaml:"api_key"`  // 短链接API密钥（可选）
	BaseURL string `yaml:"base_url"` // 短链接API基础URL
}

// ToolsConfig 工具配置
type ToolsConfig struct {
	Weather   WeatherConfig   `yaml:"weather"`   // 天气工具配置
	IP        IPToolConfig    `yaml:"ip"`        // IP工具配置
	Translate TranslateConfig `yaml:"translate"` // 翻译工具配置
	ShortURL  ShortURLConfig  `yaml:"short_url"` // 短链接工具配置
}

// LogConfig 日志配置
type LogConfig struct {
	FilePath string `yaml:"file_path"` // 日志文件路径
}

// PromptDirConfig Prompt目录配置
type PromptDirConfig struct {
	Dir string `yaml:"dir"` // Prompt文件所在目录
}

// Config 应用程序总配置
type Config struct {
	Log    LogConfig         `yaml:"log"`    // 日志配置
	Prompt PromptDirConfig   `yaml:"prompt"` // Prompt目录配置
	LLM    LLMConfig         `yaml:"llm"`    // LLM配置
	Agents []AgentItemConfig `yaml:"agents"` // 多Agent配置列表
	Tools  ToolsConfig       `yaml:"tools"`  // 工具配置
	MCP    MCPConfig         `yaml:"mcp"`    // MCP工具配置
}

// ToolParameter 工具参数定义
// 用于定义工具参数的JSON Schema
type ToolParameter struct {
	Type        string                 `json:"type"`        // 参数类型
	Description string                 `json:"description"` // 参数描述
	Required    bool                   `json:"required"`    // 是否必需
	Properties  map[string]interface{} `json:"properties"`  // 子属性（对于object类型）
	Items       *ToolParameter         `json:"items"`       // 数组项类型（对于array类型）
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`    // 错误代码
	Message string `json:"message"` // 错误消息
	Details string `json:"details"` // 错误详情
}

// Error 实现error接口
func (e *ErrorResponse) Error() string {
	return e.Message
}

// ConversationContext 对话上下文
// 用于在对话过程中传递额外信息
type ConversationContext struct {
	UserID     string                 `json:"user_id"`     // 用户ID
	SessionID  string                 `json:"session_id"`  // 会话ID
	Metadata   map[string]interface{} `json:"metadata"`    // 元数据
	StartTime  time.Time              `json:"start_time"`  // 对话开始时间
	LastActive time.Time              `json:"last_active"` // 最后活跃时间
}

// ToolExecutionResult 工具执行结果
type ToolExecutionResult struct {
	ToolName  string        `json:"tool_name"` // 工具名称
	Success   bool          `json:"success"`   // 执行是否成功
	Result    string        `json:"result"`    // 执行结果
	Error     string        `json:"error"`     // 错误信息（如果有）
	Duration  time.Duration `json:"duration"`  // 执行耗时
	Timestamp time.Time     `json:"timestamp"` // 执行时间
}

// AgentMetrics Agent性能指标
type AgentMetrics struct {
	TotalMessages   int           `json:"total_messages"`   // 总消息数
	TotalTokens     int           `json:"total_tokens"`     // 总token数
	AverageResponse time.Duration `json:"average_response"` // 平均响应时间
	ToolCallCount   int           `json:"tool_call_count"`  // 工具调用次数
	ErrorCount      int           `json:"error_count"`      // 错误次数
	LastResetTime   time.Time     `json:"last_reset_time"`  // 上次重置时间
}
