# Agent-Hwan

一个基于 Go 实现的多 Agent AI 系统，支持**意图自动路由**、流式对话、自动工具调用、MCP 协议集成、日志系统和灵活的 YAML 配置管理。

## ✨ 核心特性

- 🤖 **多Agent架构** — 启动时注册全部Agent，支持按需切换
- 🔀 **意图自动路由** — 内置路由Agent（intention_router），自动分析用户意图并分发到合适的专业Agent
- 🔧 **自动工具调用** — Agent 自动识别意图并调用外部工具完成任务
- 🌊 **流式响应** — 支持流式输出，实时展示 AI 回答，流式模式下同样支持工具调用
- 🔌 **MCP协议支持** — 通过 Model Context Protocol 动态发现和调用远程工具服务
- 📝 **日志系统** — 统一的日志管理（INFO/ERROR/WARNING → 文件）
- ⚙️ **YAML 配置** — 统一使用 `config.yaml` 管理所有配置项
- 🔒 **并发安全** — 使用互斥锁 + channel 机制保护对话历史的并发访问
- 💬 **对话管理** — 每个Agent拥有独立的对话历史和上下文维护
- 🔑 **用户认证** — JWT 鉴权，支持注册/登录，所有业务接口均需登录态
- 🗄️ **会话持久化** — 对话历史存储到 PostgreSQL，刷新页面后可完整恢复
- 🌐 **HTTP API** — 提供完整的 RESTful + SSE 接口，支持前端接入

## 🤖 内置Agent

| # | Agent ID | 名称 | 类型 | 功能 |
|---|----------|------|------|------|
| 1 | `intention_router` | 意图路由助手 | 系统Agent | 自动分析用户意图，路由到合适的专业Agent |
| 2 | `general` | 通用工具助手 | 业务Agent | 天气、翻译、IP查询、二维码等通用服务 |
| 3 | `gold_analysis` | 黄金分析助手 | 业务Agent | 专业黄金市场分析、行情查询（MCP工具） |

## 📦 内置工具

### 通用工具（general Agent）

| # | 工具名 | 功能 | 依赖服务 |
|---|--------|------|----------|
| 1 | `qweather` ☁️ | 查询城市实时天气（温度、湿度、风力等） | 和风天气 API |
| 2 | `ipapi` 🌍 | 查询 IP/域名的地理位置与网络信息 | ipapi.com |
| 3 | `qrcode` 🔢 | 生成二维码（dataurl / ascii / file） | 本地生成 |
| 4 | `translate` 🔡 | 多语言文本翻译 | LibreTranslate |
| 5 | `textprocessor` ✏️ | 文本统计、格式转换、内容提取、文本生成 | 本地处理 |
| 6 | `shorturl` 🔗 | 长 URL 转短链接（TinyURL / is.gd / v.gd） | 第三方短链接服务 |

### MCP工具（gold_analysis Agent）

通过 MCP 协议从远程服务器动态发现，具体工具取决于 MCP 服务端提供的能力。

## 🏗️ 项目结构

