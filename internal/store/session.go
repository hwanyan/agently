package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/hwanyan/agently/pkg/types"
)

// SessionStore 会话数据访问对象
type SessionStore struct {
	db *DB
}

// NewSessionStore 创建会话数据访问对象
func NewSessionStore(db *DB) *SessionStore {
	return &SessionStore{db: db}
}

// Create 创建新会话
// 参数：
//   - userID: 用户 ID
//   - agentID: Agent 标识
//   - title: 会话标题（可为空，后续由首条消息自动填充）
//
// 返回：
//   - *types.Session: 创建成功的会话
//   - error: 可能的错误
func (s *SessionStore) Create(userID, agentID, title string) (*types.Session, error) {
	query := `
		INSERT INTO sessions (user_id, agent_id, title)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, agent_id, title, summary, status, message_cnt, created_at, updated_at, archived_at
	`

	session := &types.Session{}
	var titleVal, summaryVal sql.NullString
	var archivedAt sql.NullTime

	err := s.db.QueryRow(query, userID, agentID, nullableString(title)).Scan(
		&session.ID,
		&session.UserID,
		&session.AgentID,
		&titleVal,
		&summaryVal,
		&session.Status,
		&session.MessageCnt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&archivedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}

	if titleVal.Valid {
		session.Title = titleVal.String
	}
	if summaryVal.Valid {
		session.Summary = summaryVal.String
	}
	if archivedAt.Valid {
		session.ArchivedAt = &archivedAt.Time
	}

	return session, nil
}

// GetByID 根据 ID 查询会话
// 同时校验 userID，防止越权访问
func (s *SessionStore) GetByID(id, userID string) (*types.Session, error) {
	query := `
		SELECT id, user_id, agent_id, title, summary, status, message_cnt, created_at, updated_at, archived_at
		FROM sessions
		WHERE id = $1 AND user_id = $2
	`
	return s.scanSession(s.db.QueryRow(query, id, userID))
}

// ListByUser 查询用户的会话列表（按最近活跃时间倒序）
// 参数：
//   - userID: 用户 ID
//   - agentID: Agent 标识（可为空，为空时查询所有 Agent 的会话）
//   - limit: 每页数量
//   - offset: 偏移量
func (s *SessionStore) ListByUser(userID, agentID string, limit, offset int) ([]*types.Session, error) {
	var (
		rows *sql.Rows
		err  error
	)

	if agentID != "" {
		rows, err = s.db.Query(`
			SELECT id, user_id, agent_id, title, summary, status, message_cnt, created_at, updated_at, archived_at
			FROM sessions
			WHERE user_id = $1 AND agent_id = $2 AND status = 1
			ORDER BY updated_at DESC
			LIMIT $3 OFFSET $4
		`, userID, agentID, limit, offset)
	} else {
		rows, err = s.db.Query(`
			SELECT id, user_id, agent_id, title, summary, status, message_cnt, created_at, updated_at, archived_at
			FROM sessions
			WHERE user_id = $1 AND status = 1
			ORDER BY updated_at DESC
			LIMIT $2 OFFSET $3
		`, userID, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("查询会话列表失败: %w", err)
	}
	defer rows.Close()

	var sessions []*types.Session
	for rows.Next() {
		session := &types.Session{}
		var titleVal, summaryVal sql.NullString
		var archivedAt sql.NullTime

		if err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.AgentID,
			&titleVal,
			&summaryVal,
			&session.Status,
			&session.MessageCnt,
			&session.CreatedAt,
			&session.UpdatedAt,
			&archivedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描会话数据失败: %w", err)
		}

		if titleVal.Valid {
			session.Title = titleVal.String
		}
		if summaryVal.Valid {
			session.Summary = summaryVal.String
		}
		if archivedAt.Valid {
			session.ArchivedAt = &archivedAt.Time
		}

		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

// UpdateTitle 更新会话标题
func (s *SessionStore) UpdateTitle(id, userID, title string) error {
	result, err := s.db.Exec(
		`UPDATE sessions SET title = $1 WHERE id = $2 AND user_id = $3`,
		title, id, userID,
	)
	if err != nil {
		return fmt.Errorf("更新会话标题失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("会话不存在或无权限")
	}
	return nil
}

// Archive 归档会话（软删除）
func (s *SessionStore) Archive(id, userID string) error {
	result, err := s.db.Exec(
		`UPDATE sessions SET status = 0, archived_at = NOW() WHERE id = $1 AND user_id = $2 AND status = 1`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("归档会话失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("会话不存在或已归档")
	}
	return nil
}

// scanSession 从 sql.Row 扫描会话数据
func (s *SessionStore) scanSession(row *sql.Row) (*types.Session, error) {
	session := &types.Session{}
	var titleVal, summaryVal sql.NullString
	var archivedAt sql.NullTime

	err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.AgentID,
		&titleVal,
		&summaryVal,
		&session.Status,
		&session.MessageCnt,
		&session.CreatedAt,
		&session.UpdatedAt,
		&archivedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("查询会话失败: %w", err)
	}

	if titleVal.Valid {
		session.Title = titleVal.String
	}
	if summaryVal.Valid {
		session.Summary = summaryVal.String
	}
	if archivedAt.Valid {
		session.ArchivedAt = &archivedAt.Time
	}

	return session, nil
}

// nullableString 将空字符串转换为 NULL
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
