package tools

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// readResponseBody 读取并解压缩响应体
func readResponseBody(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body

	// 检查是否是gzip压缩
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("创建gzip读取器失败: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	return io.ReadAll(reader)
}

// WeatherTool 天气查询工具
// 基于和风天气API实现
type WeatherTool struct {
	name        string       // 工具名称
	description string       // 工具描述
	apiKey      string       // 和风天气API密钥
	baseURL     string       // 和风天气API基础URL
	client      *http.Client // HTTP客户端
}

// QWeatherLocationResponse 和风天气地理位置查询响应
type QWeatherLocationResponse struct {
	Code     string `json:"code"`
	Location []struct {
		Name      string `json:"name"`      // 地点名称
		ID        string `json:"id"`        // 地点ID
		Lat       string `json:"lat"`       // 纬度
		Lon       string `json:"lon"`       // 经度
		Adm2      string `json:"adm2"`      // 地级行政区
		Adm1      string `json:"adm1"`      // 省级行政区
		Country   string `json:"country"`   // 国家
		Tz        string `json:"tz"`        // 时区
		UtcOffset string `json:"utcOffset"` // UTC偏移
		IsDst     string `json:"isDst"`     // 是否夏令时
		Type      string `json:"type"`      // 地点类型
		Rank      string `json:"rank"`      // 地点等级
		FxLink    string `json:"fxLink"`    // 该地点的天气预报网页链接
	} `json:"location"`
}

// QWeatherCurrentResponse 和风天气实时天气响应
type QWeatherCurrentResponse struct {
	Code       string `json:"code"`
	UpdateTime string `json:"updateTime"`
	FxLink     string `json:"fxLink"`
	Now        struct {
		ObsTime   string `json:"obsTime"`   // 数据观测时间
		Temp      string `json:"temp"`      // 温度，摄氏度
		FeelsLike string `json:"feelsLike"` // 体感温度，摄氏度
		Icon      string `json:"icon"`      // 天气状况图标代码
		Text      string `json:"text"`      // 天气状况文字描述
		Wind360   string `json:"wind360"`   // 风向360度
		WindDir   string `json:"windDir"`   // 风向
		WindScale string `json:"windScale"` // 风力等级
		WindSpeed string `json:"windSpeed"` // 风速，km/h
		Humidity  string `json:"humidity"`  // 相对湿度，百分比
		Precip    string `json:"precip"`    // 当前小时累计降水量，mm
		Pressure  string `json:"pressure"`  // 大气压强，hPa
		Vis       string `json:"vis"`       // 能见度，km
		Cloud     string `json:"cloud"`     // 云量，百分比
		Dew       string `json:"dew"`       // 露点温度
	} `json:"now"`
}

// NewWeatherTool 创建一个新的天气查询工具实例
// 参数：
//   - name: 工具名称（可选，默认为weather）
//   - desc: 工具描述（可选，使用默认描述）
//   - apiKey: 和风天气API密钥（必须，从配置文件或环境变量读取）
//   - baseURL: 和风天气API基础URL（必须，从配置文件或环境变量读取）
//
// 返回：
//   - *WeatherTool: 天气工具实例
func NewWeatherTool(name, desc, apiKey, baseURL string) *WeatherTool {
	return &WeatherTool{
		name:        name,
		description: desc,
		apiKey:      apiKey,
		baseURL:     baseURL,
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
				"description": "要查询天气的城市名称，支持中文城市名，如：北京、上海、广州、深圳等，也支持英文城市名",
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
	// fmt.Println("    🌤️ 开始查询天气")

	// 如果没有API密钥，返回模拟数据
	if w.apiKey == "" {
		return "", fmt.Errorf("API 密钥为空！")
	}

	// 提取城市名称
	city, ok := args["city"].(string)
	if !ok || city == "" {
		return "", fmt.Errorf("city参数必须是非空字符串")
	}
	// fmt.Printf("    🌤️ 提取到的城市名称为：%s\n", city)

	// 1. 首先查询城市的地理位置信息
	locationID, locationInfo, err := w.getCityLocation(ctx, city)
	if err != nil {
		return "", fmt.Errorf("查询城市位置失败: %w", err)
	}
	// fmt.Printf("    🌍 获取到的城市位置信息: %+v\n", locationInfo)

	// 2. 使用地理位置ID查询实时天气
	weatherData, err := w.getCurrentWeather(ctx, locationID)
	if err != nil {
		return "", fmt.Errorf("获取天气数据失败: %w", err)
	}
	// fmt.Printf("	🌤️ 获取到的天气数据: %+v\n", weatherData)

	// 3. 格式化天气数据
	return w.formatWeatherData(weatherData, locationInfo)
}

// getCityLocation 查询城市地理位置信息
func (w *WeatherTool) getCityLocation(ctx context.Context, city string) (string, map[string]string, error) {
	// fmt.Printf("    🌍 开始查询城市位置信息: %s\n", city)

	// 构建地理位置查询URL
	params := url.Values{}
	params.Add("location", city)

	// 使用配置的baseURL
	fullURL := fmt.Sprintf("%s/geo/v2/city/lookup?%s", w.baseURL, params.Encode())
	// fmt.Printf("    🌍 请求URL: %s\n", fullURL)

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		fmt.Printf("    ❌ 创建HTTP请求失败: %v\n", err)
		return "", nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置认证头（使用 X-QW-Api-Key 而不是URL参数）
	req.Header.Set("X-QW-Api-Key", w.apiKey)
	req.Header.Set("Accept-Encoding", "gzip")

	// 发送请求
	// fmt.Printf("    📡 发送HTTP请求...\n")
	resp, err := w.client.Do(req)
	if err != nil {
		fmt.Printf("    ❌ 发送HTTP请求失败: %v\n", err)
		return "", nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	// fmt.Printf("    📡 响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("    ❌ API请求失败，状态码: %d\n", resp.StatusCode)
		return "", nil, fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	// 读取并解压缩响应体
	body, err := readResponseBody(resp)
	if err != nil {
		fmt.Printf("    ❌ 读取响应体失败: %v\n", err)
		return "", nil, fmt.Errorf("读取响应体失败: %w", err)
	}
	// fmt.Printf("    📡 响应体内容: %s\n", string(body))

	// 解析响应
	var locationResp QWeatherLocationResponse
	if err := json.Unmarshal(body, &locationResp); err != nil {
		fmt.Printf("    ❌ 解析地理位置响应失败: %v\n", err)
		return "", nil, fmt.Errorf("解析地理位置响应失败: %w", err)
	}

	// 检查API返回码
	// fmt.Printf("    📡 API返回码: %s\n", locationResp.Code)
	if locationResp.Code != "200" {
		fmt.Printf("    ❌ 和风天气API返回错误，代码: %s\n", locationResp.Code)
		return "", nil, fmt.Errorf("和风天气API返回错误，代码: %s", locationResp.Code)
	}

	// 检查是否有结果
	// fmt.Printf("    📡 找到的位置数量: %d\n", len(locationResp.Location))
	if len(locationResp.Location) == 0 {
		fmt.Printf("    ❌ 未找到城市: %s\n", city)
		return "", nil, fmt.Errorf("未找到城市: %s", city)
	}

	// 使用第一个结果
	location := locationResp.Location[0]
	locationInfo := map[string]string{
		"name":    location.Name,
		"adm2":    location.Adm2,
		"adm1":    location.Adm1,
		"country": location.Country,
	}

	return location.ID, locationInfo, nil
}

// getCurrentWeather 获取实时天气信息
func (w *WeatherTool) getCurrentWeather(ctx context.Context, locationID string) (*QWeatherCurrentResponse, error) {
	// fmt.Printf("    🌤️ 开始查询实时天气，位置ID: %s\n", locationID)

	// 构建实时天气查询URL
	params := url.Values{}
	params.Add("location", locationID)

	// 使用配置的baseURL
	fullURL := fmt.Sprintf("%s/v7/weather/now?%s", w.baseURL, params.Encode())
	// fmt.Printf("    🌤️ 请求URL: %s\n", fullURL)

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		fmt.Printf("    ❌ 创建HTTP请求失败: %v\n", err)
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置认证头（使用 X-QW-Api-Key 而不是URL参数）
	req.Header.Set("X-QW-Api-Key", w.apiKey)
	req.Header.Set("Accept-Encoding", "gzip")

	// 发送请求
	// fmt.Printf("    📡 发送HTTP请求...\n")
	resp, err := w.client.Do(req)
	if err != nil {
		fmt.Printf("    ❌ 发送HTTP请求失败: %v\n", err)
		return nil, fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	// fmt.Printf("    📡 响应状态码: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("    ❌ API请求失败，状态码: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

	// 读取并解压缩响应体
	body, err := readResponseBody(resp)
	if err != nil {
		fmt.Printf("    ❌ 读取响应体失败: %v\n", err)
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}
	// fmt.Printf("    📡 响应体内容: %s\n", string(body))

	// 解析响应
	var weatherResp QWeatherCurrentResponse
	if err := json.Unmarshal(body, &weatherResp); err != nil {
		fmt.Printf("    ❌ 解析天气响应失败: %v\n", err)
		return nil, fmt.Errorf("解析天气响应失败: %w", err)
	}

	// 检查API返回码
	// fmt.Printf("    📡 天气API返回码: %s\n", weatherResp.Code)
	if weatherResp.Code != "200" {
		fmt.Printf("    ❌ 和风天气API返回错误，代码: %s\n", weatherResp.Code)
		return nil, fmt.Errorf("和风天气API返回错误，代码: %s", weatherResp.Code)
	}

	return &weatherResp, nil
}

// formatWeatherData 格式化天气数据为用户友好的格式
func (w *WeatherTool) formatWeatherData(weatherData *QWeatherCurrentResponse, locationInfo map[string]string) (string, error) {
	now := weatherData.Now

	// 构建格式化的天气信息
	result := map[string]interface{}{
		"城市":     locationInfo["name"],
		"地区":     fmt.Sprintf("%s, %s", locationInfo["adm2"], locationInfo["adm1"]),
		"国家":     locationInfo["country"],
		"观测时间":   now.ObsTime,
		"温度":     fmt.Sprintf("%s°C", now.Temp),
		"体感温度":   fmt.Sprintf("%s°C", now.FeelsLike),
		"天气状况":   now.Text,
		"风向":     now.WindDir,
		"风力等级":   fmt.Sprintf("%s级", now.WindScale),
		"风速":     fmt.Sprintf("%s km/h", now.WindSpeed),
		"相对湿度":   fmt.Sprintf("%s%%", now.Humidity),
		"大气压强":   fmt.Sprintf("%s hPa", now.Pressure),
		"能见度":    fmt.Sprintf("%s km", now.Vis),
		"云量":     fmt.Sprintf("%s%%", now.Cloud),
		"当前小时降水": fmt.Sprintf("%s mm", now.Precip),
		"露点温度":   fmt.Sprintf("%s°C", now.Dew),
		"数据来源":   "和风天气",
		"更新时间":   weatherData.UpdateTime,
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
		"城市":     city,
		"地区":     "重庆",
		"国家":     "中国",
		"观测时间":   time.Now().Format("2006-01-02T15:04+08:00"),
		"温度":     "22°C",
		"体感温度":   "24°C",
		"天气状况":   "多云",
		"风向":     "东南风",
		"风力等级":   "3级",
		"风速":     "12 km/h",
		"相对湿度":   "65%",
		"大气压强":   "1013 hPa",
		"能见度":    "10 km",
		"云量":     "60%",
		"当前小时降水": "0 mm",
		"露点温度":   "18°C",
		"数据来源":   "模拟数据",
		"更新时间":   time.Now().Format("2006-01-02T15:04+08:00"),
		"说明":     "这是模拟数据，请配置和风天气API密钥以获取实际天气信息",
	}

	resultJSON, _ := json.MarshalIndent(mockData, "", "  ")
	return string(resultJSON)
}

// ValidateConfig 验证天气工具配置
func (w *WeatherTool) ValidateConfig() error {
	if w.apiKey == "" {
		return fmt.Errorf("和风天气API密钥不能为空")
	}

	if w.baseURL == "" {
		return fmt.Errorf("和风天气API基础URL不能为空")
	}

	// 检查URL格式
	if !strings.HasPrefix(w.baseURL, "http://") && !strings.HasPrefix(w.baseURL, "https://") {
		return fmt.Errorf("和风天气API基础URL格式不正确")
	}

	return nil
}

// TestConnection 测试API连接
func (w *WeatherTool) TestConnection(ctx context.Context) error {
	if w.apiKey == "" {
		return fmt.Errorf("API密钥未设置，无法测试连接")
	}

	// 使用北京作为测试城市
	_, _, err := w.getCityLocation(ctx, "北京")
	return err
}

// GetSupportedRegions 获取支持的地区信息
func (w *WeatherTool) GetSupportedRegions() []string {
	return []string{
		"支持全球城市天气查询",
		"中国城市：北京、上海、广州、深圳、杭州、成都等",
		"国际城市：New York, London, Tokyo, Paris等",
		"支持中英文城市名称",
		"数据来源：和风天气 (QWeather)",
	}
}
