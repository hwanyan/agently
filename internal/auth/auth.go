// Package auth 提供用户认证功能（注册、登录、JWT 签发与验证）
package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hwanyan/agently/internal/store"
	"github.com/hwanyan/agently/pkg/types"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Service 认证服务
type Service struct {
	userStore      *store.UserStore
	jwtSecret      []byte
	expireHour     int
	emailVerifySvc *EmailVerifyService // 邮箱验证服务（可为 nil，表示不启用邮箱验证）
}

// Claims JWT 载荷
type Claims struct {
	UserID   string `json:"userid"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// NewService 创建认证服务
// 参数：
//   - userStore: 用户数据访问对象
//   - cfg: JWT 配置
//   - emailVerifySvc: 邮箱验证服务（可为 nil，表示不启用邮箱验证）
func NewService(userStore *store.UserStore, cfg types.JWTConfig, emailVerifySvc *EmailVerifyService) *Service {
	secret := cfg.Secret
	if secret == "" {
		secret = "default-insecure-secret-please-change"
	}
	expireHour := cfg.ExpireHour
	if expireHour <= 0 {
		expireHour = 72
	}
	return &Service{
		userStore:      userStore,
		jwtSecret:      []byte(secret),
		expireHour:     expireHour,
		emailVerifySvc: emailVerifySvc,
	}
}

// Register 用户注册
// 参数：
//   - username: 用户名（3-64 字符，只允许字母数字下划线）
//   - email: 邮箱
//   - password: 明文密码（至少 6 位）
//   - nickname: 昵称（可为空）
//   - verifyCode: 邮箱验证码（当启用邮箱验证时必填）
//
// 返回：
//   - *types.User: 注册成功的用户（不含密码）
//   - string: JWT Token
//   - error: 可能的错误
func (s *Service) Register(username, email, password, nickname, verifyCode string) (*types.User, string, error) {
	// 参数校验
	if err := validateUsername(username); err != nil {
		return nil, "", err
	}
	if err := validateEmail(email); err != nil {
		return nil, "", err
	}
	if len(password) < 6 {
		return nil, "", errors.New("密码长度不能少于 6 位")
	}

	// 邮箱验证码校验（如果启用了邮箱验证服务）
	if s.emailVerifySvc != nil {
		if verifyCode == "" {
			return nil, "", errors.New("邮箱验证码不能为空")
		}
		if err := s.emailVerifySvc.VerifyCode(email, verifyCode, "register"); err != nil {
			return nil, "", err
		}
	}

	// 对密码进行 bcrypt 哈希
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("密码加密失败: %w", err)
	}

	// 创建用户
	user, err := s.userStore.Create(username, email, string(hashed), nickname)
	if err != nil {
		return nil, "", err
	}

	// 签发 JWT
	token, err := s.signToken(user)
	if err != nil {
		return nil, "", fmt.Errorf("签发 Token 失败: %w", err)
	}

	// 清除密码字段
	user.Password = ""
	return user, token, nil
}

// Login 用户登录（支持用户名或邮箱登录）
// 参数：
//   - usernameOrEmail: 用户名或邮箱
//   - password: 明文密码
//
// 返回：
//   - *types.User: 登录成功的用户（不含密码）
//   - string: JWT Token
//   - error: 可能的错误
func (s *Service) Login(usernameOrEmail, password string) (*types.User, string, error) {
	if usernameOrEmail == "" || password == "" {
		return nil, "", errors.New("用户名/邮箱和密码不能为空")
	}

	// 判断是邮箱还是用户名
	var (
		user *types.User
		err  error
	)
	if strings.Contains(usernameOrEmail, "@") {
		user, err = s.userStore.GetByEmail(usernameOrEmail)
	} else {
		user, err = s.userStore.GetByUsername(usernameOrEmail)
	}

	// 查不出用户 or 查出的用户被禁用
	if err != nil {
		return nil, "", fmt.Errorf("查询用户失败: %w", err)
	}
	if user == nil {
		return nil, "", errors.New("用户名或密码错误")
	}
	if user.Status != 1 {
		return nil, "", errors.New("账号已被禁用，请联系管理员")
	}

	// 验证密码
	if e := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); e != nil {
		return nil, "", errors.New("用户名或密码错误")
	}

	// 更新最后登录时间（异步，不影响登录响应）
	go s.userStore.UpdateLastLogin(user.ID)

	// 签发 JWT
	token, err := s.signToken(user)
	if err != nil {
		return nil, "", fmt.Errorf("签发 Token 失败: %w", err)
	}

	// 清除密码字段
	user.Password = ""
	return user, token, nil
}

// SendVerificationCode 发送邮箱验证码（代理到 EmailVerifyService）
// 参数：
//   - email: 目标邮箱地址
//   - purpose: 用途（register=注册, reset_password=重置密码）
func (s *Service) SendVerificationCode(email, purpose string) error {
	if s.emailVerifySvc == nil {
		return errors.New("邮箱验证服务未启用")
	}
	return s.emailVerifySvc.SendVerificationCode(email, purpose)
}

// VerifyToken 验证 JWT Token，返回 Claims
func (s *Service) VerifyToken(tokenStr string) (*Claims, error) {
	// jwt.ParseWithClaims内部已经做了：签名验证 + 过期检查 + 格式校验
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// 第一步：检查签名算法是否是预期的 HMAC（防止算法替换攻击）
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期的签名算法: %v", t.Header["alg"])
		}
		// 第二步：返回密钥，库用它来验签
		return s.jwtSecret, nil
		// 第三步：验签通过后，库还会自动检查 exp 是否过期
	})
	if err != nil {
		return nil, fmt.Errorf("Token 验证失败: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("无效的 Token")
	}

	return claims, nil
}

// signToken 为用户签发 JWT Token
func (s *Service) signToken(user *types.User) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(s.expireHour) * time.Hour)),
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// validateUsername 校验用户名格式
func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 64 {
		return errors.New("用户名长度必须在 3-64 个字符之间")
	}
	for _, c := range username {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			return errors.New("用户名只能包含字母、数字、下划线和连字符")
		}
	}
	return nil
}

// validateEmail 简单校验邮箱格式
func validateEmail(email string) error {
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return errors.New("邮箱格式不正确")
	}
	if len(email) > 255 {
		return errors.New("邮箱长度不能超过 255 个字符")
	}
	return nil
}
