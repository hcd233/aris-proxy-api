package proxy

import (
	"sync/atomic"

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
}

// LLMProxyConfig holds the full proxy configuration loaded from config.yaml
//
//	@author centonhuang
//	@update 2025-11-12 10:00:00
type LLMProxyConfig struct {
	Models  map[string]ModelConfig `mapstructure:"model_list" yaml:"model_list"`
	APIKeys map[string]string      `mapstructure:"api_keys" yaml:"api_keys"`
}

// llmProxyConfigPtr 使用 atomic.Pointer 实现无锁读取，支持热加载
//
//	@author centonhuang
//	@update 2026-03-20 10:00:00
var llmProxyConfigPtr atomic.Pointer[LLMProxyConfig]

// GetLLMProxyConfig 获取当前 LLM 代理配置（无锁读取）
//
//	@return *LLMProxyConfig
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func GetLLMProxyConfig() *LLMProxyConfig {
	return llmProxyConfigPtr.Load()
}

// InitLLMProxyConfig 初始化 LLM 代理配置，从 config.yaml 首次加载
//
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func InitLLMProxyConfig() {
	cfg := lo.Must1(loadLLMProxyConfig())
	llmProxyConfigPtr.Store(cfg)
	logProxyConfig(cfg)
}

// ReloadLLMProxyConfig 热加载 LLM 代理配置，重新读取 config.yaml 并原子替换
//
// 调用方应在成功后调用 middleware.RebuildAPIKeyIndex() 刷新 API Key 反向索引
//
//	@return error
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func ReloadLLMProxyConfig() error {
	cfg, err := loadLLMProxyConfig()
	if err != nil {
		return err
	}
	llmProxyConfigPtr.Store(cfg)
	logProxyConfig(cfg)
	return nil
}

// loadLLMProxyConfig 从 config.yaml 读取并反序列化 LLM 代理配置
//
//	@return *LLMProxyConfig
//	@return error
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func loadLLMProxyConfig() (*LLMProxyConfig, error) {
	// Use "::" as key delimiter to avoid conflicts with "." in model names (e.g. "gpt-4.1")
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	cfg := &LLMProxyConfig{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// logProxyConfig 打印掩码后的代理配置到日志
//
//	@param cfg *LLMProxyConfig
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func logProxyConfig(cfg *LLMProxyConfig) {
	type maskedEndpoint struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
	}
	type maskedModel struct {
		Model     string                     `json:"model"`
		Endpoints map[string]*maskedEndpoint `json:"endpoints"`
	}
	maskedModels := make(map[string]*maskedModel, len(cfg.Models))
	for name, mc := range cfg.Models {
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

	maskedAPIKeys := make(map[string]string, len(cfg.APIKeys))
	for name, key := range cfg.APIKeys {
		maskedAPIKeys[name] = util.MaskSecret(key)
	}

	logger.Logger().Info("[Proxy] LLM proxy config loaded",
		zap.Any("models", maskedModels),
		zap.Any("apiKeys", maskedAPIKeys),
	)
}