```
agent-hwan/
├── cmd/
│   ├── config/
│   │   └── config.yaml                  # 统一配置文件（YAML）
│   └── server/
│       └── main.go                       # 程序入口（Agent注册、CLI交互、HTTP服务）
├── internal/
│   ├── agent/
│   │   ├── agent.go                      # Agent 核心（对话循环、工具调用、并发控制）
│   │   └── manager.go                    # Agent 管理器（注册、切换、意图路由）
│   ├── auth/
│   │   └── auth.go                       # 用户认证服务（注册、登录、JWT 签发与验证）
│   ├── config/
│   │   └── config.go                     # 配置加载与验证
│   ├── llm/
│   │   ├── openai.go                     # OpenAI/DeepSeek API 客户端
│   │   ├── schema.go                     # API 响应结构定义
│   │   └── stream_scanner.go             # SSE 流式响应解析
│   ├── logger/
│   │   └── logger.go                     # 日志系统（INFO/ERROR/WARNING → 文件）
│   ├── middleware/
│   │   └── auth.go                       # JWT 鉴权中间件（提取并验证 Bearer Token）
│   ├── prompt/
│   │   ├── prompt.go                     # 提示词加载与模板替换
│   │   ├── general_prompt.yaml           # 通用工具助手系统提示词
│   │   ├── gold_analysis_prompt.yaml     # 黄金分析助手系统提示词
│   │   ├── intention_router_prompt.yaml  # 意图路由Agent系统提示词
│   │   └── scene.go                      # 场景常量定义
│   ├── store/
│   │   ├── db.go                         # 数据库连接池管理
│   │   ├── user.go                       # 用户数据访问对象（CRUD）
│   │   ├── session.go                    # 会话数据访问对象（创建、查询、归档）
│   │   └── message.go                    # 消息数据访问对象（保存、查询、内存转换）
│   └── tools/
│       ├── setup.go                      # 工具统一注册 + Agent列表描述生成
│       ├── local/                        # 本地工具（插件化自注册）
│       │   ├── register.go               # 工具注册表（全局工厂注册 + 按组发现）
│       │   ├── weather.go                # 天气查询工具
│       │   ├── ip.go                     # IP 查询工具
│       │   ├── qrcode.go                 # 二维码生成工具
│       │   ├── translation.go            # 翻译工具
│       │   ├── textprocessor.go          # 文本处理工具
│       │   └── shorturl.go              # 短链接生成工具
│       └── mcp/                          # MCP工具模块
│           ├── client.go                 # MCP客户端（JSON-RPC通信）
│           └── tool.go                   # MCP工具适配器（适配types.Tool接口）
├── pkg/
│   ├── types/
│   │   └── types.go                      # 公共类型定义（Message, Tool, Config, User, Session 等）
│   └── utils/
│       └── utils.go                      # 工具函数
├── scripts/
│   └── schema.sql                        # 数据库建表 SQL（users / sessions / messages）
├── logs/
│   └── agent.log                         # 运行日志输出
├── REFERENCE/                            # 参考文档
├── go.mod
├── go.sum
└── README.md
```

## 🚀 快速开始

### 1. 环境要求

- Go 1.24+
- 网络连接（访问 LLM API 和工具 API）
- （可选）MCP 工具服务端（黄金分析助手需要）

### 2. 安装依赖

```bash
git clone <repository-url>
cd agent-hwan
go mod tidy
```

### 3. 初始化数据库

需要 PostgreSQL 16+，并安装 pgvector 扩展：

```bash
# 以超级用户连接数据库后执行
psql -U postgres -d your_database

-- 启用扩展
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 执行建表脚本
\i scripts/schema.sql
```

### 4. 配置

编辑 `cmd/config/config.yaml`：

```yaml
# 日志配置
log:
  file_path: "../../logs/agent.log"

# Prompt 目录配置
prompt:
  dir: "../../internal/prompt"

# HTTP 服务配置
http:
  host: "0.0.0.0"
  port: 8088

# 数据库配置（PostgreSQL）
database:
  dsn: "postgres://user:password@localhost:5432/agentlydb?sslmode=disable"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: 300

# JWT 配置
jwt:
  secret: "your-strong-random-secret"
  expire_hour: 72

# LLM 配置
llm:
  provider: "deepseek"
  api_key: "your-api-key"
  model: "deepseek-chat"
  base_url: "https://api.deepseek.com/v1"

# 多Agent配置列表（启动时全部注册）
agents:
  - id: "intention_router"
    name: "意图路由助手"
    description: "自动分析用户意图并路由到合适的专业Agent处理"
    prompt_scene: "intention_router_prompt"
    tool_group: "none"
    role: "system"
  - id: "general"
    name: "通用工具助手"
    description: "提供天气查询、IP查询、翻译、二维码生成等通用工具服务"
    prompt_scene: "general_prompt"
    tool_group: "general"
    role: "business"
  - id: "gold_analysis"
    name: "黄金分析助手"
    description: "提供专业的黄金市场分析、行情查询和投资参考服务"
    prompt_scene: "gold_analysis_prompt"
    tool_group: "mcp_gold"
    role: "business"

# 工具配置
tools:
  weather:
    api_key: "your-qweather-api-key"
    base_url: "https://your-qweather-endpoint"
  ip:
    access_key: "your-ipapi-access-key"
  translate:
    base_url: "http://127.0.0.1:5000"
    api_key: ""

# MCP工具配置
mcp:
  servers:
    - name: "gold-analysis-server"
      base_url: "http://127.0.0.1:8080/mcp"
      api_key: ""
      timeout: 30
```

