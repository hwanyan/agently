// Package middleware 提供 HTTP 中间件
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hwanyan/agently/internal/auth"
)

// contextKey 上下文键类型，避免与其他包的键冲突
type contextKey string

const (
	// ContextKeyUserID 上下文中存储用户 ID 的键
	ContextKeyUserID contextKey = "userid"
	// ContextKeyUsername 上下文中存储用户名的键
	ContextKeyUsername contextKey = "username"
)

// AuthMiddleware JWT 鉴权中间件
// 从 Authorization: Bearer <token> 头中提取并验证 JWT
// 验证通过后将 user_id 和 username 注入到请求上下文中
func AuthMiddleware(authSvc *auth.Service, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 从 Authorization 头提取 Token
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeAuthError(w, "缺少 Authorization 请求头")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeAuthError(w, "Authorization 格式错误，应为 Bearer <token>")
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		if tokenStr == "" {
			writeAuthError(w, "Token 不能为空")
			return
		}

		// 验证 Token
		claims, err := authSvc.VerifyToken(tokenStr)
		if err != nil {
			writeAuthError(w, "Token 无效或已过期")
			return
		}

		// 将用户信息注入上下文
		ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextKeyUsername, claims.Username)

		next(w, r.WithContext(ctx))
	}
}

// GetUserID 从请求上下文中获取用户 ID
func GetUserID(r *http.Request) string {
	if v := r.Context().Value(ContextKeyUserID); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// GetUsername 从请求上下文中获取用户名
func GetUsername(r *http.Request) string {
	if v := r.Context().Value(ContextKeyUsername); v != nil {
		if name, ok := v.(string); ok {
			return name
		}
	}
	return ""
}

// writeAuthError 写入 401 认证错误响应
func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    401,
		"message": message,
	})
}
