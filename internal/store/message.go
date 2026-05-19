package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"agent-hwan/pkg/types"
)

// MessageStore 消息数据访问对象
type MessageStore struct {
	db *DB
}

// NewMessageStore 创建消息数据访问对象
func NewMessageStore(db *DB) *MessageStore {
	return &MessageStore{db: db}
}

// SaveMessage 保存一条消息到数据库
// 参数：
//   - sessionID: 会话 ID
//   - userID: 用户 ID
//   - agentID: 处理此消息的 Agent ID
//   - msg: 内存中的消息对象
//
// 返回：
//   - *types.StoredMessage: 持久化后的消息
//   - error: 可能的错误
func (s *MessageStore) SaveMessage(sessionID, userID, agentID string, msg types.Message) (*types.StoredMessage, error) {
	// 序列化工具调用列表
	toolCallsJSON := ""
	if len(msg.ToolCallList) > 0 {
		b, err := json.Marshal(msg.ToolCallList)
		if err != nil {
			return nil, fmt.Errorf("序列化工具调用失败: %w", err)
		}
		toolCallsJSON = string(b)
	}

	// 获取当前会话的最大 seq，用于递增
	var maxSeq int
	err := s.db.QueryRow(
		`SELECT COALESCE(MAX(seq), 0) FROM messages WHERE session_id = $1`,
		sessionID,
	).Scan(&maxSeq)
	if err != nil {
		return nil, fmt.Errorf("获取消息序号失败: %w", err)
	}
	nextSeq := maxSeq + 1

	query := `
		INSERT INTO messages (session_id, user_id, role, content, tool_call_id, tool_call_list, agent_id, seq)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, session_id, user_id, role, content, tool_call_id, tool_call_list, agent_id, token_count, seq, created_at
	`

	stored := &types.StoredMessage{}
	var toolCallID sql.NullString
	var toolCallsVal sql.NullString

	err = s.db.QueryRow(
		query,
		sessionID,
		userID,
		msg.Role,
		msg.Content,
		nullableString(msg.ToolCallID),
		nullableString(toolCallsJSON),
		nullableString(agentID),
		nextSeq,
	).Scan(
		&stored.ID,
		&stored.SessionID,
		&stored.UserID,
		&stored.Role,
		&stored.Content,
		&toolCallID,
		&toolCallsVal,
		&stored.AgentID,
		&stored.TokenCount,
		&stored.Seq,
		&stored.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("保存消息失败: %w", err)
	}

	if toolCallID.Valid {
		stored.ToolCallID = toolCallID.String
	}
	if toolCallsVal.Valid {
		stored.ToolCallList = toolCallsVal.String
	}

	return stored, nil
}

// SaveMessages 批量保存消息（事务）
// 用于会话恢复时批量写入历史消息
func (s *MessageStore) SaveMessages(sessionID, userID, agentID string, msgs []types.Message) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 获取当前最大 seq
	var maxSeq int
	if err := tx.QueryRow(
		`SELECT COALESCE(MAX(seq), 0) FROM messages WHERE session_id = $1`, sessionID,
	).Scan(&maxSeq); err != nil {
		return fmt.Errorf("获取消息序号失败: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO messages (session_id, user_id, role, content, tool_call_id, tool_call_list, agent_id, seq)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	
	`)
	if err != nil {
		return fmt.Errorf("准备 SQL 语句失败: %w", err)
	}
	defer stmt.Close()

	for i, msg := range msgs {
		toolCallsJSON := ""
		if len(msg.ToolCallList) > 0 {
			b, _ := json.Marshal(msg.ToolCallList)
			toolCallsJSON = string(b)
		}

		if _, err := stmt.Exec(
			sessionID,
			userID,
			msg.Role,
			msg.Content,
			nullableString(msg.ToolCallID),
			nullableString(toolCallsJSON),
			nullableString(agentID),
			maxSeq+i+1,
		); err != nil {
			return fmt.Errorf("批量保存消息失败（第 %d 条）: %w", i+1, err)
		}
	}

	return tx.Commit()
}

// ListBySession 查询会话的消息列表（按 seq 升序）
// 参数：
//   - sessionID: 会话 ID
//   - userID: 用户 ID（用于权限校验）
//   - limit: 最多返回条数（0 表示不限制）
//   - offset: 偏移量
func (s *MessageStore) ListBySession(sessionID, userID string, limit, offset int) ([]*types.StoredMessage, error) {
	query := `
		SELECT id, session_id, user_id, role, content, tool_call_id, tool_call_list, agent_id, token_count, seq, created_at
		FROM messages
		WHERE session_id = $1 AND user_id = $2
		ORDER BY seq ASC
	`
	args := []interface{}{sessionID, userID}

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询消息列表失败: %w", err)
	}
	defer rows.Close()

	var messages []*types.StoredMessage
	for rows.Next() {
		msg := &types.StoredMessage{}
		var toolCallID, toolCallsVal sql.NullString

		if err := rows.Scan(
			&msg.ID,
			&msg.SessionID,
			&msg.UserID,
			&msg.Role,
			&msg.Content,
			&toolCallID,
			&toolCallsVal,
			&msg.AgentID,
			&msg.TokenCount,
			&msg.Seq,
			&msg.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描消息数据失败: %w", err)
		}

		if toolCallID.Valid {
			msg.ToolCallID = toolCallID.String
		}
		if toolCallsVal.Valid {
			msg.ToolCallList = toolCallsVal.String
		}

		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// ToMemoryMessages 将持久化消息列表转换为内存消息列表（用于恢复 Agent 上下文）
// 会过滤掉 system 消息（system prompt 由 Agent 自身管理）
func ToMemoryMessages(stored []*types.StoredMessage) []types.Message {
	var msgs []types.Message
	for _, s := range stored {
		if s.Role == "system" {
			continue
		}

		msg := types.Message{
			ID:         s.ID,
			Role:       s.Role,
			Content:    s.Content,
			ToolCallID: s.ToolCallID,
			Timestamp:  s.CreatedAt,
		}

		// 反序列化工具调用列表
		if s.ToolCallList != "" {
			var toolCalls []types.ToolCall
			if err := json.Unmarshal([]byte(s.ToolCallList), &toolCalls); err == nil {
				msg.ToolCallList = toolCalls
			}
		}

		msgs = append(msgs, msg)
	}
	return msgs
}
