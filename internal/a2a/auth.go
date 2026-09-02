package a2a

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"

	"github.com/hwanyan/agently/pkg/types"
)

// tenantHeader 是调用方声明自身身份（TenantID）所使用的请求头名。
// 与标准的 Authorization: Bearer <token> 配合使用，二者缺一不可：
// TenantID 用于确定"你是谁"（服务端据此做 O(1) 查找），Token 用于证明"你确实是你自称的那个人"。
const tenantHeader = "x-a2a-tenant-id"

// resolveTenants 校验并整理配置中的租户列表，构建 tenantID -> token 的查找表。
//
// 校验规则（遵循本项目"启动期 fail-fast"的惯例，配置错误直接报错、不带着错误配置起服务）：
//   - tenant_id、token 均不能为空；
//   - tenant_id 不能重复（否则无法唯一定位一个租户）；
//   - token 不能重复（否则一个 token 能冒充多个 tenant_id，双因子鉴权失去意义）。
//
// 返回空 map 表示未启用鉴权（JSON-RPC 端点对所有人开放）。
func resolveTenants(cfg *types.A2AConfig) (map[string]string, error) {
	tenants := make(map[string]string, len(cfg.Tenants))
	seenTokens := make(map[string]string, len(cfg.Tenants)) // token -> tenant_id，用于检测重复 token

	for i, t := range cfg.Tenants {
		tenantID := strings.TrimSpace(t.TenantID)
		token := strings.TrimSpace(t.Token)
		if tenantID == "" {
			return nil, fmt.Errorf("配置错误：a2a.tenants[%d] 缺少必填字段 tenant_id", i)
		}
		if token == "" {
			return nil, fmt.Errorf("配置错误：a2a.tenants[%d]（tenant_id=%q）缺少必填字段 token", i, tenantID)
		}
		if _, dup := tenants[tenantID]; dup {
			return nil, fmt.Errorf("配置错误：a2a.tenants 中 tenant_id=%q 重复出现", tenantID)
		}
		if existing, dup := seenTokens[token]; dup {
			return nil, fmt.Errorf("配置错误：a2a.tenants 中 tenant_id=%q 与 tenant_id=%q 配置了相同的 token", tenantID, existing)
		}
		tenants[tenantID] = token
		seenTokens[token] = tenantID
	}
	return tenants, nil
}

// tenantAuthInterceptor 是一个 a2asrv.CallInterceptor，基于"TenantID + Token"双因子模型
// 校验 A2A 请求的身份：
//   - Authorization: Bearer <token>  —— 证明身份的密钥；
//   - X-A2A-Tenant-Id: <tenant_id>   —— 调用方显式声明的身份（唯一事实依据）。
//
// 二者缺一不可：只有先按 tenant_id 查到该租户预先分配的 token，再与请求携带的 token 做
// 常量时间比较一致，才算校验通过。相比"拿着 token 去一堆密钥里挨个撞库反推是谁"，这种
// 显式声明身份的方式查找是 O(1) 的，且审计身份与鉴权结果天然一致、不存在歧义。
//
// 校验时机：在请求真正调用到 AgentExecutor 之前（Before 钩子）完成，一旦返回非 nil
// error，SDK 会直接把该 error 作为响应返回给客户端，不会再执行具体的业务 Agent 逻辑，
// 因此鉴权失败的请求不会消耗任何 LLM/工具调用成本。
//
// 之所以基于 a2asrv.CallInterceptor 实现，而不是在 net/http 层包一层中间件：
//   - CallInterceptor 是 A2A SDK 官方提供的、与具体传输协议（JSON-RPC/gRPC/REST）解耦的
//     扩展点，JSON-RPC 传输层（jsonrpc.go）在收到请求时已经把原始 HTTP Header 灌入了
//     CallContext.ServiceParams()，因此这里可以直接读到 Authorization / X-A2A-Tenant-Id 头；
//   - 校验失败时返回 a2a.NewError(a2a.ErrUnauthenticated, ...)，SDK 会自动映射为标准的
//     JSON-RPC 错误码 -31401 与 HTTP 401，语义与协议规范完全对齐，无需自己拼错误格式；
//   - 这里设置的 CallContext.User 会被 SDK 自动透传到 ExecutorContext.User，
//     executor.go 可直接读出 tenant_id 用于审计日志，无需额外传参。
type tenantAuthInterceptor struct {
	a2asrv.PassthroughCallInterceptor                   // 提供 After 的空实现，本拦截器只关心 Before
	tenants                           map[string]string // tenant_id -> token
}

// newTenantAuthInterceptor 创建鉴权拦截器；tenants 必须非空（由调用方保证，空 map 不应启用鉴权）
func newTenantAuthInterceptor(tenants map[string]string) *tenantAuthInterceptor {
	return &tenantAuthInterceptor{tenants: tenants}
}

// Before 校验 X-A2A-Tenant-Id 与 Authorization 请求头，通过后放行并标记调用方身份，
// 否则以 ErrUnauthenticated 拒绝。
//
// 出于安全考虑，"缺少必需的身份/凭证头"与"tenant_id 不存在"与"token 不匹配"这三种失败
// 均返回同一句笼统的错误信息，不向客户端暴露具体是哪个环节失败——避免第三方通过报错内容
// 探测出"哪些 tenant_id 是已注册的"，形成租户枚举（enumeration）攻击面。
func (i *tenantAuthInterceptor) Before(
	ctx context.Context, callCtx *a2asrv.CallContext, _ *a2asrv.Request,
) (context.Context, any, error) {
	const authFailedMsg = "鉴权失败：请携带有效的 X-A2A-Tenant-Id 与 Authorization: Bearer <token>"

	tenantVals, ok := callCtx.ServiceParams().Get(tenantHeader)
	if !ok || len(tenantVals) == 0 {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, authFailedMsg)
	}
	tenantID := strings.TrimSpace(tenantVals[0])
	if tenantID == "" {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, authFailedMsg)
	}

	authVals, ok := callCtx.ServiceParams().Get("authorization")
	if !ok || len(authVals) == 0 {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, authFailedMsg)
	}
	parts := strings.SplitN(authVals[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, authFailedMsg)
	}
	got := strings.TrimSpace(parts[1])
	if got == "" {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, authFailedMsg)
	}

	// tenantID 由调用方显式声明、非机密信息，map 查找耗时差异不构成有效的侧信道；
	// 真正的密钥比较（expected 与 got）使用常量时间比较，避免时序攻击猜出 token 内容。
	expected, exists := i.tenants[tenantID]
	if !exists {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, authFailedMsg)
	}
	if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, authFailedMsg)
	}

	// 校验通过，标记为已认证用户，Name 记录调用方显式声明并通过校验的 tenant_id；
	// SDK 会把该 User 透传到 ExecutorContext.User，供 executor.go 记录审计日志。
	callCtx.User = a2asrv.NewAuthenticatedUser(tenantID, nil)
	return ctx, nil, nil
}