### 5. 运行

```bash
cd cmd/server
go run main.go
```

### 6. 交互示例

```
========== 🤖 AI Agent 助手（多Agent模式） ==========

  ✅ Agent [intention_router] (意图路由助手) 创建成功 [系统路由]
  ✅ Agent [general] (通用工具助手) 创建成功
  ✅ Agent [gold_analysis] (黄金分析助手) 创建成功

🤖 可用Agent列表
--------------------------------------------------
▶ 1. ✅ [general] 通用工具助手
      提供天气查询、IP查询、翻译、二维码生成等通用工具服务
  2. ✅ [gold_analysis] 黄金分析助手
      提供专业的黄金市场分析、行情查询和投资参考服务
--------------------------------------------------

👤 [auto] > 北京的天气怎么样？
🔀 [自动路由] → 通用工具助手 (涉及天气查询)
🤖 助手: ...（流式输出天气信息）

👤 [auto] > 今日黄金行情如何？
🔀 [自动路由] → 黄金分析助手 (涉及黄金行情分析)
🤖 助手: ...（流式输出黄金分析结果）

👤 [auto] > switch general
✅ 已切换到: 通用工具助手 (general)，自动路由已关闭

👤 [general] > auto
✅ 已开启自动路由模式，系统将自动判断并分配最合适的Agent
```

## 🌐 HTTP API

服务默认监听 `http://localhost:8088`，所有业务接口均需 JWT 鉴权。

### 鉴权方式

```
Authorization: Bearer <token>
```

Token 通过注册或登录接口获取，默认有效期 **72 小时**。

### 接口列表

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/api/auth/register` | ❌ | 用户注册，返回 JWT Token |
| POST | `/api/auth/login` | ❌ | 用户登录（支持用户名/邮箱），返回 JWT Token |
| GET | `/api/agents` | ✅ | 获取可用 Agent 列表 |
| POST | `/api/switch` | ✅ | 切换当前激活的 Agent |
| GET | `/api/sessions` | ✅ | 查询会话列表（支持按 agent_id 过滤、分页） |
| POST | `/api/sessions` | ✅ | 创建新会话 |
| GET | `/api/sessions/{id}/messages` | ✅ | 查询会话消息历史 |
| POST | `/api/sessions/{id}/chat` | ✅ | 在会话中发送消息（SSE 流式 + 持久化） |
| DELETE | `/api/sessions/{id}` | ✅ | 归档会话（软删除） |

### SSE 事件格式（聊天接口）

`/api/sessions/{id}/chat` 响应为 `text/event-stream`，包含以下事件：

```
event: route
data: {"agent_name": "通用工具助手", "reason": "涉及天气查询"}

event: message
data: {"content": "北京今天"}

event: done
data: {}
```

> 详细接口文档见 `API_DOC.yaml`（OpenAPI 3.0 格式）。

---

## 🔍 架构设计

### 多Agent + 意图路由架构

```mermaid
graph TD
    U[用户输入] --> M[Main 入口]
    M --> AM[Agent Manager]
    AM --> |自动路由模式| IR[intention_router<br/>意图路由Agent]
    IR --> |分析意图| AM
    AM --> AG[Agent: general<br/>通用工具助手]
    AM --> AGA[Agent: gold_analysis<br/>黄金分析助手]
    AG --> LLM[LLM Client]
    AGA --> LLM
    IR --> LLM
    AG --> LT[本地工具<br/>天气/翻译/IP...]
    AGA --> MCP[MCP Client]
    MCP --> MS[MCP Server<br/>黄金分析工具服务]
