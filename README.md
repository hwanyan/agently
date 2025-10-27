# Agent-Hwan 项目

这是一个完整的基于 Go 实现的 AI Agent 项目，它含有多个 AI 工具，包括：

1. 天气查询工具 ☁️
2. IP信息查询工具 🌍
3. 二维码生成工具 🔢
4. 翻译工具 🔡
5. 文本处理工具 ✏️
6. 短链接生成工具 🔗（待实现）
7. 图片处理工具 🖼️（待实现）

## 📦 工具总揽

1. 天气查询工具 ☁️

- 工具名：qweather
- 功能：查询指定城市的当前天气信息，包括温度、湿度、风力、天气状况等
- 典型场景：查询北京/上海等城市的实时天气、风向风速、能见度等

参数
- city（必需）：要查询的城市名称，支持中文或拼音（例如 北京 / beijing）

示例
- “北京今天天气怎么样？”
- “上海今天天气如何？”


2. IP信息查询工具 🌍

- 工具名：ipapi
- 功能：查询与 IP 或域名相关的地理位置与网络信息（国家/地区/城市/经纬度/运营商等）；付费版可返回时区、货币、连接与安全信息
- 典型场景：定位 IP 地址来源、查询域名的归属与运营商信息

参数
- target（必需）：要查询的 IP 地址或域名（例如 8.8.8.8 / google.com）

示例
- “查询 8.8.8.8 的IP信息”
- “查询 google.com 的位置”

3. 二维码生成工具 🔢

- 工具名：qrcode
- 功能：将文本内容生成二维码，支持三种输出方式：
  - dataurl：返回 PNG 的 Base64 Data URL，便于在网页中直接显示
  - ascii：在终端中以字符画方式预览二维码
  - file：将二维码保存为 PNG 文件
- 典型场景：分享链接、在终端预览二维码、保存二维码图片分发或嵌入文档

参数
- text（必需）：要编码为二维码的文本/URL
- size（可选，默认 256）：二维码图片尺寸（像素，64~1024）
- level（可选，默认 M）：容错级别，L/M/Q/H
- margin（可选，默认 4）：边距像素
- output（可选，默认 dataurl）：输出格式，可选 dataurl | ascii | file
- filename（可选）：当 output=file 时保存的文件名（如 qr.png）

示例
- 生成 Data URL：生成一个内容为 https://example.com 的二维码
- 终端预览：请在终端展示这个二维码（将采用 ascii 输出）
- 保存文件：请将这个二维码保存为 png 图片，文件名设为 my_qr.png


4. 翻译工具 🔡

- 工具名：translate
- 功能：将给定文本翻译为指定目标语言，支持可选源语言与格式（plain/html）。基于 LibreTranslate API。
- 典型场景：中英互译、技术文档段落翻译、HTML 片段翻译

参数
- text（必需）：需要翻译的原文文本
- target（必需）：目标语言代码（如 zh, en, ja, ko, fr, de, es 等）
- source（可选）：源语言代码，不传则自动检测
- format（可选，默认 plain）：文本格式 plain | html

示例
- 中译英：请把“今天天气不错，适合散步。”翻译为英文
- 自动检测：把 “Bonjour tout le monde” 翻成中文
- HTML：将 <p>你好，世界</p> 翻译为英文，并保持 HTML 结构





## 🎯 项目概述

本项目演示了如何构建一个现代化的AI Agent系统，包含以下核心功能：

- 🤖 **智能对话** - 与大语言模型进行自然语言交互
- 🔧 **工具调用** - Agent可以调用外部工具执行具体任务
- 🌤️ **信息查询** - 天气查询工具，获取实时信息
- 🌐 **IP地址查询** - IP信息查询工具，获取地理位置和网络信息
- 💬 **对话管理** - 完整的对话历史管理和上下文维护
- ⚙️ **配置管理** - 灵活的配置系统，支持文件和环境变量

## 🏗️ 系统架构

