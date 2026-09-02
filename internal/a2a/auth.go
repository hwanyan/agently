package a2a

import (
	"context"
	"crypto/subtle"
	"os"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

// authTokenEnvVar 是 A2A 共享密钥的环境变量名。
// 优先级高于 config.yaml 中的 a2a.auth_token，遵循"密钥类配置优先走环境变量"的安全实践。
const authTokenEnvVar = "AGENTLY_A2A_AUTH_TOKEN"

// resolveAuthToken 解析最终生效的鉴权密钥：环境变量优先，其次回落到配置文件字段。
// 返回空字符串表示未启用鉴权。
func resolveAuthToken(cfgToken string) string {
	if v := strings.TrimSpace(os.Getenv(authTokenEnvVar)); v != "" {
		return v
	}
	return strings.TrimSpace(cfgToken)
}

// bearerAuthInterceptor 是一个 a2asrv.CallInterceptor，用于校验 A2A 请求的
// `Authorization: Bearer <token>` 请求头。
//
// 校验时机：在请求真正调用到 AgentExecutor 之前（Before 钩子）完成，一旦返回非 nil
// error，SDK 会直接把该 error 作为响应返回给客户端，不会再执行具体的业务 Agent 逻辑，
// 因此鉴权失败的请求不会消耗任何 LLM/工具调用成本。
//
// 之所以基于 a2asrv.CallInterceptor 实现，而不是在 net/http 层包一层中间件：
//   - CallInterceptor 是 A2A SDK 官方提供的、与具体传输协议（JSON-RPC/gRPC/REST）解耦的
//     扩展点，JSON-RPC 传输层（jsonrpc.go）在收到请求时已经把原始 HTTP Header 灌入了
//     CallContext.ServiceParams()，因此这里可以直接读到 Authorization 头；
//   - 校验失败时返回 a2a.NewError(a2a.ErrUnauthenticated, ...)，SDK 会自动映射为标准的
//     JSON-RPC 错误码 -31401 与 HTTP 401，语义与协议规范完全对齐，无需自己拼错误格式。
type bearerAuthInterceptor struct {
	a2asrv.PassthroughCallInterceptor // 提供 After 的空实现，本拦截器只关心 Before
	token                             string
}

// newBearerAuthInterceptor 创建鉴权拦截器；token 必须非空（由调用方保证，空 token 不应启用鉴权）
func newBearerAuthInterceptor(token string) *bearerAuthInterceptor {
	return &bearerAuthInterceptor{token: token}
}

// Before 校验 Authorization 请求头，通过后放行，否则以 ErrUnauthenticated 拒绝
func (b *bearerAuthInterceptor) Before(
	ctx context.Context, callCtx *a2asrv.CallContext, _ *a2asrv.Request,
) (context.Context, any, error) {
	vals, ok := callCtx.ServiceParams().Get("authorization") // 大小写不敏感（内部按小写存储）
	if !ok || len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, "缺少 Authorization 请求头")
	}

	parts := strings.SplitN(vals[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, "Authorization 格式错误，应为 Bearer <token>")
	}

	got := strings.TrimSpace(parts[1])
	if got == "" {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, "Token 不能为空")
	}

	// 常量时间比较，避免因响应耗时差异被侧信道猜出 token（时序攻击）
	if subtle.ConstantTimeCompare([]byte(got), []byte(b.token)) != 1 {
		return ctx, nil, a2a.NewError(a2a.ErrUnauthenticated, "Token 无效")
	}

	// 校验通过，标记为已认证用户（当前不区分具体调用方身份，仅用于审计/日志场景可扩展 Name）
	callCtx.User = a2asrv.NewAuthenticatedUser("a2a-client", nil)
	return ctx, nil, nil
}
