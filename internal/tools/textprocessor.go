package tools

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// 文本处理工具常量
const (
	// TextProcessorToolName 工具名称
	TextProcessorToolName = "textprocessor"

	// TextProcessorToolDesc 工具描述
	TextProcessorToolDesc = "文本处理工具：提供文本统计、格式转换、内容提取、文本生成等多种文本处理功能。"
)

// TextProcessorTool 文本处理工具
type TextProcessorTool struct {
	name        string // 工具名称
	description string // 工具描述
}

// NewTextProcessorTool 创建文本处理工具实例
func NewTextProcessorTool() *TextProcessorTool {
	return &TextProcessorTool{
		name:        TextProcessorToolName,
		description: TextProcessorToolDesc,
	}
}

// Name 返回工具名称
func (t *TextProcessorTool) Name() string { return t.name }

// Description 返回工具描述
func (t *TextProcessorTool) Description() string { return t.description }

// Parameters 返回参数定义（JSON Schema）
func (t *TextProcessorTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "要执行的操作类型",
				"enum": []string{
					"stats",     // 文本统计
					"uppercase", // 转大写
					"lowercase", // 转小写
					"title",     // 标题格式
					"trim",      // 去除首尾空格
					"reverse",   // 反转文本
					"extract",   // 内容提取
					"generate",  // 文本生成
					"replace",   // 文本替换
					"split",     // 文本分割
					"join",      // 文本连接
				},
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "要处理的文本内容（generate操作时可选）",
			},
			"pattern": map[string]interface{}{
				"type":        "string",
				"description": "提取模式（extract操作）或替换模式（replace操作）",
				"enum":        []string{"email", "url", "phone", "number", "chinese", "english"},
			},
			"replacement": map[string]interface{}{
				"type":        "string",
				"description": "替换后的文本（replace操作使用）",
			},
			"separator": map[string]interface{}{
				"type":        "string",
				"description": "分隔符（split/join操作使用）",
				"default":     ",",
			},
			"generate_type": map[string]interface{}{
				"type":        "string",
				"description": "生成类型（generate操作使用）",
				"enum":        []string{"uuid", "password", "random_string", "lorem"},
			},
			"length": map[string]interface{}{
				"type":        "integer",
				"description": "生成长度（generate操作的password/random_string使用）",
				"default":     12,
				"minimum":     4,
				"maximum":     128,
			},
		},
		"required": []string{"action"},
	}
}

// Execute 执行文本处理
func (t *TextProcessorTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 解析参数
	var p struct {
		Action       string   `json:"action"`
		Text         string   `json:"text"`
		Pattern      string   `json:"pattern"`
		Replacement  string   `json:"replacement"`
		Separator    string   `json:"separator"`
		GenerateType string   `json:"generate_type"`
		Length       int      `json:"length"`
		TextArray    []string `json:"text_array"`
	}

	// 设置默认值
	p.Separator = ","
	p.Length = 12

	// 从 map 中解析参数
	if v, ok := args["action"].(string); ok {
		p.Action = strings.TrimSpace(v)
	}
	if v, ok := args["text"].(string); ok {
		p.Text = v
	}
	if v, ok := args["pattern"].(string); ok {
		p.Pattern = strings.TrimSpace(v)
	}
	if v, ok := args["replacement"].(string); ok {
		p.Replacement = v
	}
	if v, ok := args["separator"].(string); ok && v != "" {
		p.Separator = v
	}
	if v, ok := args["generate_type"].(string); ok {
		p.GenerateType = strings.TrimSpace(v)
	}
	if v, ok := args["length"].(float64); ok {
		p.Length = int(v)
	}

	// 验证必需参数
	if p.Action == "" {
		return "", fmt.Errorf("参数错误: action 不能为空")
	}

	// 根据操作类型执行相应处理
	switch p.Action {
	case "stats":
		return t.processStats(p.Text)
	case "uppercase":
		return t.processUppercase(p.Text)
	case "lowercase":
		return t.processLowercase(p.Text)
	case "title":
		return t.processTitle(p.Text)
	case "trim":
		return t.processTrim(p.Text)
	case "reverse":
		return t.processReverse(p.Text)
	case "extract":
		return t.processExtract(p.Text, p.Pattern)
	case "generate":
		return t.processGenerate(p.GenerateType, p.Length)
	case "replace":
		return t.processReplace(p.Text, p.Pattern, p.Replacement)
	case "split":
		return t.processSplit(p.Text, p.Separator)
	case "join":
		// 对于join操作，需要将text按separator分割后再连接
		parts := strings.Split(p.Text, p.Separator)
		return t.processJoin(parts, p.Separator)
	default:
		return "", fmt.Errorf("不支持的操作类型: %s", p.Action)
	}
}

