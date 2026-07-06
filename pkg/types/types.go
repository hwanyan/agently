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

// StreamEventType 流式事件类型
type StreamEventType string

const (
	// StreamEventContent 文本内容事件
	StreamEventContent StreamEventType = "content"
	// StreamEventToolCall 工具调用开始事件
	StreamEventToolCall StreamEventType = "tool_call"
	// StreamEventToolResult 工具调用结果事件
	StreamEventToolResult StreamEventType = "tool_result"
	// StreamEventToolError 工具调用失败事件（非致命，LLM 仍可继续处理）
	StreamEventToolError StreamEventType = "tool_error"
	// StreamEventError 致命错误事件（整次对话无法继续）
	StreamEventError StreamEventType = "error"
)

// StreamEvent 流式响应事件
// 通过事件驱动的方式向调用方传递流式处理过程中的各类信息
type StreamEvent struct {
	Type       StreamEventType // 事件类型
	Content    string          // 文本内容（Type=content 时有值）
	ToolName   string          // 工具名称（Type=tool_call/tool_result 时有值）
	ToolArgs   string          // 工具参数（Type=tool_call 时有值，JSON格式）
	ToolResult string          // 工具执行结果（Type=tool_result 时有值）
	Error      string          // 错误信息（Type=error 时有值）
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

	// ProcessMessageStream 流式处理用户消息，返回事件流
	ProcessMessageStream(ctx context.Context, userMessage string) (<-chan StreamEvent, error)

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

// MCPServerConfig MCP服务器配置
//
// 设计说明：
//   - ID    : 稳定标识，用于在 prompt.yaml 的 tools.mcp_servers 中引用 Agent 与 MCP 服务的绑定关系
//   - Name  : 仅用于日志/前端展示的人类可读名称，可随意修改，不影响绑定
type MCPServerConfig struct {
	ID      string `yaml:"id"`       // MCP服务唯一稳定标识（用于 Agent 引用绑定）
	Name    string `yaml:"name"`     // MCP服务展示名称（人类可读，仅用于日志/UI）
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
	APIKey  string `yaml:"api_key"`  // 聚合数据API密钥
	BaseURL string `yaml:"base_url"` // 聚合数据API接口地址
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

// HTTPConfig HTTP服务配置（grpc-gateway 对外暴露的 REST/SSE 网关）
type HTTPConfig struct {
	Port int    `yaml:"port"` // HTTP服务端口
	Host string `yaml:"host"` // HTTP服务监听地址
}

// GRPCConfig gRPC 服务配置
//
// gRPC 是系统的核心服务层，HTTP 网关（grpc-gateway）会在本机反向代理到该端口。
type GRPCConfig struct {
	Port int    `yaml:"port"` // gRPC 服务端口（默认 HTTP.Port + 1）
	Host string `yaml:"host"` // 仅监听回环
}

// LogConfig 日志配置
type LogConfig struct {
	FilePath string `yaml:"file_path"` // 日志文件路径
}

// Config 应用程序总配置
type Config struct {
	Log      LogConfig      `yaml:"log"`      // 日志配置
	HTTP     HTTPConfig     `yaml:"http"`     // HTTP服务配置（grpc-gateway 网关）
	GRPC     GRPCConfig     `yaml:"grpc"`     // gRPC服务配置
	Database DatabaseConfig `yaml:"database"` // 数据库配置
	JWT      JWTConfig      `yaml:"jwt"`      // JWT 配置
	SMTP     SMTPConfig     `yaml:"smtp"`     // SMTP 邮件配置
	LLM      LLMConfig      `yaml:"llm"`      // LLM配置
	Tools    ToolsConfig    `yaml:"tools"`    // 工具配置
	MCP      MCPConfig      `yaml:"mcp"`      // MCP工具配置
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

// ============================================================
// 数据库配置
// ============================================================

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	DSN             string `yaml:"dsn"`               // PostgreSQL 连接字符串，如 postgres://user:pass@host:5432/dbname?sslmode=disable
	MaxOpenConns    int    `yaml:"max_open_conns"`    // 最大打开连接数，默认 20
	MaxIdleConns    int    `yaml:"max_idle_conns"`    // 最大空闲连接数，默认 5
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"` // 连接最大存活时间（秒），默认 300
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret     string `yaml:"secret"`      // JWT 签名密钥，生产环境必须设置强随机值
	ExpireHour int    `yaml:"expire_hour"` // Token 有效期（小时），默认 72
}

// SMTPConfig 邮件发送配置
type SMTPConfig struct {
	Host     string `yaml:"host"`     // SMTP 服务器地址，如 smtp.qq.com
	Port     int    `yaml:"port"`     // SMTP 端口，如 587（TLS）或 465（SSL）
	Username string `yaml:"username"` // SMTP 登录用户名（通常是邮箱地址）
	Password string `yaml:"password"` // SMTP 登录密码或授权码
	From     string `yaml:"from"`     // 发件人地址（可选，默认使用 Username）
}

// ============================================================
// 用户与会话领域模型
// ============================================================

// User 用户模型（对应 users 表）
type User struct {
	ID        string     `json:"id"`         // 用户ID
	Username  string     `json:"username"`   // 用户名
	Email     string     `json:"email"`      // 邮箱
	Password  string     `json:"-"`          // 不序列化到 JSON
	Nickname  string     `json:"nickname"`   // 昵称（非必须）
	AvatarURL string     `json:"avatar_url"` // 头像 URL（可选）
	Status    int        `json:"status"`     // 1=正常, 0=禁用
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastLogin *time.Time `json:"last_login"`
}

// Session 会话模型（对应 sessions 表）
type Session struct {
	ID         string     `json:"id"`          // 会话ID
	UserID     string     `json:"user_id"`     // 用户ID
	AgentID    string     `json:"agent_id"`    // agent ID（agent 的唯一标识，对应 agent 的 Name）
	Title      string     `json:"title"`       // 会话标题（首条消息自动截取或用户手动设置）
	Summary    string     `json:"summary"`     // 会话摘要（用于 RAG 检索，可由 LLM 自动生成）
	Status     int        `json:"status"`      // 1=活跃, 0=已归档
	MessageCnt int        `json:"message_cnt"` // 消息数量（冗余字段，避免 COUNT 查询）
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ArchivedAt *time.Time `json:"archived_at"` // 归档时间
}

// StoredMessage 持久化消息模型（对应 messages 表）
// 与内存中的 Message 区分，包含数据库特有字段
type StoredMessage struct {
	ID           string    `json:"id"`             // 消息ID
	SessionID    string    `json:"session_id"`     // 会话ID
	UserID       string    `json:"user_id"`        // 用户ID
	Role         string    `json:"role"`           // 消息角色: user / assistant / tool / system
	Content      string    `json:"content"`        // 消息文本内容
	ToolCallID   string    `json:"tool_call_id"`   // 工具调用ID（role=tool 时有值）
	ToolCallList string    `json:"tool_call_list"` // 工具调用列表（role=assistant 且有工具调用时有值），这是 JSONB 序列化后的字符串
	AgentID      string    `json:"agent_id"`       // 处理此消息的 Agent ID
	TokenCount   int       `json:"token_count"`    // 本条消息消耗的 token 数（估算）
	Seq          int       `json:"seq"`            // 消息在会话中的顺序号（从 1 开始）
	CreatedAt    time.Time `json:"created_at"`
}
