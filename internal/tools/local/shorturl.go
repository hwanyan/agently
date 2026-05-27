// 通用智能助手 agent 的工具：短链接生成工具（本地工具，随项目启动）
package local

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hwanyan/agently/pkg/types"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 工具自注册
func init() {
	Register("general", func(cfg *types.Config) types.Tool {
		return NewShortURLTool(cfg.Tools.ShortURL.APIKey, cfg.Tools.ShortURL.BaseURL)
	})
}

// 短链接工具常量
const (
	// ShortURLToolName 工具名称
	ShortURLToolName = "shorturl"

	// ShortURLToolDesc 工具描述
	ShortURLToolDesc = "短链接生成工具：将长URL转换为短链接，支持多种短链接服务提供商。"
)

// ShortURLTool 短链接生成工具
type ShortURLTool struct {
	name        string // 工具名称
	description string // 工具描述
	apiKey      string // API密钥（可选）
	baseURL     string // 基础URL（可选）
}

// NewShortURLTool 创建短链接工具实例
func NewShortURLTool(apiKey, baseURL string) *ShortURLTool {
	return &ShortURLTool{
		name:        ShortURLToolName,
		description: ShortURLToolDesc,
		apiKey:      apiKey,
		baseURL:     baseURL,
	}
}

// Name 返回工具名称
func (s *ShortURLTool) Name() string { return s.name }

// Description 返回工具描述
func (s *ShortURLTool) Description() string { return s.description }

// Parameters 返回参数定义（JSON Schema）
func (s *ShortURLTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "需要缩短的长URL地址（必须是有效的HTTP/HTTPS URL）",
			},
			"service": map[string]interface{}{
				"type":        "string",
				"description": "短链接服务提供商",
				"enum":        []string{"tinyurl", "isgd", "vgd"},
				"default":     "tinyurl",
			},
			"custom_alias": map[string]interface{}{
				"type":        "string",
				"description": "自定义短链接别名（部分服务支持）",
			},
		},
		"required": []string{"url"},
	}
}

// Execute 执行短链接生成
func (s *ShortURLTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 提取参数
	longURL, ok := args["url"].(string)
	if !ok || longURL == "" {
		return "", fmt.Errorf("缺少必需参数：url")
	}

	// 验证URL格式
	if !isValidURL(longURL) {
		return "", fmt.Errorf("无效的URL格式：%s", longURL)
	}

	// 获取服务提供商
	service := "tinyurl"
	if svc, ok := args["service"].(string); ok && svc != "" {
		service = svc
	}

	// 获取自定义别名
	customAlias := ""
	if alias, ok := args["custom_alias"].(string); ok {
		customAlias = alias
	}

	// 根据服务提供商生成短链接
	var shortURL string
	var err error

	switch service {
	case "tinyurl":
		shortURL, err = s.createTinyURL(ctx, longURL)
	case "isgd":
		shortURL, err = s.createIsGdURL(ctx, longURL, customAlias)
	case "vgd":
		shortURL, err = s.createVGdURL(ctx, longURL, customAlias)
	default:
		return "", fmt.Errorf("不支持的短链接服务：%s", service)
	}

	if err != nil {
		return "", fmt.Errorf("生成短链接失败：%v", err)
	}

	// 构建返回结果
	result := map[string]interface{}{
		"success":   true,
		"service":   service,
		"long_url":  longURL,
		"short_url": shortURL,
		"message":   fmt.Sprintf("成功生成短链接（服务：%s）", service),
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败：%v", err)
	}

	return string(resultJSON), nil
}

// createTinyURL 使用 TinyURL 服务生成短链接
func (s *ShortURLTool) createTinyURL(ctx context.Context, longURL string) (string, error) {
	// 构建请求URL
	apiURL := fmt.Sprintf("https://tinyurl.com/api-create.php?url=%s", url.QueryEscape(longURL))

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%v", err)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败：%v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误状态码：%d", resp.StatusCode)
	}

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败：%v", err)
	}

	shortURL := strings.TrimSpace(string(body))
	if shortURL == "" {
		return "", fmt.Errorf("API返回空结果")
	}

	return shortURL, nil
}

// createIsGdURL 使用 is.gd 服务生成短链接
func (s *ShortURLTool) createIsGdURL(ctx context.Context, longURL, customAlias string) (string, error) {
	// 构建请求URL
	apiURL := fmt.Sprintf("https://is.gd/create.php?format=simple&url=%s", url.QueryEscape(longURL))
	if customAlias != "" {
		apiURL += fmt.Sprintf("&shorturl=%s", url.QueryEscape(customAlias))
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%v", err)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败：%v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败：%v", err)
	}

	shortURL := strings.TrimSpace(string(body))
	if shortURL == "" {
		return "", fmt.Errorf("API返回空结果")
	}

	// 检查是否返回错误信息
	if strings.HasPrefix(shortURL, "Error:") {
		return "", fmt.Errorf("API错误：%s", shortURL)
	}

	return shortURL, nil
}

// createVGdURL 使用 v.gd 服务生成短链接（is.gd的替代服务）
func (s *ShortURLTool) createVGdURL(ctx context.Context, longURL, customAlias string) (string, error) {
	// 构建请求URL
	apiURL := fmt.Sprintf("https://v.gd/create.php?format=simple&url=%s", url.QueryEscape(longURL))
	if customAlias != "" {
		apiURL += fmt.Sprintf("&shorturl=%s", url.QueryEscape(customAlias))
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%v", err)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败：%v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败：%v", err)
	}

	shortURL := strings.TrimSpace(string(body))
	if shortURL == "" {
		return "", fmt.Errorf("API返回空结果")
	}

	// 检查是否返回错误信息
	if strings.HasPrefix(shortURL, "Error:") {
		return "", fmt.Errorf("API错误：%s", shortURL)
	}

	return shortURL, nil
}

// isValidURL 验证URL格式
func isValidURL(str string) bool {
	u, err := url.Parse(str)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
