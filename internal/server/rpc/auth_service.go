package rpc

import (
	"context"

	"github.com/hwanyan/agently/internal/authclient"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/server/middleware"
	agentlypb "github.com/hwanyan/agently/pb/gen"
	authpb "github.com/hwanyan/proto/auth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthServer 认证服务的 gRPC 实现（注册 / 登录 / 邮箱验证码）
//
// 用户系统已抽离到 hwan 认证服务，这里只做对外契约保持不变的薄代理：
// 对外仍是 agently.v1.AuthService 与原有 REST 路径，前端无需任何改动；
// 内部全部转发给 hwan 认证服务，由其负责校验、入库、发信与 Token 签发。
type AuthServer struct {
	agentlypb.UnimplementedAuthServiceServer
	users *authclient.Client
}

// NewAuthServer 创建认证服务实现
func NewAuthServer(users *authclient.Client) *AuthServer {
	return &AuthServer{users: users}
}

// SendVerifyCode 发送邮箱验证码
func (s *AuthServer) SendVerifyCode(ctx context.Context, req *agentlypb.SendVerifyCodeRequest) (*agentlypb.SendVerifyCodeResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "邮箱地址不能为空")
	}
	logger.Info("gRPC请求: 发送验证码 - 邮箱: %s, 用途: %s", req.GetEmail(), req.GetPurpose())
	resp, err := s.users.SendVerifyCode(ctx, req.GetEmail(), req.GetPurpose())
	if err != nil {
		logger.Error("发送验证码失败: %v", err)
		return nil, err
	}
	return &agentlypb.SendVerifyCodeResponse{Code: 0, Message: resp.GetMessage()}, nil
}

// Register 用户注册
func (s *AuthServer) Register(ctx context.Context, req *agentlypb.RegisterRequest) (*agentlypb.AuthResponse, error) {
	logger.Info("gRPC请求: 用户注册 - 用户名: %s", req.GetUsername())
	resp, err := s.users.Register(ctx, &authpb.RegisterRequest{
		Username:   req.GetUsername(),
		Email:      req.GetEmail(),
		Password:   req.GetPassword(),
		Nickname:   req.GetNickname(),
		VerifyCode: req.GetVerifyCode(),
	})
	if err != nil {
		logger.Error("用户注册失败: %v", err)
		return nil, err
	}
	logger.Info("用户注册成功: %s", req.GetUsername())
	return toAuthResponse(resp), nil
}

// Login 用户登录（支持用户名或邮箱）
func (s *AuthServer) Login(ctx context.Context, req *agentlypb.LoginRequest) (*agentlypb.AuthResponse, error) {
	logger.Info("gRPC请求: 用户登录 - 用户名: %s", req.GetUsername())
	resp, err := s.users.Login(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		logger.Error("用户登录失败: %v", err)
		return nil, err
	}
	logger.Info("用户登录成功: %s", req.GetUsername())
	return toAuthResponse(resp), nil
}

// Refresh 用 Refresh Token 换取新的 Access Token（并轮换 Refresh Token）
// 公开接口：调用时 Access Token 通常已过期，身份由 Refresh Token 本身证明。日志不记录令牌。
func (s *AuthServer) Refresh(ctx context.Context, req *agentlypb.RefreshRequest) (*agentlypb.AuthResponse, error) {
	if req.GetRefreshToken() == "" {
		return nil, status.Error(codes.Unauthenticated, "Refresh Token 不能为空")
	}
	resp, err := s.users.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, err
	}
	return toAuthResponse(resp), nil
}

// Logout 登出：吊销当前登录链路（其 Refresh Token 全部失效）
// 公开接口：Access Token 过期后用户仍应能退出登录，所以这里不走鉴权拦截器，
// 直接取请求头里的 Token 交给认证服务（它只要求签名有效，允许已过期）。
func (s *AuthServer) Logout(ctx context.Context, _ *agentlypb.LogoutRequest) (*agentlypb.LogoutResponse, error) {
	token, err := middleware.BearerTokenFromMetadata(ctx)
	if err != nil {
		return nil, err
	}
	logger.Info("gRPC请求: 用户登出")
	if err := s.users.Logout(ctx, token); err != nil {
		logger.Error("用户登出失败: %v", err)
		return nil, err
	}
	return &agentlypb.LogoutResponse{Code: 0, Message: "已退出登录"}, nil
}

// toAuthResponse 把认证服务的 AuthResponse 转为 agently 对外的响应
func toAuthResponse(resp *authpb.AuthResponse) *agentlypb.AuthResponse {
	return &agentlypb.AuthResponse{
		Code:         0,
		Token:        resp.GetToken(),
		User:         toPBUser(resp.GetUser()),
		RefreshToken: resp.GetRefreshToken(),
		ExpiresIn:    resp.GetExpiresIn(),
	}
}
