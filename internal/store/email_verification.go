// Package store 提供数据访问对象（DAO）
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// EmailVerification 邮箱验证码数据模型（对应 email_verifications 表）
type EmailVerification struct {
	ID        string
	Email     string
	Code      string
	Purpose   string
	Used      bool
	ExpiresAt time.Time
	CreatedAt time.Time
}

// EmailVerificationStore 邮箱验证码数据访问对象
type EmailVerificationStore struct {
	db *DB
}

// NewEmailVerificationStore 创建邮箱验证码数据访问对象
func NewEmailVerificationStore(db *DB) *EmailVerificationStore {
	return &EmailVerificationStore{db: db}
}

// GetLastSentAt 查询某邮箱+用途的最近一次发送时间
// 返回：
//   - *time.Time: 最近一次发送时间；若无记录返回 nil（不视为错误）
//   - error: 仅在数据库异常时返回
func (s *EmailVerificationStore) GetLastSentAt(email, purpose string) (*time.Time, error) {
	var lastSentAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT created_at FROM email_verifications
		WHERE email = $1 AND purpose = $2
		ORDER BY created_at DESC LIMIT 1
	`, email, purpose).Scan(&lastSentAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("查询最近发送时间失败: %w", err)
	}
	if !lastSentAt.Valid {
		return nil, nil
	}
	return &lastSentAt.Time, nil
}

// InvalidatePending 将某邮箱+用途下所有未使用的验证码标记为已使用（防止多码并存）
func (s *EmailVerificationStore) InvalidatePending(email, purpose string) error {
	_, err := s.db.Exec(`
		UPDATE email_verifications
		SET used = true
		WHERE email = $1 AND purpose = $2 AND used = false
	`, email, purpose)
	if err != nil {
		return fmt.Errorf("失效旧验证码失败: %w", err)
	}
	return nil
}

// Insert 插入一条新验证码记录
func (s *EmailVerificationStore) Insert(email, code, purpose string, expiresAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO email_verifications (email, code, purpose, expires_at)
		VALUES ($1, $2, $3, $4)
	`, email, code, purpose, expiresAt)
	if err != nil {
		return fmt.Errorf("写入验证码失败: %w", err)
	}
	return nil
}

// FindActive 查找最新的未使用验证码（按 email+code+purpose 精确匹配）
// 返回：
//   - *EmailVerification: 匹配到的验证码记录；未找到时返回 nil（不视为错误）
//   - error: 仅在数据库异常时返回
func (s *EmailVerificationStore) FindActive(email, code, purpose string) (*EmailVerification, error) {
	row := s.db.QueryRow(`
		SELECT id, email, code, purpose, used, expires_at, created_at
		FROM email_verifications
		WHERE email = $1 AND code = $2 AND purpose = $3 AND used = false
		ORDER BY created_at DESC LIMIT 1
	`, email, code, purpose)

	v := &EmailVerification{}
	err := row.Scan(&v.ID, &v.Email, &v.Code, &v.Purpose, &v.Used, &v.ExpiresAt, &v.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("查询验证码失败: %w", err)
	}
	return v, nil
}

// MarkUsed 按主键标记验证码为已使用
func (s *EmailVerificationStore) MarkUsed(id string) error {
	_, err := s.db.Exec(`UPDATE email_verifications SET used = true WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("更新验证码状态失败: %w", err)
	}
	return nil
}
