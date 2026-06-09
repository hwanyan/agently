-- ============================================================
-- agently 数据库 Schema
-- 数据库: PostgreSQL 16
-- 扩展: pgvector (用于 RAG 向量检索)
-- ============================================================

-- 启用 pgvector 扩展（需要提前安装：CREATE EXTENSION vector）
CREATE EXTENSION IF NOT EXISTS vector;

-- 启用 UUID 生成扩展
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- 1. 用户表 (users)
-- 存储注册用户的基本信息与认证凭据
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username    VARCHAR(64) NOT NULL UNIQUE,                  -- 用户名，唯一
    email       VARCHAR(255) NOT NULL UNIQUE,                 -- 邮箱，唯一
    password    VARCHAR(255) NOT NULL,                        -- bcrypt 哈希后的密码
    nickname    VARCHAR(128),                                 -- 昵称（可选）
    avatar_url  TEXT,                                         -- 头像 URL（可选）
    status      SMALLINT    NOT NULL DEFAULT 1,               -- 状态: 1=正常, 0=禁用
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login  TIMESTAMPTZ                                   -- 最后登录时间
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email    ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_status   ON users(status);

COMMENT ON TABLE  users              IS '用户表';
COMMENT ON COLUMN users.id           IS '用户唯一ID (UUID)';
COMMENT ON COLUMN users.username     IS '用户名，登录凭据之一，全局唯一';
COMMENT ON COLUMN users.email        IS '邮箱，登录凭据之一，全局唯一';
COMMENT ON COLUMN users.password     IS 'bcrypt 哈希后的密码，不存储明文';
COMMENT ON COLUMN users.status       IS '账号状态: 1=正常, 0=禁用';