```
agent-hwan/
├── cmd/                           # 主程序入口
│   └── main.go                   # 程序启动文件
├── internal/                     # 内部模块
│   ├── agent/                    # Agent核心实现
│   │   └── agent.go             # 基础Agent类
│   ├── config/                   # 配置管理
│   │   └── config.go            # 配置加载和验证
│   ├── llm/                      # 大语言模型客户端
│   │   ├── openai.go            # OpenAI/DeepSeek API客户端
│   │   └── stream_scanner.go    # 流式响应处理
│   ├── prompt/                   # 系统提示词管理
│   │   └── prompt.yaml          # 系统提示词配置
│   └── tools/                    # 工具实现
│       ├── weather.go           # 天气查询工具（和风天气API）
│       ├── ip.go                # IP地址查询工具（ipapi.com API）
│       ├── qrcode.go            # 二维码生成工具
│       ├── translation.go       # 翻译工具（LibreTranslate API）
│       ├── textprocessor.go     # 文本处理工具
│       └── registry.go          # 工具注册表
├── README/                       # 项目文档
│   ├── 问题答疑.md               # 技术问答文档
│   ├── 使用指南.md               # 详细使用指南
│   ├── 项目总结.md               # 项目总结和学习成果
│   └── AI agent 系统架构.md     # 系统架构详细说明
├── .gitignore                    # Git忽略文件
├── go.mod                        # Go模块文件
├── go.sum                        # Go依赖锁定文件
└── README.md                     # 项目主说明
```

### 🔍 核心模块说明

#### 1. 主程序入口 (`cmd/`)
- `main.go`: 程序启动入口，包含完整的初始化流程和用户交互界面

#### 2. Agent核心 (`internal/agent/`)
- `agent.go`: 实现Agent的核心逻辑，包括消息处理、工具调用和对话管理

#### 3. 配置管理 (`internal/config/`)
- `config.go`: 统一的配置管理，支持环境变量、JSON配置文件和YAML提示词配置

#### 4. LLM客户端 (`internal/llm/`)
- `openai.go`: 兼容OpenAI和DeepSeek API的客户端实现
- `stream_scanner.go`: 处理流式响应的扫描器

#### 5. 提示词管理 (`internal/prompt/`)
- `prompt.yaml`: 系统提示词配置文件，支持灵活的提示词管理

#### 6. 工具系统 (`internal/tools/`)
- `weather.go`: 天气查询工具，集成和风天气API
- `ip.go`: IP地址查询工具，集成ipapi.com API
- `qrcode.go`: 二维码生成工具，支持多种输出格式
- `translation.go`: 翻译工具，集成LibreTranslate API
- `textprocessor.go`: 文本处理工具，提供文本统计、格式转换、内容提取等功能
- `registry.go`: 工具注册表，统一管理所有工具

#### 7. 项目文档 (`README/`)
- `问题答疑.md`: 技术决策和常见问题解答
- `使用指南.md`: 详细的使用指南和交互示例
- `项目总结.md`: 项目完成情况和学习成果总结
- `AI agent 系统架构.md`: 详细的系统架构说明

## 🚀 快速开始

### 1. 环境准备

确保你的系统已安装：
- Go 1.19 或更高版本
- Git

### 2. 获取代码

```bash
# 克隆项目
git clone <repository-url>
cd agent-hwan

# 安装依赖
go mod tidy
```

### 3. 配置API密钥

在运行程序之前，你需要设置API密钥。有两种方式：

**方式一：环境变量（推荐）**
```bash
# LLM API密钥（必需）
export LLM_API_KEY="your-openai-api-key-here"
export LLM_MODEL="gpt-3.5-turbo"  # 可选，默认为gpt-3.5-turbo

# 天气API密钥（可选）
export QWEATHER_API_KEY="your-qweather-api-key"

# IP查询API密钥（可选）
export IPAPI_ACCESS_KEY="your-ipapi-access-key"

# 翻译工具配置（可选）
export TRANSLATE_BASE_URL="https://libretranslate.com"  # 翻译服务地址，默认使用公共实例
export TRANSLATE_API_KEY="your-translate-api-key"       # 翻译API密钥，使用付费服务时需要
```

**方式二：配置文件**
创建 `internal/config/config.json` 文件：
```json
{
  "llm": {
    "provider": "openai",
    "api_key": "your-openai-api-key-here",
    "model": "gpt-3.5-turbo",
    "base_url": ""
  },
  "agent": {
    "name": "智能助手",
    "description": "一个能够使用工具的智能AI助手",
    "max_tokens": 4096,
    "temperature": 0.7
  }
}
```

### 4. 运行程序

```bash
# 直接运行
go run cmd/main.go

# 或者编译后运行
go build -o ai-agent cmd/main.go
./ai-agent
```

### 5. 程序启动界面

启动程序后，你会看到如下界面：

