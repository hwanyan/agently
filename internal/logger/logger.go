// Package logger 提供统一的日志管理功能
// 支持 INFO、ERROR、WARNING 三个级别，日志按日期划分文件
package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// 日志级别常量
const (
	LevelInfo    = "INFO"
	LevelError   = "ERROR"
	LevelWarning = "WARNING"
)

// 全局 logger 实例
var (
	infoLogger     *log.Logger
	errorLogger    *log.Logger
	warningLogger  *log.Logger
	currentLogFile *os.File
	logBasePath    string
)

// Init 初始化日志系统
// 参数：
//   - logPath: 日志文件基础路径（会自动添加日期后缀）
//
// 返回：
//   - error: 可能的错误
func Init(logPath string) error {
	logBasePath = logPath

	// 确保日志目录存在
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 打开当前日期的日志文件
	return openDailyLogFile()
}

// openDailyLogFile 打开当日日志文件
func openDailyLogFile() error {
	// 关闭之前的日志文件
	if currentLogFile != nil {
		currentLogFile.Close()
	}

	// 生成带日期的日志文件名
	today := time.Now().Format("2006-01-02")
	logPath := fmt.Sprintf("%s-%s.log", logBasePath, today)

	// 打开日志文件（追加模式）
	var err error
	currentLogFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	// 初始化各级别的 logger
	infoLogger = log.New(currentLogFile, "["+LevelInfo+"] ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLogger = log.New(currentLogFile, "["+LevelError+"] ", log.Ldate|log.Ltime|log.Lshortfile)
	warningLogger = log.New(currentLogFile, "["+LevelWarning+"] ", log.Ldate|log.Ltime|log.Lshortfile)

	// 记录日志系统启动信息
	Info("日志系统初始化成功，日志文件: %s", logPath)

	return nil
}

// checkAndRotateLogFile 检查并轮转日志文件（如果需要）
func checkAndRotateLogFile() {
	if currentLogFile == nil {
		return
	}

	// 检查当前日期是否变化
	today := time.Now().Format("2006-01-02")
	logPath := fmt.Sprintf("%s-%s.log", logBasePath, today)

	// 如果当前日志文件不是今天的，重新打开
	if currentLogFile.Name() != logPath {
		if err := openDailyLogFile(); err != nil {
			// 如果轮转失败，继续使用当前文件，但记录错误
			Error("日志文件轮转失败: %v", err)
		}
	}
}

// Close 关闭日志文件
func Close() {
	if currentLogFile != nil {
		Info("日志系统关闭")
		currentLogFile.Close()
	}
}

// Info 输出 INFO 级别日志
func Info(format string, v ...interface{}) {
	checkAndRotateLogFile()
	if infoLogger == nil {
		return
	}
	infoLogger.Output(2, fmt.Sprintf(format, v...))
}

// Error 输出 ERROR 级别日志
func Error(format string, v ...interface{}) {
	checkAndRotateLogFile()
	if errorLogger == nil {
		return
	}
	errorLogger.Output(2, fmt.Sprintf(format, v...))
}

// Warning 输出 WARNING 级别日志
func Warning(format string, v ...interface{}) {
	checkAndRotateLogFile()
	if warningLogger == nil {
		return
	}
	warningLogger.Output(2, fmt.Sprintf(format, v...))
}

// LogUserQuery 记录用户查询
func LogUserQuery(query string, agentID string, autoRoute bool) {
	routeMode := "手动"
	if autoRoute {
		routeMode = "自动"
	}
	Info("用户查询 - 内容: %s, Agent: %s, 路由模式: %s", query, agentID, routeMode)
}

// LogAgentResponse 记录Agent响应
func LogAgentResponse(response string, agentID string, processingTime time.Duration) {
	Info("Agent响应 - Agent: %s, 处理耗时: %v, 内容: %s", agentID, processingTime, response)
}

// LogToolUsage 记录工具使用情况
func LogToolUsage(toolName string, params interface{}, success bool, result interface{}) {
	status := "成功"
	if !success {
		status = "失败"
	}
	Info("工具调用 - 工具: %s, 参数: %+v, 状态: %s, 结果: %+v", toolName, params, status, result)
}

// LogRouteDecision 记录路由决策
func LogRouteDecision(userInput string, sourceAgent string, targetAgent string, reason string) {
	Info("路由决策 - 输入: %s, 来源: %s, 目标: %s, 原因: %s", userInput, sourceAgent, targetAgent, reason)
}

// LogSystemEvent 记录系统事件
func LogSystemEvent(event string, details interface{}) {
	Info("系统事件 - %s: %+v", event, details)
}

// LogHTTPRequest 记录HTTP请求
func LogHTTPRequest(method string, path string, userID string, agentID string) {
	Info("HTTP请求 - 方法: %s, 路径: %s, 用户: %s, Agent: %s", method, path, userID, agentID)
}

// LogHTTPResponse 记录HTTP响应
func LogHTTPResponse(method string, path string, statusCode int, responseTime time.Duration) {
	Info("HTTP响应 - 方法: %s, 路径: %s, 状态码: %d, 响应时间: %v", method, path, statusCode, responseTime)
}
