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
//   - 从配置文件加载配置
//   - 加载配置后，会进行验证，如果验证失败，则会返回错误
//
// 返回：
//   - *types.Config: 加载的配置
//   - error: 可能的错误
func LoadConfiguration() (*types.Config, error) {
	configPath := "../config/config.yaml"
	cfg, err := loadConfigFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置文件失败: %w", err)
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

	// 验证 SMTP 配置（必配项）
	if config.SMTP.Host == "" {
		return fmt.Errorf("SMTP 服务器地址（smtp.host）不能为空")
	}
	if config.SMTP.Port <= 0 || config.SMTP.Port > 65535 {
		return fmt.Errorf("SMTP 端口（smtp.port）无效，应为 1-65535 之间的整数")
	}
	if config.SMTP.Username == "" {
		return fmt.Errorf("SMTP 用户名（smtp.username）不能为空")
	}
	if config.SMTP.Password == "" {
		return fmt.Errorf("SMTP 密码/授权码（smtp.password）不能为空")
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