// processStats 文本统计
func (t *TextProcessorTool) processStats(text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("参数错误: text 不能为空")
	}

	stats := map[string]interface{}{
		"工具":      "textprocessor",
		"操作":      "stats",
		"原文":      text,
		"字符总数":    len(text),
		"UTF8字符数": utf8.RuneCountInString(text),
		"字节数":     len([]byte(text)),
		"行数":      len(strings.Split(text, "\n")),
		"单词数":     len(strings.Fields(text)),
		"中文字符数":   t.countChinese(text),
		"英文字符数":   t.countEnglish(text),
		"数字字符数":   t.countDigits(text),
		"空格数":     strings.Count(text, " "),
		"制表符数":    strings.Count(text, "\t"),
		"换行符数":    strings.Count(text, "\n"),
		"处理时间":    time.Now().Format("2006-01-02 15:04:05"),
	}

	result, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(result), nil
}

// processUppercase 转大写
func (t *TextProcessorTool) processUppercase(text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("参数错误: text 不能为空")
	}

	result := map[string]interface{}{
		"工具":   "textprocessor",
		"操作":   "uppercase",
		"原文":   text,
		"结果":   strings.ToUpper(text),
		"处理时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(output), nil
}

// processLowercase 转小写
func (t *TextProcessorTool) processLowercase(text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("参数错误: text 不能为空")
	}

	result := map[string]interface{}{
		"工具":   "textprocessor",
		"操作":   "lowercase",
		"原文":   text,
		"结果":   strings.ToLower(text),
		"处理时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(output), nil
}

// processTitle 标题格式
func (t *TextProcessorTool) processTitle(text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("参数错误: text 不能为空")
	}

	result := map[string]interface{}{
		"工具":   "textprocessor",
		"操作":   "title",
		"原文":   text,
		"结果":   cases.Title(language.English).String(text),
		"处理时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(output), nil
}

// processTrim 去除首尾空格
func (t *TextProcessorTool) processTrim(text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("参数错误: text 不能为空")
	}

	result := map[string]interface{}{
		"工具":   "textprocessor",
		"操作":   "trim",
		"原文":   text,
		"结果":   strings.TrimSpace(text),
		"处理时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(output), nil
}

// processReverse 反转文本
func (t *TextProcessorTool) processReverse(text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("参数错误: text 不能为空")
	}

	runes := []rune(text)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	result := map[string]interface{}{
		"工具":   "textprocessor",
		"操作":   "reverse",
		"原文":   text,
		"结果":   string(runes),
		"处理时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(output), nil
}

// processExtract 内容提取
func (t *TextProcessorTool) processExtract(text, pattern string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("参数错误: text 不能为空")
	}
	if pattern == "" {
		return "", fmt.Errorf("参数错误: pattern 不能为空")
	}

	var matches []string
	var regex *regexp.Regexp
	var err error

	switch pattern {
	case "email":
		regex, err = regexp.Compile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	case "url":
		regex, err = regexp.Compile(`https?://[^\s]+`)
	case "phone":
		regex, err = regexp.Compile(`1[3-9]\d{9}`)
	case "number":
		regex, err = regexp.Compile(`\d+`)
	case "chinese":
		regex, err = regexp.Compile(`[\p{Han}]+`)
	case "english":
		regex, err = regexp.Compile(`[a-zA-Z]+`)
	default:
		return "", fmt.Errorf("不支持的提取模式: %s", pattern)
	}

	if err != nil {
		return "", fmt.Errorf("正则表达式编译失败: %w", err)
	}

	matches = regex.FindAllString(text, -1)

	result := map[string]interface{}{
		"工具":   "textprocessor",
		"操作":   "extract",
		"模式":   pattern,
		"原文":   text,
		"匹配数量": len(matches),
		"匹配结果": matches,
		"处理时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(output), nil
}

