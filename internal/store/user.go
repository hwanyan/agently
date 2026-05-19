package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"agent-hwan/pkg/types"
)

// UserStore 用户数据访问对象
type UserStore struct {
	db *DB
}

// NewUserStore 创建用户数据访问对象
func NewUserStore(db *DB) *UserStore {
	return &UserStore{db: db}
}

// Create 创建新用户
// 参数：
//   - username: 用户名
//   - email: 邮箱
//   - hashedPassword: 已经过 bcrypt 哈希的密码
//   - nickname: 昵称（可为空）
//
// 返回：
//   - *types.User: 创建成功的用户
//   - error: 可能的错误（如用户名/邮箱已存在）
func (s *UserStore) Create(username, email, hashedPassword, nickname string) (*types.User, error) {
	query := `
		INSERT INTO users (username, email, password, nickname)
		VALUES ($1, $2, $3, $4)
		RETURNING id, username, email, password, nickname, avatar_url, status, created_at, updated_at, last_login
	`

	user := &types.User{}
	var avatarURL sql.NullString
	var lastLogin sql.NullTime

	err := s.db.QueryRow(query, username, email, hashedPassword, nickname).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.Nickname,
		&avatarURL,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLogin,
	)
	if err != nil {
		// 检查唯一约束冲突
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("用户名或邮箱已被注册")
		}
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// GetByID 根据 ID 查询用户
func (s *UserStore) GetByID(id string) (*types.User, error) {
	query := `
		SELECT id, username, email, password, nickname, avatar_url, status, created_at, updated_at, last_login
		FROM users
		WHERE id = $1
	`
	return s.scanUser(s.db.QueryRow(query, id))
}

// GetByUsername 根据用户名查询用户
func (s *UserStore) GetByUsername(username string) (*types.User, error) {
	query := `
		SELECT id, username, email, password, nickname, avatar_url, status, created_at, updated_at, last_login
		FROM users
		WHERE username = $1
	`
	return s.scanUser(s.db.QueryRow(query, username))
}

// GetByEmail 根据邮箱查询用户
func (s *UserStore) GetByEmail(email string) (*types.User, error) {
	query := `
		SELECT id, username, email, password, nickname, avatar_url, status, created_at, updated_at, last_login
		FROM users
		WHERE email = $1
	`
	return s.scanUser(s.db.QueryRow(query, email))
}

// UpdateLastLogin 更新用户最后登录时间
func (s *UserStore) UpdateLastLogin(id string) error {
	now := time.Now()
	_, err := s.db.Exec(`UPDATE users SET last_login = $1 WHERE id = $2`, now, id)
	if err != nil {
		return fmt.Errorf("更新最后登录时间失败: %w", err)
	}
	return nil
}

// scanUser 从 sql.Row 扫描用户数据
func (s *UserStore) scanUser(row *sql.Row) (*types.User, error) {
	user := &types.User{}
	var avatarURL sql.NullString
	var lastLogin sql.NullTime

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.Nickname,
		&avatarURL,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLogin,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // 未找到，返回 nil
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	if avatarURL.Valid {
		user.AvatarURL = avatarURL.String
	}
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// isUniqueViolation 判断是否是唯一约束冲突错误（PostgreSQL 错误码 23505）
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "23505") || contains(err.Error(), "unique")
}

// contains 简单字符串包含检查
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && len(substr) > 0 &&
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())
}
