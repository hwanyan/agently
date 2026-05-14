package prompt

import (
	"agent-hwan/internal/config"
	"agent-hwan/internal/logger"
	"agent-hwan/internal/tools"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

// 获取项目下的全部 Prompt
// 参数：
//   - promptDir: Prompt 文件所在目录路径
//   - 返回结构：map[场景名称]系统提示词内容，如 map["intention_router_prompt"] = "意图路由Agent"
func GetPrompts(promptDir string) (map[string]string, error) {
	prompts := make(map[string]string)

	// 递归扫描指定目录下的所有 YAML 文件
	var files []string
	err := filepath.Walk(promptDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".yaml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("递归扫描 Prompt 目录失败: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("Prompt 目录 [%s] 中未找到任何 YAML 文件", promptDir)
	}

	// 遍历所有 YAML 文件
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			logger.Warning("读取文件 %s 失败: %v", file, err)
			continue
		}

		var config config.PromptConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			logger.Warning("解析文件 %s 失败: %v", file, err)
			continue
		}

		prompts[config.Scene] = config.SystemPrompt
	}

	return prompts, nil
}

// GetSystemPrompt 获取通用助手的系统提示词（保留兼容性）
// 参数：
//   - prompts: 所有 Prompt 配置
//   - toolRegistry: 工具注册表，用于动态注入工具信息
//
// 返回：
//   - string: 系统提示词内容
//   - error: 可能的错误
func GetSystemPrompt(prompts map[string]string, toolRegistry *tools.ToolRegistry) (string, error) {
	vars := map[string]any{
		"TOOLS_SECTION": generateToolsSection(toolRegistry),
	}
	return GetPromptByScene(prompts, GENERAL_PROMPT, vars)
}

// GetPromptByScene 根据场景名获取系统提示词
// 参数：
//   - prompts: 所有 Prompt 配置
//   - scene: 场景名称（如 "general_prompt"、"gold_analysis_prompt"）
//   - vars: 模板变量映射（如 {"TOOLS_SECTION": "...", "AGENTS_SECTION": "..."}）
//
// 返回：
//   - string: 替换模板变量后的系统提示词
//   - error: 可能的错误
func GetPromptByScene(prompts map[string]string, scene string, vars map[string]any) (string, error) {
	logger.Info("获取场景 [%s] 的系统提示词...", scene)

	// 获取对应场景的 Prompt
	promptContent, ok := prompts[scene]
	if !ok {
		return "", fmt.Errorf("❌ 找不到场景 [%s] 的提示词配置", scene)
	}

	// 替换模板变量
	result, err := replaceTemplateVars(promptContent, vars)
	if err != nil {
		return "", fmt.Errorf("替换场景 [%s] 提示词模板变量失败: %w", scene, err)
	}

	logger.Info("场景 [%s] 提示词替换成功", scene)
	return result, nil
}

// GenerateToolsSection 生成工具信息部分（导出，供外部调用）
func GenerateToolsSection(toolConfig *tools.ToolRegistry) string {
	return generateToolsSection(toolConfig)
}

// generateToolsSection 生成工具信息部分
func generateToolsSection(toolConfig *tools.ToolRegistry) string {
	var builder strings.Builder

	// 遍历所有工具信息，添加到工具列表中
	for index, tool := range toolConfig.GetAvailableTools() {
		builder.WriteString(fmt.Sprintf("%d. **%s**\n", (index + 1), tool.Name))
		builder.WriteString(fmt.Sprintf("  %s\n", tool.Description))
		builder.WriteString(fmt.Sprintf("  %s\n", tool.Arguments))
	}

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
