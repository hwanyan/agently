# Agently

一个基于 Go 实现的多 Agent AI 系统，支持**意图自动路由**、流式对话、自动工具调用、MCP 协议集成、日志系统和灵活的 YAML 配置管理。

> 🔄 **架构升级（gRPC 化重构）**：系统的路由层已全面收敛到 **gRPC**。所有对外能力以 gRPC 服务（`.proto`）定义，并通过 **grpc-gateway** 在同一 HTTP 端口反向代理，**对外的 REST 路径与 SSE 事件格式保持完全兼容**。详见 [gRPC 架构与迁移说明](#-grpc-架构与迁移说明)。

## ✨ 核心特性

- 🚀 **gRPC 优先** — 全部路由以 gRPC 服务定义（Protobuf 契约），grpc-gateway 同步暴露兼容旧版的 REST + SSE
- 🤖 **多Agent架构** — 启动时自动扫描并注册全部Agent，支持按需切换
- 🔀 **意图自动路由** — 内置路由Agent（intention_router），自动分析用户意图并分发到合适的专业Agent
- 🔁 **会话 Agent 自动同步** — 自动路由命中与会话当前 `agent_id` 不一致时，后端会同步更新会话的主 Agent，前端按 Agent 分类时可正确归组
- 🔧 **自动工具调用** — Agent 自动识别意图并调用外部工具完成任务
- 🌊 **流式响应** — 支持流式输出，实时展示 AI 回答，流式模式下同样支持工具调用
- 🔌 **MCP协议支持** — 通过 Model Context Protocol 动态发现和调用远程工具服务
- 🤝 **A2A协议支持** — 将业务 Agent 通过 Agent2Agent（A2A）协议暴露为可互操作端点，供其他 AI Agent 发现与调用
- 📝 **日志系统** — 统一的日志管理（INFO/ERROR/WARNING → 文件）
- ⚙️ **YAML 配置** — 统一使用仓库根目录下的 `config.yaml` 管理所有配置项
- 🔒 **并发安全** — 使用互斥锁 + channel 机制保护对话历史的并发访问
- 💬 **对话管理** — 每个Agent拥有独立的对话历史和上下文维护
- 🔑 **用户认证** — JWT 鉴权，支持注册/登录，所有业务接口均需登录态
- 📧 **邮箱验证** — 注册时强制邮箱验证码校验，SMTP 发送验证码，防止恶意注册
- 🗄️ **会话持久化** — 对话历史存储到 PostgreSQL，刷新页面后可完整恢复
- 🌐 **gRPC 内核 + HTTP 网关** — 业务以 gRPC 实现并默认仅监听回环，由 grpc-gateway 统一对外提供 RESTful + SSE，前端零改造接入

## 🤖 内置Agent

| # | Agent ID | 名称 | 类型 | 功能 |
|---|----------|------|------|------|
| 1 | `intention_router` | 意图路由助手 | 系统Agent | 自动分析用户意图，路由到合适的专业Agent |
| 2 | `general` | 通用工具助手 | 业务Agent | 天气、翻译、IP查询、二维码等通用服务 |
| 3 | `gold_analysis` | 黄金分析助手 | 业务Agent | 专业黄金市场分析、行情查询（MCP工具） |
| 4 | `news_analysis` | 新闻分析与总结助手 | 业务Agent | 新闻检索、要点提炼、热点解读、多源对比（MCP工具） |

## 📦 内置工具

### 通用工具（general Agent）

| # | 工具名 | 功能 | 依赖服务 |
|---|--------|------|----------|
| 1 | `weather` ☁️ | 查询城市天气（实时温湿度、风力及未来天气预报） | 聚合数据 API |
| 2 | `ipapi` 🌍 | 查询 IP/域名的地理位置与网络信息 | ipapi.com |
| 3 | `qrcode` 🔢 | 生成二维码（dataurl / ascii / file） | 本地生成 |
| 4 | `translate` 🔡 | 多语言文本翻译 | LibreTranslate |
| 5 | `textprocessor` ✏️ | 文本统计、格式转换、内容提取、文本生成 | 本地处理 |
| 6 | `shorturl` 🔗 | 长 URL 转短链接（TinyURL / is.gd / v.gd） | 第三方短链接服务 |

### MCP 工具（gold_analysis / news_analysis 等业务 Agent）

通过 MCP 协议从远程服务器动态发现，具体工具取决于 MCP 服务端提供的能力。Agent 与 MCP 服务的绑定通过 `prompt.yaml` 中的 `tools.mcp_servers` 引用 `config.yaml` 的 `mcp.servers[].id` 完成。

## 🏗️ 项目结构

```
agently/
├── main.go                                # 🆕 程序入口（已从 cmd/server 移至仓库根目录）
├── bd.sh                                  # 🆕 交叉编译脚本（GOOS=linux GOARCH=amd64 go build main.go）
├── config.yaml                            # 🆕 统一配置文件（已从 cmd/config/config.yaml 移至仓库根目录，且不再嵌套 config/ 子目录）
├── pb/                                     # gRPC 契约与生成代码
│   ├── proto/
│   │   └── agently.proto                  # 原生 proto：全部 gRPC 服务契约（Auth/Agent/Session，含 Chat 流式）
│   ├── gen/                                # protoc 生成产物（仅生成代码，勿手改）
│   │   ├── agently.pb.go                  # 消息类型
│   │   ├── agently_grpc.pb.go             # gRPC 服务端/客户端桩代码
│   │   ├── agently.pb.gw.go               # grpc-gateway 反向代理（REST 映射）
│   │   └── agently.swagger.json           # 由 proto 自动生成的 OpenAPI(swagger) 文档
│   ├── api/google/api/                     # 供 protoc 本地解析的 google.api HTTP 注解（annotations/http.proto）
│   └── gen.sh                              # 一键生成脚本（protoc + go/go-grpc/grpc-gateway/openapiv2 插件）
├── internal/
│   ├── agent/
│   │   ├── agent.go                        # Agent 核心引擎（对话循环、工具调用、并发控制）
│   │   └── manager.go                      # Agent 管理器（切换、意图路由、列表查询）
│   ├── agents/                             # Agent 定义层（统一注册 + 各 Agent 的 Prompt）
│   │   ├── embed.go                        # 通过 embed.FS 嵌入所有 prompt.yaml 到二进制
│   │   ├── registry.go                     # Agent 统一编排（解析 prompt、配置校验、MCP 探活、工具注入）
│   │   ├── general/prompt.yaml             # 通用工具助手 Prompt
│   │   ├── gold_analysis/prompt.yaml       # 黄金分析助手 Prompt（依赖 mcp_servers: [gold_analysis]）
│   │   ├── news_analysis/prompt.yaml       # 新闻分析助手 Prompt（依赖 mcp_servers: [news_analysis]）
│   │   └── intention_router/prompt.yaml    # 意图路由 Agent Prompt
│   ├── auth/
│   │   ├── auth.go                         # 用户认证服务（注册、登录、JWT 签发与验证）
│   │   ├── email_verify.go                 # 邮箱验证业务编排
│   │   └── mailer.go                       # SMTP 邮件发送器
│   ├── config/
│   │   └── config.go                       # 配置加载与验证
│   ├── server/                             # gRPC 服务实现 + 网关编排
│   │   ├── server.go                       # gRPC 服务与 grpc-gateway 网关启动编排、SSE 对话透传、统一错误处理、CORS
│   │   ├── interceptor.go                  # JWT 鉴权拦截器（Unary + Stream；鉴权策略由 proto 方法选项驱动）
│   │   ├── context.go                      # 用户信息的 context 注入/读取
│   │   ├── convert.go                      # 领域模型 → pb 消息 的转换
│   │   ├── auth_service.go                 # AuthService gRPC 实现（注册/登录/发码）
│   │   ├── agent_service.go                # AgentService gRPC 实现（列表/工具/切换）
│   │   └── session_service.go              # SessionService gRPC 实现（会话 CRUD + Chat 流式）
│   ├── a2a/                                # A2A（Agent-to-Agent）协议服务
│   │   ├── executor.go                     # 将 A2A 请求桥接到业务 Agent（流式 + Artifact 事件透出）
│   │   ├── agentcard.go                    # 根据 Agent 元信息生成 A2A AgentCard（自描述清单）
│   │   └── server.go                       # A2A HTTP 服务编排（JSON-RPC + AgentCard 端点挂载）
│   ├── llm/                                # LLM 客户端
│   │   ├── factory.go                      # 根据配置构造 LLM 客户端（NewClientFromConfig）
│   │   ├── openai.go                       # OpenAI 协议客户端实现
│   │   ├── schema.go                       # 请求/响应结构定义
│   │   └── stream_scanner.go                # 流式响应解析
│   ├── logger/
│   │   └── logger.go                       # 日志系统（INFO/ERROR/WARNING → 按日期分文件）
│   ├── store/                               # 数据访问层
│   │   ├── db.go                            # 数据库连接池封装
│   │   ├── user.go                          # 用户表访问
│   │   ├── session.go                       # 会话表访问
│   │   ├── message.go                       # 消息表访问
│   │   └── email_verification.go            # 邮箱验证码表访问
│   └── tools/
│       ├── setup.go                         # 本地工具组注入入口（SetupLocalToolGroup）
│       ├── local/                           # 本地工具（插件化自注册）
│       │   ├── registry.go                  # 全局工具注册表（Register / GetToolsByGroup）
│       │   ├── weather.go / ip.go / qrcode.go / translation.go / textprocessor.go / shorturl.go
│       └── mcp/                             # MCP 客户端与工具适配
│           ├── client.go                    # MCP 协议客户端（tools/list、tools/call）
│           └── adapter.go                   # 将 MCP 工具适配为 types.Tool
├── pkg/
│   ├── types/
│   │   └── types.go                        # 公共类型定义（Config / HTTPConfig / GRPCConfig 等）
│   └── utils/
│       └── utils.go                         # 工具函数
├── scripts/
│   ├── 000_init_db.sql                     # 🆕 初始化数据库与用户（超级用户执行，创建 agentlydb）
│   └── 001_create_tables.sql               # 建表 SQL（users / sessions / messages / email_verifications）
├── go.mod
├── go.sum
└── README.md
```

> **已移除**：原 `cmd/server/`、`cmd/config/` 目录层级，程序入口 `main.go` 与配置文件 `config.yaml` 现均**直接位于仓库根目录**（配置文件不再嵌套 `config/` 子目录）；`REFERENCE/` 参考文档目录已删除。历史上的 `internal/http/http.go`（手写的 `net/http` 路由层）与 `internal/middleware/auth.go`（HTTP JWT 中间件）也已移除，职责分别由 grpc-gateway + gRPC 服务实现、以及 gRPC 鉴权拦截器取代。

## 🚀 快速开始

### 1. 环境要求

- Go 1.25+
- PostgreSQL 16+（含 pgvector 扩展）
- SMTP 邮件服务（必配，用于发送注册验证码）
- 网络连接（访问 LLM API 和工具 API）
- （可选）MCP 工具服务端（黄金分析助手、新闻分析助手需要）
- （仅在需要**重新生成** gRPC 代码/接口文档时）`protoc` 及以下插件：`protoc-gen-go`、`protoc-gen-go-grpc`、`protoc-gen-grpc-gateway`、`protoc-gen-openapiv2`。生成产物已随仓库提交，普通运行无需安装这些工具。

### 2. 安装依赖

```bash
git clone <repository-url>
cd agently
go mod tidy
```

### 3. 初始化数据库

需要 PostgreSQL 16+，并安装 pgvector 扩展：

```bash
# 1) 以超级用户身份创建数据库与账号（首次部署执行）
psql -U postgres -f scripts/000_init_db.sql

# 2) 连接到 agentlydb 后启用扩展并执行建表脚本
psql -U postgres -d agentlydb
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
\i scripts/001_create_tables.sql
```

### 4. 配置

编辑仓库根目录下的 `config.yaml`：

```yaml
# 日志配置
log:
  file_path: "./logs/agently"   # 日志文件路径前缀（相对于运行目录，实际文件名会自动追加日期）

# HTTP 服务配置（grpc-gateway 对外暴露的 REST/SSE 网关）
http:
  host: "0.0.0.0"
  port: 8080

# gRPC 服务配置（系统核心服务层，HTTP 网关在本机反向代理到此端口）
grpc:
  host: "127.0.0.1"  # 仅监听回环：外部流量统一经 HTTP 网关进入
  port: 8081          # 留空或 0 则默认取 http.port + 1

# A2A（Agent-to-Agent）协议服务配置
# 将每个业务 Agent 暴露为符合 a2a-protocol.org 协议的独立端点，供其他 AI Agent 互操作调用
a2a:
  host: "0.0.0.0"     # A2A 服务监听地址
  port: 9090          # A2A 服务端口（0 表示不启动 A2A 服务）
  public_url: ""      # 对外可达基础 URL（用于 AgentCard 的 url 字段），给第三方使用必须配置；留空则按 host:port 本机推导（仅限本地/内网直连测试）
  auth_token: ""      # JSON-RPC 端点的 Bearer Token 共享密钥；留空表示不鉴权（仅建议本地/内网调试）。
                      # 生产环境建议改用环境变量 AGENTLY_A2A_AUTH_TOKEN 注入（优先级高于本字段）

# 数据库配置（PostgreSQL）
database:
  dsn: "postgres://agently:password@localhost:5432/agentlydb?sslmode=disable"
  max_open_conns: 20
  max_idle_conns: 5
  conn_max_lifetime: 300

# JWT 配置
jwt:
  secret: "your-strong-random-secret"
  expire_hour: 72

# SMTP 邮件配置（必配项，未配置则无法启动）
smtp:
  host: "smtp.qq.com"              # SMTP 服务器地址
  port: 587                         # SMTP 端口（587=TLS, 465=SSL）
  username: "your-email@qq.com"     # SMTP 登录用户名（通常是邮箱地址）
  password: "your-smtp-auth-code"   # SMTP 登录密码或授权码
  from: ""                          # 发件人地址（可选，默认使用 username）

# LLM 配置
llm:
  provider: "deepseek"
  api_key: "your-api-key"
  model: "deepseek-chat"
  base_url: "https://api.deepseek.com/v1"

# 工具配置
tools:
  weather:
    api_key: "your-juhe-api-key"
    base_url: "https://apis.juhe.cn/simpleWeather/query"
  ip:
    access_key: "your-ipapi-access-key"
  translate:
    base_url: "https://libretranslate.com"
    api_key: ""

# MCP 工具配置
# id   ：稳定标识，被 Agent 的 prompt.yaml 中 tools.mcp_servers 引用，不要随意修改
# name ：仅用于日志/前端展示，可随意修改，不影响绑定关系
mcp:
  servers:
    - id: "gold_analysis"
      name: "黄金分析 MCP 服务"
      base_url: "http://127.0.0.1:9091"
      api_key: ""
      timeout: 60
    - id: "news_analysis"
      name: "新闻分析 MCP 服务"
      base_url: "http://127.0.0.1:9092"
      api_key: ""
      timeout: 60
```

> **注意**：Agent 列表不需要在 `config.yaml` 中配置。所有 Agent 的 `prompt.yaml` 通过 `internal/agents/embed.go` 的 `embed.FS` 在编译期嵌入二进制，系统启动时会自动扫描每个包含 `prompt.yaml` 的子目录并注册 Agent；Agent 的唯一标识由 `prompt.yaml` 内的 `agent_id` 字段显式声明（与所在目录名无关），无需外部文件路径配置。

### 5. 运行

在仓库根目录直接运行（配置默认从 `./config.yaml` 加载）：

```bash
go run main.go
```

如需交叉编译为 Linux 可执行文件：

```bash
bash bd.sh agently-server   # 生成 agently-server（GOOS=linux GOARCH=amd64）
```

### 6. 启动与交互示例

> 本项目已移除 CLI 交互模式，改为**纯服务端**运行：启动后同时提供 gRPC 服务（内部回环）与 grpc-gateway 的 REST/SSE 接口，通过 HTTP 客户端（前端 / curl / Postman）交互。

**启动日志（示例）：**

```
[INFO] Agently 系统启动
[INFO] 构建项目中全部 agent 与其对应的基本信息的映射关系...
[INFO] 系统Agent [intention_router] 已自动设置为路由器
[INFO] 业务Agent [general] 已设为默认激活Agent
[INFO] Agent [general] 已注册到管理器
[INFO] Agent [gold_analysis] 已注册到管理器
[INFO] Agent [news_analysis] 已注册到管理器
[INFO] 数据库连接成功，认证服务已初始化
[INFO] 邮箱验证服务已启用（SMTP: smtp.qq.com）
[INFO] 鉴权策略已加载：共 11 个方法，其中公开（免鉴权）方法 3 个: /agently.v1.AuthService/Login, /agently.v1.AuthService/Register, /agently.v1.AuthService/SendVerifyCode
[INFO] gRPC 服务启动，监听地址: 127.0.0.1:8081
[INFO] HTTP 网关启动，监听地址: 0.0.0.0:8080
```

**通过 HTTP 接口交互（以本地直连网关 `http://localhost:8080` 为例）：**

```bash
# 1) 登录获取 JWT Token
curl -X POST http://localhost:8080/v1/auth:login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"mypassword123"}'
# 返回：{"code":0,"token":"eyJhbGciOi...","user":{...}}

TOKEN="eyJhbGciOi..."   # 替换为上一步返回的 token

# 2) 创建会话（需鉴权）
curl -X POST http://localhost:8080/v1/sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"general","title":""}'
# 返回：{"code":0,"session":{"id":"<session_id>", ...}}

# 3) 在会话中发消息（自动路由 + SSE 流式）
curl -N -X POST http://localhost:8080/v1/sessions/<session_id>:chat \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"message":"北京的天气怎么样？","auto_route":true}'
```

**SSE 流式响应（对话接口输出）：**

```
event: route
data: {"agent_name": "通用工具助手", "reason": "涉及天气查询"}

event: tool_call
data: {"tool_name": "weather", "tool_args": "{\"city\":\"北京\"}"}

event: tool_result
data: {"tool_name": "weather", "tool_result": "北京 晴 26℃..."}

event: message
data: {"content": "北京今天"}

event: message
data: {"content": "天气晴朗..."}

event: done
data: {}
```

> `curl -N` 关闭输出缓冲以实时查看 SSE 流。自动路由模式下，若命中的 Agent 与会话当前 `agent_id` 不同，后端会同步更新会话主 Agent（见下文「会话 Agent 自动同步」）。

## 🌐 HTTP API（grpc-gateway 兼容层）

> 下列 REST/SSE 接口由 **grpc-gateway** 自动映射到对应的 gRPC 方法，供前端与第三方零改造接入。若需原生 gRPC 调用，见 [gRPC 架构与迁移说明](#-grpc-架构与迁移说明)。

> **路径约定（v1，资源导向）**：契约中的路径统一采用 `/v1/...`，**不含产品名前缀**；产品名/部署前缀（如 `/api/agently`）由 nginx/ingress 统一添加，保持契约与部署解耦。动作型接口（切换/对话/归档）使用 `:动词` 自定义方法。下表以**网关内部路径**（`/v1/...`）为准；若 nginx 配置了 `/api/agently/` → 去前缀反代，则前端对外路径为 `/api/agently/v1/...`。

服务默认监听 `http://localhost:8080`（gRPC 默认监听回环 `127.0.0.1:8081`），除 `auth` 系列外的业务接口均需 JWT 鉴权。

### 鉴权方式

```
Authorization: Bearer <token>
```

Token 通过注册或登录接口获取，默认有效期 **72 小时**。

### 注册流程

1. 调用 `POST /v1/auth:sendVerifyCode` 发送验证码到邮箱
2. 用户查收邮件，获取 6 位数字验证码（有效期 5 分钟）
3. 调用 `POST /v1/auth:register` 提交用户名、邮箱、密码和验证码完成注册

### 接口列表

| 方法 | 路径（网关内部 `/v1`） | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/v1/auth:sendVerifyCode` | ❌ | 发送邮箱验证码（注册/重置密码） |
| POST | `/v1/auth:register` | ❌ | 用户注册（需先获取验证码），返回 JWT Token |
| POST | `/v1/auth:login` | ❌ | 用户登录（支持用户名/邮箱），返回 JWT Token |
| GET | `/v1/agents` | ✅ | 获取可用 Agent 列表 |
| GET | `/v1/tools` | ✅ | 获取所有可用工具列表（按工具名去重） |
| POST | `/v1/agents:switch` | ✅ | 切换当前激活的 Agent |
| GET | `/v1/sessions` | ✅ | 查询会话列表（支持按 agent_id 过滤、分页） |
| POST | `/v1/sessions` | ✅ | 创建新会话 |
| GET | `/v1/sessions/{id}/messages` | ✅ | 查询会话消息历史 |
| POST | `/v1/sessions/{id}:chat` | ✅ | 在会话中发送消息（SSE 流式 + 持久化） |
| POST | `/v1/sessions/{id}:archive` | ✅ | 归档会话（软删除） |

### SSE 事件格式（聊天接口）

`POST /v1/sessions/{id}:chat` 响应为 `text/event-stream`，包含以下事件：

```
event: route
data: {"agent_name": "通用工具助手", "reason": "涉及天气查询"}

event: message
data: {"content": "北京今天"}

event: done
data: {}
```

> **会话 Agent 自动同步**：在自动路由模式下（`auto_route: true`），若路由命中的 Agent 与会话当前 `agent_id` 不同，后端会调用 `SessionStore.UpdateAgentID` 同步会话的主 Agent（仅对 `status=1` 活跃会话生效）。前端在下次拉取会话列表或会话详情时，会看到 `agent_id` 已更新，从而归类到正确的 Agent 分组（例如新闻类问题归入"新闻与总结"）。

> **接口契约以 `pb/proto/agently.proto` 为唯一来源**；供前端/第三方查阅的 OpenAPI 文档 `pb/gen/agently.swagger.json` 由 `bash pb/gen.sh` 从 proto **自动生成**（勿手改），随契约同步演进。可用 Swagger UI / Redoc 等工具加载查看。

---

## 🔍 架构设计

### 多Agent + 意图路由架构

```mermaid
graph TD
    U[客户端请求] --> GW[HTTP 网关 / gRPC]
    GW --> AM[Agent Manager]
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

### HTTP 服务架构（gRPC + grpc-gateway）

```mermaid
graph TD
    FE[前端 / REST 客户端] --> |HTTP + JWT| GW[grpc-gateway<br/>REST/SSE 网关]
    GW --> |回环反向代理 127.0.0.1:grpc_port| INT[AuthInterceptor<br/>JWT 鉴权拦截器]
    LC[本机 gRPC 客户端<br/>可选] -.-> |gRPC + metadata JWT| INT
    INT --> |注入 user_id| SVC[gRPC 服务实现]
    SVC --> AS2[AuthService]
    SVC --> AGS[AgentService]
    SVC --> SES[SessionService]
    AS2 --> US[UserStore<br/>PostgreSQL]
    SES --> |会话/消息| SS[Session/Message Store<br/>PostgreSQL]
    SES --> |AI 对话| AM2[Agent Manager]
    AM2 --> AG2[Target Agent]
    AG2 --> |ChatEvent 流| SES
    SES --> |gRPC stream| GW
    GW --> |转写为 SSE| FE
```

> gRPC 默认仅监听 `127.0.0.1`，作为网关内部服务；上图中「本机 gRPC 客户端」为可选路径（同机可达）。外部客户端一律经 HTTP 网关接入。

> 网关对 `/sessions/{id}/chat` 使用自定义 SSE 处理器：它作为 gRPC 客户端调用服务端流式 `Chat`，再把每个 `ChatEvent` 转写成与旧版完全一致的 `event: <type>\ndata: <json>` 事件，因此**前端 SSE 逻辑无需任何改动**。

### 意图路由流程

```mermaid
sequenceDiagram
    participant U as 客户端
    participant S as SessionService.Chat（gRPC）
    participant AM as Agent Manager
    participant IR as Router Agent
    participant LLM as LLM
    participant TA as Target Agent

    U->>S: POST /v1/sessions/{id}:chat（经网关）
    S->>AM: Route(userMessage)
    AM->>IR: ProcessMessage(userMessage)
    IR->>LLM: Chat(分析意图)
    LLM-->>IR: {"agent_id":"general","reason":"..."}
    IR-->>AM: 路由结果
    AM-->>S: targetAgentID
    S->>TA: ProcessMessageStream(userMessage)
    TA-->>S: 流式事件
    S-->>U: SSE 流式输出
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

- AI 回答通过 **SSE 流式**返回给客户端（不再有 CLI 终端输出）
- 运行日志（初始化、路由、工具调用、错误）→ **日志文件**（按 `log.file_path` 前缀 + 日期，如 `logs/agently-2026-08-27.log`）
- 日志级别：`INFO`、`ERROR`、`WARNING`
- 基于 Go 标准库 `log` 实现，轻量无第三方依赖

---

## 🔌 gRPC 架构与迁移说明

本项目已完成一次**路由层的 gRPC 化重构**：过去手写的 `net/http` 路由（`internal/http`）与 HTTP JWT 中间件（`internal/middleware`）被彻底移除，所有请求路由统一由 gRPC 服务契约描述，并通过 grpc-gateway 暴露兼容旧版的 REST/SSE。

### 设计目标

- **单一契约来源**：接口以 `pb/proto/agently.proto` 为唯一事实来源，消息结构、方法签名、HTTP 映射集中维护。
- **对外零破坏**：REST 路径、鉴权方式（`Authorization: Bearer <token>`）、SSE 事件格式（`event`/`data`）与错误响应体 `{code, message}` 均与旧版保持一致，前端无需改动。
- **原生 gRPC 能力**：底层为强类型 gRPC 服务，便于本机/同环境内的服务间调用。默认 gRPC 仅监听回环（`127.0.0.1`），作为网关的内部实现不对外暴露；如确有对外原生 gRPC 需求，可将 `grpc.host` 显式设为 `0.0.0.0`。

### 服务划分（`agently.v1`）

> 契约唯一来源为 `pb/proto/agently.proto`（仅存放原生 proto）；生成代码统一输出到 `pb/gen/`（Go 包 `agentlypb`，导入路径 `github.com/hwanyan/agently/pb/gen`）。

| gRPC 服务 | 方法 | 映射的 REST 路径 | 鉴权 |
|-----------|------|------------------|------|
| `AuthService` | `SendVerifyCode` | `POST /v1/auth:sendVerifyCode` | ❌ |
| `AuthService` | `Register` | `POST /v1/auth:register` | ❌ |
| `AuthService` | `Login` | `POST /v1/auth:login` | ❌ |
| `AgentService` | `ListAgents` | `GET /v1/agents` | ✅ |
| `AgentService` | `ListTools` | `GET /v1/tools` | ✅ |
| `AgentService` | `SwitchAgent` | `POST /v1/agents:switch` | ✅ |
| `SessionService` | `ListSessions` | `GET /v1/sessions` | ✅ |
| `SessionService` | `CreateSession` | `POST /v1/sessions` | ✅ |
| `SessionService` | `ListMessages` | `GET /v1/sessions/{session_id}/messages` | ✅ |
| `SessionService` | `Chat`（服务端流式） | `POST /v1/sessions/{session_id}:chat` | ✅ |
| `SessionService` | `ArchiveSession` | `POST /v1/sessions/{session_id}:archive` | ✅ |

> 路径采用资源导向 + `/v1` 版本前缀，动作型接口用 `:动词` 自定义方法；产品名前缀由 nginx/ingress 添加，不写入契约。

### 请求路由流程

```mermaid
sequenceDiagram
    participant C as 客户端（REST 或 gRPC）
    participant GW as grpc-gateway
    participant I as AuthInterceptor
    participant S as gRPC 服务实现
    participant B as 业务层（Manager/Store/Auth）

    C->>GW: HTTP 请求（REST 客户端）
    GW->>I: 反向代理为 gRPC 调用（透传 authorization）
    Note over C,I: 原生 gRPC 客户端可直接调用 I（携带 metadata）
    I->>I: 校验 Bearer Token，注入 user_id（公开方法跳过）
    I->>S: 转交请求
    S->>B: 复用既有业务逻辑
    B-->>S: 结果
    S-->>GW: 响应 / 流
    GW-->>C: REST JSON / SSE
```

### 鉴权：从 HTTP 中间件到 gRPC 拦截器（契约即策略）

- 旧：`internal/middleware/auth.go` 从 HTTP 头解析并校验 JWT（已移除）。
- 新：`internal/server/interceptor.go` 提供 **Unary + Stream 双拦截器**，从 gRPC metadata 的 `authorization` 读取 Bearer Token 并校验，成功后把 `user_id`/`username` 注入 `context`。
- 网关通过 `runtime.WithMetadata` 把 HTTP `Authorization` 头透传为 gRPC metadata。
- **鉴权策略写在 proto 契约里**：在 `agently.proto` 中定义自定义方法选项 `(agently.v1.public)`，对公开方法（注册/登录/发码）显式标注 `option (agently.v1.public) = true;`。拦截器启动时遍历 proto 描述符注册表构建「方法 → 是否公开」策略表。
- 语义为 **default-secure**：只有显式声明 `public=true` 的方法免鉴权，其余（含未来新增方法）一律需要鉴权，从根本上杜绝"新增方法即意外公开"的隐患。

```proto
extend google.protobuf.MethodOptions {
  bool public = 90001;   // public=true 表示免鉴权；缺省视为需要鉴权
}

rpc Login(LoginRequest) returns (AuthResponse) {
  option (google.api.http) = { post: "/v1/auth:login" body: "*" };
  option (agently.v1.public) = true;   // 显式声明为公开接口
}
```

### 流式对话（SSE 完全兼容）

- gRPC 侧 `Chat` 为**服务端流式** RPC，逐条下发 `ChatEvent`（`type` ∈ `route/message/tool_call/tool_result/tool_error/error/done`）。
- 网关侧对 `/v1/sessions/{id}:chat` 注册**自定义 SSE 处理器**：内部以 gRPC 客户端调用 `Chat`，把每个 `ChatEvent` 转写为 `event: <type>\ndata: <json>\n\n`，与旧版 SSE 字节级一致。
- **覆盖生成处理器的关键**：`runtime.ServeMux.Handle` 采用"前插"策略，匹配时从前往后取首个命中，故**后注册者优先**。因此自定义 SSE 处理器必须在 `RegisterSessionServiceHandler` **之后**注册，才能覆盖自动生成的流式 `Chat` 处理器。
- 首个事件采用「预读」策略：若首个事件是鉴权/校验类错误（未登录、会话不存在、消息为空），以标准 `4xx` JSON 返回；否则切换到 SSE 流。

### JSON 兼容性细节

- 网关 Marshaler 启用 `UseProtoNames`（字段 snake_case，如 `agent_id`/`is_active`/`created_at`）与 `EmitUnpopulated`（输出零值字段），与旧接口保持一致。
- 时间字段统一格式化为 RFC3339 字符串；可空字段（如 `last_login`/`archived_at`）为空时输出空字符串。
- 错误响应通过统一的错误处理器输出 `{"code": <HTTP 状态码>, "message": "..."}`，`code` 采用 HTTP 状态码而非 gRPC code。

### 端口

- HTTP 网关端口：`http.port`（默认 `8080`）——对外提供 REST/SSE。
- gRPC 端口：`grpc.port`（留空/0 时默认取 `http.port + 1`）；`grpc.host` 默认 `127.0.0.1`，即 **gRPC 仅监听本机回环、不对外暴露**，网关经回环反向代理到该端口。外部流量一律从 HTTP 网关进入。

### 重新生成 gRPC 代码

`.proto` 变更后，在项目根目录执行：

```bash
bash pb/gen.sh
```

脚本等价于：

```bash
protoc \
  -I pb/api \
  -I pb/proto \
  --go_out=pb/gen --go_opt=paths=source_relative \
  --go-grpc_out=pb/gen --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=pb/gen --grpc-gateway_opt=paths=source_relative \
  --openapiv2_out=pb/gen \
  --openapiv2_opt=logtostderr=true,json_names_for_fields=false \
  pb/proto/agently.proto
```

> - `google/api/annotations.proto`、`google/api/http.proto` 已随仓库置于 `pb/api/google/api/` 下，protoc 可离线解析 HTTP 注解，无需访问网络或 BSR。
> - `--openapiv2_out` 由 `protoc-gen-openapiv2` 插件（属 grpc-gateway 生态）从带 `google.api.http` 注解的 proto 生成 `agently.swagger.json`；`json_names_for_fields=false` 使字段名保持 snake_case，与网关 `UseProtoNames` 一致。若本机无该插件：`go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest`。

### 原生 gRPC 调用示例（Go）

```go
conn, _ := grpc.NewClient("127.0.0.1:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
client := agentlypb.NewAgentServiceClient(conn)

ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
resp, err := client.ListAgents(ctx, &agentlypb.ListAgentsRequest{})
```

---

## 🤝 A2A 协议接入说明

A2A（Agent2Agent，a2a-protocol.org）是 Google 牵头的开放协议，用于不同 AI Agent 之间的互操作。本项目已将 `internal/agents/` 下定义的每个**业务 Agent**（如 `news_analysis`、`gold_analysis`、`general`）通过官方 Go SDK（`github.com/a2aproject/a2a-go/v2`）暴露为符合 A2A 协议的独立端点，任何遵循 A2A 协议的 Agent 或客户端都能发现并调用它们。

### 端点约定

A2A 服务监听独立端口（`a2a.port`，默认 `9090`），每个业务 Agent 暴露两个端点（以 `news_analysis` 为例）：

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/a2a/news_analysis` | 该 Agent 的 A2A **JSON-RPC** 端点（`message/send`、`message/stream`、`tasks/get` 等） |
| GET | `/a2a/news_analysis/.well-known/agent-card.json` | 该 Agent 的 **AgentCard**（自描述清单，供发现与能力协商） |

AgentCard 中 `supportedInterfaces[].url` 指向对应 JSON-RPC 端点，其他 Agent 发现卡片后即可直接调用。

### 配置

```yaml
a2a:
  host: "0.0.0.0"       # A2A 服务监听地址
  port: 9090            # A2A 服务端口（0 表示不启动 A2A 服务）
  public_url: ""        # 对外可达基础 URL；给第三方使用必须配置（如 nginx/ingress 反代后的域名或服务器对外 IP），留空则本机推导（仅限本地/内网直连测试）
  auth_token: ""        # JSON-RPC 端点的 Bearer Token 共享密钥；留空表示不鉴权（仅建议本地/内网调试）
```

### 快速体验

```bash
# 1) 查看 news_analysis 的 AgentCard（未启用鉴权时无需 Token 即可访问）
curl http://localhost:9090/a2a/news_analysis/.well-known/agent-card.json

# 2) 若已启用鉴权，调用 JSON-RPC 端点需携带 Bearer Token
curl -X POST http://localhost:9090/a2a/news_analysis \
  -H "Authorization: Bearer <你配置的 token>" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"user","parts":[{"kind":"text","text":"帮我分析今天的科技新闻"}]}}}'

# 3) 使用官方 A2A CLI 发现并调用（安装：go install github.com/a2aproject/a2a-go/v2/cmd/a2a@latest）
a2a discover http://localhost:9090/a2a/news_analysis
a2a send    http://localhost:9090/a2a/news_analysis "帮我分析今天的科技新闻"
```

也可以用任意 A2A SDK/客户端（如 `a2aclient.NewFromCard` 从 AgentCard 创建客户端后 `SendMessage`）进行调用。

### 鉴权（Bearer Token）

JSON-RPC 端点支持基于共享密钥（Bearer Token）的鉴权，实现见 `internal/a2a/auth.go`：

- **启用方式**：配置 `a2a.auth_token`，或（推荐）设置环境变量 `AGENTLY_A2A_AUTH_TOKEN`（优先级高于配置文件，避免密钥明文落盘）。留空则不鉴权，仅建议本地/内网调试使用。
- **校验方式**：基于 A2A SDK 官方提供的 `a2asrv.CallInterceptor` 扩展点，在请求进入具体业务 Agent 之前完成校验（鉴权失败不消耗任何 LLM/工具调用成本）；密钥比较使用 `crypto/subtle.ConstantTimeCompare` 常量时间比较，避免时序攻击。
- **协议对齐**：鉴权失败返回 `a2a.ErrUnauthenticated`，由 SDK 自动映射为标准 JSON-RPC 错误码 `-31401` 与 HTTP `401`。
- **AgentCard 如实声明**：一旦启用鉴权，对应 Agent 的 AgentCard 会在 `securitySchemes`/`securityRequirements` 中声明需要 `Bearer` 认证（符合 A2A 协议规定的发现机制），第三方客户端读取 Card 后即可提前知道要携带 Token，无需先试探性调用一次再被拒绝。
- **AgentCard 端点本身不鉴权**：按 A2A 惯例，`.well-known/agent-card.json` 保持公开可读以供发现；仅 JSON-RPC 调用端点受 Token 保护。

### 实现说明与限制

- 桥接层见 `internal/a2a/`：`executor.go` 将 A2A 消息转成文本、调用对应业务 Agent 的 `ProcessMessageStream`，流式产出 `Working` 状态事件 + 增量 `Artifact` 事件（承载回复文本），最终以 `Completed`/`Failed` 终态事件收尾（支持 `message/stream` 流式；回复内容统一在 `task.artifacts` 中，不再重复写入 `task.history` 的 Message）。
- **会话隔离**：业务 Agent 的对话历史（`BasicAgent.history`）为实例级共享，与现有 gRPC 对话层行为一致；A2A 层的多会话/多客户端并发会共享同一份历史并受 `streamDone` 串行化。如需按 A2A `contextId` 严格隔离多会话，需后续为每个上下文维护独立的 Agent 实例或历史。
- **鉴权粒度**：当前是"整个 A2A 服务一把共享密钥"，不区分调用方身份、不区分不同业务 Agent 的权限。如需按调用方分别签发/吊销凭证，或按 Agent 精细化授权，需扩展为多 Token/OAuth2 等方案。

---

## 📧 个人开发者验证码/短信验证方案

对于个人开发者来说，实现注册时的用户身份验证，有以下几种主流方案：

### 一、邮箱验证码（推荐个人开发者首选）

**原理**：服务端生成随机验证码 → 通过 SMTP 发送到用户邮箱 → 用户回填验证码 → 服务端校验

**实现方式**：

| 方案 | 成本 | 说明 |
|------|------|------|
| **个人邮箱 SMTP** | 免费 | 用 QQ邮箱/163邮箱 开启 SMTP 服务，获取授权码即可发送。适合小规模 |
| **Resend** | 免费额度 3000封/月 | 现代邮件 API，开发体验好，有 Go SDK |
| **SendGrid** | 免费额度 100封/天 | Twilio 旗下，老牌邮件服务 |
| **AWS SES** | $0.10/1000封 | 适合有 AWS 账号的开发者 |
| **Mailgun** | 免费 5000封/月（前3个月） | API 友好 |

**QQ邮箱 SMTP 配置示例**：

```yaml
smtp:
  host: "smtp.qq.com"
  port: 587
  username: "your_qq@qq.com"
  password: "你的QQ邮箱授权码"  # 不是QQ密码，是在邮箱设置中生成的授权码
```

### 二、短信验证码

**原理**：服务端调用短信服务商 API → 发送验证码到手机 → 用户回填

| 服务商 | 单价 | 说明 |
|--------|------|------|
| **腾讯云 SMS** | ~0.04元/条 | 需企业认证，个人可用但有限制 |
| **阿里云 SMS** | ~0.04元/条 | 同上 |
| **Twilio** | ~$0.05/条（国际） | 个人可用，无需企业认证，但贵 |
| **互亿无线** | 免费测试额度 | 适合开发测试 |

> ⚠️ **注意**：国内短信服务通常需要**企业资质**才能申请签名和模板，个人开发者门槛较高。

### 三、验证码的通用实现要点

无论邮箱还是短信，核心逻辑一致：

```
1. 生成验证码 → 密码学安全随机数（crypto/rand），6位数字
2. 存储验证码 → 数据库/Redis，记录：邮箱/手机号、验证码、过期时间、是否已用
3. 发送验证码 → SMTP/短信API
4. 校验验证码 → 匹配 + 未过期 + 未使用，校验后标记已用
5. 安全措施：
   - 频率限制（60秒内不重复发送）
   - 每日上限（同一邮箱/手机号每天最多 N 次）
   - 验证码有效期（5-10分钟）
   - 错误次数限制（连续错误 5 次锁定）
```

### 四、个人开发者的最佳实践建议

1. **起步阶段**：用 QQ/163 邮箱 SMTP 发验证码，零成本
2. **用户量增长后**：切换到 Resend / SendGrid 等专业邮件服务
3. **如果必须短信**：考虑 Twilio（海外用户）或腾讯云 SMS（国内用户，需企业认证）
4. **替代方案**：可以考虑 OAuth 第三方登录（微信/GitHub/Google），免去验证码环节

> 本项目已实现完整的邮箱验证码方案（业务编排见 [internal/auth/email_verify.go](internal/auth/email_verify.go)，SMTP 协议层见 [internal/auth/mailer.go](internal/auth/mailer.go)，数据访问见 [internal/store/email_verification.go](internal/store/email_verification.go)），只需在 `config.yaml` 中配置 SMTP 信息即可投入使用。

---

## 🛠️ 开发指南

### 添加新Agent

采用 **约定优于配置** 模式，新增 Agent 只需 **1 步**：

1. 在 `internal/agents/` 下创建任意名称的目录（建议与 Agent 语义相关，便于代码维护，但不作为标识使用），并在其中创建 `prompt.yaml`：

```bash
mkdir internal/agents/my_agent
```

```yaml
# internal/agents/my_agent/prompt.yaml
agent_id: "my_agent"    # Agent 唯一标识，全局必须唯一，注册与路由均以此字段为准
name: "我的Agent"
description: "Agent功能描述"
role: "business"        # 角色：system（系统级）/ business（业务级）

# 工具依赖声明（方案 B）
# - local_groups : 引用 internal/tools/local 中按组注册的本地工具组
# - mcp_servers  : 引用 config.yaml 中 mcp.servers[].id
tools:
  local_groups:
    - "general"          # 例：使用本地通用工具组
  mcp_servers: []        # 例：不依赖任何 MCP 服务（如需要则填 server id 列表）

system_prompt: |
  你是一个专业的...
```

2. **完成！** 系统启动时会通过 `internal/agents/embed.go` 的 `embed.FS` 自动发现并注册该 Agent（新增目录后需重新编译，因为 `//go:embed */prompt.yaml` 在编译期完成嵌入）。

> **约定说明**：
> - Agent 唯一标识由 `prompt.yaml` 中的 `agent_id` 字段显式声明，与所在目录名**无强制关联**（目录名可自由重命名/重构代码组织，不影响对外的 Agent ID），全局要求唯一，重复声明会在启动期直接 fail-fast
> - `prompt.yaml` 中必须包含 `agent_id`、`name`、`description`、`role`、`tools`、`system_prompt` 六个字段
> - `tools.local_groups` 与 `tools.mcp_servers` 可同时声明，一个 Agent 可混合使用本地工具组与多个 MCP 服务
> - 工具信息无需写入 Prompt，系统通过 OpenAI function calling 机制自动将已注册工具传递给 LLM
> - 路由 Agent（`role: system`，内置目录名 `intention_router`，其 `agent_id` 为 `intent_router`）的 Prompt 中可使用 `{{AGENTS_SECTION}}` 模板变量，系统会自动替换为所有业务 Agent 的列表描述
> - 无需修改任何 Go 代码；如声明了 `mcp_servers`，需在 `config.yaml` 的 `mcp.servers` 中确保对应 `id` 已配置（启动期会做严格的引用-存在性校验与 MCP 服务探活，任一失败即 fail-fast）

### 添加新的本地工具（插件化）

采用 **init() 自注册** 模式，新增工具只需在 `internal/tools/local/` 下创建文件，无需修改其他任何文件：

1. 在 `internal/tools/local/` 下创建新文件，实现 `types.Tool` 接口并通过 `init()` 自注册：

```go
package local

import (
    "github.com/hwanyan/agently/pkg/types"
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

2. **完成！** 工具会在启动时通过 `init()` 自动注册到全局注册表，`SetupLocalToolGroup` 会按 Agent `prompt.yaml` 中声明的 `tools.local_groups` 自动发现并加载。

> **工具注册机制说明**：
> - `internal/tools/local/registry.go` 维护全局工具注册表，提供 `Register(group, factory)` 和 `GetToolsByGroup(group, cfg)` 两个核心函数
> - 每个工具文件通过 `init()` 调用 `Register` 将自己的工厂函数注册到对应的工具组
> - `internal/tools/setup.go` 中的 `SetupLocalToolGroup(aiAgent, cfg, group)` 调用 `GetToolsByGroup(group, cfg)` 自动获取所有工具并注册到 Agent
> - 工具信息通过 OpenAI function calling 的 `tools` 参数传递给 LLM，无需在 Prompt 中手动描述

### 接入MCP工具服务

1. 实现一个 MCP 服务端（遵循 JSON-RPC 2.0 协议），提供：
   - `tools/list` — 返回可用工具列表
   - `tools/call` — 执行工具调用

2. 在 `config.yaml` 的 `mcp.servers` 中添加服务配置（**`id` 必填且需全局唯一**）：

```yaml
mcp:
  servers:
    - id: "my_mcp"                      # 稳定标识，被 prompt.yaml 引用
      name: "我的 MCP 服务"               # 仅用于日志/UI 展示
      base_url: "http://127.0.0.1:9090"
      api_key: ""
      timeout: 60
```

3. 在新 Agent 的 `prompt.yaml` 中通过 `tools.mcp_servers` 声明依赖（值必须与 `config.yaml` 中的 `mcp.servers[].id` 完全一致）：

```yaml
tools:
  local_groups: []
  mcp_servers:
    - "my_mcp"
```

> **fail-fast 校验**：启动时若 `prompt.yaml` 中引用的 `id` 在 `config.yaml` 中不存在，或对应的 MCP 服务无法 ListTools 探活，进程会直接退出并打印明确的错误原因，避免运行时静默降级。

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
| `google.golang.org/grpc` | gRPC 服务端/客户端运行时 |
| `google.golang.org/protobuf` | Protobuf 运行时与 JSON 编解码（protojson） |
| `github.com/grpc-ecosystem/grpc-gateway/v2` | grpc-gateway：REST/SSE ↔ gRPC 反向代理 |
| `github.com/a2aproject/a2a-go/v2` | A2A（Agent2Agent）协议 Go SDK：暴露业务 Agent 为可互操作端点 |
| `github.com/skip2/go-qrcode` | 二维码生成 |
| `github.com/golang-jwt/jwt/v5` | JWT 签发与验证 |
| `github.com/lib/pq` | PostgreSQL 驱动 |
| `golang.org/x/crypto` | bcrypt 密码哈希 |
| `golang.org/x/text` | 文本处理 |
| `gopkg.in/yaml.v2` | YAML 配置解析 |

## ⚠️ 注意事项

1. **API 密钥安全**：`config.yaml` 中包含敏感信息，请勿提交到公开仓库（已在 `.gitignore` 中排除本地覆盖配置 `config_local.yaml`）
2. **JWT 密钥**：生产环境必须在 `config.yaml` 的 `jwt.secret` 中设置强随机值，切勿使用默认值
3. **SMTP 必配**：邮箱验证服务为必配项，未配置 SMTP 时项目无法启动。推荐使用 QQ 邮箱（smtp.qq.com:587）或网易邮箱（smtp.163.com:465），需开启 SMTP 服务并获取授权码
4. **数据库依赖**：数据库不可用时项目无法启动，所有业务接口（注册/登录/会话/Agent管理）依赖数据库
5. **网络依赖**：天气、翻译、短链接等工具需要网络访问对应的 API 服务
6. **翻译服务**：默认使用 LibreTranslate 公共实例（`https://libretranslate.com`），也可自行部署私有实例并替换 `base_url`
7. **MCP服务**：黄金分析助手、新闻分析助手依赖各自的 MCP 工具服务端，如服务未启动或探活失败，启动会 fail-fast 报错
8. **日志路径**：默认日志输出到 `logs/agently-<date>.log`（`log.file_path` 为前缀），路径相对于运行目录（即执行 `go run main.go` 时的当前目录，通常是仓库根目录）
9. **Agent独立性**：每个Agent拥有独立的对话历史和工具集，切换Agent不会丢失之前的对话
10. **路由Agent**：`intention_router` 是系统Agent，不对外展示，不可手动切换到该Agent
11. **消息字段**：持久化消息中工具调用列表字段名为 `tool_call_list`（JSON 序列化后的字符串）
12. **验证码频率限制**：同一邮箱 60 秒内只能发送一次验证码，验证码有效期 5 分钟
13. **会话 Agent 同步**：自动路由模式下，会话的 `agent_id` 会随路由结果发生变化（仅对 `status=1` 活跃会话生效）；手动模式（`auto_route: false`）不会触发同步

## 📄 许可证

MIT License
