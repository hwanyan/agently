// Package server 提供 Agently 全部 gRPC 服务实现、鉴权拦截器以及
// gRPC + grpc-gateway 的启动编排。它取代了原先手写的 net/http 路由层，
// 使所有请求路由统一收敛到 gRPC。
package server

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
