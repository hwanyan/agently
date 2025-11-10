package tools

import (
	"agent-hwan/internal/agent"
	"agent-hwan/pkg/types"
	"fmt"
)

// SetupTools 设置工具
func SetupTools(aiAgent *agent.BasicAgent, cfg *types.Config) {
	// 添加天气工具
	fmt.Println("    🌤️  添加天气工具...")
	weatherTool := NewWeatherTool(cfg.Tools.Weather.APIKey, cfg.Tools.Weather.BaseURL)
	aiAgent.AddTool(weatherTool)
	fmt.Println("    ✓ 天气工具已添加")

	// 添加IP查询工具
	fmt.Println("    🌐 添加IP查询工具...")
	ipTool := NewIPTool(cfg.Tools.IP.AccessKey)
	aiAgent.AddTool(ipTool)
	fmt.Println("    ✓ IP查询工具已添加")

	// 添加二维码生成工具
	fmt.Println("    🔳 添加二维码生成工具...")
	qrTool := NewQRCodeTool()
	aiAgent.AddTool(qrTool)
	fmt.Println("    ✓ 二维码生成工具已添加")

	// 添加翻译工具
	fmt.Println("    🔡 添加翻译工具...")
	trTool := NewTranslationTool(cfg.Tools.Translate.APIKey, cfg.Tools.Translate.BaseURL)
	aiAgent.AddTool(trTool)
	fmt.Println("    ✓ 翻译工具已添加")

	// 添加文本处理工具
	fmt.Println("    ✏️ 添加文本处理工具...")
	textTool := NewTextProcessorTool()
	aiAgent.AddTool(textTool)
	fmt.Println("    ✓ 文本处理工具已添加")

	// 添加短链接生成工具
	fmt.Println("    🔗 添加短链接生成工具...")
	shortURLTool := NewShortURLTool(cfg.Tools.ShortURL.APIKey, cfg.Tools.ShortURL.BaseURL)
	aiAgent.AddTool(shortURLTool)
	fmt.Println("    ✓ 短链接生成工具已添加")

	// 显示工具统计
	tools := aiAgent.GetTools()
	fmt.Printf("  📊 总共添加了 %d 个工具\n", len(tools))
}
