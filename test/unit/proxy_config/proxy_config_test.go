package proxy_config

import (
	"os"
	"sort"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// testEndpoint 测试用端点数据
type testEndpoint struct {
	Alias    string `json:"alias"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
}

// testAPIKey 测试用 API Key 数据
type testAPIKey struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// keyLookupTest API Key 反向查找测试
type keyLookupTest struct {
	Key         string `json:"key"`
	ExpectName  string `json:"expect_name"`
	ExpectFound bool   `json:"expect_found"`
}

// testCase 测试用例
type testCase struct {
	Name                     string          `json:"name"`
	Description              string          `json:"description"`
	Endpoints                []testEndpoint  `json:"endpoints"`
	APIKeys                  []testAPIKey    `json:"api_keys"`
	ExpectModels             []string        `json:"expect_models"`
	ExpectOpenAIProviders    []string        `json:"expect_openai_providers"`
	ExpectAnthropicProviders []string        `json:"expect_anthropic_providers"`
	KeyLookupTests           []keyLookupTest `json:"key_lookup_tests"`
}

// endpointConfig 端点配置（纯内存结构，用于测试组装逻辑）
type endpointConfig struct {
	APIKey  string
	BaseURL string
}

// modelConfig 模型配置（纯内存结构，用于测试组装逻辑）
type modelConfig struct {
	Model     string
	Endpoints map[string]*endpointConfig
}

func loadCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []testCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

func findCase(t *testing.T, cases []testCase, name string) testCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %q not found", name)
	return testCase{}
}

// assembleModels 模拟从端点列表组装模型配置（与 Service 层从 DB 查询后的组装逻辑一致）
func assembleModels(t *testing.T, endpoints []testEndpoint) map[string]modelConfig {
	t.Helper()
	models := make(map[string]modelConfig)
	for _, ep := range endpoints {
		mc, exists := models[ep.Alias]
		if !exists {
			mc = modelConfig{
				Model:     ep.Model,
				Endpoints: make(map[string]*endpointConfig),
			}
		}
		mc.Endpoints[ep.Provider] = &endpointConfig{
			APIKey:  ep.APIKey,
			BaseURL: ep.BaseURL,
		}
		models[ep.Alias] = mc
	}
	return models
}

func TestProxyConfig_ModelListAssembly(t *testing.T) {
	allCases := loadCases(t)

	tests := []string{"basic_openai_model", "multi_provider_model", "multiple_models_and_keys"}
	for _, caseName := range tests {
		t.Run(caseName, func(t *testing.T) {
			tc := findCase(t, allCases, caseName)
			models := assembleModels(t, tc.Endpoints)

			gotModels := make([]string, 0, len(models))
			for alias := range models {
				gotModels = append(gotModels, alias)
			}
			sort.Strings(gotModels)
			sort.Strings(tc.ExpectModels)

			if len(gotModels) != len(tc.ExpectModels) {
				t.Errorf("model count mismatch: got %d, want %d", len(gotModels), len(tc.ExpectModels))
			}
			for i, got := range gotModels {
				if i < len(tc.ExpectModels) && got != tc.ExpectModels[i] {
					t.Errorf("model[%d] = %s, want %s", i, got, tc.ExpectModels[i])
				}
			}

			var gotOpenAI []string
			for alias, mc := range models {
				if _, ok := mc.Endpoints[enum.ProviderOpenAI]; ok {
					gotOpenAI = append(gotOpenAI, alias)
				}
			}
			sort.Strings(gotOpenAI)
			sort.Strings(tc.ExpectOpenAIProviders)

			if len(gotOpenAI) != len(tc.ExpectOpenAIProviders) {
				t.Errorf("openai provider count mismatch: got %v, want %v", gotOpenAI, tc.ExpectOpenAIProviders)
			}

			var gotAnthropic []string
			for alias, mc := range models {
				if _, ok := mc.Endpoints[enum.ProviderAnthropic]; ok {
					gotAnthropic = append(gotAnthropic, alias)
				}
			}
			sort.Strings(gotAnthropic)
			sort.Strings(tc.ExpectAnthropicProviders)

			if len(gotAnthropic) != len(tc.ExpectAnthropicProviders) {
				t.Errorf("anthropic provider count mismatch: got %v, want %v", gotAnthropic, tc.ExpectAnthropicProviders)
			}
		})
	}
}

func TestProxyConfig_EndpointFields(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "basic_openai_model")
	models := assembleModels(t, tc.Endpoints)

	mc, ok := models["gpt-4o"]
	if !ok {
		t.Fatal("model gpt-4o not found in config")
	}

	if mc.Model != "gpt-4o-2024-08-06" {
		t.Errorf("upstream model = %s, want gpt-4o-2024-08-06", mc.Model)
	}

	ep, ok := mc.Endpoints[enum.ProviderOpenAI]
	if !ok {
		t.Fatal("openai endpoint not found for gpt-4o")
	}

	if ep.APIKey != "sk-test-openai-key" {
		t.Errorf("api key = %s, want sk-test-openai-key", ep.APIKey)
	}

	if ep.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("base url = %s, want https://api.openai.com/v1", ep.BaseURL)
	}
}

func TestProxyConfig_MultiProviderEndpoints(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "multi_provider_model")
	models := assembleModels(t, tc.Endpoints)

	mc, ok := models["deepseek-chat"]
	if !ok {
		t.Fatal("model deepseek-chat not found in config")
	}

	if len(mc.Endpoints) != 2 {
		t.Errorf("endpoint count = %d, want 2", len(mc.Endpoints))
	}

	openaiEP, ok := mc.Endpoints[enum.ProviderOpenAI]
	if !ok {
		t.Fatal("openai endpoint not found")
	}
	if openaiEP.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("openai base url = %s, want https://api.deepseek.com/v1", openaiEP.BaseURL)
	}

	anthropicEP, ok := mc.Endpoints[enum.ProviderAnthropic]
	if !ok {
		t.Fatal("anthropic endpoint not found")
	}
	if anthropicEP.BaseURL != "https://api.deepseek.com" {
		t.Errorf("anthropic base url = %s, want https://api.deepseek.com", anthropicEP.BaseURL)
	}
}

func TestProxyConfig_APIKeyLookup(t *testing.T) {
	allCases := loadCases(t)
	tc := findCase(t, allCases, "api_key_lookup")

	// Build key -> name map (simulates DB query result)
	apiKeyMap := make(map[string]string, len(tc.APIKeys))
	for _, k := range tc.APIKeys {
		apiKeyMap[k.Key] = k.Name
	}

	for _, lt := range tc.KeyLookupTests {
		t.Run(lt.Key, func(t *testing.T) {
			name, found := apiKeyMap[lt.Key]

			if found != lt.ExpectFound {
				t.Errorf("key lookup found = %v, want %v", found, lt.ExpectFound)
			}
			if found && name != lt.ExpectName {
				t.Errorf("key lookup name = %s, want %s", name, lt.ExpectName)
			}
		})
	}
}
