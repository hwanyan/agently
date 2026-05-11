// Package llm 提供与大语言模型交互的功能
//
// 本文件定义了 OpenAI Chat Completions API 的响应解析结构体。
// 这些结构体是对 API 返回的 JSON 数据的 Go 映射，必须严格依据官方 API 文档来定义。
//
// 官方文档参考：
//   - OpenAI Chat Completions API: https://platform.openai.com/docs/api-reference/chat/create
//   - OpenAI 流式响应（Streaming）: https://platform.openai.com/docs/api-reference/chat/streaming
//
// 兼容性说明：
//
//	DeepSeek 等兼容 OpenAI 格式的 LLM 提供商也使用相同的响应结构。
//	如果接入不兼容 OpenAI 格式的 LLM（如 Anthropic Claude 原生 API），
//	则需要根据该服务的官方文档另行定义对应的解析结构。
package llm

import "strings"

// ========================================================================
// 非流式响应结构（用于 Chat Completions API 的标准响应）
// 对应 OpenAI API 文档: https://platform.openai.com/docs/api-reference/chat/object
// ========================================================================

// ChatCompletionResponse 非流式聊天补全响应
// 本质：对 OpenAI Chat Completions API 返回的完整 JSON 响应的 Go 映射。
// 响应中的核心字段是 choices 数组，每个 choice 包含一个完整的 message 对象。
//
// 示例 JSON 响应：
//
//	{
//	  "id": "chatcmpl-xxx",
//	  "object": "chat.completion",
//	  "choices": [{
//	    "message": {
//	      "role": "assistant",
//	      "content": "你好！",
//	      "tool_calls": [...]
//	    }
//	  }]
//	}
type ChatCompletionResponse struct {
	Choices []ChatCompletionChoice `json:"choices"` // 响应选择列表（通常只有一个元素）
}

// ChatCompletionChoice 非流式响应中的单个选择项
type ChatCompletionChoice struct {
	Message ChatCompletionMessage `json:"message"` // 完整的助手消息
}

// ChatCompletionMessage 非流式响应中的完整消息
// 与流式响应中的 delta 不同，这里的 message 包含完整的内容和工具调用信息。
type ChatCompletionMessage struct {
	Role      string                      `json:"role"`       // 消息角色，通常为 "assistant"
	Content   string                      `json:"content"`    // 完整的文本回复内容
	ToolCalls []ChatCompletionMessageTool `json:"tool_calls"` // 工具调用列表（模型决定调用工具时返回）
}

// ChatCompletionMessageTool 非流式响应中的工具调用信息
// 每个工具调用包含唯一 ID、类型和函数调用详情。
type ChatCompletionMessageTool struct {
	ID       string                        `json:"id"`       // 工具调用的唯一标识（如 "call_abc123"）
	Type     string                        `json:"type"`     // 调用类型，目前固定为 "function"
	Function ChatCompletionMessageToolFunc `json:"function"` // 函数调用详情
}

// ChatCompletionMessageToolFunc 非流式响应中的函数调用详情
type ChatCompletionMessageToolFunc struct {
	Name        string `json:"name"`        // 函数名称
	Description string `json:"description"` // 函数描述（部分模型会返回）
	Arguments   string `json:"arguments"`   // 函数参数（JSON 格式字符串）
}

// ========================================================================
// 流式响应结构（用于 Chat Completions API 的 SSE 流式响应）
// 对应 OpenAI API 文档: https://platform.openai.com/docs/api-reference/chat/streaming
// ========================================================================

