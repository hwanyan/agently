# DeepSeek LLM 客户端使用指南

## 概述

本项目提供了一个完整的DeepSeek API客户端实现，支持与DeepSeek AI模型进行交互。DeepSeek是一个高性能、成本效益的大语言模型，特别在中文理解和代码生成方面表现优异。

## 功能特性

### 🚀 核心功能
- **标准对话**：支持单轮和多轮对话
- **流式响应**：实时流式输出，提升用户体验
- **工具调用**：完整支持Function Calling功能
- **多模型支持**：支持deepseek-chat、deepseek-coder等多个模型

### 💰 成本优势
- **Token估算**：内置智能Token计算
- **成本预估**：实时计算API调用成本
- **高性价比**：相比OpenAI更具成本优势

### 🔧 技术特性
- **接口兼容**：与OpenAI客户端接口完全兼容
- **错误处理**：完善的错误处理和重试机制
- **配置灵活**：支持多种配置方式
- **类型安全**：完整的Go类型定义

## 快速开始

### 1. 环境准备

```bash
# 设置DeepSeek API密钥
export DEEPSEEK_API_KEY="your-deepseek-api-key"
```

### 2. 基本使用

```go
package main

import (
    "context"
    "fmt"
    "log"

    "ai-agent-tutorial/internal/llm"
    "ai-agent-tutorial/pkg/types"
)

func main() {
    // 创建客户端
    client := llm.NewDeepSeekClient("your-api-key", "deepseek-chat", "")
    
    // 准备消息
    messages := []types.Message{
        {
            Role:    "user",
            Content: "你好，请介绍一下DeepSeek AI。",
        },
    }
    
    // 发送请求
    ctx := context.Background()
    response, err := client.Chat(ctx, messages, nil)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("DeepSeek:", response.Content)
}
```

### 3. 运行示例

```bash
# 运行DeepSeek演示程序
go run -tags ignore examples/deepseek_demo.go
```

## 详细功能

### 多轮对话

```go
func multiTurnChat() {
    client := llm.NewDeepSeekClient(apiKey, "deepseek-chat", "")
    
    messages := []types.Message{
        {Role: "user", Content: "我想学习Go语言"},
    }
    
    // 第一轮
    response1, _ := client.Chat(ctx, messages, nil)
    messages = append(messages, *response1)
    
    // 第二轮
    messages = append(messages, types.Message{
        Role: "user", 
        Content: "从哪里开始学习比较好？",
    })
    
    response2, _ := client.Chat(ctx, messages, nil)
    fmt.Println(response2.Content)
}
```

### 流式响应

```go
func streamChat() {
    client := llm.NewDeepSeekClient(apiKey, "deepseek-chat", "")
    
    messages := []types.Message{
        {Role: "user", Content: "写一首关于AI的诗"},
    }
    
    stream, err := client.ChatStream(ctx, messages, nil)
    if err != nil {
        log.Fatal(err)
    }
    
    for chunk := range stream {
        fmt.Print(chunk)
    }
}
```

### 代码生成

```go
func codeGeneration() {
    // 使用代码专用模型
    client := llm.NewDeepSeekClient(apiKey, "deepseek-coder", "")
    
    messages := []types.Message{
        {
            Role: "user",
            Content: "用Go实现一个HTTP服务器",
        },
    }
    
    response, _ := client.Chat(ctx, messages, nil)
    fmt.Println("生成的代码:")
    fmt.Println(response.Content)
}
```

## 支持的模型

| 模型名称 | 描述 | 适用场景 |
|---------|------|----------|
| `deepseek-chat` | 通用对话模型 | 日常对话、问答、文本生成 |
| `deepseek-coder` | 代码专用模型 | 代码生成、代码解释、编程助手 |
| `deepseek-math` | 数学专用模型 | 数学问题、公式推导、计算 |
| `deepseek-reasoner` | 推理专用模型 | 逻辑推理、复杂问题分析 |

## 配置选项

### 客户端配置

```go
// 基本配置
client := llm.NewDeepSeekClient(
    "your-api-key",           // API密钥
    "deepseek-chat",          // 模型名称
    "https://api.deepseek.com/v1", // API基础URL（可选）
)

// 运行时配置
client.SetModel("deepseek-coder")  // 切换模型
client.SetBaseURL("custom-url")    // 设置自定义URL
```

### 环境变量配置

```bash
export DEEPSEEK_API_KEY="your-api-key"
export DEEPSEEK_MODEL="deepseek-chat"
export DEEPSEEK_BASE_URL="https://api.deepseek.com/v1"
```

## 成本估算

```go
// Token估算
tokenCount := client.EstimateTokens("你的文本内容")
fmt.Printf("预估Token数: %d\n", tokenCount)

// 成本计算
cost := client.CalculateCost(promptTokens, completionTokens)
fmt.Printf("预估成本: $%.6f\n", cost)
```

## 错误处理

```go
response, err := client.Chat(ctx, messages, nil)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "API错误"):
        log.Printf("API调用失败: %v", err)
    case strings.Contains(err.Error(), "网络"):
        log.Printf("网络连接问题: %v", err)
    default:
        log.Printf("未知错误: %v", err)
    }
    return
}
```

## 最佳实践

### 1. 模型选择
- **通用对话**：使用 `deepseek-chat`
- **代码相关**：使用 `deepseek-coder`
- **数学计算**：使用 `deepseek-math`
- **复杂推理**：使用 `deepseek-reasoner`

### 2. 性能优化
- 合理控制对话历史长度
- 使用流式响应提升用户体验
- 批量处理多个请求时注意并发控制

### 3. 成本控制
- 定期检查Token使用量
- 根据需求选择合适的模型
- 利用缓存减少重复请求

### 4. 错误处理
- 实现重试机制
- 设置合理的超时时间
- 记录详细的错误日志

## 与OpenAI客户端的对比

| 特性 | DeepSeek | OpenAI |
|------|----------|--------|
| **成本** | 更低 | 较高 |
| **中文支持** | 优秀 | 良好 |
| **代码生成** | 专业模型 | 通用模型 |
| **API兼容性** | 完全兼容 | 标准接口 |
| **响应速度** | 快速 | 快速 |

## 故障排除

### 常见问题

1. **API密钥错误**
   ```
   错误: DeepSeek API错误 [invalid_api_key]: Invalid API key
   解决: 检查DEEPSEEK_API_KEY环境变量是否正确设置
   ```

2. **网络连接问题**
   ```
   错误: 发送HTTP请求失败: dial tcp: i/o timeout
   解决: 检查网络连接，考虑使用代理
   ```

3. **模型不存在**
   ```
   错误: DeepSeek API错误 [model_not_found]: Model not found
   解决: 检查模型名称是否正确，使用GetSupportedModels()查看支持的模型
   ```

### 调试技巧

```go
// 启用详细日志
client := llm.NewDeepSeekClient(apiKey, model, baseURL)

// 测试连接
if err := client.TestConnection(ctx); err != nil {
    log.Printf("连接测试失败: %v", err)
}

// 验证配置
if err := client.ValidateConfig(); err != nil {
    log.Printf("配置验证失败: %v", err)
}
```

## 更多资源

- [DeepSeek官方文档](https://platform.deepseek.com/docs)
- [API参考](https://platform.deepseek.com/api-docs)
- [模型介绍](https://deepseek.com/models)
- [定价信息](https://platform.deepseek.com/pricing)

## 贡献

欢迎提交Issue和Pull Request来改进这个DeepSeek客户端实现！