```

### HTTP 服务架构

```mermaid
graph TD
    FE[前端 / 客户端] --> |HTTP + JWT| MW[AuthMiddleware<br/>JWT 鉴权中间件]
    MW --> |注入 user_id| H[Handler]
    H --> |会话操作| SS[SessionStore<br/>PostgreSQL]
    H --> |消息操作| MS2[MessageStore<br/>PostgreSQL]
    H --> |AI 对话| AM2[Agent Manager]
    AM2 --> AG2[Target Agent]
    AG2 --> |SSE 流式| FE
    FE --> |无需鉴权| AUTH[/api/auth/*<br/>注册 / 登录]
    AUTH --> AS[AuthService<br/>bcrypt + JWT]
    AS --> US[UserStore<br/>PostgreSQL]
```

### 意图路由流程

```mermaid
sequenceDiagram
    participant U as 用户
    participant M as Main
    participant AM as Agent Manager
    participant IR as Router Agent
    participant LLM as LLM
    participant TA as Target Agent

    U->>M: 输入问题
    M->>AM: Route(userMessage)
    AM->>IR: ProcessMessage(userMessage)
    IR->>LLM: Chat(分析意图)
    LLM-->>IR: {"agent_id":"general","reason":"..."}
    IR-->>AM: 路由结果
    AM-->>M: targetAgentID
    M->>TA: ProcessMessageStream(userMessage)
    TA-->>M: 流式响应
    M-->>U: 实时输出
```

### 核心对话流程

```mermaid
sequenceDiagram
    participant U as 用户
    participant A as Target Agent
    participant L as LLM
    participant T as Tool (本地/MCP)

    U->>A: ProcessMessageStream(msg)
    A->>L: ChatStream(history, tools)
    L-->>A: StreamChunk (工具调用)
    A->>T: Execute(args)
    T-->>A: 工具结果
    A->>L: ChatStream(history + 工具结果)
    L-->>A: StreamChunk (文本)
    A-->>U: chan string (流式文本)
```

### 并发安全机制

- **`sync.Mutex`**：保护 `history` 切片的并发读写
- **`sync.RWMutex`**：Agent Manager 使用读写锁支持并发读取
- **`chan struct{}` (streamDone)**：确保流式 goroutine 完成 history 写入后，下一次请求才能开始
- **`waitStreamDone()`**：在 `ProcessMessage`、`ProcessMessageStream`、`GetConversationHistory`、`ClearHistory` 入口处等待上一次流式任务完成

### 日志系统

- 交互式输出（提示、AI 回答）→ **终端 stdout**
- 运行日志（初始化、工具调用、错误）→ **日志文件** (`logs/agent.log`)
- 日志级别：`INFO`、`ERROR`、`WARNING`
- 基于 Go 标准库 `log` 实现，轻量无第三方依赖

## 🛠️ 开发指南

### 添加新Agent

1. 在 `internal/prompt/` 下创建新的 prompt YAML 文件：

```yaml
scene: "my_agent_prompt"
name: "我的Agent"
system_prompt: |
  你是一个专业的...
```

> **注意**：工具信息无需写入 Prompt，系统通过 OpenAI function calling 机制自动将已注册工具传递给 LLM。

2. 在 `internal/prompt/scene.go` 中添加场景常量：

```go
const MY_AGENT_PROMPT = "my_agent_prompt"
```

3. 在 `config.yaml` 的 `agents` 列表中添加配置：

```yaml
agents:
  - id: "my_agent"
    name: "我的Agent"
    description: "Agent描述"
    prompt_scene: "my_agent_prompt"
    tool_group: "general"  # 或自定义工具组
```

4. 如需自定义工具组，在 `main.go` 的 `initAllAgents` 中添加对应的工具注册逻辑。

5. 更新 `intention_router_prompt.yaml` 中的路由规则，添加新Agent的路由条件。

### 添加新的本地工具（插件化）

采用 **init() 自注册** 模式，新增工具只需在 `internal/tools/local/` 下创建文件，无需修改其他任何文件：

1. 在 `internal/tools/local/` 下创建新文件，实现 `types.Tool` 接口并通过 `init()` 自注册：

```go
package local

import (
    "agent-hwan/pkg/types"
    "context"
)

func init() {
    Register("general", func(cfg *types.Config) types.Tool {
        return NewMyTool(cfg.Tools.XXX.APIKey)
    })
}

type MyTool struct { /* ... */ }

