package server

import (
	"context"
	"strings"

	"github.com/hwanyan/agently/internal/auth"
	"github.com/hwanyan/agently/internal/logger"
	agentlypb "github.com/hwanyan/agently/pb/gen"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// AuthInterceptor JWT 鉴权拦截器
//
// 取代原先的 HTTP 中间件：从 gRPC metadata 的 authorization 头中提取并验证 Bearer Token，
// 验证通过后把 user_id/username 注入 context。
//
// 是否鉴权由「契约即策略」驱动：读取每个方法在 proto 中声明的自定义选项
// (agently.v1.public)。语义为 default-secure —— 只有显式声明 public=true 的方法才免鉴权，
// 其余（包括未来新增的方法）一律需要鉴权，避免“新增即公开”的隐患。
type AuthInterceptor struct {
	authSvc *auth.Service
	// publicMethods 全方法名 → 是否公开的策略表，启动时由 proto 方法选项构建
	publicMethods map[string]bool
}

// NewAuthInterceptor 创建鉴权拦截器，并根据 proto 方法选项构建鉴权策略表
func NewAuthInterceptor(authSvc *auth.Service) *AuthInterceptor {
	policy := buildAuthPolicy()

	// 打印公开方法清单，便于启动期审计
	var publics []string
	for m, isPublic := range policy {
		if isPublic {
			publics = append(publics, m)
		}
	}
	logger.Info("鉴权策略已加载：共 %d 个方法，其中公开（免鉴权）方法 %d 个: %s",
		len(policy), len(publics), strings.Join(publics, ", "))

	return &AuthInterceptor{authSvc: authSvc, publicMethods: policy}
}

// buildAuthPolicy 遍历全局 proto 描述符注册表，读取每个方法的 (agently.v1.public) 选项，
// 构建「完整方法名 → 是否公开」的策略表。
func buildAuthPolicy() map[string]bool {
	policy := make(map[string]bool)
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			sd := services.Get(i)
			methods := sd.Methods()
			for j := 0; j < methods.Len(); j++ {
				md := methods.Get(j)
				fullMethod := "/" + string(sd.FullName()) + "/" + string(md.Name())
				policy[fullMethod] = readPublicOption(md)
			}
		}
		return true
	})
	return policy
}

// readPublicOption 读取方法的 (agently.v1.public) 布尔选项，缺省为 false（需鉴权）
func readPublicOption(md protoreflect.MethodDescriptor) bool {
	opts, ok := md.Options().(*descriptorpb.MethodOptions)
	if !ok || opts == nil {
		return false
	}
	if v, ok := proto.GetExtension(opts, agentlypb.E_Public).(bool); ok {
		return v
	}
	return false
}

// isPublicMethod 依据策略表判断方法是否免鉴权；未登记的方法一律视为需要鉴权（default-secure）
func (a *AuthInterceptor) isPublicMethod(fullMethod string) bool {
	return a.publicMethods[fullMethod]
}

// authenticate 校验 metadata 中的 Bearer Token，成功则返回带用户信息的新 context
func (a *AuthInterceptor) authenticate(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "缺少 Authorization 请求头")
	}
	vals := md.Get("authorization")
	if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
		return nil, status.Error(codes.Unauthenticated, "缺少 Authorization 请求头")
	}

	parts := strings.SplitN(vals[0], " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, status.Error(codes.Unauthenticated, "Authorization 格式错误，应为 Bearer <token>")
	}
	tokenStr := strings.TrimSpace(parts[1])
	if tokenStr == "" {
		return nil, status.Error(codes.Unauthenticated, "Token 不能为空")
	}

	claims, err := a.authSvc.VerifyToken(tokenStr)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "Token 无效或已过期")
	}

	return withUser(ctx, claims.UserID, claims.Username), nil
}

// Unary 一元 RPC 鉴权拦截器
func (a *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 公开方法直接放行（info.FullMethod 是形如 /agently.v1.SessionService/CreateSession 的完整方法名，拦截器据此查策略表判断是否需要鉴权）
		if a.isPublicMethod(info.FullMethod) {
			// handler 是 "下一环" 链中的下一个拦截器，或最终的业务方法。
			// 只有调用 handler(...) 请求才会继续往下走；不调用（直接 return err）就等于拦截掉了。
			return handler(ctx, req)
		}
		// 校验 Bearer Token
		// 鉴权成功后把 user_id/username 塞进 newCtx，业务方法里就能用 UserIDFromContext(ctx) 取到当前用户
		newCtx, err := a.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// Stream 流式 RPC 鉴权拦截器
func (a *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if a.isPublicMethod(info.FullMethod) {
			return handler(srv, ss)
		}
		newCtx, err := a.authenticate(ss.Context())
		if err != nil {
			return err
		}
		// ServerStream 的 Context() 不可直接替换，用 wrappedStream 包一层替换 ctx
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: newCtx})
	}
}

// wrappedStream 包装 grpc.ServerStream 以替换其 context（注入用户信息）
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