```
========== 🤖 AI Agent 助手 ==========

📋 步骤1: 加载配置...
✅ 配置加载成功！

🔗 步骤2: 创建LLM客户端...
✅ LLM客户端创建成功！

🔗 步骤3: 注册工具信息...
✅ 工具信息注册成功！

📋 步骤4: 获取全部的 Prompt 配置...
✅ Prompt配置获取成功！

🧠 步骤5: 创建AI Agent...
✅ AI Agent '智能助手' 创建成功！

🔧 步骤6: 添加工具...
    🌤️  添加天气工具...
    ✓ 天气工具已添加
    🌐 添加IP查询工具...
    ✓ IP查询工具已添加
  📊 总共添加了 2 个工具
✅ 所有工具添加完成！

🎯 步骤7: 开始交互...

🎉 AI Agent已准备就绪！

💡 你可以尝试以下问题：
   • 北京的天气怎么样？
   • 上海今天天气如何？
   • 查询 8.8.8.8 的IP信息
   • 查询 google.com 的位置
   • 什么是AI Agent？

🔧 可用命令：
   • 'help' - 显示帮助信息
   • 'history' - 查看对话历史
   • 'clear' - 清空对话历史
   • 'tools' - 查看可用工具
   • 'quit' 或 'exit' - 退出程序

============================================================

👤 用户: 
```

## 📚 核心概念详解

### 1. Agent架构

AI Agent采用了经典的"感知-思考-行动"循环：

```go
type Agent interface {
    ProcessMessage(ctx context.Context, userMessage string) (string, error)
    AddTool(tool Tool)
    GetConversationHistory() []Message
}
```

**核心流程：**
1. **接收用户输入** - 获取用户的问题或指令
2. **分析和理解** - 使用LLM理解用户意图
3. **决策** - 判断是否需要使用工具
4. **执行工具** - 如果需要，调用相应的工具
5. **生成响应** - 基于工具结果生成最终回答

### 2. 工具系统

工具是Agent扩展能力的核心机制：

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]interface{}
    Execute(ctx context.Context, args map[string]interface{}) (string, error)
}
```

**工具特点：**
- **标准化接口** - 所有工具实现统一接口
- **参数验证** - 支持JSON Schema参数定义
- **错误处理** - 完善的错误处理机制
- **可扩展性** - 易于添加新工具

### 3. LLM集成

支持多种大语言模型：

```go
type LLMClient interface {
    Chat(ctx context.Context, messages []Message, tools []Tool) (*Message, error)
    ChatStream(ctx context.Context, messages []Message, tools []Tool) (<-chan string, error)
}
```

**特性：**
- **流式响应** - 支持实时流式输出
- **工具调用** - 自动处理工具调用协议
- **上下文管理** - 维护完整的对话历史

## 🔧 内置工具

### 1. 天气查询工具

查询城市天气信息：

```go
// 使用示例
用户: "北京今天天气怎么样？"
助手: "北京今天天气：晴朗，温度25°C，湿度60%"
```

**功能：**
- 城市天气查询
- 支持中文城市名称
- 集成和风天气API

### 2. IP地址查询工具

查询IP地址的地理位置和网络信息：

```go
// 使用示例
用户: "查询 8.8.8.8 的IP信息"
助手: "IP地址 8.8.8.8 位于美国加利福尼亚州山景城，运营商为Google LLC"
```

**功能：**
- IP地址地理定位
- 域名解析
- 网络信息查询
- 支持IPv4和IPv6

## 💡 使用示例

### 基础对话

```
👤 用户: 你好！
🤖 助手: 你好！我是智能助手，可以帮你解决各种问题。我能查询天气信息、查询IP地址信息等。有什么可以帮助你的吗？
```

### 天气查询

```
👤 用户: 北京今天天气怎么样？

🤖 助手: 我来为你查询北京的天气信息。

根据查询结果，北京今天的天气情况如下：
- 天气状况：晴朗
- 温度：22°C
- 湿度：45%
- 风速：5km/h

天气不错，适合外出活动！
```

### IP地址查询

```
👤 用户: 查询 8.8.8.8 的IP信息

🤖 助手: 我来为你查询这个IP地址的信息。

根据查询结果，IP地址 8.8.8.8 的信息如下：
- IP地址：8.8.8.8
- 国家：美国
- 地区：加利福尼亚州
- 城市：山景城
- 运营商：Google LLC
- 坐标：纬度 37.4056, 经度 -122.0775

这是Google的公共DNS服务器地址。
```

## 🔧 可用命令详解

### help - 帮助信息
显示完整的帮助信息，包括可用工具和命令说明。

### history - 对话历史
查看完整的对话历史记录，包括用户输入和AI响应。

### tools - 工具列表
显示所有可用工具的详细信息，包括工具名称、描述和参数。

### clear - 清空历史
清空当前会话的对话历史记录。

### quit/exit - 退出程序
安全退出程序。

## 🛠️ 开发指南

### 添加新工具

1. **创建工具结构体**：
```go
type MyTool struct {
    name        string
    description string
}
```

2. **实现Tool接口**：
```go
func (t *MyTool) Name() string { return t.name }
func (t *MyTool) Description() string { return t.description }
func (t *MyTool) Parameters() map[string]interface{} { /* 参数定义 */ }
func (t *MyTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    // 工具逻辑实现
}
```

3. **注册工具**：
```go
myTool := NewMyTool()
agent.AddTool(myTool)
```

### 扩展LLM支持

1. **实现LLMClient接口**：
```go
type MyLLMClient struct {
    // 客户端配置
}

