// Package auth 提供 SMTP 邮件发送器（mailer），与具体业务（验证码、通知等）解耦。
//
// Mailer 只关心 SMTP 协议交互，不知道"验证码"这种业务概念；
// 上层通过 SendMail(to, subject, body) 即可投递任意邮件。
package auth

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/pkg/types"
)

// Mailer SMTP 邮件发送器
type Mailer struct {
	cfg types.SMTPConfig
}

// NewMailer 创建邮件发送器
func NewMailer(cfg types.SMTPConfig) *Mailer {
	return &Mailer{cfg: cfg}
}

// Enabled 是否已配置 SMTP（未配置时上层应走"开发模式"）
func (m *Mailer) Enabled() bool {
	return m.cfg.Host != ""
}

// SendMail 发送一封文本邮件
//
// 设计要点：
//  1. 自动适配端口：465 走隐式 TLS（tls.Dial），其它端口（如 587/25）走 STARTTLS 升级
//  2. 手动调用 smtp.Client 的每一步，便于在日志里精确定位是哪一步失败
//     （Hello / StartTLS / Auth / Mail / Rcpt / Data / Close 任何一步的错误
//     都会包含 SMTP 服务器返回的状态码和文本，例如 `535 Login fail`、
//     `250 Ok: queued as XXX`）
//  3. 邮件头使用 RFC 2047 Base64 编码中文主题，附带 Date / Message-ID，
//     降低被反垃圾策略拦截的概率
func (m *Mailer) SendMail(to, subject, body string) error {
	cfg := m.cfg

	// 发件人
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}

	// 主题做 RFC 2047 Base64 编码，避免中文乱码 / 被反垃圾误判
	encodedSubject := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?="

	// 生成 Message-ID（host 部分用 SMTP host，足够唯一）
	messageID := fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), randomToken(8), cfg.Host)

	var headers strings.Builder
	headers.WriteString("From: " + from + "\r\n")
	headers.WriteString("To: " + to + "\r\n")
	headers.WriteString("Subject: " + encodedSubject + "\r\n")
	headers.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	headers.WriteString("Message-ID: " + messageID + "\r\n")
	headers.WriteString("MIME-Version: 1.0\r\n")
	headers.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	headers.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	headers.WriteString("\r\n")
	headers.WriteString(body)
	msg := []byte(headers.String())

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	logger.Info("SMTP 开始发送: addr=%s, from=%s, to=%s, msg_id=%s", addr, from, to, messageID)

	// === 1. 建立到 SMTP 服务器的连接 ===
	var (
		conn net.Conn
		err  error
	)
	tlsCfg := &tls.Config{ServerName: cfg.Host}
	if cfg.Port == 465 {
		// 465: 隐式 TLS，连接一建立就是加密的
		logger.Info("SMTP 使用隐式 TLS 连接(465)")
		conn, err = tls.Dial("tcp", addr, tlsCfg)
	} else {
		// 587 / 25 / 其它: 先建明文 TCP，稍后通过 STARTTLS 升级
		logger.Info("SMTP 使用明文连接，稍后 STARTTLS 升级(%d)", cfg.Port)
		conn, err = net.DialTimeout("tcp", addr, 15*time.Second)
	}
	if err != nil {
		logger.Error("SMTP 连接失败: addr=%s, err=%v", addr, err)
		return fmt.Errorf("SMTP 连接失败: %w", err)
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		logger.Error("SMTP 握手失败(NewClient): %v", err)
		return fmt.Errorf("SMTP 握手失败: %w", err)
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			logger.Warning("SMTP 关闭连接异常: %v", cerr)
		}
	}()

	// === 2. EHLO ===
	if err = client.Hello("localhost"); err != nil {
		logger.Error("SMTP EHLO 失败: %v", err)
		return fmt.Errorf("SMTP EHLO 失败: %w", err)
	}
	logger.Info("SMTP EHLO 成功")

	// === 3. STARTTLS（非 465 端口且服务器支持时）===
	if cfg.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(tlsCfg); err != nil {
				logger.Error("SMTP STARTTLS 失败: %v", err)
				return fmt.Errorf("SMTP STARTTLS 失败: %w", err)
			}
			logger.Info("SMTP STARTTLS 升级成功")
		} else {
			logger.Warning("SMTP 服务器不支持 STARTTLS，将使用明文传输（不安全）")
		}
	}

	// === 4. AUTH ===
	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err = client.Auth(auth); err != nil {
			logger.Error("SMTP AUTH 失败: user=%s, err=%v", cfg.Username, err)
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
		logger.Info("SMTP AUTH 成功: user=%s", cfg.Username)
	}

	// === 5. MAIL FROM ===
	if err = client.Mail(from); err != nil {
		logger.Error("SMTP MAIL FROM 失败: from=%s, err=%v", from, err)
		return fmt.Errorf("SMTP MAIL FROM 失败: %w", err)
	}
	logger.Info("SMTP MAIL FROM 成功: %s", from)

	// === 6. RCPT TO ===
	if err = client.Rcpt(to); err != nil {
		logger.Error("SMTP RCPT TO 失败: to=%s, err=%v", to, err)
		return fmt.Errorf("SMTP RCPT TO 失败: %w", err)
	}
	logger.Info("SMTP RCPT TO 成功: %s", to)

	// === 7. DATA ===
	wc, err := client.Data()
	if err != nil {
		logger.Error("SMTP DATA 命令失败: %v", err)
		return fmt.Errorf("SMTP DATA 命令失败: %w", err)
	}
	if _, err = wc.Write(msg); err != nil {
		_ = wc.Close()
		logger.Error("SMTP 写入邮件正文失败: %v", err)
		return fmt.Errorf("SMTP 写入邮件正文失败: %w", err)
	}
	// Close 是关键：这里会等待服务器对 "." 的最终应答
	// 成功时服务器返回类似: 250 Ok: queued as 24062217xxx
	// 这一步成功 = 邮件已被 SMTP 服务器接收并进入投递队列
	if err = wc.Close(); err != nil {
		logger.Error("SMTP 提交邮件正文失败(可能被服务器拒收): %v", err)
		return fmt.Errorf("SMTP 提交邮件正文失败: %w", err)
	}
	logger.Info("SMTP DATA 提交成功，邮件已进入服务器投递队列: to=%s, msg_id=%s", to, messageID)

	// === 8. QUIT ===
	if err = client.Quit(); err != nil {
		// QUIT 失败不影响邮件已交付的事实，仅警告
		logger.Warning("SMTP QUIT 异常: %v", err)
	}

	return nil
}

// randomToken 生成 n 位十六进制随机串，用于 Message-ID
func randomToken(n int) string {
	const chars = "0123456789abcdef"
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "00000000"
		}
		b[i] = chars[idx.Int64()]
	}
	return string(b)
}