-- ============================================================
-- 2. 会话表 (sessions)
-- 一个会话对应用户与某个 Agent 的一次完整对话上下文
-- 会话是消息的容器，也是 RAG 检索的基本单位
-- ============================================================
CREATE TABLE IF NOT EXISTS sessions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id    VARCHAR(64) NOT NULL,                         -- Agent 标识，如 "general", "gold_analysis"
    title       VARCHAR(255),                                 -- 会话标题（首条消息自动截取或用户手动设置）
    summary     TEXT,                                         -- 会话摘要（用于 RAG 检索，可由 LLM 自动生成）
    status      SMALLINT    NOT NULL DEFAULT 1,               -- 状态: 1=活跃, 0=已归档
    message_cnt INTEGER     NOT NULL DEFAULT 0,               -- 消息总数（冗余字段，避免 COUNT 查询）
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),           -- 最后活跃时间（每次新消息时更新）
    archived_at TIMESTAMPTZ                                   -- 归档时间
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id    ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_agent_id   ON sessions(agent_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_agent ON sessions(user_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_sessions_updated_at ON sessions(updated_at DESC);  -- 按最近活跃排序
CREATE INDEX IF NOT EXISTS idx_sessions_status     ON sessions(status);

COMMENT ON TABLE  sessions              IS '会话表，一个会话对应一次完整的对话上下文';
COMMENT ON COLUMN sessions.user_id      IS '所属用户 ID';
COMMENT ON COLUMN sessions.agent_id     IS '对话的 Agent 标识';
COMMENT ON COLUMN sessions.title        IS '会话标题，首条用户消息的前 50 字，或用户手动设置';
COMMENT ON COLUMN sessions.summary      IS '会话摘要，由 LLM 自动生成，用于 RAG 语义检索';
COMMENT ON COLUMN sessions.message_cnt  IS '消息总数，冗余字段，避免频繁 COUNT';
COMMENT ON COLUMN sessions.status       IS '会话状态: 1=活跃, 0=已归档';

-- ============================================================
-- 3. 消息表 (messages)
-- 存储每条对话消息，包含用户消息、AI 回复、工具调用结果
-- ============================================================
CREATE TABLE IF NOT EXISTS messages (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      UUID        NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            VARCHAR(16) NOT NULL,                     -- 消息角色: user / assistant / tool / system
    content         TEXT        NOT NULL DEFAULT '',          -- 消息文本内容
    tool_call_id    VARCHAR(128),                             -- 工具调用 ID（role=tool 时有值）
    tool_call_list JSONB,                                    -- 工具调用列表（role=assistant 且有工具调用时有值）
    agent_id        VARCHAR(64),                              -- 处理此消息的 Agent ID
    token_count     INTEGER     NOT NULL DEFAULT 0,           -- 本条消息消耗的 token 数（估算）
    seq             INTEGER     NOT NULL DEFAULT 0,           -- 消息在会话中的顺序号（从 1 开始）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 核心查询索引：按会话查询消息列表（最常用）
CREATE INDEX IF NOT EXISTS idx_messages_session_id     ON messages(session_id, seq ASC);
-- 按用户查询所有消息（用于统计）
CREATE INDEX IF NOT EXISTS idx_messages_user_id        ON messages(user_id);
-- 按角色过滤（如只查 user/assistant 消息）
CREATE INDEX IF NOT EXISTS idx_messages_role           ON messages(session_id, role);
-- 时间范围查询
CREATE INDEX IF NOT EXISTS idx_messages_created_at     ON messages(created_at DESC);

COMMENT ON TABLE  messages               IS '消息表，存储每条对话消息';
COMMENT ON COLUMN messages.session_id    IS '所属会话 ID';
COMMENT ON COLUMN messages.user_id       IS '所属用户 ID（冗余，避免 JOIN）';
COMMENT ON COLUMN messages.role          IS '消息角色: user/assistant/tool/system';
COMMENT ON COLUMN messages.content       IS '消息文本内容';
COMMENT ON COLUMN messages.tool_call_id  IS '工具调用 ID，role=tool 时有值，与 tool_calls 中的 id 对应';
COMMENT ON COLUMN messages.tool_call_list IS 'JSONB 格式的工具调用列表，role=assistant 且触发工具调用时有值';
COMMENT ON COLUMN messages.agent_id      IS '处理此消息的 Agent 标识';
COMMENT ON COLUMN messages.token_count   IS '本条消息消耗的 token 数（估算值）';
COMMENT ON COLUMN messages.seq           IS '消息在会话中的顺序号，从 1 开始递增';

-- ============================================================
-- 4. 消息向量表 (message_embeddings)
-- 为 RAG 检索存储消息的向量表示
-- 依赖 pgvector 扩展，需先执行: CREATE EXTENSION IF NOT EXISTS vector;
-- 向量维度 1536 对应 OpenAI text-embedding-3-small
-- 向量维度 1024 对应 DeepSeek embedding 模型
-- ============================================================
CREATE TABLE IF NOT EXISTS message_embeddings (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id  UUID        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    session_id  UUID        NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    embedding   vector(1536) NOT NULL,                     -- 消息内容的向量表示
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- HNSW 索引（高性能近似最近邻搜索，适合生产环境）
CREATE INDEX IF NOT EXISTS idx_message_embeddings_hnsw
    ON message_embeddings USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- 按用户过滤向量检索（RAG 时只检索当前用户的历史）
CREATE INDEX IF NOT EXISTS idx_message_embeddings_user_id
    ON message_embeddings(user_id);

COMMENT ON TABLE  message_embeddings            IS '消息向量表，用于 RAG 语义检索';
COMMENT ON COLUMN message_embeddings.embedding  IS '消息内容的向量表示，维度 1536（OpenAI）或 1024（DeepSeek）';

-- ============================================================
-- 5. 自动更新 updated_at 的触发器
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE OR REPLACE TRIGGER trg_sessions_updated_at
    BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ============================================================
-- 6. 会话消息计数自动维护触发器
-- 每次插入消息时自动更新 sessions.message_cnt 和 sessions.updated_at
-- ============================================================
CREATE OR REPLACE FUNCTION update_session_on_message()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE sessions
    SET message_cnt = message_cnt + 1,
        updated_at  = NOW()
    WHERE id = NEW.session_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER trg_messages_update_session
    AFTER INSERT ON messages
    FOR EACH ROW EXECUTE FUNCTION update_session_on_message();

-- ============================================================
-- 7. 邮箱验证码表 (email_verifications)
-- 存储邮箱验证码，用于注册、重置密码等场景
-- ============================================================
CREATE TABLE IF NOT EXISTS email_verifications (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email       VARCHAR(255) NOT NULL,                        -- 目标邮箱
    code        VARCHAR(10)  NOT NULL,                        -- 验证码（6位数字）
    purpose     VARCHAR(32)  NOT NULL DEFAULT 'register',     -- 用途: register, reset_password
    used        BOOLEAN      NOT NULL DEFAULT false,          -- 是否已使用
    expires_at  TIMESTAMPTZ  NOT NULL,                        -- 过期时间
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_verifications_email   ON email_verifications(email, purpose, used);
CREATE INDEX IF NOT EXISTS idx_email_verifications_expires ON email_verifications(expires_at);

COMMENT ON TABLE  email_verifications           IS '邮箱验证码表';
COMMENT ON COLUMN email_verifications.email     IS '目标邮箱地址';
COMMENT ON COLUMN email_verifications.code      IS '6位数字验证码';
COMMENT ON COLUMN email_verifications.purpose   IS '用途: register=注册, reset_password=重置密码';
COMMENT ON COLUMN email_verifications.used      IS '是否已使用';
COMMENT ON COLUMN email_verifications.expires_at IS '验证码过期时间';
