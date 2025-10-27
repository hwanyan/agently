package tools

import (
	"agent-hwan/pkg/types"
	"encoding/json"
	"fmt"
)

// ToolInfo 工具信息结构
type ToolInfo types.FunctionCall

// ToolRegistry 工具注册表
// 提供统一的工具创建和配置管理
type ToolRegistry struct{}

// NewToolRegistry 创建工具注册表实例
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{}
}

// GetAvailableTools 获取所有可用工具的信息
func (r *ToolRegistry) GetAvailableTools() []ToolInfo {
	return []ToolInfo{
		{
			Name:        WeatherToolName,
			Description: WeatherToolDesc,
			Arguments:   getWeatherArguments(),
		},
		{
			Name:        IPToolName,
			Description: IPToolDesc,
			Arguments:   getIPArguments(),
		},
		{
			Name:        QRCodeToolName,
			Description: QRCodeToolDesc,
			Arguments:   getQRCodeArguments(),
		},
		{
			Name:        TranslationToolName,
			Description: TranslationToolDesc,
			Arguments:   getTranslationArguments(),
		},
		{
			Name:        TextProcessorToolName,
			Description: TextProcessorToolDesc,
			Arguments:   getTextProcessorArguments(),
		},
	}
}

// getWeatherArguments 获取天气工具的参数描述
func getWeatherArguments() string {
	weatherTool := &WeatherTool{}
	paramsJSON, err := json.MarshalIndent(weatherTool.Parameters(), "", "  ")
	if err != nil {
		return fmt.Sprintf("参数格式化失败: %v", err)
	} else {
		return fmt.Sprintf("参数结构:\n%s", string(paramsJSON))
	}
}

// getIPArguments 获取IP查询工具的参数描述
func getIPArguments() string {
	ipTool := &IPTool{}
	paramsJSON, err := json.MarshalIndent(ipTool.Parameters(), "", "  ")
	if err != nil {
		return fmt.Sprintf("参数格式化失败: %v", err)
	} else {
		return fmt.Sprintf("参数结构:\n%s", string(paramsJSON))
	}
}

// getQRCodeArguments 获取二维码工具的参数描述
func getQRCodeArguments() string {
	qrTool := &QRCodeTool{}
	paramsJSON, err := json.MarshalIndent(qrTool.Parameters(), "", "  ")
	if err != nil {
		return fmt.Sprintf("参数格式化失败: %v", err)
	} else {
		return fmt.Sprintf("参数结构:\n%s", string(paramsJSON))
	}
}

// getTranslationArguments 获取翻译工具的参数描述
func getTranslationArguments() string {
	trTool := &TranslationTool{}
	paramsJSON, err := json.MarshalIndent(trTool.Parameters(), "", "  ")
	if err != nil {
		return fmt.Sprintf("参数格式化失败: %v", err)
	} else {
		return fmt.Sprintf("参数结构:\n%s", string(paramsJSON))
	}
}

// getTextProcessorArguments 获取文本处理工具的参数描述
func getTextProcessorArguments() string {
	textTool := &TextProcessorTool{}
	paramsJSON, err := json.MarshalIndent(textTool.Parameters(), "", "  ")
	if err != nil {
		return fmt.Sprintf("参数格式化失败: %v", err)
	} else {
		return fmt.Sprintf("参数结构:\n%s", string(paramsJSON))
	}
}
