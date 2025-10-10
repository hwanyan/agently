// Package utils 提供通用的工具函数
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// GenerateID 生成唯一ID
// 使用当前时间戳和随机数生成唯一标识符
// 返回：
//   - string: 生成的唯一ID
func GenerateID() string {
	// 使用时间戳作为前缀
	timestamp := time.Now().UnixNano()
	
	// 生成4字节随机数
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomHex := hex.EncodeToString(randomBytes)
	
	return fmt.Sprintf("%d_%s", timestamp, randomHex)
}

// TruncateString 截断字符串
// 如果字符串长度超过指定长度，则截断并添加省略号
// 参数：
//   - s: 要截断的字符串
//   - maxLen: 最大长度
//
// 返回：
//   - string: 截断后的字符串
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	
	if maxLen <= 3 {
		return s[:maxLen]
	}
	
	return s[:maxLen-3] + "..."
}

// FormatDuration 格式化时间间隔
// 将time.Duration格式化为人类可读的字符串
// 参数：
//   - d: 时间间隔
//
// 返回：
//   - string: 格式化后的字符串
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1e6)
	} else if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

// SanitizeInput 清理用户输入
// 移除潜在的危险字符和多余的空白字符
// 参数：
//   - input: 用户输入
//
// 返回：
//   - string: 清理后的输入
func SanitizeInput(input string) string {
	// 移除首尾空白字符
	input = strings.TrimSpace(input)
	
	// 替换多个连续的空白字符为单个空格
	input = strings.Join(strings.Fields(input), " ")
	
	// 移除一些潜在危险的字符（根据需要调整）
	dangerousChars := []string{"\x00", "\x01", "\x02", "\x03", "\x04", "\x05"}
	for _, char := range dangerousChars {
		input = strings.ReplaceAll(input, char, "")
	}
	
	return input
}

// IsEmpty 检查字符串是否为空
// 检查字符串是否为空或只包含空白字符
// 参数：
//   - s: 要检查的字符串
//
// 返回：
//   - bool: 如果为空返回true，否则返回false
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// Contains 检查切片是否包含指定元素
// 泛型函数，可以用于任何可比较的类型
// 参数：
//   - slice: 要搜索的切片
//   - item: 要查找的元素
//
// 返回：
//   - bool: 如果包含返回true，否则返回false
func Contains[T comparable](slice []T, item T) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// RemoveDuplicates 移除切片中的重复元素
// 泛型函数，保持元素的原始顺序
// 参数：
//   - slice: 包含重复元素的切片
//
// 返回：
//   - []T: 移除重复元素后的切片
func RemoveDuplicates[T comparable](slice []T) []T {
	seen := make(map[T]bool)
	result := make([]T, 0, len(slice))
	
	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// SafeString 安全地从interface{}转换为字符串
// 参数：
//   - v: 要转换的值
//
// 返回：
//   - string: 转换后的字符串
//   - bool: 转换是否成功
func SafeString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case fmt.Stringer:
		return val.String(), true
	default:
		return fmt.Sprintf("%v", val), false
	}
}

// SafeInt 安全地从interface{}转换为整数
// 参数：
//   - v: 要转换的值
//
// 返回：
//   - int: 转换后的整数
//   - bool: 转换是否成功
func SafeInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	case float32:
		return int(val), true
	case float64:
		return int(val), true
	default:
		return 0, false
	}
}

// SafeFloat64 安全地从interface{}转换为float64
// 参数：
//   - v: 要转换的值
//
// 返回：
//   - float64: 转换后的浮点数
//   - bool: 转换是否成功
func SafeFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

// Retry 重试函数
// 在指定次数内重试执行函数，直到成功或达到最大重试次数
// 参数：
//   - maxRetries: 最大重试次数
//   - delay: 重试间隔
//   - fn: 要执行的函数
//
// 返回：
//   - error: 最后一次执行的错误，如果成功则为nil
func Retry(maxRetries int, delay time.Duration, fn func() error) error {
	var lastErr error
	
	for i := 0; i <= maxRetries; i++ {
		if err := fn(); err == nil {
			return nil // 成功执行
		} else {
			lastErr = err
			if i < maxRetries {
				time.Sleep(delay)
			}
		}
	}
	
	return fmt.Errorf("重试%d次后仍然失败，最后错误: %w", maxRetries, lastErr)
}