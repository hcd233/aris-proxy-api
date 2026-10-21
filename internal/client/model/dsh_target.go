package model

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// dshEntry DSH profile patch 的单个条目：DSH 只认 id / name / config 三个键
type dshEntry struct {
	ID     string         `yaml:"id"`
	Name   string         `yaml:"name"`
	Config map[string]any `yaml:"config"`
}

// dshPlainScalar 可安全作为 YAML 裸标量写出的字符集（凭证值走这里判断是否需要加引号）
var dshPlainScalar = regexp.MustCompile(`^[A-Za-z0-9._~+/=:-]+$`)

// DSHTarget 写入 ~/.dsh/profiles/<profile>/cordis.patch.yml 与 ~/.dsh/.credentials.yaml
type DSHTarget struct{}

func (DSHTarget) Key() string   { return constant.ClientModelTargetDSH }
func (DSHTarget) Label() string { return constant.ClientModelLabelDSH }

// ConfigPath DSH profile 的补丁层路径
func (DSHTarget) ConfigPath(home string) string {
	return filepath.Join(
		home,
		constant.ClientModelDSHProfilesPath,
		dshProfileName(home),
		constant.ClientModelDSHPatchFile,
	)
}

// Write 把模型写进 llm-pi-ai 条目，并把 API Key 写进凭证文件的 refs 引用
func (DSHTarget) Write(path, host, apiKey string, models []TargetModel) error {
	if _, statErr := os.Stat(filepath.Dir(path)); statErr != nil {
		return ierr.New(ierr.ErrValidation, constant.ClientModelExportDSHProfileMissing)
	}
	// 先写凭证：patch 里的 apiKeyEnv 只是引用，key 本身由 DSH 从凭证文件解析。
	if err := patchDSHCredentials(dshCredentialsPath(path), apiKey); err != nil {
		return err
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return ierr.Wrap(ierr.ErrInternal, readErr, "read dsh patch file")
	}
	entry, ok, parseErr := parseDSHEntry(string(content))
	if parseErr != nil {
		return parseErr
	}
	if !ok {
		entry = dshEntry{ID: constant.ClientModelDSHEntryID, Name: constant.ClientModelDSHEntryName}
	}
	if entry.Name == "" {
		entry.Name = constant.ClientModelDSHEntryName
	}
	mergeDSHProvider(&entry, host, models)

	block, marshalErr := marshalDSHEntry(entry)
	if marshalErr != nil {
		return marshalErr
	}
	return backupAndWrite(path, []byte(spliceDSHEntry(string(content), block)))
}

// dshProfileName 选择 profile：优先 desktop，其次唯一的 profile，否则字典序第一个
func dshProfileName(home string) string {
	entries, err := os.ReadDir(filepath.Join(home, constant.ClientModelDSHProfilesPath))
	if err != nil {
		return constant.ClientModelDSHDefaultProfile
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return constant.ClientModelDSHDefaultProfile
	}
	for _, name := range names {
		if name == constant.ClientModelDSHDefaultProfile {
			return name
		}
	}
	// ponytail: 多 profile 取字典序第一个；需要指定 profile 时再加 DSH_PROFILE 开关
	slices.Sort(names)
	return names[0]
}

// dshCredentialsPath 由补丁层路径反推 DSH home 下的凭证文件
func dshCredentialsPath(patchPath string) string {
	dshHome := filepath.Dir(filepath.Dir(filepath.Dir(patchPath)))
	return filepath.Join(dshHome, constant.ClientModelDSHCredentialsFile)
}

// parseDSHEntry 从 patch 文本中取出 llm-pi-ai 条目；content 为空或条目缺失时 ok=false
func parseDSHEntry(content string) (dshEntry, bool, error) {
	block, _, _, found := splitDSHEntry(content)
	if !found {
		return dshEntry{}, false, nil
	}
	var entries []dshEntry
	if err := yaml.Unmarshal([]byte(block), &entries); err != nil {
		return dshEntry{}, false, ierr.Wrap(ierr.ErrInternal, err, "parse dsh patch entry")
	}
	if len(entries) == 0 {
		return dshEntry{}, false, nil
	}
	return entries[0], true, nil
}

