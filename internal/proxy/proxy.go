package proxy

import (
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// ModelConfig holds the configuration for a single upstream LLM model
//
//	@author centonhuang
//	@update 2026-03-05 17:23:02
type ModelConfig struct {
	Model   string `mapstructure:"model" yaml:"model"`
	APIKey  string `mapstructure:"api_key" yaml:"api_key"`
	BaseURL string `mapstructure:"base_url" yaml:"base_url"`
}

// LLMProxyConfig holds the full proxy configuration loaded from config.yaml
//
//	@author centonhuang
//	@update 2025-11-12 10:00:00
type LLMProxyConfig struct {
	Models  map[string]ModelConfig `mapstructure:"model_list" yaml:"model_list"`
	APIKeys map[string]string      `mapstructure:"api_keys" yaml:"api_keys"`
}

var llmProxyConfig *LLMProxyConfig

// GetLLMProxyConfig returns the singleton LLM proxy config
//
//	@return *LLMProxyConfig
//	@author centonhuang
//	@update 2025-11-12 10:00:00
func GetLLMProxyConfig() *LLMProxyConfig {
	return llmProxyConfig
}

// maskAPIKey masks an API key for safe logging, keeping first 4 and last 4 characters
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return fmt.Sprintf("%s***%s", key[:4], key[len(key)-4:])
}

// InitLLMProxyConfig initializes the LLM proxy configuration from config.yaml.
// A *zap.Logger is accepted to avoid an import cycle (logger -> config).
//
//	@author centonhuang
//	@update 2026-03-06 00:00:00
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

	// Build masked model list for structured logging
	maskedModels := make(map[string]map[string]string, len(llmProxyConfig.Models))
	for name, mc := range llmProxyConfig.Models {
		maskedModels[name] = map[string]string{
			"model":    mc.Model,
			"base_url": mc.BaseURL,
			"api_key":  maskAPIKey(mc.APIKey),
		}
	}

	// Build masked API keys for structured logging
	maskedAPIKeys := make(map[string]string, len(llmProxyConfig.APIKeys))
	for name, key := range llmProxyConfig.APIKeys {
		maskedAPIKeys[name] = maskAPIKey(key)
	}

	logger.Logger().Info("[Proxy] LLM proxy config loaded",
		zap.Any("models", maskedModels),
		zap.Any("apiKeys", maskedAPIKeys),
	)
}
