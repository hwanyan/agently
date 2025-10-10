// Package tools 提供AI Agent可以使用的各种工具
package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"agent-hwan/pkg/utils"
)

// 定义计算器工具的默认名称和描述
const (
	calDefaultName = "calculator"
	calDefaultDesc = "执行基本的数学运算，支持加法、减法、乘法、除法运算"
)

// CalculatorTool 计算器工具
// 这个工具可以执行基本的数学运算，包括加减乘除
type CalculatorTool struct {
	name        string // 工具名称
	description string // 工具描述
}

// NewCalculatorTool 创建一个新的计算器工具实例
// 返回：
//   - *CalculatorTool: 计算器工具实例
func NewCalculatorTool(name, desc string) *CalculatorTool {
	// 如果 name 为空字符串，则使用默认名称
	if name == "" {
		name = calDefaultName
	}
	// 如果 desc 为空字符串，则使用默认描述
	if desc == "" {
		desc = calDefaultDesc
	}
	return &CalculatorTool{
		name:        name,
		description: desc,
	}
}

// Name 返回工具名称
// 实现types.Tool接口
func (c *CalculatorTool) Name() string {
	return c.name
}

// Description 返回工具描述
// 实现types.Tool接口
func (c *CalculatorTool) Description() string {
	return c.description
}

// Parameters 返回工具参数定义
// 实现types.Tool接口，返回JSON Schema格式的参数定义
func (c *CalculatorTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"expression": map[string]interface{}{
				"type":        "string",
				"description": "要计算的数学表达式，例如：'2 + 3', '10 * 5', '100 / 4'",
			},
		},
		"required": []string{"expression"},
	}
}

// Execute 执行计算器工具
// 实现types.Tool接口
// 参数：
//   - ctx: 上下文
//   - args: 工具参数，应包含"expression"字段
//
// 返回：
//   - string: 计算结果
//   - error: 可能的错误
func (c *CalculatorTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 从参数中获取表达式
	expressionInterface, exists := args["expression"]
	if !exists {
		return "", fmt.Errorf("缺少必需参数: expression")
	}

	// 安全地转换为字符串
	expression, ok := utils.SafeString(expressionInterface)
	if !ok {
		return "", fmt.Errorf("expression参数必须是字符串类型")
	}

	// 清理输入
	expression = utils.SanitizeInput(expression)
	if utils.IsEmpty(expression) {
		return "", fmt.Errorf("表达式不能为空")
	}

	// 解析并计算表达式
	result, err := c.evaluateExpression(expression)
	if err != nil {
		return "", fmt.Errorf("计算表达式失败: %w", err)
	}

	return fmt.Sprintf("计算结果: %s = %g", expression, result), nil
}

// evaluateExpression 解析并计算数学表达式
// 这是一个简化的表达式计算器，支持基本的四则运算
// 参数：
//   - expression: 数学表达式字符串
//
// 返回：
//   - float64: 计算结果
//   - error: 可能的错误
func (c *CalculatorTool) evaluateExpression(expression string) (float64, error) {
	// 移除所有空格
	expression = strings.ReplaceAll(expression, " ", "")

	// 检查表达式是否为空（似乎和前面的判断表达式是否为空重复了）
	if expression == "" {
		return 0, fmt.Errorf("表达式为空")
	}

	// 先处理加法和减法（优先级较低）
	for _, op := range []string{"+", "-"} {
		if idx := c.findLastOperator(expression, op); idx != -1 {
			left := expression[:idx]
			right := expression[idx+1:]

			leftVal, err := c.evaluateExpression(left)
			if err != nil {
				return 0, err
			}

			rightVal, err := c.evaluateExpression(right)
			if err != nil {
				return 0, err
			}

			switch op {
			case "+":
				return leftVal + rightVal, nil
			case "-":
				return leftVal - rightVal, nil
			}
		}
	}

	// 再处理乘法和除法（优先级较高）
	for _, op := range []string{"*", "/"} {
		if idx := c.findLastOperator(expression, op); idx != -1 {
			left := expression[:idx]
			right := expression[idx+1:]

			leftVal, err := c.evaluateExpression(left)
			if err != nil {
				return 0, err
			}

			rightVal, err := c.evaluateExpression(right)
			if err != nil {
				return 0, err
			}

			switch op {
			case "*":
				return leftVal * rightVal, nil
			case "/":
				if rightVal == 0 {
					return 0, fmt.Errorf("除数不能为零")
				}
				return leftVal / rightVal, nil
			}
		}
	}

	// 如果没有找到运算符，尝试解析为数字
	return c.parseNumber(expression)
}

// findLastOperator 查找表达式中最后一个指定运算符的位置
// 这样可以确保正确的运算优先级
// 参数：
//   - expression: 表达式字符串
//   - operator: 要查找的运算符
//
// 返回：
//   - int: 运算符的位置，如果未找到返回-1
func (c *CalculatorTool) findLastOperator(expression, operator string) int {
	// 从右到左查找，确保正确的结合性
	for i := len(expression) - 1; i >= 0; i-- {
		if string(expression[i]) == operator {
			// 确保不是负号（如果是减号且在开头或前面是运算符）
			if operator == "-" && (i == 0 || c.isOperator(string(expression[i-1]))) {
				continue
			}
			return i
		}
	}
	return -1
}

// isOperator 检查字符是否为运算符
func (c *CalculatorTool) isOperator(char string) bool {
	return char == "+" || char == "-" || char == "*" || char == "/"
}

// parseNumber 解析数字字符串
// 参数：
//   - numStr: 数字字符串
//
// 返回：
//   - float64: 解析后的数字
//   - error: 可能的错误
func (c *CalculatorTool) parseNumber(numStr string) (float64, error) {
	// 移除首尾空格
	numStr = strings.TrimSpace(numStr)

	if numStr == "" {
		return 0, fmt.Errorf("数字字符串为空")
	}

	// 使用strconv.ParseFloat解析浮点数
	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("无法解析数字 '%s': %w", numStr, err)
	}

	return num, nil
}

// GetSupportedOperations 获取支持的运算操作列表
// 返回：
//   - []string: 支持的运算符列表
func (c *CalculatorTool) GetSupportedOperations() []string {
	return []string{
		"加法 (+)",
		"减法 (-)",
		"乘法 (*)",
		"除法 (/)",
	}
}

// ValidateExpression 验证表达式的有效性
// 参数：
//   - expression: 要验证的表达式
//
// 返回：
//   - bool: 表达式是否有效
//   - error: 验证错误信息
func (c *CalculatorTool) ValidateExpression(expression string) (bool, error) {
	// 基本验证：检查是否包含有效字符
	validChars := "0123456789+-*/.() "
	for _, char := range expression {
		if !strings.ContainsRune(validChars, char) {
			return false, fmt.Errorf("表达式包含无效字符: %c", char)
		}
	}

	// 检查括号匹配（简化版本）
	openCount := strings.Count(expression, "(")
	closeCount := strings.Count(expression, ")")
	if openCount != closeCount {
		return false, fmt.Errorf("括号不匹配")
	}

	// 检查是否以运算符结尾
	expression = strings.TrimSpace(expression)
	if len(expression) > 0 {
		lastChar := string(expression[len(expression)-1])
		if c.isOperator(lastChar) {
			return false, fmt.Errorf("表达式不能以运算符结尾")
		}
	}

	return true, nil
}