// ChatCompletionStreamResponse 流式聊天补全响应（单个 SSE chunk）
// 本质：对 OpenAI 流式 API 中每一行 "data: {...}" 的 JSON 数据的 Go 映射。
// 与非流式响应的关键区别：使用 delta（增量）而非 message（完整消息）。
//
// 流式传输机制：
//   - 文本内容：每个 chunk 的 delta.content 包含一小段文本片段，逐步拼接形成完整回复
//   - 工具调用：分片传输，第一个 chunk 携带 id/type/function.name，
//     后续 chunk 逐步追加 function.arguments 的片段
//   - 结束标记：最后一个 chunk 的 finish_reason 为 "stop"（纯文本）或 "tool_calls"（工具调用）
//   - 流结束：收到 "data: [DONE]" 表示整个流式传输结束
//
// 示例 JSON chunk（文本内容）：
//
//	{"choices": [{"delta": {"content": "你好"}, "finish_reason": null}]}
//
// 示例 JSON chunk（工具调用首片）：
//
//	{"choices": [{"delta": {"tool_calls": [{"index": 0, "id": "call_xxx", "type": "function", "function": {"name": "get_weather", "arguments": ""}}]}, "finish_reason": null}]}
//
// 示例 JSON chunk（工具调用参数片段）：
//
//	{"choices": [{"delta": {"tool_calls": [{"index": 0, "function": {"arguments": "{\"city\":"}}]}, "finish_reason": null}]}
type ChatCompletionStreamResponse struct {
	Choices []ChatCompletionStreamChoice `json:"choices"` // 响应选择列表（通常只有一个元素）
}

// ChatCompletionStreamChoice 流式响应中的单个选择项
type ChatCompletionStreamChoice struct {
	Delta        ChatCompletionStreamDelta `json:"delta"`         // 增量内容（与非流式的 message 对应）
	FinishReason *string                   `json:"finish_reason"` // 结束原因：nil=未结束, "stop"=文本完成, "tool_calls"=工具调用完成
}

// ChatCompletionStreamDelta 流式响应中的增量数据
// 每个 chunk 只包含新增的部分，需要在客户端侧累积拼接。
type ChatCompletionStreamDelta struct {
	Content   string                         `json:"content"`    // 文本内容片段（逐步追加）
	ToolCalls []ChatCompletionStreamToolCall `json:"tool_calls"` // 工具调用片段（需按 index 累积）
}

// ChatCompletionStreamToolCall 流式响应中的工具调用片段
// 流式模式下工具调用是分片传输的：
//   - 首次出现：携带 index、id、type、function.name
//   - 后续片段：仅携带 index 和 function.arguments 的增量部分
//
// 因此需要在客户端按 index 维护一个 map，逐步累积各字段。
type ChatCompletionStreamToolCall struct {
	Index    int                              `json:"index"`    // 工具调用索引（用于区分同一响应中的多个并行工具调用）
	ID       string                           `json:"id"`       // 工具调用唯一标识（仅首次出现时非空）
	Type     string                           `json:"type"`     // 调用类型（仅首次出现时非空，固定为 "function"）
	Function ChatCompletionStreamToolCallFunc `json:"function"` // 函数调用详情片段
}

// ChatCompletionStreamToolCallFunc 流式响应中的函数调用详情片段
type ChatCompletionStreamToolCallFunc struct {
	Name      string `json:"name"`      // 函数名称（仅首次出现时非空）
	Arguments string `json:"arguments"` // 函数参数片段（每个 chunk 追加一部分，最终拼接为完整 JSON）
}

// ========================================================================
// 流式工具调用累积器（用于在流式响应处理中累积分片的工具调用信息）
// 这是为了在客户端侧处理流式工具调用分片而抽象出来的编程辅助结构
// ========================================================================

// StreamToolCallAccumulator 用于在流式响应中累积工具调用片段
// 由于 OpenAI 流式 API 将工具调用分片传输，需要在客户端侧
// 按 index 逐步累积 ID、类型、函数名和参数片段，最终汇总为完整的工具调用。
type StreamToolCallAccumulator struct {
	ID       string          // 工具调用唯一标识
	Type     string          // 调用类型
	FuncName string          // 函数名称
	FuncArgs strings.Builder // 函数参数累积器（逐步追加参数 JSON 片段）
}
