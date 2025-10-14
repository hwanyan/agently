package prompt

import (
	"agent-hwan/internal/config"
	"agent-hwan/internal/tools"
	"agent-hwan/pkg/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

// 获取项目下的全部 Prompt
func GetPrompts() (map[string]string, error) {
	prompts := make(map[string]string)

	// 递归扫描整个项目下的所有 YAML 文件
	var files []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".yaml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("  ⚠️  递归扫描项目文件失败: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("  ⚠️  项目中未找到任何 YAML 文件")
	}

	// 遍历所有 YAML 文件
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("  ⚠️  读取文件 %s 失败: %v\n", file, err)
			continue
		}

		var config config.PromptConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			fmt.Printf("  ⚠️  解析文件 %s 失败: %v\n", file, err)
			continue
		}

		prompts[config.Scene] = config.SystemPrompt
	}

	return prompts, nil
}

// GetSystemPrompt 根据场景获取系统提示词
// 参数：
//   - scene: 场景名称，如 "system_prompt"
//   - toolConfig: 工具配置，用于动态注入工具信息
//
// 返回：
//   - string: 系统提示词内容
//   - error: 可能的错误
func GetSystemPrompt(prompts map[string]string, toolConfig *types.ToolsConfig) (string, error) {
	fmt.Println("  💼 获取系统提示词...")

	// 获取系统 Prompt
	systemPrompt, ok := prompts[SYSTEM_PROMPT]
	if !ok {
		return "", fmt.Errorf("❌ 找不到系统提示词配置")
	}

	// 生成工具信息
	toolsSection := generateToolsSection(toolConfig)

	// 替换工具占位符
	// 方式一：直接替换
	// systemPrompt := strings.ReplaceAll(systemPrompt, "{{TOOLS_SECTION}}", toolsSection)
	// 方式二：使用正则表达式替换
	// 准备模板变量
	inputs := map[string]any{
		"TOOLS_SECTION": toolsSection,
	}
	systemPrompt, err := replaceTemplateVars(systemPrompt, inputs)
	if err != nil {
		return "", fmt.Errorf("替换系统提示词模板变量失败: %w", err)
	}

	fmt.Printf("✅ 系统提示词替换成功！\n%s", systemPrompt)
	return systemPrompt, nil
}

// generateToolsSection 生成工具信息部分
func generateToolsSection(toolConfig *types.ToolsConfig) string {
	toolCount := 1
	var builder strings.Builder

	// 添加天气工具
	builder.WriteString(fmt.Sprintf("%d. **%s**\n", toolCount, toolConfig.Weather.Name))
	builder.WriteString(fmt.Sprintf("%s\n", toolConfig.Weather.Desc))
	// 将 Parameters() 返回的 map 转换为格式化的 JSON 字符串
	weatherTool := tools.WeatherTool{}
	paramsJSON, err := json.MarshalIndent(weatherTool.Parameters(), "", "  ")
	if err != nil {
		builder.WriteString(fmt.Sprintf("参数格式化失败: %v\n\n", err))
	} else {
		builder.WriteString(fmt.Sprintf("参数结构:\n%s\n\n", string(paramsJSON)))
	}
	toolCount++

	return strings.TrimSpace(builder.String())
}

// replaceTemplateVars 替换模板中的变量
func replaceTemplateVars(template string, inputs map[string]any) (string, error) {
	var err error
	// 使用正则表达式查找所有 {{var}} 格式的变量
	re := regexp.MustCompile(`{{([^}]+)}}`)
	templateReplacedContent := re.ReplaceAllStringFunc(template, func(match string) string {
		// 提取变量名
		varName := strings.TrimSpace(match[2 : len(match)-2])

		// 尝试从inputs变量模板中获取模板变量
		if value, ok := inputs[varName]; ok {
			// 返回value的字符串表示
			return fmt.Sprint(value)
		}

		// 如果模板变量不在inputs中，则模板变量被替换为空值
		err = fmt.Errorf("template var not existed in inputs param, template: %s", match)
		return match
	})

	return templateReplacedContent, err
}