func NewMyTool(apiKey string) *MyTool { /* ... */ }

func (t *MyTool) Name() string                { return "mytool" }
func (t *MyTool) Description() string          { return "我的工具描述" }
func (t *MyTool) Parameters() map[string]interface{} { /* JSON Schema */ }
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    // 实现工具逻辑
}
```

2. **完成！** 工具会在启动时通过 `init()` 自动注册到全局注册表，`SetupGeneralTools` 会自动发现并加载。

> **工具注册机制说明**：
> - `local/register.go` 维护全局工具注册表，提供 `Register(group, factory)` 和 `GetToolsByGroup(group, cfg)` 两个核心函数
> - 每个工具文件通过 `init()` 调用 `Register` 将自己的工厂函数注册到对应的工具组
> - `setup.go` 中的 `SetupGeneralTools` 调用 `GetToolsByGroup("general", cfg)` 自动获取所有工具并注册到 Agent
> - 工具信息通过 OpenAI function calling 的 `tools` 参数传递给 LLM，无需在 Prompt 中手动描述

### 接入MCP工具服务

1. 实现一个 MCP 服务端（遵循 JSON-RPC 2.0 协议），提供：
   - `tools/list` — 返回可用工具列表
   - `tools/call` — 执行工具调用

2. 在 `config.yaml` 的 `mcp.servers` 中添加服务配置：

```yaml
mcp:
  servers:
    - name: "my-mcp-server"
      base_url: "http://127.0.0.1:9090/mcp"
      api_key: ""
      timeout: 30
```

3. 在 `agents` 配置中指定 `tool_group: "mcp_gold"`（或自定义工具组）。

### 切换 LLM 提供商

修改 `config.yaml` 中的 `llm` 配置即可，支持任何兼容 OpenAI API 格式的服务：

```yaml
llm:
  provider: "openai"
  api_key: "sk-xxx"
  model: "gpt-4"
  base_url: "https://api.openai.com/v1"
```

## 📋 依赖

| 依赖 | 用途 |
|------|------|
| `github.com/chzyer/readline` | 终端交互（支持历史记录、行编辑） |
| `github.com/skip2/go-qrcode` | 二维码生成 |
| `github.com/golang-jwt/jwt/v5` | JWT 签发与验证 |
| `golang.org/x/crypto` | bcrypt 密码哈希 |
| `golang.org/x/text` | 文本处理 |
| `gopkg.in/yaml.v2` | YAML 配置解析 |

## ⚠️ 注意事项

1. **API 密钥安全**：`config.yaml` 中包含敏感信息，请勿提交到公开仓库（已在 `.gitignore` 中排除）
2. **JWT 密钥**：生产环境必须在 `config.yaml` 的 `jwt.secret` 中设置强随机值，切勿使用默认值
3. **数据库依赖**：数据库不可用时，HTTP 服务仍可正常启动；所有业务接口（注册/登录/会话/Agent管理）均会返回 **503 Service Unavailable** 并附带明确提示，而非静默 404，前端可据此统一展示「服务暂时不可用，请稍后重试」
4. **网络依赖**：天气、翻译、短链接等工具需要网络访问对应的 API 服务
5. **翻译服务**：默认使用本地 LibreTranslate 实例（`http://127.0.0.1:5000`），需自行部署或替换为公共实例
6. **MCP服务**：黄金分析助手依赖 MCP 工具服务端，如服务未启动，Agent 仍可创建但无可用工具
7. **日志路径**：默认日志输出到 `logs/agent.log`，路径相对于运行目录
8. **Agent独立性**：每个Agent拥有独立的对话历史和工具集，切换Agent不会丢失之前的对话
9. **路由Agent**：`intention_router` 是系统Agent，不对外展示，不可手动切换到该Agent
10. **消息字段**：持久化消息中工具调用列表字段名为 `tool_call_list`（JSON 序列化后的字符串）

## 📄 许可证

MIT License