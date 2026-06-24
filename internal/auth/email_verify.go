// Package auth 提供邮箱验证码功能（业务编排层）
//
// 职责：
//   - 生成验证码、控制频率限制、控制有效期
//   - 渲染验证码邮件正文模板
//   - 协调 store.EmailVerificationStore（数据持久化）和 Mailer（SMTP 发送）
//
// 不做：
//   - 不直接写 SQL（交给 store.EmailVerificationStore）
//   - 不直接调用 net/smtp（交给 Mailer）
package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/internal/store"
	"github.com/hwanyan/agently/pkg/types"
)

// 验证码业务常量
const (
	// codeLength 验证码位数
	codeLength = 6
	// codeTTL 验证码有效期
	codeTTL = 5 * time.Minute
	// resendInterval 同一邮箱的最小重发间隔
	resendInterval = 60 * time.Second
)

// 用途常量
const (
	PurposeRegister      = "register"
	PurposeResetPassword = "reset_password"
)

// EmailVerifyService 邮箱验证服务（业务编排）
type EmailVerifyService struct {
	repo   *store.EmailVerificationStore
	mailer *Mailer
}

// NewEmailVerifyService 创建邮箱验证服务
//
// 该构造函数保留 (db, smtpCfg) 的签名以兼容现有调用方（main.go）；
// 内部自动构造 EmailVerificationStore 和 Mailer。
func NewEmailVerifyService(db *store.DB, cfg types.SMTPConfig) *EmailVerifyService {
	return &EmailVerifyService{
		repo:   store.NewEmailVerificationStore(db),
		mailer: NewMailer(cfg),
	}
}

// SendVerificationCode 生成并发送邮箱验证码
// 参数：
//   - email: 目标邮箱地址
//   - purpose: 用途（register=注册, reset_password=重置密码）
//
// 返回：
//   - error: 可能的错误
func (s *EmailVerifyService) SendVerificationCode(email, purpose string) error {
	if email == "" {
		return errors.New("邮箱地址不能为空")
	}
	if purpose == "" {
		purpose = PurposeRegister
	}

	// 1. 频率限制：同一邮箱 60 秒内只能发送一次
	lastSentAt, err := s.repo.GetLastSentAt(email, purpose)
	if err != nil {
		return err
	}
	if lastSentAt != nil && time.Since(*lastSentAt) < resendInterval {
		return errors.New("发送过于频繁，请 60 秒后重试")
	}

	// 2. 生成验证码
	code, err := generateCode(codeLength)
	if err != nil {
		return fmt.Errorf("生成验证码失败: %w", err)
	}
	expiresAt := time.Now().Add(codeTTL)

	// 3. 失效旧码 + 写入新码（best-effort：失效失败不阻断流程）
	if e := s.repo.InvalidatePending(email, purpose); e != nil {
		logger.Warning("失效旧验证码失败（忽略继续）: email=%s, err=%v", email, e)
	}
	if err = s.repo.Insert(email, code, purpose, expiresAt); err != nil {
		return err
	}

	// 4. 发送邮件
	if err = s.dispatchEmail(email, code, purpose); err != nil {
		return fmt.Errorf("发送验证邮件失败: %w", err)
	}

	return nil
}

// VerifyCode 校验邮箱验证码
// 参数：
//   - email: 邮箱地址
//   - code: 用户输入的验证码
//   - purpose: 用途
//
// 返回：
//   - error: 验证失败时返回错误
func (s *EmailVerifyService) VerifyCode(email, code, purpose string) error {
	if email == "" || code == "" {
		return errors.New("邮箱和验证码不能为空")
	}
	if purpose == "" {
		purpose = PurposeRegister
	}

	// 1. 查找最新的未使用验证码
	v, err := s.repo.FindActive(email, code, purpose)
	if err != nil {
		return err
	}
	if v == nil {
		return errors.New("验证码错误或已失效")
	}

	// 2. 检查是否过期
	if time.Now().After(v.ExpiresAt) {
		return errors.New("验证码已过期，请重新获取")
	}

	// 3. 标记为已使用
	if err = s.repo.MarkUsed(v.ID); err != nil {
		return err
	}

	return nil
}

// dispatchEmail 渲染验证码邮件并交给 Mailer 发送
//
// SMTP 未配置时进入开发模式：仅在日志中打印验证码、不真正发邮件。
func (s *EmailVerifyService) dispatchEmail(to, code, purpose string) error {
	if !s.mailer.Enabled() {
		logger.Warning("SMTP 未配置，验证码仅记录到日志: to=%s, code=%s, purpose=%s", to, code, purpose)
		return nil
	}
	subject, body := renderCodeMail(code, purpose)
	return s.mailer.SendMail(to, subject, body)
}

// renderCodeMail 渲染验证码邮件的主题与正文
func renderCodeMail(code, purpose string) (subject, body string) {
	switch purpose {
	case PurposeRegister:
		return "注册验证码 - Agently", fmt.Sprintf(
			"您好！\r\n\r\n您正在注册 Agently 账号，验证码为：%s\r\n\r\n验证码有效期为 5 分钟，请尽快完成验证。\r\n\r\n如果这不是您本人的操作，请忽略此邮件。",
			code,
		)
	case PurposeResetPassword:
		return "重置密码验证码 - Agently", fmt.Sprintf(
			"您好！\r\n\r\n您正在重置 Agently 账号密码，验证码为：%s\r\n\r\n验证码有效期为 5 分钟，请尽快完成操作。\r\n\r\n如果这不是您本人的操作，请忽略此邮件。",
			code,
		)
	default:
		return "验证码 - Agently", fmt.Sprintf("您的验证码为：%s，有效期 5 分钟。", code)
	}
}

// generateCode 生成指定位数的数字验证码（密码学安全随机数）
func generateCode(length int) (string, error) {
	code := ""
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code += fmt.Sprintf("%d", n.Int64())
	}
	return code, nil
}
