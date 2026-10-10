// Package a2a 负责将业务 Agent 通过 A2A（Agent-to-Agent）协议对外暴露。
//
// A2A 是 Google 牵头的开放协议（a2a-protocol.org），用于不同 AI Agent 之间的互操作。
// 本包基于官方 Go SDK（github.com/a2aproject/a2a-go/v2）实现服务端：
//   - executor.go   ：把 A2A 请求桥接到 agent.Manager 中的某个业务 Agent
//   - agentcard.go  ：根据 Agent 元信息生成 A2A AgentCard（自描述清单）
//   - server.go     ：A2A HTTP 服务编排，为每个业务 Agent 挂载 JSON-RPC 与 AgentCard 端点
package a2a

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"

	"github.com/hwanyan/agently/internal/agent"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/pkg/types"
)

// newAgentExecutor 创建一个 A2A AgentExecutor，将外部 A2A 消息桥接到指定的业务 Agent。
//
// 执行流程：
//  1. 从 A2A Message 中提取用户文本；
//  2. 从 agent.Manager 取出目标业务 Agent；
//  3. 先创建 Task（TaskStateSubmitted）、再下发 Working 状态事件，然后以流式方式调用 Agent，
//     把文本增量以 Artifact 事件实时透出；
//  4. 最终下发 Completed 终态事件结束任务（回复文本已由 Artifact 承载，不再重复产出 Message）。
//
// 为什么必须先创建 Task：
// SDK 内部的任务状态管理器（github.com/a2aproject/a2a-go/v2 的
// internal/taskupdate.Manager.Process）规定，一次执行产出的第一个事件必须是 *a2a.Task
// 或 *a2a.Message，否则会以 InvalidAgentResponseError（JSON-RPC code=-32006）拒绝，
// 报错形如 "processor error: execution failed before a task was created: first event
// must be a Task or a message: invalid agent response"。因此任何需要下发
// TaskStatusUpdateEvent/TaskArtifactUpdateEvent 的执行路径，都必须先 yield 一个
// a2a.NewSubmittedTask(...) 作为第一个事件（简单一次性回复、不需要走 Task 生命周期的场景，
// 例如下面"空消息"/"Agent 不存在"分支，可以直接 yield 一个裸 Message 并结束，不受此约束）。
//
// 为什么回复文本只用 Artifact、不再额外产出 Message：
// A2A 协议中 Agent 的文本输出可落在两个通道——Artifact（写入 task.artifacts，语义是"任务产物"，
// 支持 append 增量、可实现流式）与 Message（写入 task.history，语义是"对话消息"，一次性完整）。
// 二者是 Task 的不同字段，若把同一份文本同时写入这两个通道，消费方会收到两份重复内容。
// 因此这里用 Artifact 承载流式回复、以 Completed 状态收尾，既保留流式能力又避免重复。
func newAgentExecutor(mgr *agent.Manager, agentID string) a2asrv.AgentExecutor {
	return a2asrv.AgentExecutorFunc(func(ctx context.Context, ec *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
		// 在 Go 语言的 iter.Seq2[V, E] 迭代器模型中，yield 是一个由迭代器消费者（调用方）
		// 传入的回调函数，其类型为 func(V, E) bool，用于将迭代器生成的值逐个传递给调用方，
		// 并控制迭代是否继续
		return func(yield func(a2a.Event, error) bool) {
			text := extractText(ec.Message)
			if strings.TrimSpace(text) == "" {
				yield(a2a.NewMessage(a2a.MessageRoleAgent,
					a2a.NewTextPart("（收到空消息，请提供需要处理的内容）")), nil)
				return
			}

			target, ok := mgr.Get(agentID)
			if !ok {
				yield(a2a.NewMessage(a2a.MessageRoleAgent,
					a2a.NewTextPart(fmt.Sprintf("业务 Agent [%s] 不存在或未注册", agentID))), nil)
				return
			}

			// 审计日志：caller 是调用方在请求中显式声明并通过校验的 tenant_id，
			// 由 tenantAuthInterceptor 鉴权通过后写入 CallContext.User，SDK 自动透传到
			// ExecutorContext.User（见 auth.go 中 Before 方法的注释）。未启用鉴权、或该请求
			// 未经过鉴权拦截器时，ec.User 仍非 nil 但 Authenticated=false，统一记为 "anonymous"。
			caller := callerName(ec)
			logger.Info("A2A 请求：租户 [%s]，Agent [%s]，TaskID [%s]，消息：%s", caller, agentID, ec.TaskID, text)

			// 必须先创建 Task，再发送后续的状态/产物更新事件：SDK 的任务状态管理器
			// （internal/taskupdate.Manager.Process）要求一次执行的第一个事件必须是
			// *a2a.Task 或 *a2a.Message，否则会以 InvalidAgentResponseError(-32006)
			// 拒绝——错误信息形如 "processor error: execution failed before a task was
			// created: first event must be a Task or a message: invalid agent response"。
			// 此前这里直接发送 TaskStatusUpdateEvent 作为首个事件，不满足该约束，
			// 导致所有走本分支（正常处理路径）的请求必现此错误。
			if !yield(a2a.NewSubmittedTask(ec, ec.Message), nil) {
				return
			}

			// 通知客户端任务进入执行中（非终态，后续事件继续被消费）
			// 这里通过 yield 发送一个“任务执行中”的状态事件。如果 yield 返回 false，
			// 说明消费者已终止（可能因超时或客户端断开），迭代器立即 return，停止后续事件处理
			if !yield(a2a.NewStatusUpdateEvent(ec, a2a.TaskStateWorking, nil), nil) {
				return
			}

			stream, err := target.ProcessMessageStream(ctx, text)
			if err != nil {
				yield(a2a.NewStatusUpdateEvent(ec, a2a.TaskStateFailed,
					a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("处理失败: "+err.Error()))), nil)
				return
			}

			var full strings.Builder
			var artifactID a2a.ArtifactID
			var fatalErr string
			firstChunk := true

			/* 可以把 iter.Seq2 想象成一个生产-消费管道：
			管道内的 yield 是“投递口”，事件通过它送到消费者手中。
			消费者的循环体每执行一次，就取走一个事件；若消费者不再取（break），则投递口关闭，生产者停止生产。
			因此，yield 是连接生产者（Agent 处理逻辑）与消费者（A2A 服务器）的桥梁，是流式事件驱动的核心机制。
			*/
			for ev := range stream {
				switch ev.Type {
				case types.StreamEventContent:
					full.WriteString(ev.Content)
					// 流式场景下以 Artifact 增量事件实时透出文本
					var event *a2a.TaskArtifactUpdateEvent
					if firstChunk {
						event = a2a.NewArtifactEvent(ec, a2a.NewTextPart(ev.Content))
						artifactID = event.Artifact.ID
						firstChunk = false
					} else {
						event = a2a.NewArtifactUpdateEvent(ec, artifactID, a2a.NewTextPart(ev.Content))
					}
					// 每次从底层 stream 收到一个内容块时，通过 yield 发送一个 ArtifactUpdateEvent。
					// 同样，如果消费者返回 false，迭代器终止，不会继续等待后续流数据。
					if !yield(event, nil) {
						return
					}
				case types.StreamEventError:
					// 致命错误（如 LLM 调用失败）：记录错误信息，循环结束后以失败终态收尾，
					// 而不是把错误文本当作正常回复写入 Artifact。
					fatalErr = ev.Error
				case types.StreamEventToolCall, types.StreamEventToolResult, types.StreamEventToolError:
					// 工具调用属于过程事件，不计入最终回复文本。这里把它作为一个"不带消息的 Working
					// 状态更新"透出，调用过程信息放在事件的 metadata 扩展字段里（键名见
					// toolEventMetaKey）：状态更新里没有 message/文本，因此不会混进回复正文；
					// 不认识该扩展的 A2A 客户端会直接忽略 metadata，兼容性不受影响。
					// Working 属于非终态，SDK 允许在 Completed 之前重复下发。
					toolEv := a2a.NewStatusUpdateEvent(ec, a2a.TaskStateWorking, nil)
					toolEv.Metadata = toolEventMetadata(ev)
					if !yield(toolEv, nil) {
						return
					}
				}
			}

			// 致命错误：以失败终态结束任务
			if fatalErr != "" {
				logger.Warning("A2A 请求处理失败：租户 [%s]，Agent [%s]，TaskID [%s]，原因：%s", caller, agentID, ec.TaskID, fatalErr)
				yield(a2a.NewStatusUpdateEvent(ec, a2a.TaskStateFailed,
					a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("处理失败: "+fatalErr))), nil)
				return
			}

			// 无任何文本产出时兜底，避免任务"空转"结束。
			// 注意：此时 Task 已经创建（mgr.lastStored 非空），不能再像顶部两个早退分支那样
			// 直接 yield 一个裸 Message——SDK 的任务状态管理器规定"消息事件不允许出现在
			// Task 已落库之后"（会报 "message not allowed after task was stored"），
			// 因此这里必须和上下两个分支保持一致，用 StatusUpdateEvent 收尾，
			// 把提示文本放进其 Status.Message 字段。
			if full.Len() == 0 {
				yield(a2a.NewStatusUpdateEvent(ec, a2a.TaskStateCompleted,
					a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("（未产生文本回复）"))), nil)
				return
			}

			// 正常结束：下发 Completed 终态事件。回复文本已由上面的 Artifact 增量事件完整承载，
			// 此处不再产出内容重复的 Message，避免消费方在 artifacts 与 history 中收到两份相同内容。
			logger.Info("A2A 请求处理完成：租户 [%s]，Agent [%s]，TaskID [%s]，回复长度：%d", caller, agentID, ec.TaskID, full.Len())
			yield(a2a.NewStatusUpdateEvent(ec, a2a.TaskStateCompleted, nil), nil)
		}
	})
}

