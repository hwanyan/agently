// Package authclient 封装对 hwan 认证服务（AuthService）的 gRPC 调用。
//
// 用户系统（注册 / 登录 / 邮箱验证码 / Token 校验）已从 agently 抽离到 hwan 统一提供，
// agently 不再持有用户表、JWT 密钥和 SMTP 配置，所有身份相关能力都经由本客户端获得。
package authclient

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/hwan/app/auth/verifier"
	authpb "github.com/hwanyan/proto/auth"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// 单次调用的默认超时（SendVerifyCode 需要等待 SMTP 投递，留得更长）
const (
	defaultTimeout  = 5 * time.Second
	sendMailTimeout = 30 * time.Second
	unavailableMsg  = "认证服务暂不可用，请稍后重试"
)

// Identity 校验通过后的用户身份（来自 Access Token 载荷）
type Identity struct {
	UserID   string
	Username string
	Roles    []string // 平台级角色（user / admin）；团队/项目内角色由 agently 自行维护
}

// Client hwan 认证服务 gRPC 客户端
type Client struct {
	conn     *grpc.ClientConn
	client   authpb.AuthServiceClient
	verifier *verifier.Verifier // Access Token 本地验签（公钥取自认证服务 GetJWKS，带缓存）
}

// reasonInvalidServiceToken 认证服务在服务凭证缺失/无效时附带的 ErrorInfo.Reason
const reasonInvalidServiceToken = "INVALID_SERVICE_TOKEN"

// serviceTokenCreds 把服务调用方凭证附加到每个出站 RPC 的 metadata（authorization: Bearer <token>）
type serviceTokenCreds string

func (t serviceTokenCreds) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + string(t)}, nil
}

// RequireTransportSecurity 返回 false：认证服务部署在内网，当前以明文 gRPC 通信。
// 若跨网络/跨主机部署，应改用 TLS，避免凭证被窃听。
func (serviceTokenCreds) RequireTransportSecurity() bool { return false }

// New 创建认证服务客户端
// 参数：
//   - addr:  hwan 认证服务 gRPC 地址，如 "127.0.0.1:50054"
//   - token: 调用方凭证，对应认证服务 server.services 里 name 为 agently 的那一项的 token
//
// grpc.NewClient 是惰性连接：这里不会真正拨号，认证服务暂时不可用也不会阻塞 agently 启动；
// 实际调用时才建立连接，不可用时按 Unavailable 返回。
func New(addr, token string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("认证服务地址（auth_service.addr）不能为空")
	}
	if token == "" {
		return nil, fmt.Errorf("认证服务调用方凭证（auth_service.token）不能为空")
	}
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(serviceTokenCreds(token)),
	)
	if err != nil {
		return nil, fmt.Errorf("创建认证服务 gRPC 客户端失败: %w", err)
	}
	client := authpb.NewAuthServiceClient(conn)
	return &Client{conn: conn, client: client, verifier: verifier.New(client)}, nil
}

// Close 关闭连接
func (c *Client) Close() error {
	return c.conn.Close()
}

// VerifyToken 在本地校验 Access Token（签名、过期、iss、aud），返回用户身份。
// 不回源认证服务、不查库；公钥由 verifier 缓存，首次使用及密钥轮换时才会调用 GetJWKS。
//
// 错误语义：
//   - Unauthenticated：Token 无效或已过期（客户端应先尝试 Refresh，失败再重新登录）
//   - Unavailable：取不到验签公钥（认证服务不可达且无缓存）—— 与「Token 无效」区分，避免把用户踢下线
func (c *Client) VerifyToken(ctx context.Context, token string) (*Identity, error) {
	claims, err := c.verifier.Verify(ctx, token)
	if err != nil {
		if errors.Is(err, verifier.ErrKeysUnavailable) {
			logger.Error("无法取得 Token 验签公钥: %v", err)
			return nil, status.Error(codes.Unavailable, unavailableMsg)
		}
		return nil, status.Error(codes.Unauthenticated, "Token 无效或已过期")
	}
	return &Identity{UserID: claims.UserID(), Username: claims.Username, Roles: claims.Roles}, nil
}

// Refresh 用 Refresh Token 换取新的 Access Token + 新的 Refresh Token（轮换）
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*authpb.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	resp, err := c.client.Refresh(ctx, &authpb.RefreshRequest{RefreshToken: refreshToken})
	return resp, translate("Refresh", err)
}

// Logout 登出：通知认证服务吊销该登录链路下的全部 Refresh Token，之后无法再刷新。
// Token 允许已过期（由认证服务校验签名）。幂等。
//
// 注意：Access Token 在业务服务本地验签、没有「已登出名单」，所以已签发的 Access Token 在自然过期前
// （最长 access_ttl，默认 15 分钟）仍能通过验签，这是有意接受的取舍；敏感操作请另行强校验。
func (c *Client) Logout(ctx context.Context, token string) error {
	callCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	_, err := c.client.Logout(callCtx, &authpb.LogoutRequest{Token: token})
	return translate("Logout", err)
}

// SendVerifyCode 发送邮箱验证码
func (c *Client) SendVerifyCode(ctx context.Context, email, purpose string) (*authpb.SendVerifyCodeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, sendMailTimeout)
	defer cancel()

	resp, err := c.client.SendVerifyCode(ctx, &authpb.SendVerifyCodeRequest{Email: email, Purpose: purpose})
	return resp, translate("SendVerifyCode", err)
}

// Register 用户注册
func (c *Client) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	resp, err := c.client.Register(ctx, req)
	return resp, translate("Register", err)
}

// Login 用户登录（支持用户名或邮箱）
func (c *Client) Login(ctx context.Context, username, password string) (*authpb.AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	resp, err := c.client.Login(ctx, &authpb.LoginRequest{Username: username, Password: password})
	return resp, translate("Login", err)
}

// translate 处理调用错误：
//   - nil 原样返回
//   - 连接类 / 超时类错误：详情写日志，对外统一为 Unavailable，避免泄露内网地址等信息
//   - 服务凭证被认证服务拒绝（auth_service.token 配错或已被移除）：属于部署问题，不是用户的问题，
//     写 ERROR 日志并按 Unavailable 返回，避免所有用户被误判为「未登录」而踢下线
//   - 其它业务错误（带 code 与面向用户的文案）：原样透传，上层网关会映射为对应 HTTP 状态码
func translate(method string, err error) error {
	if err == nil {
		return nil
	}
	st := status.Convert(err)
	if isInvalidServiceToken(st) {
		logger.Error("调用认证服务 %s 被拒绝：服务凭证（auth_service.token）无效，请检查与认证服务 server.services（agently 项）的 token 是否一致", method)
		return status.Error(codes.Unavailable, unavailableMsg)
	}
	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		logger.Error("调用认证服务 %s 失败: code=%s, msg=%s", method, st.Code(), st.Message())
		return status.Error(codes.Unavailable, unavailableMsg)
	}
	return err
}

// isInvalidServiceToken 判断错误是否为认证服务因服务凭证无效而拒绝（通过 ErrorInfo.Reason 识别）
func isInvalidServiceToken(st *status.Status) bool {
	if st.Code() != codes.Unauthenticated {
		return false
	}
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok && info.GetReason() == reasonInvalidServiceToken {
			return true
		}
	}
	return false
}