func (c *MyLLMClient) Chat(ctx context.Context, messages []Message, tools []Tool) (*Message, error) {
    // 实现聊天逻辑
}
```

2. **集成到Agent**：
```go
llmClient := NewMyLLMClient(config)
agent := agent.NewBasicAgent("助手", "描述", llmClient, systemPrompt)
```

## 🔍 技术特点

### 1. 并发安全
- 使用读写锁保护共享状态
- 支持多协程并发访问
- 线程安全的消息处理

### 2. 错误处理
- 完善的错误处理机制
- 详细的错误信息
- 优雅的错误恢复

### 3. 可扩展性
- 模块化设计
- 插件式工具系统
- 易于添加新功能

### 4. 配置管理
- 支持多种配置源
- 配置验证
- 环境变量支持

## 📖 学习路径

### 初学者
1. 理解基本概念（Agent、Tool、LLM）
2. 运行示例程序
3. 尝试添加简单工具
4. 修改系统提示词

### 进阶开发者
1. 深入理解架构设计
2. 实现复杂工具
3. 集成其他LLM服务
4. 优化性能和错误处理

### 高级开发者
1. 扩展Agent能力
2. 实现分布式部署
3. 添加监控和日志
4. 构建生产级系统

## 🚀 扩展建议

### 初级扩展
1. **添加新工具**
   - 文件操作工具
   - 时间日期工具
   - 单位转换工具
   - 二维码生成工具
   - 翻译工具

2. **改进现有功能**
   - 优化天气查询显示
   - 增加IP查询的安全信息

### 中级扩展
1. **数据持久化**
   - 对话历史存储
   - 用户偏好记录
   - 工具使用统计

2. **Web界面**
   - HTTP API服务
   - Web聊天界面
   - 实时消息推送

### 高级扩展
1. **多Agent协作**
   - Agent间通信
   - 任务分发机制
   - 协作决策

2. **生产级特性**
   - 监控和日志
   - 性能优化
   - 安全加固

## 🤝 贡献指南

欢迎贡献代码！请遵循以下步骤：

1. Fork项目
2. 创建功能分支
3. 提交更改
4. 创建Pull Request

## ⚠️ 注意事项

1. **API密钥**：确保设置了有效的LLM API密钥
2. **网络连接**：需要稳定的网络连接访问各种API
3. **API费用**：使用第三方API可能会产生费用，请注意使用量
4. **响应时间**：首次响应可能需要几秒钟时间

## 🐛 常见问题

### 问题1：程序启动失败
```
❌ 加载配置失败: 未找到LLM API密钥
```
**解决方案**：设置环境变量 `LLM_API_KEY` 或修改配置文件

### 问题2：API调用失败
```
❌ 处理消息时出错: LLM调用失败: API请求失败，状态码: 401
```
**解决方案**：检查API密钥是否正确，是否有足够的API配额

### 问题3：工具调用失败
```
❌ 处理消息时出错: 处理工具调用失败: 工具执行失败
```
**解决方案**：检查工具参数是否正确，网络连接是否正常

## 🎓 学习建议

1. **从简单开始**：先尝试基本的交互功能
2. **观察工具调用**：注意AI如何决定使用哪个工具
3. **实验不同问题**：尝试各种类型的问题，观察AI的响应
4. **查看历史记录**：使用 `history` 命令了解对话流程
5. **阅读代码**：深入理解每个模块的实现原理

## 🎉 项目特色

- ✅ **完整的 AI Agent 实现** - 包含感知、决策、执行完整流程
- 🔧 **多样化工具系统** - 内置天气查询、IP查询工具
- 💬 **自然对话** - 支持流式响应和上下文管理
- 📚 **详细文档** - 从入门到高级的完整学习资料
- 🛠️ **可扩展架构** - 易于添加新工具和功能
- 🌐 **多API集成** - 支持多种第三方服务集成

## 📄 许可证

本项目采用MIT许可证。

---

**开始您的 AI Agent 开发之旅吧！🎉**

*如果你觉得这个项目有帮助，请给项目一个⭐️，并分享给其他学习者！*