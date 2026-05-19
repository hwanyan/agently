// Package store 提供数据库访问层
package store

import (
	"database/sql"
	"fmt"
	"time"

	"agent-hwan/pkg/types"

	_ "github.com/lib/pq" // PostgreSQL 驱动
)

// DB 是全局数据库连接池的封装
type DB struct {
	*sql.DB
}

// New 创建并初始化数据库连接池
// 参数：
//   - cfg: 数据库配置
//
// 返回：
//   - *DB: 数据库连接池实例
//   - error: 可能的错误
func New(cfg types.DatabaseConfig) (*DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("数据库 DSN 不能为空")
	}

	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 配置连接池参数
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 50
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 5
	}
	maxLifetime := cfg.ConnMaxLifetime
	if maxLifetime <= 0 {
		maxLifetime = 300
	}

	// 设置最大打开连接数
	db.SetMaxOpenConns(maxOpen)
	// 设置最大空闲连接数
	db.SetMaxIdleConns(maxIdle)
	// 设置连接可被复用的最大存活时间
	db.SetConnMaxLifetime(time.Duration(maxLifetime) * time.Second)

	// 验证连接是否可用
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	return &DB{db}, nil
}

// Close 关闭数据库连接池
func (d *DB) Close() error {
	return d.DB.Close()
}
