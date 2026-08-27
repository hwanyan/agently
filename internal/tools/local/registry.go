// Package local 提供本地工具的插件化注册机制
// 每个工具通过 init() 自注册到全局注册表，支持按工具组自动发现和加载
package local

import (
	"github.com/hwanyan/agently/pkg/types"
)

// ToolFactory 工具工厂函数类型
// 接收配置提供者（而非固定的配置快照），使工具可以在每次真正使用配置值时
// （通常是 Execute 内部）读取到 config.yaml 热更新后的最新值，返回工具实例
type ToolFactory func(provider types.ConfigProvider) types.Tool

// toolEntry 工具注册条目
type toolEntry struct {
	group   string      // 所属工具组（如 "general"）
	factory ToolFactory // 工厂函数
}

// 全局工具注册表（所有的本地工具通过 init 函数在项目启动时自注册）
var globalRegistry []toolEntry

// Register 注册工具工厂到全局注册表
// 参数：
//   - group: 工具组标识（如 "general"）
//   - factory: 工具工厂函数
func Register(group string, factory ToolFactory) {
	globalRegistry = append(globalRegistry, toolEntry{
		group:   group,
		factory: factory,
	})
}

// GetToolsByGroup 按工具组获取所有工具实例
// 参数：
//   - group: 工具组标识
//   - provider: 配置提供者（用于让工具在真正使用时读取到最新的 API Key 等配置）
//
// 返回：
//   - []types.Tool: 该组下所有工具实例
func GetToolsByGroup(group string, provider types.ConfigProvider) []types.Tool {
	var tools []types.Tool
	for _, entry := range globalRegistry {
		if entry.group == group {
			tools = append(tools, entry.factory(provider))
		}
	}
	return tools
}
