package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// 定义天气工具的默认名称和描述
const (
	whrDefaultName = "weather"
	whrDefaultDesc = "查询指定城市的当前天气信息，包括温度、湿度、天气状况等"
)

// WeatherTool 天气查询工具
// 这个工具演示如何调用外部API获取数据
type WeatherTool struct {
	name        string       // 工具名称
	description string       // 工具描述
	apiKey      string       // API密钥
	client      *http.Client // HTTP客户端
}

// NewWeatherTool 创建一个新的天气查询工具实例
// 参数：
//   - apiKey: 天气API的密钥（可以使用免费的OpenWeatherMap API）
//
// 返回：
//   - *WeatherTool: 天气工具实例
func NewWeatherTool(name, desc, apiKey string) *WeatherTool {
	// 如果 name 为空字符串，则使用默认名称
	if name == "" {
		name = whrDefaultName
	}
	// 如果 desc 为空字符串，则使用默认描述
	if desc == "" {
		desc = whrDefaultDesc
	}
	return &WeatherTool{
		name:        name,
		description: desc,
		apiKey:      apiKey,
		client: &http.Client{
			Timeout: 10 * time.Second, // 设置10秒超时
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
				"description": "要查询天气的城市名称，例如：北京、上海、New York等",
			},
			"country": map[string]interface{}{
				"type":        "string",
				"description": "国家代码（可选），例如：CN、US等",
			},
		},
		"required": []string{"city"},
	}
}

// Execute 执行天气查询
// 参数：
//   - ctx: 上下文
//   - args: 工具参数，包含city和可选的country字段
//
// 返回：
//   - string: 天气信息的JSON字符串
//   - error: 可能的错误
//
// 实现types.Tool接口
func (w *WeatherTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 提取城市名称
	city, ok := args["city"].(string)
	if !ok || city == "" {
		return "", fmt.Errorf("city参数必须是非空字符串")
	}

	// 提取国家代码（可选）
	country, _ := args["country"].(string)

	// 如果没有API密钥，返回模拟数据
	if w.apiKey == "" {
		return w.getMockWeatherData(city), nil
	}

	// 构建查询参数
	query := city
	if country != "" {
		query = fmt.Sprintf("%s,%s", city, country)
	}

	// 调用天气API
	weatherData, err := w.fetchWeatherData(ctx, query)
	if err != nil {
		return "", fmt.Errorf("获取天气数据失败: %w", err)
	}

	return weatherData, nil
}

// fetchWeatherData 从OpenWeatherMap API获取天气数据
func (w *WeatherTool) fetchWeatherData(ctx context.Context, query string) (string, error) {
	// 构建API URL
	baseURL := "http://api.openweathermap.org/data/2.5/weather"
	params := url.Values{}
	params.Add("q", query)
	params.Add("appid", w.apiKey)
	params.Add("units", "metric") // 使用摄氏度
	params.Add("lang", "zh_cn")   // 中文描述

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 发送请求
	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应体失败: %w", err)
	}

	// 解析并格式化天气数据
	return w.formatWeatherData(body)
}

// formatWeatherData 格式化天气数据为用户友好的格式
func (w *WeatherTool) formatWeatherData(data []byte) (string, error) {
	var weatherResp struct {
		Name string `json:"name"`
		Main struct {
			Temp      float64 `json:"temp"`
			FeelsLike float64 `json:"feels_like"`
			Humidity  int     `json:"humidity"`
			Pressure  int     `json:"pressure"`
		} `json:"main"`
		Weather []struct {
			Main        string `json:"main"`
			Description string `json:"description"`
		} `json:"weather"`
		Wind struct {
			Speed float64 `json:"speed"`
		} `json:"wind"`
		Sys struct {
			Country string `json:"country"`
		} `json:"sys"`
	}

	if err := json.Unmarshal(data, &weatherResp); err != nil {
		return "", fmt.Errorf("解析天气数据失败: %w", err)
	}

	// 构建格式化的天气信息
	result := map[string]interface{}{
		"城市":   weatherResp.Name,
		"国家":   weatherResp.Sys.Country,
		"温度":   fmt.Sprintf("%.1f°C", weatherResp.Main.Temp),
		"体感温度": fmt.Sprintf("%.1f°C", weatherResp.Main.FeelsLike),
		"湿度":   fmt.Sprintf("%d%%", weatherResp.Main.Humidity),
		"气压":   fmt.Sprintf("%d hPa", weatherResp.Main.Pressure),
		"风速":   fmt.Sprintf("%.1f m/s", weatherResp.Wind.Speed),
	}

	if len(weatherResp.Weather) > 0 {
		result["天气状况"] = weatherResp.Weather[0].Description
	}

	// 转换为JSON字符串
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("格式化结果失败: %w", err)
	}

	return string(resultJSON), nil
}

// getMockWeatherData 返回模拟的天气数据
// 当没有API密钥时使用，用于演示目的
func (w *WeatherTool) getMockWeatherData(city string) string {
	mockData := map[string]interface{}{
		"城市":   city,
		"国家":   "CN",
		"温度":   "22.5°C",
		"体感温度": "24.0°C",
		"湿度":   "65%",
		"气压":   "1013 hPa",
		"风速":   "3.2 m/s",
		"天气状况": "多云",
		"说明":   "这是模拟数据，请配置真实的API密钥以获取实际天气信息",
	}

	resultJSON, _ := json.MarshalIndent(mockData, "", "  ")
	return string(resultJSON)
}