// splitDSHEntry 按行切出 llm-pi-ai 条目：条目从 `- id: llm-pi-ai` 起，到下一个顶层条目止。
// 注释与其余条目原样保留，重写只影响本条目。
func splitDSHEntry(content string) (block, before, after string, found bool) {
	lines := strings.SplitAfter(content, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == constant.ClientModelDSHPatchEntryPrefix+constant.ClientModelDSHEntryID {
			start = i
			break
		}
	}
	if start < 0 {
		return "", content, "", false
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], constant.ClientModelYAMLListItemPrefix) {
			end = i
			break
		}
	}
	// 条目尾部的空行与注释属于下一个条目，留在 after 里，不参与重写
	for end > start+1 {
		trimmed := strings.TrimSpace(lines[end-1])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			break
		}
		end--
	}
	return strings.Join(lines[start:end], ""), strings.Join(lines[:start], ""), strings.Join(lines[end:], ""), true
}

// spliceDSHEntry 把新条目塞回原文：有旧条目则原位替换，没有则追加到文件末尾
func spliceDSHEntry(content, block string) string {
	_, before, after, found := splitDSHEntry(content)
	if found {
		return before + block + "\n" + after
	}
	if content == "" {
		return block + "\n"
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "\n" + block + "\n"
}

// mergeDSHProvider 只覆盖本工具管理的字段，provider 上其余手改字段原样保留
func mergeDSHProvider(entry *dshEntry, host string, models []TargetModel) {
	if entry.Config == nil {
		entry.Config = map[string]any{}
	}
	providers, _ := entry.Config[constant.ClientModelKeyProviders].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
	}
	provider, _ := providers[constant.ClientModelDSHProviderID].(map[string]any)
	if provider == nil {
		provider = map[string]any{}
	}
	if _, ok := provider[constant.ClientModelKeyDisplayName].(string); !ok {
		provider[constant.ClientModelKeyDisplayName] = constant.ClientModelDSHProviderDisplayName
	}
	provider[constant.ClientModelKeyAPI] = constant.ClientModelAPIOpenAI
	provider[constant.ClientModelKeyAPIKeyEnv] = constant.ClientModelDSHCredentialRef
	provider[constant.ClientModelKeyBaseURL] = host + constant.OpenAIProxyPrefix
	provider[constant.ClientModelKeyModels] = mergeDSHModels(provider[constant.ClientModelKeyModels], models)
	providers[constant.ClientModelDSHProviderID] = provider
	entry.Config[constant.ClientModelKeyProviders] = providers
}

// mergeDSHModels 以服务端列表为准重排模型：同名条目保留手写字段（reasoningEfforts 等），
// 只刷新服务端下发的规格；name 缺失或仍是上次写下的 alias 时按展示名刷新；未选中的条目被移除。
func mergeDSHModels(existing any, models []TargetModel) []any {
	previous := map[string]map[string]any{}
	rawModels, _ := existing.([]any)
	for _, raw := range rawModels {
		spec, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := spec[constant.ClientModelKeyID].(string); ok {
			previous[id] = spec
		}
	}
	merged := make([]any, 0, len(models))
	for _, m := range models {
		spec := previous[m.Alias]
		if spec == nil {
			spec = map[string]any{}
		}
		spec[constant.ClientModelKeyID] = m.Alias
		if specName, _ := spec[constant.ClientModelKeyName].(string); specName == "" || specName == m.Alias {
			spec[constant.ClientModelKeyName] = dshModelDisplayName(m.Alias)
		}
		// 长度缺失时回落默认值：0 写进 DSH 会让 provider 拒绝加载整份配置
		contextLen := m.ContextLength
		if contextLen <= 0 {
			contextLen = constant.ClientModelDefaultContext
		}
		maxTokens := m.MaxOutputTokens
		if maxTokens <= 0 {
			maxTokens = constant.ClientModelPiDefaultMaxTokens
		}
		spec[constant.ClientModelKeyContextWindow] = contextLen
		spec[constant.ClientModelKeyMaxTokens] = maxTokens
		spec[constant.ClientModelKeyInput] = dshInputModalities(m.Capabilities)
		merged = append(merged, spec)
	}
	return merged
}

