package a2a

import (
	"github.com/a2aproject/a2a-go/v2/a2a"

	"github.com/hwanyan/agently/internal/agent"
)

// bearerSecuritySchemeName 是本项目在 AgentCard.SecuritySchemes 中声明 Bearer 鉴权方案时使用的 key，
// 与 SecurityRequirements 中引用的 key 必须一致。
const bearerSecuritySchemeName = "bearerAuth"

// buildAgentCard 根据业务 Agent 的元信息与对外端点地址生成 A2A AgentCard。
//
// AgentCard 是 A2A 协议中 Agent 的自描述清单，供其他 Agent / 客户端发现能力、
// 能力协商与调用。每个业务 Agent 生成一张独立的 AgentCard，其 SupportedInterfaces
// 指向该 Agent 专属的 JSON-RPC 端点。
//
// 参数 authEnabled 为 true 时，会在 Card 中声明 securitySchemes/securityRequirements，
// 显式告知第三方客户端"调用该端点必须携带 Authorization: Bearer <token>"——这是 A2A 协议
// 规定的标准发现机制（对应 OpenAPI 3.0 Security Scheme/Requirement Object），客户端可据此
// 在真正发起调用前就知道要带什么凭证，而不是先试探性调用一次再被 401 拒绝。
func buildAgentCard(info agent.AgentInfo, endpointURL string, authEnabled bool) *a2a.AgentCard {
	card := &a2a.AgentCard{
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

	if authEnabled {
		card.SecuritySchemes = a2a.NamedSecuritySchemes{
			bearerSecuritySchemeName: a2a.HTTPAuthSecurityScheme{
				Scheme:      "Bearer",
				Description: "调用方需在 Authorization 请求头携带共享密钥：Authorization: Bearer <token>",
			},
		}
		card.SecurityRequirements = a2a.SecurityRequirementsOptions{
			a2a.SecurityRequirements{bearerSecuritySchemeName: {}},
		}
	}

	return card
}
