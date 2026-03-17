package proxy

import (
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// EndpointConfig holds connection info for a single protocol endpoint
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type EndpointConfig struct {
	APIKey  string `mapstructure:"api_key" yaml:"api_key"`
	BaseURL string `mapstructure:"base_url" yaml:"base_url"`
}

// ModelConfig holds the configuration for a single upstream LLM model.
// A model may expose multiple protocol endpoints (e.g. both openai and anthropic).
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type ModelConfig struct {
	Model     string                                `mapstructure:"model" yaml:"model"`
	Endpoints map[enum.ProviderType]*EndpointConfig `mapstructure:"endpoints" yaml:"endpoints"`
	// Legacy fields kept for backward-compatible YAML parsing; migrated to Endpoints during init.
	APIKey  string            `mapstructure:"api_key" yaml:"api_key"`
	BaseURL string            `mapstructure:"base_url" yaml:"base_url"`
	Type    enum.ProviderType `mapstructure:"type" yaml:"type"`
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

// InitLLMProxyConfig initializes the LLM proxy configuration from config.yaml.
// A *zap.Logger is accepted to avoid an import cycle (logger -> config).
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
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

	// Migrate legacy single-endpoint format to new multi-endpoint format
	for name, mc := range llmProxyConfig.Models {
		if len(mc.Endpoints) == 0 && mc.BaseURL != "" {
			// Legacy format detected: type + base_url + api_key → endpoints map
			providerType := mc.Type
			if providerType == "" {
				providerType = enum.ProviderOpenAI
			}
			mc.Endpoints = map[enum.ProviderType]*EndpointConfig{
				providerType: {
					APIKey:  mc.APIKey,
					BaseURL: mc.BaseURL,
				},
			}
			// Clear legacy fields after migration
			mc.APIKey = ""
			mc.BaseURL = ""
			mc.Type = ""
			llmProxyConfig.Models[name] = mc
		}
	}

	// Build masked model list for structured logging
	type maskedEndpoint struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	type maskedModel struct {
		Model     string                     `json:"model"`
		Endpoints map[string]*maskedEndpoint `json:"endpoints"`
	}
	maskedModels := make(map[string]*maskedModel, len(llmProxyConfig.Models))
	for name, mc := range llmProxyConfig.Models {
		mm := &maskedModel{
			Model:     mc.Model,
			Endpoints: make(map[string]*maskedEndpoint, len(mc.Endpoints)),
		}
		for provider, ep := range mc.Endpoints {
			mm.Endpoints[provider] = &maskedEndpoint{
				BaseURL: ep.BaseURL,
				APIKey:  util.MaskSecret(ep.APIKey),
			}
		}
		maskedModels[name] = mm
	}

	// Build masked API keys for structured logging
	maskedAPIKeys := make(map[string]string, len(llmProxyConfig.APIKeys))
	for name, key := range llmProxyConfig.APIKeys {
		maskedAPIKeys[name] = util.MaskSecret(key)
	}

	logger.Logger().Info("[Proxy] LLM proxy config loaded",
		zap.Any("models", maskedModels),
		zap.Any("apiKeys", maskedAPIKeys),
	)
}
