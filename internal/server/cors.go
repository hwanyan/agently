package server

import "net/http"

// corsMiddleware 处理跨域请求（仅作用于 HTTP 网关层，与 gRPC 鉴权拦截器
// [internal/server/middleware] 是两类不同的中间件：前者面向浏览器跨域访问控制，
// 后者面向 gRPC 请求鉴权，职责不同因此不合并）
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