// dshModelDisplayName 由 alias 派生对外展示名：按 `-` 分词逐词首字母大写，
// 品牌惯例写法（deepseek → DeepSeek）查表覆盖。
func dshModelDisplayName(alias string) string {
	words := strings.Split(alias, constant.ClientModelDSHNameWordSep)
	for i, word := range words {
		if brand, isBrand := constant.ClientModelDisplayNameBrands[strings.ToLower(word)]; isBrand {
			words[i] = brand
			continue
		}
		words[i] = capitalizeFirstLetter(word)
	}
	return strings.Join(words, constant.ClientModelDSHNameWordSep)
}

// capitalizeFirstLetter 首字符大写、其余原样；空词与多字节首字符都安全
func capitalizeFirstLetter(word string) string {
	first, size := utf8.DecodeRuneInString(word)
	if size == 0 {
		return word
	}
	return string(unicode.ToUpper(first)) + word[size:]
}

// dshInputModalities 与 Pi 同一套 schema：只保留 text/image，其余模态丢弃
func dshInputModalities(capabilities []string) []string {
	modalities := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		if slices.Contains(constant.ClientModelPiInputModalities, capability) {
			modalities = append(modalities, capability)
		}
	}
	if len(modalities) == 0 {
		return []string{enum.InputModalityText}
	}
	return modalities
}

// marshalDSHEntry 按 2 空格缩进序列化单条目（与 DSH profile 既有风格一致）
func marshalDSHEntry(entry dshEntry) (string, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode([]dshEntry{entry}); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "encode dsh patch entry")
	}
	if err := encoder.Close(); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "close dsh patch encoder")
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

// patchDSHCredentials 把 API Key 写进凭证文件的 refs.<ref>，其余内容逐行保留
func patchDSHCredentials(path, secret string) error {
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ierr.Wrap(ierr.ErrInternal, err, "read dsh credentials")
	}
	if errors.Is(err, fs.ErrNotExist) {
		content = []byte(constant.ClientModelDSHCredentialsHeader)
	}
	updated := upsertYAMLRef(string(content), constant.ClientModelDSHCredentialRef, secret)
	return backupAndWrite(path, []byte(updated))
}

// upsertYAMLRef 在顶层 refs 块内更新或追加一个引用，块外内容不动
func upsertYAMLRef(content, ref, secret string) string {
	entry := constant.ClientModelYAMLIndent + ref + constant.ClientModelYAMLKeySep + yamlScalar(secret)
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	refsAt := -1
	for i, line := range lines {
		if strings.HasPrefix(line, constant.ClientModelKeyRefs+":") {
			refsAt = i
			break
		}
	}
	if refsAt < 0 {
		return strings.Join(append(lines, constant.ClientModelKeyRefs+":", entry), "\n") + "\n"
	}
	lines[refsAt] = constant.ClientModelKeyRefs + ":"

	end := len(lines)
	for i := refsAt + 1; i < len(lines); i++ {
		if lines[i] != "" && !strings.HasPrefix(lines[i], " ") {
			end = i
			break
		}
	}
	for i := refsAt + 1; i < end; i++ {
		if strings.HasPrefix(lines[i], constant.ClientModelYAMLIndent+ref+":") {
			lines[i] = entry
			return strings.Join(lines, "\n") + "\n"
		}
	}
	insertAt := end
	for insertAt > refsAt+1 && lines[insertAt-1] == "" {
		insertAt--
	}
	merged := make([]string, 0, len(lines)+1)
	merged = append(merged, lines[:insertAt]...)
	merged = append(merged, entry)
	merged = append(merged, lines[insertAt:]...)
	return strings.Join(merged, "\n") + "\n"
}

// yamlScalar 凭证值按需加引号，避免 key 里的特殊字符改变 YAML 语义
func yamlScalar(value string) string {
	if dshPlainScalar.MatchString(value) {
		return value
	}
	return strconv.Quote(value)
}
