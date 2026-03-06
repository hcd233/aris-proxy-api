package config

import (
	"github.com/samber/lo"
	"github.com/spf13/viper"
)

// ModelConfig 模型参数配置
//
//	@author centonhuang
//	@update 2026-03-05 17:23:02
type ModelConfig struct {
	Model   string `mapstructure:"model" yaml:"model"`
	APIKey  string `mapstructure:"api_key" yaml:"api_key"`
	BaseURL string `mapstructure:"base_url" yaml:"base_url"`
}

// LLMProxyConfig 代理配置（从config.yaml加载）
//
//	@author centonhuang
//	@update 2025-11-12 10:00:00
type LLMProxyConfig struct {
	Models  map[string]ModelConfig `mapstructure:"model_list" yaml:"model_list"`
	APIKeys map[string]string      `mapstructure:"api_keys" yaml:"api_keys"`
}

var llmProxyConfig *LLMProxyConfig

// GetLLMProxyConfig 获取代理配置（单例）
//
//	@return *ProxyConfig
//	@author centonhuang
//	@update 2025-11-12 10:00:00
func GetLLMProxyConfig() *LLMProxyConfig {
	return llmProxyConfig
}

// InitLLMProxyConfig 初始化代理配置
//
//	@author centonhuang
//	@update 2026-03-05 19:42:49
func InitLLMProxyConfig() {
	// Use "::" as key delimiter to avoid conflicts with "." in model names (e.g. "gpt-4.1")
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	llmProxyConfig = &LLMProxyConfig{}

	lo.Must0(v.ReadInConfig())
	lo.Must0(v.Unmarshal(llmProxyConfig))
}
