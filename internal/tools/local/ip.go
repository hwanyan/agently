// 通用智能助手 agent 的工具：IP地址查询工具（本地工具，随项目启动）
package local

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/pkg/types"
	"github.com/hwanyan/agently/pkg/utils"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// 工具自注册
func init() {
	Register("general", func(provider types.ConfigProvider) types.Tool {
		return NewIPTool(provider)
	})
}

// IP地址查询工具默认配置常量
const (
	// IPToolName 默认IP工具名称（工具名称需保持英文）
	IPToolName = "ipapi"

	// IPToolDesc 默认IP工具描述
	IPToolDesc = "查询与IP地址相关的各种信息，包括 IP 地址和未知数据等常规信息，升级套餐后可获取时区数据、货币数据、连接数据和安全数据。"

	// IPAPIBaseURL 默认IP查询API基础URL
	IPAPIBaseURL = "http://api.ipapi.com/api/"
)

// ipapi IP信息查询工具
type IPTool struct {
	name        string               // 工具名称
	description string               // 工具描述
	provider    types.ConfigProvider // 配置提供者：每次 Execute 时读取最新的 access key（支持热更新）
	baseURL     string               // API基础URL
	client      *http.Client         // HTTP客户端
}

// IPQueryResponse IP查询响应结构 (api.ipapi.com)
type IPQueryResponse struct {
	IP            string  `json:"ip"`             // IP地址
	Type          string  `json:"type"`           // IP类型 (ipv4/ipv6)
	ContinentCode string  `json:"continent_code"` // 洲代码
	ContinentName string  `json:"continent_name"` // 洲名称
	CountryCode   string  `json:"country_code"`   // 国家代码
	CountryName   string  `json:"country_name"`   // 国家名称
	RegionCode    string  `json:"region_code"`    // 地区代码
	RegionName    string  `json:"region_name"`    // 地区名称
	City          string  `json:"city"`           // 城市
	Zip           string  `json:"zip"`            // 邮编
	Latitude      float64 `json:"latitude"`       // 纬度
	Longitude     float64 `json:"longitude"`      // 经度
	Location      *struct {
		GeonameID               int    `json:"geoname_id"`
		Capital                 string `json:"capital"`
		CountryFlag             string `json:"country_flag"`
		CountryFlagEmoji        string `json:"country_flag_emoji"`
		CountryFlagEmojiUnicode string `json:"country_flag_emoji_unicode"`
		CallingCode             string `json:"calling_code"`
		IsEU                    bool   `json:"is_eu"`
		Languages               []struct {
			Code   string `json:"code"`
			Name   string `json:"name"`
			Native string `json:"native"`
		} `json:"languages"`
	} `json:"location,omitempty"`
	TimeZone *struct {
		ID               string `json:"id"`
		CurrentTime      string `json:"current_time"`
		GMTOffset        int    `json:"gmt_offset"`
		Code             string `json:"code"`
		IsDaylightSaving bool   `json:"is_daylight_saving"`
	} `json:"time_zone,omitempty"`
	Currency *struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Plural string `json:"plural"`
		Symbol string `json:"symbol"`
	} `json:"currency,omitempty"`
	Connection *struct {
		ASN int    `json:"asn"`
		ISP string `json:"isp"`
	} `json:"connection,omitempty"`
	Security *struct {
		IsProxy     bool   `json:"is_proxy"`
		ProxyType   string `json:"proxy_type"`
		IsCrawler   bool   `json:"is_crawler"`
		CrawlerName string `json:"crawler_name"`
		IsTor       bool   `json:"is_tor"`
	} `json:"security,omitempty"`
	Success bool `json:"success"` // 查询是否成功
	Error   *struct {
		Code int    `json:"code"`
		Type string `json:"type"`
		Info string `json:"info"`
	} `json:"error,omitempty"` // 错误信息
}

// NewIPTool 创建一个新的IP查询工具实例
//
// 参数：
//   - provider: 配置提供者，用于在每次调用时读取最新的 access key（支持热更新）
//
// 返回：
//   - *IPTool: IP查询工具实例
func NewIPTool(provider types.ConfigProvider) *IPTool {
	return &IPTool{
		name:        IPToolName,
		description: IPToolDesc,
		provider:    provider,
		baseURL:     IPAPIBaseURL,
		client: &http.Client{
			Timeout: 10 * time.Second, // 设置10秒超时
		},
	}
}

// Name 返回工具名称
// 实现types.Tool接口
func (i *IPTool) Name() string {
	return i.name
}

// Description 返回工具描述
// 实现types.Tool接口
func (i *IPTool) Description() string {
	return i.description
}

// Parameters 返回工具的参数定义
// 实现types.Tool接口
func (i *IPTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"target": map[string]interface{}{
				"type":        "string",
				"description": "查询与IP地址相关的各种信息，包括常规信息、位置数据、时区数据、货币数据、连接数据和安全数据。",
			},
		},
		"required": []string{"target"},
	}
}

