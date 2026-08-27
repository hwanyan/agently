// Package llm 提供与大语言模型交互的功能
package llm

import (
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/pkg/types"
)

// NewClientFromConfig 根据应用配置构造 LLM 客户端
// 当前实现仅支持 OpenAI 协议的客户端；未来如需支持多 provider，
// 可在此处根据 cfg.LLM 的字段进行分支选择，对调用方保持透明。
//
// provider 支持配置热更新：客户端内部持有 provider 而非固定的 API Key/Model/BaseURL，
// 每次 Chat/ChatStream 调用时都会读取 config.yaml 变更后最新生效的 LLM 配置。
func NewClientFromConfig(provider types.ConfigProvider) types.LLMClient {
	client := NewOpenAIClient(provider)
	logger.Info("LLM 客户端创建成功")
	return client
}
