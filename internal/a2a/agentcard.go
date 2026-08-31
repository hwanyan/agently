package a2a

import (
	"github.com/a2aproject/a2a-go/v2/a2a"

	"github.com/hwanyan/agently/internal/agent"
)

// buildAgentCard 根据业务 Agent 的元信息与对外端点地址生成 A2A AgentCard。
//
// AgentCard 是 A2A 协议中 Agent 的自描述清单，供其他 Agent / 客户端发现能力、
// 能力协商与调用。每个业务 Agent 生成一张独立的 AgentCard，其 SupportedInterfaces
// 指向该 Agent 专属的 JSON-RPC 端点。
func buildAgentCard(info agent.AgentInfo, endpointURL string) *a2a.AgentCard {
	return &a2a.AgentCard{
		Name:        info.Name,
		Description: info.Description,
		Version:     "1.0.0",
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(endpointURL, a2a.TransportProtocolJSONRPC),
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Capabilities:       a2a.AgentCapabilities{Streaming: true},
		Skills: []a2a.AgentSkill{
			{
				ID:          info.ID,
				Name:        info.Name,
				Description: info.Description,
				Tags:        []string{info.ID},
			},
		},
	}
}
