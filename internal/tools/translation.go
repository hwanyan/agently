package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// 翻译工具常量
const (
	// TranslationToolName 工具名称
	TranslationToolName = "translate"

	// TranslationToolDesc 工具描述
	TranslationToolDesc = "翻译工具：将给定文本翻译为指定目标语言，支持可选源语言与格式（plain/html）。基于 LibreTranslate API。"
)

// TranslationTool 翻译工具
type TranslationTool struct {
	name        string       // 工具名称
	description string       // 工具描述
	apiKey      string       // 翻译工具的API密钥
	baseURL     string       // 翻译工具的基础URL
	client      *http.Client // HTTP客户端
}

// NewTranslationTool 创建翻译工具实例
// 配置来源：
//   - 环境变量 TRANSLATE_BASE_URL（可选）
//   - 环境变量 TRANSLATE_API_KEY（可选）
//
// 若未配置，使用默认公共实例（可能有速率限制）
func NewTranslationTool(apiKey, baseURL string) *TranslationTool {
	return &TranslationTool{
		name:        TranslationToolName,
		description: TranslationToolDesc,
		apiKey:      apiKey,
		baseURL:     baseURL,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Name 返回工具名称
func (t *TranslationTool) Name() string { return TranslationToolName }

// Description 返回工具描述
func (t *TranslationTool) Description() string { return TranslationToolDesc }

// Parameters 返回参数定义（JSON Schema）
func (t *TranslationTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "需要翻译的原文文本",
			},
			"target": map[string]interface{}{
				"type":        "string",
				"description": "目标语言代码（例如 zh, en, ja, ko, fr, de, es 等）",
			},
			"source": map[string]interface{}{
				"type":        "string",
				"description": "源语言代码（可选，不传则自动检测）",
			},
			"format": map[string]interface{}{
				"type":        "string",
				"description": "文本格式：plain 或 html，默认 plain",
				"enum":        []string{"plain", "html"},
				"default":     "plain",
			},
		},
		"required": []string{"text", "target"},
	}
}

// Execute 执行翻译
func (t *TranslationTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 解析参数
	var p struct {
		Text   string `json:"text"`
		Target string `json:"target"`
		Source string `json:"source"`
		Format string `json:"format"`
	}
	if v, ok := args["text"].(string); ok {
		p.Text = strings.TrimSpace(v)
	}
	if v, ok := args["target"].(string); ok {
		p.Target = strings.TrimSpace(v)
	}
	if v, ok := args["source"].(string); ok {
		p.Source = strings.TrimSpace(v)
	}
	if v, ok := args["format"].(string); ok && v != "" {
		p.Format = v
	} else {
		p.Format = "plain"
	}

	if p.Text == "" {
		return "", fmt.Errorf("参数错误: text 不能为空")
	}
	if p.Target == "" {
		return "", fmt.Errorf("参数错误: target 不能为空")
	}
	if p.Format != "plain" && p.Format != "html" {
		return "", fmt.Errorf("参数错误: format 仅支持 plain 或 html")
	}

	// 构造请求体
	payload := map[string]interface{}{
		"q":      p.Text,
		"target": p.Target,
		"format": p.Format,
	}
	if p.Source != "" {
		payload["source"] = p.Source
	} else {
		payload["source"] = "auto"
	}
	if t.apiKey != "" {
		payload["api_key"] = t.apiKey
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL+"/translate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Agent-Hwan-TranslateTool/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		return "", fmt.Errorf("翻译API请求失败: HTTP %d, 响应: %v", resp.StatusCode, apiErr)
	}

	var result struct {
		TranslatedText string `json:"translatedText"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	out := map[string]interface{}{
		"工具":   "translate",
		"来源语言": payload["source"],
		"目标语言": p.Target,
		"原文":   p.Text,
		"译文":   result.TranslatedText,
		"服务":   t.baseURL,
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化输出失败: %w", err)
	}
	return string(b), nil
}
