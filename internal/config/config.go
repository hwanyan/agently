// Package config 提供配置管理功能
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"agent-hwan/pkg/types"
)

// LoadConfig 从JSON文件加载配置
// 参数：
//   - configPath: 配置文件路径
//
// 返回：
//   - *types.Config: 加载的配置
//   - error: 可能的错误
func LoadConfig(configPath string) (*types.Config, error) {
	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	// fmt.Printf("  ✅ 读取配置文件成功：%s\n", string(data))

	// 解析JSON配置
	var config types.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	// fmt.Printf("  ✅ 解析配置文件成功：%v\n", config)

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}
	// fmt.Printf("  ✅ 验证配置文件成功\n")

	return &config, nil
}

// LoadConfigFromEnv 从环境变量加载配置
// 返回：
//   - *types.Config: 加载的配置
//   - error: 可能的错误
func LoadConfigFromEnv() (*types.Config, error) {
	config := &types.Config{
		LLM: types.LLMConfig{
			Provider: getEnvOrDefault("LLM_PROVIDER", "openai"),
			APIKey:   os.Getenv("LLM_API_KEY"),
			Model:    getEnvOrDefault("LLM_MODEL", "gpt-3.5-turbo"),
			BaseURL:  os.Getenv("LLM_BASE_URL"),
		},
		Agent: types.AgentConfig{
			Name:        getEnvOrDefault("AGENT_NAME", "智能助手"),
			Description: getEnvOrDefault("AGENT_DESCRIPTION", "一个能够使用工具的智能AI助手"),
			MaxTokens:   getEnvIntOrDefault("AGENT_MAX_TOKENS", 4096),
			Temperature: getEnvFloatOrDefault("AGENT_TEMPERATURE", 0.7),
		},
	}

	// 验证配置
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return config, nil
}

// SaveConfig 保存配置到JSON文件
// 参数：
//   - config: 要保存的配置
//   - configPath: 配置文件路径
//
// 返回：
//   - error: 可能的错误
func SaveConfig(config *types.Config, configPath string) error {
	// 序列化配置为JSON（利用 json.MarshalIndent 进行了格式美化）
	// 下面的写法是一种固定格式（也可以将最后的 "  " 替换为 "\t"）
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// validateConfig 验证配置的有效性
func validateConfig(config *types.Config) error {
	// 验证LLM配置
	if config.LLM.Provider == "" {
		return fmt.Errorf("LLM提供商不能为空")
	}

	if config.LLM.APIKey == "" {
		return fmt.Errorf("LLM API密钥不能为空")
	}

	if config.LLM.Model == "" {
		return fmt.Errorf("LLM模型不能为空")
	}

	// 验证Agent配置
	if config.Agent.Name == "" {
		return fmt.Errorf("Agent名称不能为空")
	}

	if config.Agent.MaxTokens <= 0 {
		return fmt.Errorf("最大token数必须大于0")
	}

	if config.Agent.Temperature < 0 || config.Agent.Temperature > 2 {
		return fmt.Errorf("温度参数必须在0-2之间")
	}

	return nil
}

// getEnvOrDefault 获取环境变量或返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault 获取整数类型的环境变量或返回默认值
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		// 简单的字符串到整数转换
		// 在实际项目中，应该使用strconv.Atoi并处理错误
		switch value {
		case "1024":
			return 1024
		case "2048":
			return 2048
		case "4096":
			return 4096
		case "8192":
			return 8192
		default:
			return defaultValue
		}
	}
	return defaultValue
}

// getEnvFloatOrDefault 获取浮点数类型的环境变量或返回默认值
func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		// 简单的字符串到整数转换
		// 在实际项目中，应该使用strconv.ParseFloat并处理错误
		switch value {
		case "0.0":
			return 0.0
		case "0.1":
			return 0.1
		case "0.3":
			return 0.3
		case "0.5":
			return 0.5
		case "0.7":
			return 0.7
		case "0.9":
			return 0.9
		case "1.0":
			return 1.0
		default:
			return defaultValue
		}
	}
	return defaultValue
}

// GetDefaultConfig 获取默认配置
// 返回：
//   - *types.Config: 默认配置
func GetDefaultConfig() *types.Config {
	return &types.Config{
		LLM: types.LLMConfig{
			Provider: "openai",
			APIKey:   "your-openai-api-key-here",
			Model:    "gpt-3.5-turbo",
			BaseURL:  "",
		},
		Agent: types.AgentConfig{
			Name:        "智能助手",
			Description: "一个能够使用工具的智能AI助手",
			MaxTokens:   4096,
			Temperature: 0.7,
		},
	}
}

// CreateDefaultConfigFile 创建默认配置文件
// 参数：
//   - configPath: 配置文件路径
//
// 返回：
//   - error: 可能的错误
func CreateDefaultConfigFile(configPath string) error {
	// 检查文件是否已存在
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("配置文件已存在: %s", configPath)
	}

	// 获取默认配置
	config := GetDefaultConfig()

	// 保存到文件
	return SaveConfig(config, configPath)
}

// UpdateConfig 更新配置中的特定字段
// 参数：
//   - config: 要更新的配置
//   - updates: 更新的字段映射
//
// 返回：
//   - error: 可能的错误
func UpdateConfig(config *types.Config, updates map[string]interface{}) error {
	for key, value := range updates {
		switch key {
		case "llm.provider":
			if v, ok := value.(string); ok {
				config.LLM.Provider = v
			}
		case "llm.api_key":
			if v, ok := value.(string); ok {
				config.LLM.APIKey = v
			}
		case "llm.model":
			if v, ok := value.(string); ok {
				config.LLM.Model = v
			}
		case "llm.base_url":
			if v, ok := value.(string); ok {
				config.LLM.BaseURL = v
			}
		case "agent.name":
			if v, ok := value.(string); ok {
				config.Agent.Name = v
			}
		case "agent.description":
			if v, ok := value.(string); ok {
				config.Agent.Description = v
			}
		case "agent.max_tokens":
			if v, ok := value.(int); ok {
				config.Agent.MaxTokens = v
			}
		case "agent.temperature":
			if v, ok := value.(float64); ok {
				config.Agent.Temperature = v
			}
		default:
			return fmt.Errorf("未知的配置字段: %s", key)
		}
	}

	// 验证更新后的配置
	return validateConfig(config)
}
