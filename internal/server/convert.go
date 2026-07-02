package server

import (
	"time"

	agentlypb "github.com/hwanyan/agently/pb/gen"
	"github.com/hwanyan/agently/pkg/types"
)

// formatTime 将时间格式化为 RFC3339；零值返回空字符串
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// formatTimePtr 将 *time.Time 格式化为 RFC3339；nil 返回空字符串
func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

// toPBUser 将领域模型 User 转为 pb.User（不含密码）
func toPBUser(u *types.User) *agentlypb.User {
	if u == nil {
		return nil
	}
	return &agentlypb.User{
		Id:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Nickname:  u.Nickname,
		AvatarUrl: u.AvatarURL,
		Status:    int32(u.Status),
		CreatedAt: formatTime(u.CreatedAt),
		UpdatedAt: formatTime(u.UpdatedAt),
		LastLogin: formatTimePtr(u.LastLogin),
	}
}

// toPBSession 将领域模型 Session 转为 pb.Session
func toPBSession(s *types.Session) *agentlypb.Session {
	if s == nil {
		return nil
	}
	return &agentlypb.Session{
		Id:         s.ID,
		UserId:     s.UserID,
		AgentId:    s.AgentID,
		Title:      s.Title,
		Summary:    s.Summary,
		Status:     int32(s.Status),
		MessageCnt: int32(s.MessageCnt),
		CreatedAt:  formatTime(s.CreatedAt),
		UpdatedAt:  formatTime(s.UpdatedAt),
		ArchivedAt: formatTimePtr(s.ArchivedAt),
	}
}

// toPBMessage 将持久化消息转为 pb.StoredMessage
func toPBMessage(m *types.StoredMessage) *agentlypb.StoredMessage {
	if m == nil {
		return nil
	}
	return &agentlypb.StoredMessage{
		Id:           m.ID,
		SessionId:    m.SessionID,
		UserId:       m.UserID,
		Role:         m.Role,
		Content:      m.Content,
		ToolCallId:   m.ToolCallID,
		ToolCallList: m.ToolCallList,
		AgentId:      m.AgentID,
		TokenCount:   int32(m.TokenCount),
		Seq:          int32(m.Seq),
		CreatedAt:    formatTime(m.CreatedAt),
	}
}
