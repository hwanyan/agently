package a2a

import (
	"github.com/a2aproject/a2a-go/v2/a2a"

	"github.com/hwanyan/agently/internal/agent"
)

// AgentCard.SecuritySchemes 中声明的两个安全方案 key，与 SecurityRequirements 中引用的 key 必须一致。
// tenantIdSecuritySchemeName 对应 X-A2A-Tenant-Id 头（APIKeySecurityScheme），
// bearerSecuritySchemeName 对应 Authorization: Bearer <token>（HTTPAuthSecurityScheme）。
const (
	tenantIdSecuritySchemeName = "tenantId"
	bearerSecuritySchemeName   = "bearerAuth"
)

// buildAgentCard 根据业务 Agent 的元信息与对外端点地址生成 A2A AgentCard。
//
// AgentCard 是 A2A 协议中 Agent 的自描述清单，供其他 Agent / 客户端发现能力、
// 能力协商与调用。每个业务 Agent 生成一张独立的 AgentCard，其 SupportedInterfaces
// 指向该 Agent 专属的 JSON-RPC 端点。
//
// 参数 authEnabled 为 true 时，会在 Card 中同时声明 tenantId（APIKeySecurityScheme，对应
// X-A2A-Tenant-Id 头）与 bearerAuth（HTTPAuthSecurityScheme，对应 Authorization: Bearer <token>）
// 两个安全方案，并放进同一条 SecurityRequirements 记录——A2A 协议规定同一条记录内的多个
// key 是"AND"语义（须同时满足），如实表达"必须同时携带 TenantID 与 Token 二者缺一不可"。
// 这是 A2A 协议规定的标准发现机制（对应 OpenAPI 3.0 Security Scheme/Requirement Object），
// 客户端可据此在真正发起调用前就知道要带什么凭证，而不是先试探性调用一次再被 401 拒绝。
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
		// 列出服务支持的所有认证方案（名称到定义的映射）
		card.SecuritySchemes = a2a.NamedSecuritySchemes{
			// 类型：APIKeySecurityScheme，表示使用一个自定义 HTTP 头来传递 API 密钥
			tenantIdSecuritySchemeName: a2a.APIKeySecurityScheme{
				// Header，即放在请求头中
				Location:    a2a.APIKeySecuritySchemeLocationHeader,
				Name:        "X-A2A-Tenant-Id",
				Description: "由 Agently 一方分配的租户唯一标识，调用方需显式声明自身身份",
			},
			// 类型：HTTPAuthSecurityScheme，表示使用 HTTP 标准认证头 Authorization
			bearerSecuritySchemeName: a2a.HTTPAuthSecurityScheme{
				Scheme:      "Bearer",
				Description: "与 X-A2A-Tenant-Id 配对使用的租户专属密钥：Authorization: Bearer <token>",
			},
		}
		// SecurityRequirementsOptions 声明在调用该服务的接口时，必须满足的安全要求组合。
		// 它是一个列表，每个元素表示一组要求（AND 关系）；
		// 不同元素之间是 OR 关系（即可选组合，但本例只有一个组合，故为唯一要求）。
		card.SecurityRequirements = a2a.SecurityRequirementsOptions{
			// 同一条 SecurityRequirements 记录内包含两个 key，表示两者需同时满足（AND）
			a2a.SecurityRequirements{
				tenantIdSecuritySchemeName: {},
				bearerSecuritySchemeName:   {},
			},
		}
	}

	return card
}
