package rpc

import (
	"context"

	"github.com/hwanyan/agently/internal/auth"
	"github.com/hwanyan/agently/internal/logger"
	agentlypb "github.com/hwanyan/agently/pb/gen"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthServer 认证服务的 gRPC 实现（注册 / 登录 / 邮箱验证码）
type AuthServer struct {
	agentlypb.UnimplementedAuthServiceServer
	authSvc *auth.Service
}

// NewAuthServer 创建认证服务实现
func NewAuthServer(authSvc *auth.Service) *AuthServer {
	return &AuthServer{authSvc: authSvc}
}

// SendVerifyCode 发送邮箱验证码
func (s *AuthServer) SendVerifyCode(ctx context.Context, req *agentlypb.SendVerifyCodeRequest) (*agentlypb.SendVerifyCodeResponse, error) {
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "邮箱地址不能为空")
	}
	logger.Info("gRPC请求: 发送验证码 - 邮箱: %s, 用途: %s", req.GetEmail(), req.GetPurpose())
	if err := s.authSvc.SendVerificationCode(req.GetEmail(), req.GetPurpose()); err != nil {
		logger.Error("发送验证码失败: %v", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	logger.Info("验证码发送成功: %s", req.GetEmail())
	return &agentlypb.SendVerifyCodeResponse{Code: 0, Message: "验证码已发送，请查收邮箱"}, nil
}

// Register 用户注册
func (s *AuthServer) Register(ctx context.Context, req *agentlypb.RegisterRequest) (*agentlypb.AuthResponse, error) {
	logger.Info("gRPC请求: 用户注册 - 用户名: %s", req.GetUsername())
	user, token, err := s.authSvc.Register(req.GetUsername(), req.GetEmail(), req.GetPassword(), req.GetNickname(), req.GetVerifyCode())
	if err != nil {
		logger.Error("用户注册失败: %v", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	logger.Info("用户注册成功: %s", req.GetUsername())
	return &agentlypb.AuthResponse{Code: 0, Token: token, User: toPBUser(user)}, nil
}

// Login 用户登录（支持用户名或邮箱）
func (s *AuthServer) Login(ctx context.Context, req *agentlypb.LoginRequest) (*agentlypb.AuthResponse, error) {
	logger.Info("gRPC请求: 用户登录 - 用户名: %s", req.GetUsername())
	user, token, err := s.authSvc.Login(req.GetUsername(), req.GetPassword())
	if err != nil {
		logger.Error("用户登录失败: %v", err)
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	logger.Info("用户登录成功: %s", req.GetUsername())
	return &agentlypb.AuthResponse{Code: 0, Token: token, User: toPBUser(user)}, nil
}
