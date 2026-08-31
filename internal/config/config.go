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

// loadConfig 从YAML文件加载配置
func loadConfig(configPath string) (*types.Config, error) {
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
	// 验证运行端口
	if config.GRPC.Port == 0 {
		return fmt.Errorf("未设置 rpc 服务运行的端口")
	}
	if config.HTTP.Port == 0 {
		return fmt.Errorf("未设置 http 服务运行的端口")
	}
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
