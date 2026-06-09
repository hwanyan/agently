// Package auth 提供邮箱验证码功能
package auth

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/smtp"
	"time"

	"github.com/hwanyan/agently/internal/store"
	"github.com/hwanyan/agently/pkg/types"
)

// EmailVerifyService 邮箱验证服务
type EmailVerifyService struct {
	db         *store.DB
	smtpConfig types.SMTPConfig
}

// NewEmailVerifyService 创建邮箱验证服务
func NewEmailVerifyService(db *store.DB, cfg types.SMTPConfig) *EmailVerifyService {
	return &EmailVerifyService{
		db:         db,
		smtpConfig: cfg,
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
		purpose = "register"
	}

	// 频率限制：同一邮箱 60 秒内只能发送一次
	var lastSentAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT created_at FROM email_verifications
		WHERE email = $1 AND purpose = $2
		ORDER BY created_at DESC LIMIT 1
	`, email, purpose).Scan(&lastSentAt)
	if err == nil && lastSentAt.Valid {
		if time.Since(lastSentAt.Time) < 60*time.Second {
			return errors.New("发送过于频繁，请 60 秒后重试")
		}
	}

	// 生成 6 位数字验证码
	code, err := generateCode(6)
	if err != nil {
		return fmt.Errorf("生成验证码失败: %w", err)
	}

	// 验证码有效期 5 分钟
	expiresAt := time.Now().Add(5 * time.Minute)

	// 将之前未使用的验证码标记为已失效
	_, _ = s.db.Exec(`
		UPDATE email_verifications
		SET used = true
		WHERE email = $1 AND purpose = $2 AND used = false
	`, email, purpose)

	// 存储验证码
	_, err = s.db.Exec(`
		INSERT INTO email_verifications (email, code, purpose, expires_at)
		VALUES ($1, $2, $3, $4)
	`, email, code, purpose, expiresAt)
	if err != nil {
		return fmt.Errorf("存储验证码失败: %w", err)
	}

	// 发送邮件
	if e := s.sendEmail(email, code, purpose); e != nil {
		return fmt.Errorf("发送验证邮件失败: %w", e)
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
		purpose = "register"
	}

	var (
		id        string
		expiresAt time.Time
	)

	// 查询最新的未使用验证码
	err := s.db.QueryRow(`
		SELECT id, expires_at FROM email_verifications
		WHERE email = $1 AND code = $2 AND purpose = $3 AND used = false
		ORDER BY created_at DESC LIMIT 1
	`, email, code, purpose).Scan(&id, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("验证码错误或已失效")
		}
		return fmt.Errorf("查询验证码失败: %w", err)
	}

	// 检查是否过期
	if time.Now().After(expiresAt) {
		return errors.New("验证码已过期，请重新获取")
	}

	// 标记为已使用
	_, err = s.db.Exec(`UPDATE email_verifications SET used = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("更新验证码状态失败: %w", err)
	}

	return nil
}

// sendEmail 通过 SMTP 发送验证码邮件
func (s *EmailVerifyService) sendEmail(to, code, purpose string) error {
	cfg := s.smtpConfig

	// 如果未配置 SMTP，打印到日志（开发模式）
	if cfg.Host == "" {
		fmt.Printf("[DEV] 邮箱验证码 -> %s: %s (用途: %s)\n", to, code, purpose)
		return nil
	}

	// 构建邮件主题和正文
	subject := "验证码"
	body := ""
	switch purpose {
	case "register":
		subject = "注册验证码 - Agently"
		body = fmt.Sprintf(
			"您好！\n\n您正在注册 Agently 账号，验证码为：%s\n\n验证码有效期为 5 分钟，请尽快完成验证。\n\n如果这不是您本人的操作，请忽略此邮件。",
			code,
		)
	case "reset_password":
		subject = "重置密码验证码 - Agently"
		body = fmt.Sprintf(
			"您好！\n\n您正在重置 Agently 账号密码，验证码为：%s\n\n验证码有效期为 5 分钟，请尽快完成操作。\n\n如果这不是您本人的操作，请忽略此邮件。",
			code,
		)
	default:
		body = fmt.Sprintf("您的验证码为：%s，有效期 10 分钟。", code)
	}

	// 构建邮件内容
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	// SMTP 认证
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	// 发送邮件
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
	if err != nil {
		return fmt.Errorf("SMTP 发送失败: %w", err)
	}

	return nil
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