// toolEventMetaKey 是工具调用过程事件在 A2A 状态更新事件 metadata 中使用的扩展键。
// 取值是一个对象：{"phase": "start"|"success"|"error", "name": 工具名, "arguments": 入参 JSON 字符串}。
// 调用方（如 agenteam 的 internal/a2a 客户端）按同一约定解析，用于在界面上展示"调用了哪个工具"。
// 只透出工具名与入参，不透出工具执行结果（结果可能很大，也可能含敏感数据，且界面不需要）。
const toolEventMetaKey = "toolEvent"

// toolEventMetadata 把一个工具调用过程事件转换为 A2A 事件 metadata。
func toolEventMetadata(ev types.StreamEvent) map[string]any {
	phase := "start"
	switch ev.Type {
	case types.StreamEventToolResult:
		phase = "success"
	case types.StreamEventToolError:
		phase = "error"
	}
	payload := map[string]any{"phase": phase, "name": ev.ToolName}
	if ev.Type == types.StreamEventToolCall && ev.ToolArgs != "" {
		payload["arguments"] = ev.ToolArgs
	}
	return map[string]any{toolEventMetaKey: payload}
}

// callerName 从 ExecutorContext.User 中提取调用方标识（tenant_id），用于审计日志。
// User 由 tenantAuthInterceptor.Before 鉴权通过后设置并被 SDK 透传到此处（见 auth.go）；
// 未启用鉴权、或该请求未经过鉴权拦截器时，User.Authenticated 为 false，统一记为 "anonymous"。
func callerName(ec *a2asrv.ExecutorContext) string {
	if ec.User != nil && ec.User.Authenticated && ec.User.Name != "" {
		return ec.User.Name
	}
	return "anonymous"
}

// extractText 从 A2A Message 的 Parts 中提取全部文本内容
func extractText(msg *a2a.Message) string {
	if msg == nil {
		return ""
	}
	var b strings.Builder
	for _, p := range msg.Parts {
		if p == nil {
			continue
		}
		if t := p.Text(); t != "" {
			b.WriteString(t)
		}
	}
	return b.String()
}
