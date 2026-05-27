// Package config 提供配置管理功能
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/hwanyan/agently/pkg/types"

	"gopkg.in/yaml.v2"
)

// PromptConfig 提示词配置结构
type PromptConfig struct {
	Scene        string `yaml:"scene"`
	Name         string `yaml:"name"`
	SystemPrompt string `yaml:"system_prompt"`
	Settings     struct {
		Language          string `yaml:"language"`
		Tone              string `yaml:"tone"`
		MaxResponseLength int    `yaml:"max_response_length"`
	} `yaml:"settings"`
}

// LoadConfiguration 加载整个项目所需的配置
// 说明：
//   - 首先尝试从配置文件加载，如果配置文件存在则使用
//   - 如果配置文件不存在，则从环境变量加载
//   - 加载配置后，会进行验证，如果验证失败，则会返回错误
//
// 返回：
//   - *types.Config: 加载的配置
//   - error: 可能的错误
func LoadConfiguration() (*types.Config, error) {
	// 首先尝试从配置文件加载
	configPath := "../config/config.yaml"
	// 读取配置文件
	if _, err := os.Stat(configPath); err == nil {
		// 加载配置文件并验证
		cfg, e := loadConfigFromFile(configPath)
		if e == nil {
			return cfg, nil
		}
	}

	// 从环境变量加载
	cfg, err := loadConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("从环境变量加载配置失败: %w", err)
	}

	return cfg, nil
}

// loadConfigFromFile 从YAML文件加载配置
// 参数：
//   - configPath: 配置文件路径
//
// 说明：
//
//   - 加载配置后，会进行验证，如果验证失败，则会返回错误
//
// 返回：
//   - *types.Config: 加载的配置
//   - error: 可能的错误
func loadConfigFromFile(configPath string) (*types.Config, error) {
	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析YAML配置
	var config types.Config
	if e := yaml.Unmarshal(data, &config); e != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", e)
	}

	// 验证配置
	if e := validateConfig(&config); e != nil {
		return nil, fmt.Errorf("配置验证失败: %w", e)
	}

	return &config, nil
}

// loadConfigFromEnv 从环境变量加载配置
// 说明：
//
//   - 加载配置后，会进行验证，如果验证失败，则会返回错误
//
// 返回：
//   - *types.Config: 加载的配置
//   - error: 可能的错误
func loadConfigFromEnv() (*types.Config, error) {
	config := &types.Config{
		Log: types.LogConfig{
			FilePath: getEnvOrDefault("LOG_FILE_PATH", "logs/agent.log"),
		},
		Prompt: types.PromptDirConfig{
			Dir: getEnvOrDefault("PROMPT_DIR", "../../internal/agents"),
		},
		LLM: types.LLMConfig{
			Provider: getEnvOrDefault("LLM_PROVIDER", "openai"),
			APIKey:   os.Getenv("LLM_API_KEY"),
			Model:    os.Getenv("LLM_MODEL"),
			BaseURL:  os.Getenv("LLM_BASE_URL"),
		},
		Tools: types.ToolsConfig{
			Weather: types.WeatherConfig{
				APIKey:  os.Getenv("WEATHER_API_KEY"),
				BaseURL: os.Getenv("WEATHER_BASE_URL"),
			},
			IP: types.IPToolConfig{
				AccessKey: os.Getenv("IPAPI_ACCESS_KEY"),
			},
			Translate: types.TranslateConfig{
				APIKey:  os.Getenv("TRANSLATE_API_KEY"),
				BaseURL: getEnvOrDefault("TRANSLATE_BASE_URL", "https://libretranslate.com"),
			},
			ShortURL: types.ShortURLConfig{
				APIKey:  os.Getenv("SHORTURL_API_KEY"),
				BaseURL: os.Getenv("SHORTURL_BASE_URL"),
			},
		},
	}

	// 验证配置
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return config, nil
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

	// 验证天气工具配置
	if config.Tools.Weather.APIKey == "" {
		return fmt.Errorf("天气API密钥不能为空")
	}

	if config.Tools.Weather.BaseURL == "" {
		return fmt.Errorf("天气基础URL不能为空")
	} else {
		if !strings.HasPrefix(config.Tools.Weather.BaseURL, "http://") && !strings.HasPrefix(config.Tools.Weather.BaseURL, "https://") {
			return fmt.Errorf("天气API基础URL格式不正确")
		}
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
