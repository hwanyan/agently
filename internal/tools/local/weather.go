// 通用智能助手 agent 的工具：天气查询工具（本地工具，随项目启动）
package local

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/pkg/types"
	"github.com/hwanyan/agently/pkg/utils"
)

// 工具自注册
func init() {
	Register("general", func(provider types.ConfigProvider) types.Tool {
		return NewWeatherTool(provider)
	})
}

// 天气工具配置常量（定制化内容）
const (
	// WeatherToolName 默认天气工具名称
	WeatherToolName = "weather"

	// WeatherToolDesc 默认天气工具描述
	WeatherToolDesc = "查询指定城市的天气信息，包括实时温度、湿度、风向、风力、空气质量，以及未来几天的天气预报。支持中文城市名称，如：北京、上海、广州等。"
)

// WeatherTool 天气查询工具
// 基于聚合数据（JuHe）simpleWeather API 实现
type WeatherTool struct {
	name        string                 // 工具名称
	description string                 // 工具描述
	provider    types.ConfigProvider   // 配置提供者：每次 Execute 时读取最新的 API Key / BaseURL（支持热更新）
	client      *http.Client           // HTTP客户端
}

// JuheWeatherResponse 聚合数据天气查询响应
type JuheWeatherResponse struct {
	ErrorCode int    `json:"error_code"` // 返回码，0为查询成功
	Reason    string `json:"reason"`     // 返回说明
	Result    struct {
		City     string `json:"city"` // 城市名称
		Realtime struct {
			Info        string `json:"info"`        // 天气情况，如：晴、多云
			Wid         string `json:"wid"`         // 天气标识id
			Temperature string `json:"temperature"` // 温度
			Humidity    string `json:"humidity"`    // 湿度
			Direct      string `json:"direct"`      // 风向
			Power       string `json:"power"`       // 风力
			Aqi         string `json:"aqi"`         // 空气质量指数
		} `json:"realtime"` // 天气实况
		Future []struct {
			Date        string `json:"date"`        // 日期
			Temperature string `json:"temperature"` // 温度，最低温/最高温
			Weather     string `json:"weather"`     // 天气情况
			Direct      string `json:"direct"`      // 风向
		} `json:"future"` // 近5天天气情况
	} `json:"result"` // 返回结果集
}

// NewWeatherTool 创建一个新的天气查询工具实例
// 参数：
//   - provider: 配置提供者，用于在每次调用时读取最新的 API Key / BaseURL（支持热更新）
//
// 返回：
//   - *WeatherTool: 天气工具实例
func NewWeatherTool(provider types.ConfigProvider) *WeatherTool {
	return &WeatherTool{
		name:        WeatherToolName,
		description: WeatherToolDesc,
		provider:    provider,
		client: &http.Client{
			Timeout: 15 * time.Second, // 设置15秒超时
		},
	}
}

// Name 返回工具名称
// 实现types.Tool接口
func (w *WeatherTool) Name() string {
	return w.name
}

// Description 返回工具描述
// 实现types.Tool接口
func (w *WeatherTool) Description() string {
	return w.description
}

// Parameters 返回工具的参数定义
// 实现types.Tool接口
func (w *WeatherTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"city": map[string]interface{}{
				"type":        "string",
				"description": "要查询天气的城市名称，支持中文城市名称，如：北京、上海、广州等。",
			},
		},
		"required": []string{"city"},
	}
}

// Execute 执行天气查询
// 参数：
//   - ctx: 上下文
//   - args: 工具参数，包含city字段
//
// 返回：
//   - string: 天气信息的JSON字符串
//   - error: 可能的错误
//
// 实现types.Tool接口
func (w *WeatherTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 每次调用都读取最新配置，天然支持 config.yaml 热更新
	cfg := w.provider()
	apiKey := cfg.Tools.Weather.APIKey
	baseURL := cfg.Tools.Weather.BaseURL

	// 如果没有API密钥，返回错误
	if apiKey == "" {
		return "", fmt.Errorf("API 密钥为空！")
	}

	// 提取城市名称
	city, ok := args["city"].(string)
	if !ok || city == "" {
		return "", fmt.Errorf("city参数必须是非空字符串")
	}

	// 查询天气
	weatherData, err := w.getWeather(ctx, city, apiKey, baseURL)
	if err != nil {
		return "", fmt.Errorf("获取天气数据失败: %w", err)
	}

	// 格式化天气数据
	return w.formatWeatherData(weatherData)
}

// getWeather 查询指定城市天气信息
func (w *WeatherTool) getWeather(ctx context.Context, city, apiKey, baseURL string) (*JuheWeatherResponse, error) {
	// 构建查询参数（city 需要 utf8 urlencode，url.Values 会自动处理）
	params := url.Values{}
	params.Add("city", city)
	params.Add("key", apiKey)

	// 拼接完整请求URL
	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		logger.Error("创建HTTP请求失败: %v", err)
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 发送请求
	resp, err := w.client.Do(req)
	if err != nil {
		logger.Error("发送HTTP请求失败: %v", err)
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		logger.Error("API请求失败，状态码: %d", resp.StatusCode)
		return nil, fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	// 读取并解压缩响应体
	body, err := utils.ReadResponseBody(resp)
	if err != nil {
		logger.Error("读取响应体失败: %v", err)
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 解析响应
	var weatherResp JuheWeatherResponse
	if err := json.Unmarshal(body, &weatherResp); err != nil {
		logger.Error("解析天气响应失败: %v", err)
		return nil, fmt.Errorf("解析天气响应失败: %w", err)
	}

	// 检查API返回码（0 为查询成功）
	if weatherResp.ErrorCode != 0 {
		logger.Error("聚合数据API返回错误，错误码: %d，原因: %s", weatherResp.ErrorCode, weatherResp.Reason)
		return nil, fmt.Errorf("聚合数据API返回错误，错误码: %d，原因: %s", weatherResp.ErrorCode, weatherResp.Reason)
	}

	return &weatherResp, nil
}

// formatWeatherData 格式化天气数据为用户友好的格式
func (w *WeatherTool) formatWeatherData(weatherData *JuheWeatherResponse) (string, error) {
	realtime := weatherData.Result.Realtime

	// 构建实时天气信息
	result := map[string]interface{}{
		"城市":   weatherData.Result.City,
		"天气状况": realtime.Info,
		"温度":   fmt.Sprintf("%s°C", realtime.Temperature),
		"相对湿度": fmt.Sprintf("%s%%", realtime.Humidity),
		"风向":   realtime.Direct,
		"风力":   realtime.Power,
		"空气质量": realtime.Aqi,
		"数据来源": "聚合数据",
	}

	// 构建未来几天天气预报
	future := make([]map[string]string, 0, len(weatherData.Result.Future))
	for _, f := range weatherData.Result.Future {
		future = append(future, map[string]string{
			"日期": f.Date,
			"温度": f.Temperature,
			"天气": f.Weather,
			"风向": f.Direct,
		})
	}
	result["未来天气"] = future

	// 转换为JSON字符串
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}

	return string(resultJSON), nil
}

// ValidateConfig 验证天气工具配置（校验当前生效的最新配置）
func (w *WeatherTool) ValidateConfig() error {
	cfg := w.provider()
	apiKey := cfg.Tools.Weather.APIKey
	baseURL := cfg.Tools.Weather.BaseURL

	if apiKey == "" {
		return fmt.Errorf("聚合数据API密钥不能为空")
	}

	if baseURL == "" {
		return fmt.Errorf("聚合数据API接口地址不能为空")
	}

	// 检查URL格式
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return fmt.Errorf("聚合数据API接口地址格式不正确")
	}

	return nil
}