// Execute 执行IP地址查询
// 参数：
//   - ctx: 上下文
//   - args: 工具参数，包含target字段（可选）
//
// 返回：
//   - string: IP信息的JSON字符串
//   - error: 可能的错误
//
// 实现types.Tool接口
func (i *IPTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 每次调用都读取最新配置，支持 config.yaml 热更新
	accessKey := i.provider().Tools.IP.AccessKey

	// 提取目标IP或域名
	var target string
	if targetVal, ok := args["target"]; ok {
		if targetStr, ok := targetVal.(string); ok {
			target = strings.TrimSpace(targetStr)
		}
	}

	// 如果没有提供目标，查询当前公网IP
	if target == "" {
		return i.queryCurrentIP(ctx, accessKey)
	}

	// 验证输入格式
	if err := i.validateTarget(target); err != nil {
		return "", fmt.Errorf("输入验证失败: %w", err)
	}

	// 查询指定IP或域名
	return i.queryIP(ctx, target, accessKey)
}

// validateTarget 验证输入的IP地址或域名格式
func (i *IPTool) validateTarget(target string) error {
	// 检查是否是有效的IP地址
	if net.ParseIP(target) != nil {
		return nil // 有效的IP地址
	}

	// 检查是否是有效的域名
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	if domainRegex.MatchString(target) {
		return nil // 有效的域名
	}

	return fmt.Errorf("无效的IP地址或域名格式: %s", target)
}

// queryCurrentIP 查询当前公网IP信息
func (i *IPTool) queryCurrentIP(ctx context.Context, accessKey string) (string, error) {
	// 构建查询URL（查询当前公网IP）
	url := fmt.Sprintf("%scheck?access_key=%s", i.baseURL, accessKey)

	return i.performQuery(ctx, url, "当前公网IP")
}

// queryIP 查询指定IP或域名的信息
func (i *IPTool) queryIP(ctx context.Context, target, accessKey string) (string, error) {
	// 构建查询URL
	url := fmt.Sprintf("%s%s?access_key=%s", i.baseURL, target, accessKey)

	return i.performQuery(ctx, url, target)
}

// performQuery 执行查询
func (i *IPTool) performQuery(ctx context.Context, url, target string) (string, error) {
	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; AI-Agent/1.0)")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	// 发送请求
	resp, err := i.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		// 读取错误响应体以获取更多信息
		errorBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API请求失败，状态码: %d, URL: %s, 响应: %s", resp.StatusCode, url, string(errorBody))
	}

	// 读取并解压缩响应体
	body, err := utils.ReadResponseBody(resp)
	if err != nil {
		logger.Error("读取响应体失败: %v", err)
		return "", fmt.Errorf("读取响应体失败: %w", err)
	}

	// 解析响应
	var ipResp IPQueryResponse
	if err := json.Unmarshal(body, &ipResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查查询状态（注：使用免费版时，Success 字段的值也为 False）
	if !ipResp.Success {
		if ipResp.Error != nil {
			return "", fmt.Errorf("IP查询失败: %s", ipResp.Error.Info)
		}
	}

	// 格式化结果
	return i.formatIPInfo(&ipResp, target)
}

// formatIPInfo 格式化IP信息为用户友好的格式
func (i *IPTool) formatIPInfo(ipResp *IPQueryResponse, originalTarget string) (string, error) {
	// 构建格式化的IP信息
	result := map[string]interface{}{
		"查询目标": originalTarget,
		"IP地址": ipResp.IP,
		"IP类型": ipResp.Type,
		"国家":   ipResp.CountryName,
		"国家代码": ipResp.CountryCode,
		"洲":    ipResp.ContinentName,
		"地区":   ipResp.RegionName,
		"城市":   ipResp.City,
		"邮编":   ipResp.Zip,
		"坐标":   fmt.Sprintf("纬度: %.4f, 经度: %.4f", ipResp.Latitude, ipResp.Longitude),
		"数据来源": "api.ipapi.com",
		"查询时间": time.Now().Format("2006-01-02 15:04:05"),
	}

	// 添加时区信息
	if ipResp.TimeZone != nil {
		result["时区"] = ipResp.TimeZone.ID
		result["当前时间"] = ipResp.TimeZone.CurrentTime
		result["GMT偏移"] = ipResp.TimeZone.GMTOffset
	}

	// 添加连接信息
	if ipResp.Connection != nil {
		result["运营商"] = ipResp.Connection.ISP
		result["ASN"] = ipResp.Connection.ASN
	}

	// 添加货币信息
	if ipResp.Currency != nil {
		result["货币"] = fmt.Sprintf("%s (%s)", ipResp.Currency.Name, ipResp.Currency.Code)
	}

	// 添加安全信息
	if ipResp.Security != nil {
		securityInfo := []string{}
		if ipResp.Security.IsProxy {
			securityInfo = append(securityInfo, "代理")
		}
		if ipResp.Security.IsCrawler {
			securityInfo = append(securityInfo, "爬虫")
		}
		if ipResp.Security.IsTor {
			securityInfo = append(securityInfo, "Tor")
		}
		if len(securityInfo) > 0 {
			result["安全标记"] = strings.Join(securityInfo, ", ")
		}
	}

	// 清理空字段
	cleanedResult := make(map[string]interface{})
	for key, value := range result {
		if strVal, ok := value.(string); ok && strVal != "" {
			cleanedResult[key] = value
		} else if !ok { // 非字符串类型直接保留
			cleanedResult[key] = value
		}
	}

	// 转换为JSON字符串
	resultJSON, err := json.MarshalIndent(cleanedResult, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}

	return string(resultJSON), nil
}

// ValidateConfig 验证IP工具配置
func (i *IPTool) ValidateConfig() error {
	if i.name == "" {
		return fmt.Errorf("IP工具名称不能为空")
	}

	if i.description == "" {
		return fmt.Errorf("IP工具描述不能为空")
	}

	return nil
}