// processGenerate 文本生成
func (t *TextProcessorTool) processGenerate(generateType string, length int) (string, error) {
	if generateType == "" {
		return "", fmt.Errorf("参数错误: generate_type 不能为空")
	}

	var generated string
	var err error

	switch generateType {
	case "uuid":
		generated, err = t.generateUUID()
	case "password":
		generated, err = t.generatePassword(length)
	case "random_string":
		generated, err = t.generateRandomString(length)
	case "lorem":
		generated = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
	default:
		return "", fmt.Errorf("不支持的生成类型: %s", generateType)
	}

	if err != nil {
		return "", fmt.Errorf("生成失败: %w", err)
	}

	result := map[string]interface{}{
		"工具":   "textprocessor",
		"操作":   "generate",
		"类型":   generateType,
		"长度":   len(generated),
		"结果":   generated,
		"处理时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(output), nil
}

// processReplace 文本替换
func (t *TextProcessorTool) processReplace(text, pattern, replacement string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("参数错误: text 不能为空")
	}
	if pattern == "" {
		return "", fmt.Errorf("参数错误: pattern 不能为空")
	}

	var replaced string
	switch pattern {
	case "email", "url", "phone", "number", "chinese", "english":
		// 使用正则表达式替换
		var regex *regexp.Regexp
		var err error
		switch pattern {
		case "email":
			regex, err = regexp.Compile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
		case "url":
			regex, err = regexp.Compile(`https?://[^\s]+`)
		case "phone":
			regex, err = regexp.Compile(`1[3-9]\d{9}`)
		case "number":
			regex, err = regexp.Compile(`\d+`)
		case "chinese":
			regex, err = regexp.Compile(`[\p{Han}]+`)
		case "english":
			regex, err = regexp.Compile(`[a-zA-Z]+`)
		}
		if err != nil {
			return "", fmt.Errorf("正则表达式编译失败: %w", err)
		}
		replaced = regex.ReplaceAllString(text, replacement)
	default:
		// 直接字符串替换
		replaced = strings.ReplaceAll(text, pattern, replacement)
	}

	result := map[string]interface{}{
		"工具":   "textprocessor",
		"操作":   "replace",
		"原文":   text,
		"模式":   pattern,
		"替换为":  replacement,
		"结果":   replaced,
		"处理时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(output), nil
}

// processSplit 文本分割
func (t *TextProcessorTool) processSplit(text, separator string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("参数错误: text 不能为空")
	}

	parts := strings.Split(text, separator)

	result := map[string]interface{}{
		"工具":   "textprocessor",
		"操作":   "split",
		"原文":   text,
		"分隔符":  separator,
		"片段数量": len(parts),
		"结果":   parts,
		"处理时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(output), nil
}

// processJoin 文本连接
func (t *TextProcessorTool) processJoin(parts []string, separator string) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("参数错误: 没有要连接的文本片段")
	}

	joined := strings.Join(parts, separator)

	result := map[string]interface{}{
		"工具":   "textprocessor",
		"操作":   "join",
		"片段":   parts,
		"分隔符":  separator,
		"结果":   joined,
		"处理时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}
	return string(output), nil
}

// 辅助函数
func (t *TextProcessorTool) countChinese(text string) int {
	count := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			count++
		}
	}
	return count
}

func (t *TextProcessorTool) countEnglish(text string) int {
	count := 0
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			count++
		}
	}
	return count
}

func (t *TextProcessorTool) countDigits(text string) int {
	count := 0
	for _, r := range text {
		if unicode.IsDigit(r) {
			count++
		}
	}
	return count
}

func (t *TextProcessorTool) generatePassword(length int) (string, error) {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	return t.generateRandomFromChars(chars, length)
}

func (t *TextProcessorTool) generateRandomString(length int) (string, error) {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	return t.generateRandomFromChars(chars, length)
}

func (t *TextProcessorTool) generateRandomFromChars(chars string, length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("长度必须大于0")
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	result := make([]byte, length)
	for i, b := range bytes {
		result[i] = chars[int(b)%len(chars)]
	}

	return string(result), nil
}

func (t *TextProcessorTool) generateUUID() (string, error) {
	// 生成16字节的随机数据
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// 设置版本号 (4) 和变体位
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // Version 4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // Variant bits

	// 格式化为标准UUID字符串格式
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}
