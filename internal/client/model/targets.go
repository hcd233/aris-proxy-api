package model

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// ─── OpenCode ───

// opencodeProviderOptions OpenCode provider options 块
type opencodeProviderOptions struct {
	BaseURL string            `json:"baseURL"`
	Headers map[string]string `json:"headers"`
}

// opencodeModel OpenCode 单模型配置
type opencodeModel struct {
	Name        string          `json:"name"`
	Attachment  bool            `json:"attachment,omitempty"`
	Modalities  *modalitiesSpec `json:"modalities,omitempty"`
	Limit       limitSpec       `json:"limit"`
	Temperature bool            `json:"temperature"`
	ToolCall    bool            `json:"tool_call"`
}

type modalitiesSpec struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type limitSpec struct {
	Context int `json:"context"`
	Output  int `json:"output"`
}

// opencodeConfig opencode.json 顶层结构（仅操作本工具管理的 provider，其余字段保留）
type opencodeConfig struct {
	Provider map[string]opencodeProvider `json:"provider"`
}

type opencodeProvider struct {
	Name    string                   `json:"name,omitempty"`
	NPM     string                   `json:"npm,omitempty"`
	Options *opencodeProviderOptions `json:"options,omitempty"`
	Models  map[string]opencodeModel `json:"models"`
}

func defaultCapabilities() []string { return []string{enum.InputModalityText} }

// OpenCodeTarget 写入 ~/.config/opencode/opencode.json
type OpenCodeTarget struct{}

func (OpenCodeTarget) Key() string   { return constant.ClientModelTargetOpenCode }
func (OpenCodeTarget) Label() string { return constant.ClientModelLabelOpenCode }
func (OpenCodeTarget) ConfigPath(home string) string {
	return filepath.Join(home, constant.ClientModelOpenCodePath)
}

// Write merge 本工具 provider 的 models 到既有配置；已存在 provider 时仅更新 models 键
func (OpenCodeTarget) Write(path, host, apiKey string, models []TargetModel) error {
	var cfg opencodeConfig
	if err := readJSONFile(path, &cfg); err != nil {
		return err
	}
	if cfg.Provider == nil {
		cfg.Provider = map[string]opencodeProvider{}
	}
	existing, exists := cfg.Provider[constant.ClientModelProviderID]
	if !exists {
		existing = opencodeProvider{
			Name: constant.ClientModelProviderID,
			NPM:  constant.ClientModelOpenCodeNPM,
			Options: &opencodeProviderOptions{
				BaseURL: host,
				Headers: map[string]string{constant.HTTPHeaderAuthorization: constant.ClientModelAuthBearer + apiKey},
			},
			Models: map[string]opencodeModel{},
		}
	}
	if existing.Models == nil {
		existing.Models = map[string]opencodeModel{}
	}
	for _, m := range models {
		caps := m.Capabilities
		if len(caps) == 0 {
			caps = defaultCapabilities()
		}
		contextLen := m.ContextLength
		if contextLen <= 0 {
			contextLen = constant.ClientModelDefaultContext
		}
		outputLen := m.MaxOutputTokens
		if outputLen <= 0 {
			outputLen = constant.ClientModelOpenCodeDefaultOutput
		}
		entry := opencodeModel{
			Name:        upperFirst(m.Alias),
			Modalities:  &modalitiesSpec{Input: caps, Output: []string{enum.InputModalityText}},
			Limit:       limitSpec{Context: contextLen, Output: outputLen},
			Temperature: true,
			ToolCall:    true,
		}
		if slices.Contains(caps, enum.InputModalityImage) {
			entry.Attachment = true
		}
		existing.Models[m.Alias] = entry
	}
	cfg.Provider[constant.ClientModelProviderID] = existing

	data, err := sonicMarshal(&cfg)
	if err != nil {
		return err
	}
	if err := backupAndWrite(path, data); err != nil {
		return err
	}
	return secureDefaultDir(path)
}

// ─── Pi ───

// piModel Pi models.json 单模型配置
type piModel struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Reasoning     bool     `json:"reasoning"`
	Input         []string `json:"input"`
	ContextWindow int      `json:"contextWindow"`
	MaxTokens     int      `json:"maxTokens"`
	Cost          piCost   `json:"cost"`
}

type piCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// piConfig Pi models.json：provider -> models 数组（其余字段经 Raw 透传保留）
type piConfig struct {
	Raw map[string]any `json:"-"`
}

func (c *piConfig) UnmarshalJSON(data []byte) error { return sonicUnmarshal(data, &c.Raw) }

func (c *piConfig) MarshalJSON() ([]byte, error) { return sonicMarshal(c.Raw) }

// PiTarget 写入 ~/.pi/agent/models.json
type PiTarget struct{}

func (PiTarget) Key() string   { return constant.ClientModelTargetPi }
func (PiTarget) Label() string { return constant.ClientModelLabelPi }
func (PiTarget) ConfigPath(home string) string {
	return filepath.Join(home, constant.ClientModelPiPath)
}

// Write merge 模型到 providers.<providerID>.models 列表（Pi models.json 顶层为 providers 包裹）
func (PiTarget) Write(path, host, apiKey string, models []TargetModel) error {
	var cfg piConfig
	if err := readJSONFile(path, &cfg); err != nil {
		return err
	}
	if cfg.Raw == nil {
		cfg.Raw = map[string]any{}
	}
	providers, _ := cfg.Raw[constant.ClientModelKeyProviders].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
		cfg.Raw[constant.ClientModelKeyProviders] = providers
	}
	provider, _ := providers[constant.ClientModelProviderID].(map[string]any)
	if provider == nil {
		provider = map[string]any{
			constant.ClientModelKeyName:    constant.ClientModelProviderID,
			constant.ClientModelKeyBaseUrl: host,
			constant.ClientModelKeyAPIKey:  apiKey,
			constant.ClientModelKeyAPI:     constant.ClientModelAPIOpenAI,
		}
		providers[constant.ClientModelProviderID] = provider
	}
	rawModels, _ := provider[constant.ClientModelKeyModels].([]any)
	byID := map[string]bool{}
	for _, rm := range rawModels {
		if m, ok := rm.(map[string]any); ok {
			if id, ok := m[constant.ClientModelKeyID].(string); ok {
				byID[id] = true
			}
		}
	}
	for _, m := range models {
		if byID[m.Alias] {
			continue
		}
		caps := m.Capabilities
		if len(caps) == 0 {
			caps = defaultCapabilities()
		}
		contextLen := m.ContextLength
		if contextLen <= 0 {
			contextLen = constant.ClientModelDefaultContext
		}
		maxTokens := m.MaxOutputTokens
		if maxTokens <= 0 {
			maxTokens = constant.ClientModelPiDefaultMaxTokens
		}
		rawModels = append(rawModels, piModel{
			ID:            m.Alias,
			Name:          m.Alias,
			Reasoning:     true,
			Input:         caps,
			ContextWindow: contextLen,
			MaxTokens:     maxTokens,
			Cost:          piCost{},
		})
		byID[m.Alias] = true
	}
	provider[constant.ClientModelKeyModels] = rawModels

	data, err := sonicMarshal(&cfg)
	if err != nil {
		return err
	}
	if err := backupAndWrite(path, data); err != nil {
		return err
	}
	return secureDefaultDir(path)
}

// ─── 共享工具 ───

// upperFirst 首字母大写（ASCII 足够；别名通常以字母开头）
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// secureDefaultDir 默认安装路径位于 ~/.pi 或 ~/.config 下时收紧目录权限到 0700
func secureDefaultDir(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "resolve home directory")
	}
	dir := filepath.Dir(path)
	if withinHome(dir, home) && dir != home {
		if chmodErr := os.Chmod(dir, 0o700); chmodErr != nil { //nolint:gosec // directory needs 0700 for execute
			return ierr.Wrap(ierr.ErrInternal, chmodErr, "secure config directory")
		}
	}
	return nil
}

// withinHome 判断 path 是否位于 home 内
func withinHome(path, home string) bool {
	rel, err := filepath.Rel(home, path)
	if err != nil {
		return false
	}
	return rel != constant.ClientModelParentRel && !strings.HasPrefix(rel, constant.ClientModelParentRelSep)
}
