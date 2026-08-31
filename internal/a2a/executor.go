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
//  3. 先下发 Working 状态事件，再以流式方式调用 Agent，把文本增量以 Artifact 事件实时透出；
//  4. 最终下发 Completed 终态事件结束任务（回复文本已由 Artifact 承载，不再重复产出 Message）。
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

			logger.Info("A2A 请求：Agent [%s]，消息：%s", agentID, text)

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
			default:
				// tool_call / tool_result / tool_error 属于过程事件，不计入最终回复；
				// 如需要在流式过程透出工具调用，可在此扩展为额外的 Artifact 事件。
			}
		}

		// 致命错误：以失败终态结束任务
		if fatalErr != "" {
			yield(a2a.NewStatusUpdateEvent(ec, a2a.TaskStateFailed,
				a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("处理失败: "+fatalErr))), nil)
			return
		}

		// 无任何文本产出时兜底，避免任务"空转"结束
		if full.Len() == 0 {
			yield(a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("（未产生文本回复）")), nil)
			return
		}

		// 正常结束：下发 Completed 终态事件。回复文本已由上面的 Artifact 增量事件完整承载，
		// 此处不再产出内容重复的 Message，避免消费方在 artifacts 与 history 中收到两份相同内容。
		yield(a2a.NewStatusUpdateEvent(ec, a2a.TaskStateCompleted, nil), nil)
		}
	})
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
