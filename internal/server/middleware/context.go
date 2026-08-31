// Package middleware 提供 gRPC 服务的横切关注点：鉴权拦截器与请求上下文传递。
//
// 之所以独立成包（而不是散落在 server 根包中），是因为鉴权拦截器与上下文注入是
// 典型的"中间件层"关注点，与具体某个 RPC 方法的业务逻辑（internal/server/rpc）
// 以及服务启动编排（internal/server 根包）在职责上完全正交，理应可以独立演进、
// 独立测试，也便于未来新增更多横切中间件（如限流、审计日志）时统一收拢在此。
package middleware

import "context"

// ctxKey 上下文键类型，避免与其他包的键冲突
type ctxKey string

const (
	ctxKeyUserID   ctxKey = "user_id"
	ctxKeyUsername ctxKey = "username"
)

// withUser 将用户信息注入 context（由鉴权拦截器调用）
func withUser(ctx context.Context, userID, username string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUserID, userID)
	ctx = context.WithValue(ctx, ctxKeyUsername, username)
	return ctx
}

// UserIDFromContext 从 context 中读取用户 ID（鉴权拦截器注入）
func UserIDFromContext(ctx context.Context) string {
	if v := ctx.Value(ctxKeyUserID); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// UsernameFromContext 从 context 中读取用户名
func UsernameFromContext(ctx context.Context) string {
	if v := ctx.Value(ctxKeyUsername); v != nil {
		if name, ok := v.(string); ok {
			return name
		}
	}
	return ""
}
