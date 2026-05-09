// Package logger 提供统一的日志管理功能
// 支持 INFO、ERROR、WARNING 三个级别，日志输出到文件
package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// 日志级别常量
const (
	LevelInfo    = "INFO"
	LevelError   = "ERROR"
	LevelWarning = "WARNING"
)

// 全局 logger 实例
var (
	infoLogger    *log.Logger
	errorLogger   *log.Logger
	warningLogger *log.Logger
	logFile       *os.File
)

// Init 初始化日志系统
// 参数：
//   - logPath: 日志文件路径
//
// 返回：
//   - error: 可能的错误
func Init(logPath string) error {
	// 确保日志目录存在
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 打开日志文件（追加模式）
	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	// 初始化各级别的 logger
	infoLogger = log.New(logFile, "[INFO] ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger = log.New(logFile, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile)
	warningLogger = log.New(logFile, "[WARNING] ", log.Ldate|log.Ltime|log.Lshortfile)

	return nil
}

// Close 关闭日志文件
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

// Info 输出 INFO 级别日志
func Info(format string, v ...interface{}) {
	if infoLogger == nil {
		return
	}
	infoLogger.Output(2, fmt.Sprintf(format, v...))
}

// Error 输出 ERROR 级别日志
func Error(format string, v ...interface{}) {
	if errorLogger == nil {
		return
	}
	errorLogger.Output(2, fmt.Sprintf(format, v...))
}

// Warning 输出 WARNING 级别日志
func Warning(format string, v ...interface{}) {
	if warningLogger == nil {
		return
	}
	warningLogger.Output(2, fmt.Sprintf(format, v...))
}
