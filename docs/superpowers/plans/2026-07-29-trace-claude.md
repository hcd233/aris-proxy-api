# Claude Trace（多 Agent 统一抽象）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 抽取 codex trace 的客户端/服务端抽象（adapter + registry），接入 Claude Code hook + transcript 的 trace 上报与对话投影。

**Architecture:** 客户端 `aris trace ingest --agent <name>` 通过 `AgentAdapter`（ParseHook / ClassifyTranscriptLine / StdoutAck）抹平 codex 与 claude 的格式差异，spool/offset/批量上报复用；服务端 batch envelope 加 `agent` 字段，完成事件与对话投影按 agent registry 分发。

**Tech Stack:** Go 1.24+、cobra、sonic、huma、gorm、shell 安装脚本模板。

**Spec:** `docs/superpowers/specs/2026-07-29-trace-claude-design.md`

## Global Constraints

- 客户端行为 **fail-open**：任何采集错误都不能让 `trace ingest` 以非零码退出（会影响 Codex / Claude Code 正常运行）。
- 日志/错误信息**不得**包含 API Key、JWT、完整 payload。
- 原始 payload 必须原样入库（`sonic.NoCopyRawMessage` / `traceschema.RawJSON`），未知字段不丢。
- breaking change 已获批准：DTO legacy 单事件字段删除、hook 命令改为显式 `--agent`。
- 新 agent 接入成本 = 客户端加 1 个 adapter 文件 + 服务端 2 行 registry 注册。
- 工作目录：`.worktrees/claude-trace-ingest-2026-07-29`（分支 `feature/claude-trace-ingest-2026-07-29`）。
- 聚焦测试命令：`go test -count=1 ./internal/client/trace/ ./internal/domain/trace/ ./test/unit/trace/ ./test/e2e/trace/`
- conv lint：`go run ./cmd/server lint conv <changed pkgs...>`；静态检查：`go run ./cmd/server lint static <changed pkgs...>`

---

### Task 1: 客户端 AgentAdapter 抽象 + codex 行为迁移

**Files:**
- Create: `internal/client/trace/adapter.go`
- Create: `internal/client/trace/codex.go`
- Create: `internal/client/trace/codex_test.go`
- Modify: `internal/client/trace/ingest.go`
- Modify: `internal/client/trace/rollout.go`
- Modify: `internal/client/trace/spool.go`（`PendingRecord` 加 `Agent`）
- Modify: `internal/client/trace/config.go`（删 `Config.Agent`）
- Modify: `cmd/client/trace.go`（`--agent` flag）
- Modify: `test/e2e/trace/hook_test.go`（传 `--agent codex`）

**Interfaces:**
- Consumes: 现有 `Paths`、`Spool`、`ConfigStore`、`nextSequence`。
- Produces（后续任务依赖）:
  ```go
  type HookInfo struct { SessionID, EventName, Model, CWD, SessionSource, TurnID, CallID, TranscriptPath string }
  type TranscriptMeta struct { RecordType, Event, TurnID, CallID string }
  type AgentAdapter interface {
      Name() string
      ParseHook(raw []byte) (HookInfo, error)
      ClassifyTranscriptLine(raw []byte) TranscriptMeta
      StdoutAck(info HookInfo) string
  }
  func LookupAdapter(name string) (AgentAdapter, error)
  func NewIngestor(paths Paths, client *http.Client, adapter AgentAdapter) *Ingestor
  func (r *RolloutReader) ReadNew(ctx context.Context, sessionID, transcriptPath string) ([]PendingRecord, error) // 签名不变，内部用构造时注入的 adapter
  IngestCommandOptions{Paths, In, Out, HTTPClient, AgentName string}
  ```

- [ ] **Step 1: 写失败测试 `internal/client/trace/codex_test.go`**

```go
package trace

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestLookupAdapter_KnownAndUnknown(t *testing.T) {
	t.Parallel()
	if _, err := LookupAdapter(constant.TraceAgentCodex); err != nil {
		t.Fatalf("codex adapter must be registered: %v", err)
	}
	if _, err := LookupAdapter("nope"); err == nil {
		t.Fatal("unknown agent must return error")
	}
}

func TestCodexAdapter_ParseHook(t *testing.T) {
	t.Parallel()
	adapter, err := LookupAdapter(constant.TraceAgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"hook_event_name":"PreToolUse","session_id":"s1","turn_id":"t1","tool_use_id":"call-1","transcript_path":"/tmp/r.jsonl","model":"gpt-5","cwd":"/repo","source":"startup"}`)
	info, err := adapter.ParseHook(raw)
	if err != nil {
		t.Fatalf("parse hook: %v", err)
	}
	if info.EventName != "PreToolUse" || info.SessionID != "s1" || info.TurnID != "t1" ||
		info.CallID != "call-1" || info.TranscriptPath != "/tmp/r.jsonl" || info.Model != "gpt-5" {
		t.Fatalf("unexpected hook info: %+v", info)
	}
}

func TestCodexAdapter_StdoutAck(t *testing.T) {
	t.Parallel()
	adapter, _ := LookupAdapter(constant.TraceAgentCodex)
	if got := adapter.StdoutAck(HookInfo{EventName: constant.TraceEventStop}); got != "{}" {
		t.Fatalf("Stop ack = %q, want {}", got)
	}
	if got := adapter.StdoutAck(HookInfo{EventName: constant.TraceEventSessionStart}); got != "" {
		t.Fatalf("SessionStart ack = %q, want empty", got)
	}
}

func TestCodexAdapter_ClassifyTranscriptLine(t *testing.T) {
	t.Parallel()
	adapter, _ := LookupAdapter(constant.TraceAgentCodex)

	meta := adapter.ClassifyTranscriptLine([]byte(`{"timestamp":"2026-07-09T07:53:04.719Z","type":"response_item","payload":{"type":"function_call","call_id":"call-1","internal_chat_message_metadata_passthrough":{"turn_id":"t1"}}}`))
	if meta.RecordType != constant.TraceRecordTypeResponseItem || meta.Event != "function_call" ||
		meta.TurnID != "t1" || meta.CallID != "call-1" {
		t.Fatalf("unexpected meta: %+v", meta)
	}

	meta = adapter.ClassifyTranscriptLine([]byte(`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t2"}}`))
	if meta.RecordType != constant.TraceRecordTypeEventMsg || meta.Event != "task_complete" || meta.TurnID != "t2" {
		t.Fatalf("unexpected meta: %+v", meta)
	}

	meta = adapter.ClassifyTranscriptLine([]byte(`{"type":"future_type","payload":{"type":"x"}}`))
	if meta.RecordType != constant.TraceRolloutTypeUnknown {
		t.Fatalf("unknown envelope type must map to unknown record type, got %+v", meta)
	}
}
```

- [ ] **Step 2: 运行测试确认编译失败**

Run: `go test -count=1 ./internal/client/trace/`
Expected: FAIL — `undefined: LookupAdapter`

- [ ] **Step 3: 创建 `internal/client/trace/adapter.go`**

```go
package trace

import "github.com/hcd233/aris-proxy-api/internal/common/ierr"

// HookInfo 是各 agent hook stdin JSON 的归一化身份视图。
type HookInfo struct {
	SessionID      string
	EventName      string
	Model          string
	CWD            string
	SessionSource  string
	TurnID         string // codex: turn_id；claude: prompt_id
	CallID         string // 工具调用关联 ID（tool_use_id）
	TranscriptPath string
}

// TranscriptMeta 是单行 transcript/rollout 记录的归一化分类结果。
type TranscriptMeta struct {
	RecordType string
	Event      string
	TurnID     string
	CallID     string
}

// AgentAdapter 抹平不同 agent CLI 的 hook 与 transcript 格式差异。
// 新 agent 接入：实现本接口并在 init() 中 registerAdapter。
type AgentAdapter interface {
	Name() string
	// ParseHook 解析 hook stdin JSON；结构非法时返回 error（调用方 fail-open）。
	ParseHook(raw []byte) (HookInfo, error)
	// ClassifyTranscriptLine 分类单行 transcript；不返回 error，未知类型标记 unknown。
	ClassifyTranscriptLine(raw []byte) TranscriptMeta
	// StdoutAck 返回该 hook 事件需要回显的 stdout 内容；空串 = 静默。
	StdoutAck(info HookInfo) string
}

var agentAdapters = map[string]AgentAdapter{}

func registerAdapter(adapter AgentAdapter) {
	agentAdapters[adapter.Name()] = adapter
}

// LookupAdapter 按名称查找 adapter；未知名称返回 error（调用方写本地日志并 fail-open）。
func LookupAdapter(name string) (AgentAdapter, error) {
	if adapter, ok := agentAdapters[name]; ok {
		return adapter, nil
	}
	return nil, ierr.New(ierr.ErrValidation, "unknown trace agent")
}
```

- [ ] **Step 4: 创建 `internal/client/trace/codex.go`（从 ingest.go/rollout.go 搬移，逻辑零变化）**

```go
package trace

import (
	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func init() {
	registerAdapter(codexAdapter{})
}

type codexAdapter struct{}

type codexHookEnvelope struct {
	HookEventName  string `json:"hook_event_name"`
	SessionID      string `json:"session_id"`
	Model          string `json:"model,omitempty"`
	CWD            string `json:"cwd,omitempty"`
	Source         string `json:"source,omitempty"`
	TurnID         string `json:"turn_id,omitempty"`
	ToolUseID      string `json:"tool_use_id,omitempty"`
	TranscriptPath string `json:"transcript_path,omitempty"`
}

func (codexAdapter) Name() string { return constant.TraceAgentCodex }

func (codexAdapter) ParseHook(raw []byte) (HookInfo, error) {
	var env codexHookEnvelope
	if err := sonic.Unmarshal(raw, &env); err != nil {
		return HookInfo{}, err
	}
	return HookInfo{
		SessionID:      env.SessionID,
		EventName:      env.HookEventName,
		Model:          env.Model,
		CWD:            env.CWD,
		SessionSource:  env.Source,
		TurnID:         env.TurnID,
		CallID:         env.ToolUseID,
		TranscriptPath: env.TranscriptPath,
	}, nil
}

func (codexAdapter) StdoutAck(info HookInfo) string {
	if info.EventName == constant.TraceEventStop {
		return constant.EmptyJSONObject
	}
	return ""
}

type codexRolloutEnvelope struct {
	Type    string                 `json:"type"`
	Payload sonic.NoCopyRawMessage `json:"payload"`
}

type codexRolloutPayload struct {
	Type        string `json:"type"`
	TurnID      string `json:"turn_id,omitempty"`
	CallID      string `json:"call_id,omitempty"`
	Passthrough struct {
		TurnID string `json:"turn_id,omitempty"`
	} `json:"internal_chat_message_metadata_passthrough"`
}

func (p codexRolloutPayload) turnID() string {
	if p.TurnID != "" {
		return p.TurnID
	}
	return p.Passthrough.TurnID
}

func (codexAdapter) ClassifyTranscriptLine(raw []byte) TranscriptMeta {
	var envelope codexRolloutEnvelope
	if err := sonic.Unmarshal(raw, &envelope); err != nil {
		return TranscriptMeta{RecordType: constant.TraceRolloutTypeUnknown, Event: constant.TraceRolloutTypeUnknown}
	}
	var payload codexRolloutPayload
	if len(envelope.Payload) > 0 {
		_ = sonic.Unmarshal(envelope.Payload, &payload) //nolint:errcheck // best-effort field extraction
	}
	return TranscriptMeta{
		RecordType: codexRolloutRecordType(envelope.Type),
		Event:      payload.Type,
		TurnID:     payload.turnID(),
		CallID:     payload.CallID,
	}
}

func codexRolloutRecordType(recordType string) string {
	switch recordType {
	case constant.TraceRolloutTypeSessionMeta,
		constant.TraceRolloutTypeTurnContext,
		constant.TraceRolloutTypeResponseItem,
		constant.TraceRolloutTypeEventMsg:
		return recordType
	default:
		return constant.TraceRolloutTypeUnknown
	}
}
```

- [ ] **Step 5: 修改 `internal/client/trace/ingest.go` 接入 adapter + agent 传播**

删除 `hookEnvelope` 类型定义（已搬入 codex.go）。`ingestBatch` 加 `Agent` 字段；`Ingestor` 注入 adapter：

```go
type ingestBatch struct {
	SessionID string         `json:"session_id"`
	Agent     string         `json:"agent,omitempty"`
	Model     string         `json:"model,omitempty"`
	CWD       string         `json:"cwd,omitempty"`
	Source    string         `json:"source,omitempty"`
	Records   []ingestRecord `json:"records"`
}

type Ingestor struct {
	adapter AgentAdapter
	paths   Paths
	config  ConfigStore
	spool   *Spool
	rollout *RolloutReader
	client  *http.Client
}

type IngestCommandOptions struct {
	Paths      Paths
	In         io.Reader
	Out        io.Writer
	HTTPClient *http.Client
	AgentName  string
}

func NewIngestor(paths Paths, client *http.Client, adapter AgentAdapter) *Ingestor {
	if client == nil {
		client = &http.Client{Timeout: constant.TraceClientHTTPTimeout}
	} else if client.Timeout == 0 {
		clone := *client
		clone.Timeout = constant.TraceClientHTTPTimeout
		client = &clone
	}
	spool := NewSpool(paths, constant.TraceClientSpoolLimit)
	return &Ingestor{
		adapter: adapter,
		paths:   paths,
		config:  NewConfigStore(paths),
		spool:   spool,
		rollout: NewRolloutReader(paths, spool, adapter),
		client:  client,
	}
}

func (i *Ingestor) Ingest(ctx context.Context, raw []byte) error {
	info, err := i.adapter.ParseHook(raw)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode hook input")
	}
	if info.SessionID == "" || info.EventName == "" {
		return ierr.New(ierr.ErrValidation, "hook input missing identity")
	}
	spoolID, sequence, err := nextSequence(ctx, i.paths)
	if err != nil {
		return err
	}
	record := PendingRecord{
		SessionID:      info.SessionID,
		Agent:          i.adapter.Name(),
		Model:          info.Model,
		CWD:            info.CWD,
		SessionSource:  info.SessionSource,
		Source:         constant.TraceRecordSourceHook,
		RecordType:     constant.TraceRecordTypeHookEvent,
		Event:          info.EventName,
		TurnID:         info.TurnID,
		CallID:         info.CallID,
		ClientSequence: sequence,
		DedupKey:       fmt.Sprintf(constant.TraceClientHookDedupFormat, spoolID, sequence),
		Payload:        append(sonic.NoCopyRawMessage{}, raw...),
	}
	if err := i.spool.Append(ctx, record); err != nil {
		return err
	}
	if info.TranscriptPath != "" {
		if _, err := i.rollout.ReadNew(ctx, info.SessionID, info.TranscriptPath); err != nil {
			writeLocalError(i.paths, constant.TraceClientLogCategoryRollout)
		}
	}
	config, err := i.config.Load(ctx)
	if err != nil {
		return err
	}
	if config.Host == "" || config.APIKey == "" {
		return ierr.New(ierr.ErrValidation, "trace client is not initialized")
	}
	return i.flush(ctx, config)
}
```

`flush()` 中 request 构造加两行：

```go
	request := ingestBatch{
		SessionID: batch[0].SessionID,
		Agent:     batch[0].Agent,
		Records:   make([]ingestRecord, 0, len(batch)),
	}
```

`RunIngestCommand` 改为：

```go
func RunIngestCommand(ctx context.Context, opts IngestCommandOptions) error {
	paths := opts.Paths
	if paths.Root == "" {
		resolved, err := DefaultPaths()
		if err != nil {
			return nil //nolint:nilerr // fail-open: never block agent CLI
		}
		paths = resolved
	}
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	adapter, err := LookupAdapter(opts.AgentName)
	if err != nil {
		writeLocalError(paths, constant.TraceClientLogCategoryIngest)
		return nil //nolint:nilerr // fail-open: never block agent CLI
	}
	raw, err := io.ReadAll(io.LimitReader(in, constant.TraceClientHookInputLimit+1))
	if err != nil || len(raw) > constant.TraceClientHookInputLimit {
		writeLocalError(paths, constant.TraceClientLogCategoryIngest)
		return nil //nolint:nilerr // fail-open: never block agent CLI
	}
	if info, parseErr := adapter.ParseHook(raw); parseErr == nil {
		if ack := adapter.StdoutAck(info); ack != "" {
			_, _ = io.WriteString(out, ack) //nolint:errcheck // best-effort stdout
		}
	}
	if err := NewIngestor(paths, opts.HTTPClient, adapter).Ingest(ctx, raw); err != nil {
		writeLocalError(paths, constant.TraceClientLogCategoryIngest)
	} //nolint:nilerr // fail-open: never block agent CLI
	return nil
}
```

同时把文件里 `fail-open: never block Codex` 注释统一改为 `fail-open: never block agent CLI`（语义泛化）。

- [ ] **Step 6: 修改 `internal/client/trace/rollout.go` 分类走 adapter**

删除 `rolloutEnvelope`、`rolloutPayload`、`rolloutRecordType`、`rolloutRecord`（逻辑已在 codex.go）。`RolloutReader` 构造注入 adapter：

```go
type RolloutReader struct {
	paths   Paths
	spool   *Spool
	adapter AgentAdapter
}

func NewRolloutReader(paths Paths, spool *Spool, adapter AgentAdapter) *RolloutReader {
	return &RolloutReader{paths: paths, spool: spool, adapter: adapter}
}
```

`parseRolloutLines` 中 `rolloutRecord(sessionID, state.Line, raw)` 调用替换为方法 `r.rolloutRecord(sessionID, state.Line, raw)`：

```go
func (r *RolloutReader) rolloutRecord(sessionID string, line int64, raw []byte) (PendingRecord, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := sonic.Unmarshal(raw, &envelope); err != nil {
		return PendingRecord{}, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode transcript envelope")
	}
	meta := r.adapter.ClassifyTranscriptLine(raw)
	digest := sha256.Sum256(raw)
	lineCopy := line
	return PendingRecord{
		SessionID:      sessionID,
		Agent:          r.adapter.Name(),
		Source:         constant.TraceRecordSourceRollout,
		RecordType:     meta.RecordType,
		Event:          meta.Event,
		TurnID:         meta.TurnID,
		CallID:         meta.CallID,
		TranscriptLine: &lineCopy,
		DedupKey: fmt.Sprintf(
			constant.TraceClientRolloutDedupFormat,
			sessionID,
			line,
			hex.EncodeToString(digest[:]),
		),
		Payload: append(sonic.NoCopyRawMessage{}, raw...),
	}, nil
}
```

（envelope 解析只为校验 JSON 行结构合法；sonic.Valid 的过滤在调用前已有，保留该校验后 envelope 结构体可去掉——实现时若无其他用途，直接删掉 envelope 解析，只保留 `ClassifyTranscriptLine`。以最小代码为准。）

注释更新：`RolloutReader` 顶部注明其为通用 transcript 增量读取器（codex rollout / claude session 均适用）；错误消息 `inspect Codex rollout` 等改为 `inspect transcript`（连同 `open/seek/read Codex rollout` 一并泛化）。

- [ ] **Step 7: 修改 `internal/client/trace/spool.go` 与 `config.go`**

`PendingRecord` 加字段：

```go
	Agent         string                 `json:"agent,omitempty"`
```

`config.go`：删除 `Config.Agent` 字段（grep 已确认无使用方；旧 config.json 里的 `agent` 键 sonic 反序列化时自动忽略）。

- [ ] **Step 8: 修改 `cmd/client/trace.go` 加 `--agent` flag**

```go
func newTraceIngestCommand() *cobra.Command {
	var agentName string
	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Ingest one agent hook event",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return trace.RunIngestCommand(cmd.Context(), trace.IngestCommandOptions{
				In:        cmd.InOrStdin(),
				Out:       cmd.OutOrStdout(),
				AgentName: agentName,
			})
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "agent whose hook fired (codex|claude)")
	return cmd
}
```

- [ ] **Step 9: 更新 e2e hook 测试 `test/e2e/trace/hook_test.go`**

`runTraceIngest` 增加 agent 参数并在命令行带上：

```go
func runTraceIngest(t *testing.T, binary, home, agent, payload string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, "trace", "ingest", "--agent", agent)
	cmd.Env = append(os.Environ(), "HOME="+home)
	cmd.Stdin = strings.NewReader(payload)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("trace ingest: %v", err)
	}
	return string(output)
}
```

现有 `TestCodexHook_PersistsAndReportsAllEvents` 的调用点改为 `runTraceIngest(t, binary, home, "codex", payload)`。同时在该测试的 fake server decode 结构体上加 `Agent string \`json:"agent"\`` 并断言 `request.Agent == "codex"`：

```go
		var request struct {
			Agent   string `json:"agent"`
			Records []struct {
				Event    string `json:"hook_event_name"`
				DedupKey string `json:"dedup_key"`
			} `json:"records"`
		}
```

- [ ] **Step 10: 运行测试**

Run: `go build ./... && go test -count=1 ./internal/client/trace/ ./test/e2e/trace/`
Expected: PASS（codex 行为零回归）

- [ ] **Step 11: Conv lint + 提交**

Run: `go run ./cmd/server lint conv ./internal/client/trace/ ./cmd/client/ && go run ./cmd/server lint static ./internal/client/trace/ ./cmd/client/`
Expected: PASS

```bash
git add internal/client/trace cmd/client test/e2e/trace/hook_test.go
git commit -m "refactor(trace): 抽取客户端 AgentAdapter 抽象，codex 行为迁移（--agent 必填）"
```

---

### Task 2: claude 客户端 adapter

**Files:**
- Modify: `internal/common/constant/sql.go`（claude 常量）
- Create: `internal/client/trace/claude.go`
- Create: `internal/client/trace/claude_test.go`
- Modify: `test/e2e/trace/hook_test.go`（追加 claude 用例）

**Interfaces:**
- Consumes: Task 1 的 `AgentAdapter` / `registerAdapter` / `RegisterAdapter` 注册机制。
- Produces: `LookupAdapter("claude")` 可用；常量 `TraceClaudeRecordUser` / `TraceClaudeRecordAssistant` / `TraceClaudeRecordAttachment` / `TraceClaudeRecordSystem` / `TraceClaudeEventUserPrompt` / `TraceClaudeEventToolResult` / `TraceClaudeEventAssistantMessage` / `TraceEventSessionEnd` / `TraceEventPostToolUseFailure` / `TraceAgentClaude`。

- [ ] **Step 1: 写失败测试 `internal/client/trace/claude_test.go`**

```go
package trace

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestClaudeAdapter_ParseHook(t *testing.T) {
	t.Parallel()
	adapter, err := LookupAdapter(constant.TraceAgentClaude)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"hook_event_name":"PreToolUse","session_id":"abc123","prompt_id":"p-1","tool_use_id":"toolu_01","transcript_path":"/home/u/.claude/projects/-x/abc123.jsonl","cwd":"/repo"}`)
	info, err := adapter.ParseHook(raw)
	if err != nil {
		t.Fatalf("parse hook: %v", err)
	}
	if info.EventName != "PreToolUse" || info.SessionID != "abc123" ||
		info.TurnID != "p-1" || info.CallID != "toolu_01" ||
		info.TranscriptPath != "/home/u/.claude/projects/-x/abc123.jsonl" || info.CWD != "/repo" {
		t.Fatalf("unexpected hook info: %+v", info)
	}

	sessionStart := []byte(`{"hook_event_name":"SessionStart","session_id":"abc123","source":"startup","model":"claude-sonnet-5"}`)
	info, err = adapter.ParseHook(sessionStart)
	if err != nil {
		t.Fatalf("parse session start: %v", err)
	}
	if info.Model != "claude-sonnet-5" || info.SessionSource != "startup" {
		t.Fatalf("unexpected session start info: %+v", info)
	}
}

func TestClaudeAdapter_StdoutAckAlwaysSilent(t *testing.T) {
	t.Parallel()
	adapter, _ := LookupAdapter(constant.TraceAgentClaude)
	for _, event := range []string{constant.TraceEventStop, constant.TraceEventSessionStart, "UserPromptSubmit", "SessionEnd"} {
		if got := adapter.StdoutAck(HookInfo{EventName: event}); got != "" {
			t.Fatalf("claude %s ack = %q, must always be empty", event, got)
		}
	}
}

func TestClaudeAdapter_ClassifyTranscriptLine(t *testing.T) {
	t.Parallel()
	adapter, _ := LookupAdapter(constant.TraceAgentClaude)

	cases := []struct {
		name       string
		raw        string
		recordType string
		event      string
		callID     string
	}{
		{"user prompt", `{"type":"user","uuid":"u1","promptId":"p1","message":{"role":"user","content":"你好"}}`, constant.TraceClaudeRecordUser, constant.TraceClaudeEventUserPrompt, ""},
		{"tool result", `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_01","content":"ok"}]}}`, constant.TraceClaudeRecordUser, constant.TraceClaudeEventToolResult, "toolu_01"},
		{"assistant", `{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"..."},{"type":"tool_use","id":"toolu_02","name":"Bash","input":{"command":"ls"}}]}}`, constant.TraceClaudeRecordAssistant, constant.TraceClaudeEventAssistantMessage, "toolu_02"},
		{"attachment", `{"type":"attachment","attachment":{"type":"skill_listing","content":"..."}}`, constant.TraceClaudeRecordAttachment, constant.TraceClaudeRecordAttachment, ""},
		{"system subtype", `{"type":"system","subtype":"turn_duration","durationMs":100}`, constant.TraceClaudeRecordSystem, "turn_duration", ""},
		{"permission mode passthrough", `{"type":"permission-mode","permissionMode":"default"}`, "permission-mode", "permission-mode", ""},
		{"unknown", `{"type":"future_record","x":1}`, constant.TraceRolloutTypeUnknown, "future_record", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta := adapter.ClassifyTranscriptLine([]byte(tc.raw))
			if meta.RecordType != tc.recordType || meta.Event != tc.event || meta.CallID != tc.callID {
				t.Fatalf("got %+v, want type=%s event=%s call=%s", meta, tc.recordType, tc.event, tc.callID)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./internal/client/trace/`
Expected: FAIL — `constant.TraceAgentClaude` 等未定义

- [ ] **Step 3: 在 `internal/common/constant/sql.go` trace 段追加常量**

```go
	TraceAgentClaude = "claude"

	TraceEventSessionEnd         = "SessionEnd"
	TraceEventPostToolUseFailure = "PostToolUseFailure"

	TraceClaudeRecordUser       = "user"
	TraceClaudeRecordAssistant  = "assistant"
	TraceClaudeRecordAttachment = "attachment"
	TraceClaudeRecordSystem     = "system"

	TraceClaudeEventUserPrompt       = "user_prompt"
	TraceClaudeEventToolResult       = "tool_result"
	TraceClaudeEventAssistantMessage = "assistant_message"
```

- [ ] **Step 4: 创建 `internal/client/trace/claude.go`**

```go
package trace

import (
	"bytes"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func init() {
	registerAdapter(claudeAdapter{})
}

type claudeAdapter struct{}

type claudeHookEnvelope struct {
	HookEventName  string `json:"hook_event_name"`
	SessionID      string `json:"session_id"`
	Model          string `json:"model,omitempty"`
	CWD            string `json:"cwd,omitempty"`
	Source         string `json:"source,omitempty"`
	PromptID       string `json:"prompt_id,omitempty"`
	ToolUseID      string `json:"tool_use_id,omitempty"`
	TranscriptPath string `json:"transcript_path,omitempty"`
}

func (claudeAdapter) Name() string { return constant.TraceAgentClaude }

func (claudeAdapter) ParseHook(raw []byte) (HookInfo, error) {
	var env claudeHookEnvelope
	if err := sonic.Unmarshal(raw, &env); err != nil {
		return HookInfo{}, err
	}
	return HookInfo{
		SessionID:      env.SessionID,
		EventName:      env.HookEventName,
		Model:          env.Model,
		CWD:            env.CWD,
		SessionSource:  env.Source,
		TurnID:         env.PromptID,
		CallID:         env.ToolUseID,
		TranscriptPath: env.TranscriptPath,
	}, nil
}

// StdoutAck claude hook 的 stdout 会被注入上下文（SessionStart/UserPromptSubmit），必须恒静默。
func (claudeAdapter) StdoutAck(HookInfo) string { return "" }

type claudeTranscriptRecord struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Message *struct {
		Content sonic.NoCopyRawMessage `json:"content"`
	} `json:"message"`
}

type claudeBlockIdentity struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	ToolUseID string `json:"tool_use_id"`
}

// claude 记录级 type 原样透传的白名单；其余未识别 type 标记 unknown。
var claudePassthroughRecordTypes = map[string]bool{
	"permission-mode":        true,
	"file-history-snapshot":  true,
	"last-prompt":            true,
	"summary":                true,
	"progress":               true,
}

func (claudeAdapter) ClassifyTranscriptLine(raw []byte) TranscriptMeta {
	var record claudeTranscriptRecord
	if err := sonic.Unmarshal(raw, &record); err != nil {
		return TranscriptMeta{RecordType: constant.TraceRolloutTypeUnknown, Event: constant.TraceRolloutTypeUnknown}
	}
	switch record.Type {
	case constant.TraceClaudeRecordUser:
		return classifyClaudeUserRecord(record.Message)
	case constant.TraceClaudeRecordAssistant:
		return classifyClaudeAssistantRecord(record.Message)
	case constant.TraceClaudeRecordSystem:
		event := record.Subtype
		if event == "" {
			event = constant.TraceClaudeRecordSystem
		}
		return TranscriptMeta{RecordType: constant.TraceClaudeRecordSystem, Event: event}
	case constant.TraceClaudeRecordAttachment:
		return TranscriptMeta{
			RecordType: constant.TraceClaudeRecordAttachment,
			Event:      constant.TraceClaudeRecordAttachment,
		}
	case claudePassthrough(record.Type):
		return TranscriptMeta{RecordType: record.Type, Event: record.Type}
	default:
		return TranscriptMeta{RecordType: constant.TraceRolloutTypeUnknown, Event: record.Type}
	}
}

func claudePassthrough(recordType string) string {
	if claudePassthroughRecordTypes[recordType] {
		return recordType
	}
	return ""
}

// classifyClaudeUserRecord content 为字符串 → 真实用户输入；为数组 → 工具结果回传。
func classifyClaudeUserRecord(message *struct {
	Content sonic.NoCopyRawMessage `json:"content"`
}) TranscriptMeta {
	meta := TranscriptMeta{RecordType: constant.TraceClaudeRecordUser, Event: constant.TraceClaudeEventUserPrompt}
	if message == nil || len(message.Content) == 0 {
		return meta
	}
	if bytes.HasPrefix(bytes.TrimSpace(message.Content), []byte{'['}) {
		meta.Event = constant.TraceClaudeEventToolResult
		var blocks []claudeBlockIdentity
		if err := sonic.Unmarshal(message.Content, &blocks); err == nil {
			for _, block := range blocks {
				if block.ToolUseID != "" {
					meta.CallID = block.ToolUseID
					break
				}
			}
		}
	}
	return meta
}

func classifyClaudeAssistantRecord(message *struct {
	Content sonic.NoCopyRawMessage `json:"content"`
}) TranscriptMeta {
	meta := TranscriptMeta{
		RecordType: constant.TraceClaudeRecordAssistant,
		Event:      constant.TraceClaudeEventAssistantMessage,
	}
	if message == nil {
		return meta
	}
	var blocks []claudeBlockIdentity
	if err := sonic.Unmarshal(message.Content, &blocks); err == nil {
		for _, block := range blocks {
			if block.Type == "tool_use" && block.ID != "" {
				meta.CallID = block.ID
				break
			}
		}
	}
	return meta
}
```

注意：`classifyClaudeUserRecord` / `classifyClaudeAssistantRecord` 的参数类型匿名结构体无法跨函数书写——实现时把 `claudeTranscriptRecord.Message` 抽成命名类型：

```go
type claudeTranscriptMessage struct {
	Content sonic.NoCopyRawMessage `json:"content"`
}

type claudeTranscriptRecord struct {
	Type    string                 `json:"type"`
	Subtype string                 `json:"subtype"`
	Message *claudeTranscriptMessage `json:"message"`
}
```

两个 classify 函数签名相应改为 `(*claudeTranscriptMessage) TranscriptMeta`。

- [ ] **Step 5: 运行测试**

Run: `go test -count=1 ./internal/client/trace/`
Expected: PASS

- [ ] **Step 6: e2e 追加 claude hook 用例 `test/e2e/trace/hook_test.go`**

```go
func TestClaudeHook_SilentStdoutAndReportsAgent(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var agentEnvelopes []string
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Agent   string `json:"agent"`
			Records []struct {
				Source   string `json:"source"`
				Event    string `json:"hook_event_name"`
				DedupKey string `json:"dedup_key"`
			} `json:"records"`
		}
		if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		results := make([]client.RecordResult, 0, len(request.Records))
		mu.Lock()
		agentEnvelopes = append(agentEnvelopes, request.Agent)
		for _, record := range request.Records {
			if record.Source == "hook" {
				seen = append(seen, record.Event)
			}
			results = append(results, client.RecordResult{DedupKey: record.DedupKey, Status: "accepted"})
		}
		mu.Unlock()
		data, _ := sonic.Marshal(struct {
			Results []client.RecordResult `json:"results"`
		}{Results: results})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()

	home := t.TempDir()
	paths := client.Paths{Root: filepath.Join(home, ".aris")}
	if err := os.MkdirAll(paths.TraceDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	configJSON := `{"host":"` + server.URL + `","apiKey":"test-key"}`
	if err := os.WriteFile(paths.ConfigFile(), []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := buildTraceClient(t)

	transcript := filepath.Join(home, "session.jsonl")
	transcriptLines := []string{
		`{"type":"permission-mode","permissionMode":"default","sessionId":"claude-session"}`,
		`{"type":"user","uuid":"u1","promptId":"p1","sessionId":"claude-session","message":{"role":"user","content":"列一下文件"}}`,
		`{"type":"assistant","uuid":"a1","sessionId":"claude-session","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"user","uuid":"u2","sessionId":"claude-session","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file.go"}]}}`,
	}
	if err := os.WriteFile(transcript, []byte(strings.Join(transcriptLines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	hooks := []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "PostToolUseFailure", "Stop", "SessionEnd"}
	for _, event := range hooks {
		payload := `{"hook_event_name":"` + event + `","session_id":"claude-session","prompt_id":"p1","transcript_path":"` + transcript + `"}`
		if stdout := runTraceIngest(t, binary, home, "claude", payload); stdout != "" {
			t.Fatalf("claude %s stdout = %q, must always be empty", event, stdout)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(agentEnvelopes) == 0 {
		t.Fatal("no envelopes reported")
	}
	for _, agent := range agentEnvelopes {
		if agent != "claude" {
			t.Fatalf("batch agent = %q, want claude", agent)
		}
	}
	for _, event := range hooks {
		found := false
		for _, s := range seen {
			if s == event {
				found = true
			}
		}
		if !found {
			t.Errorf("hook event %s was not reported", event)
		}
	}
}
```

注意：claude 旧版无 `prompt_id` 时 hook payload 里 TurnID 为空，本用例始终带 `prompt_id`。

- [ ] **Step 7: 运行测试 + lint + 提交**

Run: `go build ./... && go test -count=1 ./internal/client/trace/ ./test/e2e/trace/`
Expected: PASS

Run: `go run ./cmd/server lint conv ./internal/client/trace/ && go run ./cmd/server lint static ./internal/client/trace/`
Expected: PASS

```bash
git add internal/common/constant/sql.go internal/client/trace test/e2e/trace/hook_test.go
git commit -m "feat(trace): claude hook 与 transcript adapter（客户端）"
```

---

### Task 3: 服务端 DTO + command agent 化，删除 legacy 单事件路径

**Files:**
- Modify: `internal/common/constant/sql.go`（done registry 需要的常量已有；无新增）
- Modify: `internal/dto/trace.go`（body 精简 + `agent` + records 必填）
- Modify: `internal/application/trace/port/handler.go`
- Modify: `internal/application/trace/command/report_trace_event.go`
- Modify: `internal/handler/trace.go`（`HandleReportTraceEvent`）
- Modify: `test/unit/trace/usecase_test.go`、`dto_convention_test.go`、`raw_json_schema_test.go`、`batch_result_test.go`
- Modify: `test/e2e/trace/trace_test.go`

**Interfaces:**
- Consumes: 无前置任务依赖（可与 Task 1/2 并行，但测试要过需要新 DTO）。
- Produces:
  ```go
  // port
  type ReportTraceEventCommand struct {
      SessionID, Agent, Model, CWD, Source, APIKeyName string
      UserID  uint
      Records []ReportTraceRecord
  }
  // dto.ReportTraceEventReqBody{ SessionID, Agent, Model, CWD, Source string; Records []*ReportTraceRecordReq }
  ```

- [ ] **Step 1: 写/改失败测试**

`test/unit/trace/usecase_test.go` 追加（既有用例全部补 `Agent: constant.TraceAgentCodex` 到 command 构造，或依赖默认——按现状它们构造 `port.ReportTraceEventCommand` 不传 Agent，应继续通过）：

```go
func TestReportTraceEvent_AgentDefaultsToCodex(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	h := command.NewReportTraceEventHandler(repo)
	_, err := h.Handle(context.Background(), port.ReportTraceEventCommand{
		SessionID: "s-agent-default",
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:x:1",
			Payload: []byte(`{"session_id":"s-agent-default"}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tr, _ := repo.FindBySessionID(context.Background(), "s-agent-default")
	if tr == nil || tr.Agent != constant.TraceAgentCodex {
		t.Fatalf("agent must default to codex, got %+v", tr)
	}
}

func TestReportTraceEvent_RejectsUnknownAgent(t *testing.T) {
	t.Parallel()
	h := command.NewReportTraceEventHandler(NewFakeRepo())
	_, err := h.Handle(context.Background(), port.ReportTraceEventCommand{
		SessionID: "s-bad-agent",
		Agent:     "gemini",
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:x:2",
			Payload: []byte(`{"session_id":"s-bad-agent"}`),
		}},
	})
	if err == nil {
		t.Fatal("unknown agent must be rejected")
	}
}

func TestReportTraceEvent_RejectsAgentMismatch(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	h := command.NewReportTraceEventHandler(repo)
	first := port.ReportTraceEventCommand{
		SessionID: "s-mismatch", Agent: constant.TraceAgentCodex,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:m:1", Payload: []byte(`{"session_id":"s-mismatch"}`),
		}},
	}
	if _, err := h.Handle(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.Agent = constant.TraceAgentClaude
	second.Records[0].DedupKey = "hook:m:2"
	if _, err := h.Handle(context.Background(), second); err == nil {
		t.Fatal("agent mismatch on existing session must be rejected")
	}
}

func TestReportTraceEvent_ClaudeDoneOnSessionEndNotStop(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	h := command.NewReportTraceEventHandler(repo)
	report := func(event, dedup string) {
		t.Helper()
		_, err := h.Handle(context.Background(), port.ReportTraceEventCommand{
			SessionID: "s-claude-done", Agent: constant.TraceAgentClaude,
			Records: []port.ReportTraceRecord{{
				Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
				HookEventName: event, Event: event, DedupKey: dedup,
				Payload: []byte(`{"session_id":"s-claude-done"}`),
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	report("Stop", "hook:c:1")
	tr, _ := repo.FindBySessionID(context.Background(), "s-claude-done")
	if tr == nil || tr.Status != constant.TraceStatusActive {
		t.Fatalf("claude Stop must not finish trace, got %+v", tr)
	}
	report("SessionEnd", "hook:c:2")
	tr, _ = repo.FindBySessionID(context.Background(), "s-claude-done")
	if tr.Status != constant.TraceStatusDone {
		t.Fatalf("claude SessionEnd must finish trace, got %+v", tr)
	}
}
```

`test/e2e/trace/trace_test.go` 的 `TestE2E_TraceReportFlow` 改为 batch 上报（legacy 单事件路径已删）：

```go
	body := &dto.ReportTraceEventReqBody{
		SessionID: "e2e-s1",
		Agent:     "claude",
		Records: []*dto.ReportTraceRecordReq{{
			Source:        constant.TraceRecordSourceHook,
			RecordType:    constant.TraceRecordTypeHookEvent,
			HookEventName: "UserPromptSubmit",
			DedupKey:      "hook:e2e:1",
			Payload:       traceschema.RawJSON(`{"hook_event_name":"UserPromptSubmit","session_id":"e2e-s1","prompt":"hello"}`),
		}},
	}
```

并断言 `tr.Agent == "claude"`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/trace/ ./test/e2e/trace/`
Expected: FAIL — DTO 字段 / `Agent` 未定义

- [ ] **Step 3: 重写 `internal/dto/trace.go` 的上报部分**

`ReportTraceEventReq` / `ReportTraceEventReqBody` 替换为：

```go
// ReportTraceEventReq 上报请求（API Key 鉴权，agent hook 批量记录）
type ReportTraceEventReq struct {
	Body *ReportTraceEventReqBody `json:"body" doc:"agent trace 批量记录"`
}

// ReportTraceEventReqBody 批量上报 envelope。原始内容一律放在 records[i].payload，
// envelope 只承担索引与归属字段。
type ReportTraceEventReqBody struct {
	Records   []*ReportTraceRecordReq `json:"records" required:"true" minItems:"1" doc:"批量原始记录"`
	SessionID string                  `json:"session_id" required:"true" minLength:"1" doc:"agent session id"`
	Agent     string                  `json:"agent,omitempty" enum:"codex,claude" doc:"agent 类型（默认 codex）"`
	Model     string                  `json:"model,omitempty" doc:"模型"`
	CWD       string                  `json:"cwd,omitempty" doc:"工作目录"`
	Source    string                  `json:"source,omitempty" doc:"startup/resume/clear/compact"`
}
```

删除：`Raw` 字段、`UnmarshalJSON` 自定义方法、全部 legacy 单事件字段（`HookEventName`/`TurnID`/`TranscriptPath`/`PermissionMode`/`Prompt`/`ToolName`/`ToolUseID`/`ToolInput`/`ToolResponse`/`LastAssistantMessage`/`AgentID`/`AgentType`/`Trigger`）以及 `_ struct{} additionalProperties` 行。`ReportTraceRecordReq` 不动。注释里 `codex hook stdin 输入` 改为 `agent trace 批量记录`。文件顶部 `traceschema` import 保留（`ReportTraceRecordReq.Payload` 仍用）。

- [ ] **Step 4: 修改 port 与 command**

`internal/application/trace/port/handler.go` 的 `ReportTraceEventCommand` 替换为：

```go
// ReportTraceEventCommand 上报事件命令
type ReportTraceEventCommand struct {
	SessionID  string
	Agent      string
	Model      string
	CWD        string
	Source     string
	APIKeyName string
	UserID     uint
	Records    []ReportTraceRecord
}
```

（删除 `HookEventName` / `TurnID` / `RawPayload`。）

`internal/application/trace/command/report_trace_event.go` 重写核心：

```go
var traceDoneEvents = map[string][]string{
	constant.TraceAgentCodex:  {constant.TraceEventStop, constant.TraceEventTaskComplete},
	constant.TraceAgentClaude: {constant.TraceEventSessionEnd},
}

func (h *reportTraceEventHandler) Handle(
	ctx context.Context,
	cmd port.ReportTraceEventCommand,
) ([]port.ReportTraceRecordResult, error) {
	if cmd.SessionID == "" {
		return nil, ierr.New(ierr.ErrValidation, "hook payload missing session_id")
	}
	agent := cmd.Agent
	if agent == "" {
		agent = constant.TraceAgentCodex
	}
	doneEvents, ok := traceDoneEvents[agent]
	if !ok {
		return nil, ierr.New(ierr.ErrValidation, "unknown trace agent")
	}
	if len(cmd.Records) == 0 {
		return nil, ierr.New(ierr.ErrValidation, "empty trace records")
	}
	records := normalizeRecords(cmd)
	t, err := h.ensureTrace(ctx, cmd, agent)
	if err != nil {
		return nil, err
	}
	results, isComplete := insertRecords(
		ctx, h.repo, t.ID, cmd.SessionID, records, true, doneEvents,
	)
	if isComplete {
		if err := h.repo.MarkDone(ctx, cmd.SessionID); err != nil {
			return results, err
		}
	}
	return results, nil
}

func normalizeRecords(cmd port.ReportTraceEventCommand) []port.ReportTraceRecord {
	records := cmd.Records
	for i := range records {
		record := &records[i]
		if record.Source == "" {
			record.Source = constant.TraceRecordSourceHook
		}
		if record.RecordType == "" {
			record.RecordType = constant.TraceRecordTypeHookEvent
		}
		if record.Event == "" {
			record.Event = record.HookEventName
		}
	}
	return records
}
```

`ensureTrace` 接收 agent 并校验冲突：

```go
func (h *reportTraceEventHandler) ensureTrace(
	ctx context.Context,
	cmd port.ReportTraceEventCommand,
	agent string,
) (*trace.Trace, error) {
	t, err := h.repo.FindBySessionID(ctx, cmd.SessionID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return h.repo.UpsertBySessionID(ctx, &trace.Trace{
			Agent:      agent,
			SessionID:  cmd.SessionID,
			APIKeyName: cmd.APIKeyName,
			UserID:     cmd.UserID,
			Model:      cmd.Model,
			CWD:        cmd.CWD,
			Source:     cmd.Source,
			Status:     constant.TraceStatusActive,
		})
	}
	if t.Agent != "" && t.Agent != agent {
		return nil, ierr.New(ierr.ErrValidation, "trace agent mismatch for session")
	}
	modelName := t.Model
	if cmd.Model != "" {
		modelName = cmd.Model
	}
	cwd := t.CWD
	if cmd.CWD != "" {
		cwd = cmd.CWD
	}
	source := t.Source
	if cmd.Source != "" {
		source = cmd.Source
	}
	return h.repo.UpsertBySessionID(ctx, &trace.Trace{
		ID:         t.ID,
		Agent:      agent,
		SessionID:  cmd.SessionID,
		APIKeyName: t.APIKeyName,
		UserID:     t.UserID,
		Model:      modelName,
		CWD:        cwd,
		Source:     source,
		Status:     t.Status,
		Metadata:   t.Metadata,
	})
}
```

`insertRecords` / `completeEvent` 改为 registry 驱动；`validRecord` 中 hook/rollout source 白名单、`requireDedupKey` 恒 true（删掉参数与 legacy 分支）：

```go
func insertRecords(
	ctx context.Context,
	repo trace.TraceRepository,
	traceID uint,
	sessionID string,
	records []port.ReportTraceRecord,
	doneEvents []string,
) ([]port.ReportTraceRecordResult, bool) {
	results := make([]port.ReportTraceRecordResult, 0, len(records))
	isComplete := false
	for _, record := range records {
		result := port.ReportTraceRecordResult{DedupKey: record.DedupKey}
		if !validRecord(record) {
			result.Status = constant.TraceRecordStatusRejected
			result.Message = constant.TraceRecordMessageInvalid
			results = append(results, result)
			continue
		}
		inserted, err := repo.InsertEvent(ctx, &trace.TraceEvent{
			TraceID:        traceID,
			SessionID:      sessionID,
			Source:         record.Source,
			RecordType:     record.RecordType,
			Event:          record.Event,
			TurnID:         record.TurnID,
			CallID:         record.CallID,
			TranscriptLine: record.TranscriptLine,
			ClientSequence: record.ClientSequence,
			DedupKey:       record.DedupKey,
			Payload:        record.Payload,
		})
		switch {
		case err != nil:
			result.Status = constant.TraceRecordStatusRejected
			result.Message = constant.TraceRecordMessageStorageFailed
		case !inserted:
			result.Status = constant.TraceRecordStatusDuplicate
		default:
			result.Status = constant.TraceRecordStatusAccepted
		}
		results = append(results, result)
		if result.Status != constant.TraceRecordStatusRejected && lo.Contains(doneEvents, record.Event) {
			isComplete = true
		}
	}
	return results, isComplete
}

func validRecord(record port.ReportTraceRecord) bool {
	isSourceValid := record.Source == constant.TraceRecordSourceHook ||
		record.Source == constant.TraceRecordSourceRollout
	if !isSourceValid || record.RecordType == "" || len(record.Payload) == 0 {
		return false
	}
	return record.DedupKey != ""
}
```

（import 增加 `github.com/samber/lo`。）

- [ ] **Step 5: 修改 `internal/handler/trace.go` 的 `HandleReportTraceEvent`**

替换整个函数体为 batch-only：

```go
// HandleReportTraceEvent 上报 agent 批量记录（API Key 鉴权）
func (h *traceHandler) HandleReportTraceEvent(
	ctx context.Context,
	req *dto.ReportTraceEventReq,
) (*dto.HTTPResponse[*dto.ReportTraceEventRsp], error) {
	rsp := &dto.ReportTraceEventRsp{}
	if req.Body == nil || len(req.Body.Records) == 0 {
		rsp.Error = ierr.ErrValidation.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	cmd := port.ReportTraceEventCommand{
		SessionID:  req.Body.SessionID,
		Agent:      req.Body.Agent,
		Model:      req.Body.Model,
		CWD:        req.Body.CWD,
		Source:     req.Body.Source,
		APIKeyName: util.CtxValueString(ctx, constant.CtxKeyAPIKeyName),
		UserID:     util.CtxValueUint(ctx, constant.CtxKeyUserID),
		Records: lo.Map(req.Body.Records, func(
			record *dto.ReportTraceRecordReq,
			_ int,
		) port.ReportTraceRecord {
			return port.ReportTraceRecord{
				Source:         record.Source,
				RecordType:     record.RecordType,
				HookEventName:  record.HookEventName,
				Event:          record.Event,
				TurnID:         record.TurnID,
				CallID:         record.CallID,
				TranscriptLine: record.TranscriptLine,
				ClientSequence: record.ClientSequence,
				DedupKey:       record.DedupKey,
				Payload:        record.Payload,
			}
		}),
	}

	results, err := h.report.Handle(ctx, cmd)
	if err != nil {
		logger.WithCtx(ctx).Error("[TraceHandler] Report event failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.Results = lo.Map(results, func(
		result port.ReportTraceRecordResult,
		_ int,
	) *dto.ReportTraceRecordResult {
		return &dto.ReportTraceRecordResult{
			DedupKey: result.DedupKey,
			Status:   result.Status,
			Message:  result.Message,
		}
	})
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

- [ ] **Step 6: 更新 DTO 规范测试**

`test/unit/trace/dto_convention_test.go`：
- `TestReportTraceEventReq_DTOFollowsHumaBodyConvention`：删除 `HookEventName` 字段断言，改为断言 `Records` 字段存在且 json tag 为 `"records"`、`Agent` 字段 json tag 为 `"agent,omitempty"`。
- 删除 `TestReportTraceEventReqBody_MarshalPreservesDynamicFields`（tool_input 等字段已随 legacy 删除）。
- `TestReportTraceEventReq_PreservesUnknownRawRecordFields` 保留（只依赖 Records/Payload，不受影响）。

`test/unit/trace/raw_json_schema_test.go`：删 `TestReportTraceEventReqBody_HumaSchemaAcceptsDynamicJSON`，替换为 schema 校验 records/agent：

```go
func TestReportTraceEventReqBody_HumaSchema(t *testing.T) {
	t.Parallel()
	registry := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	schema := huma.SchemaFromType(registry, reflect.TypeOf(dto.ReportTraceEventReqBody{}))
	if schema.Properties["records"] == nil {
		t.Fatal("request body must expose records")
	}
	agent := schema.Properties["agent"]
	if agent == nil {
		t.Fatal("request body must expose agent")
	}
	found := false
	for _, v := range agent.Enum {
		if v == "claude" {
			found = true
		}
	}
	if !found {
		t.Fatalf("agent enum must contain claude: %+v", agent.Enum)
	}
}
```

- [ ] **Step 7: 运行测试**

Run: `go build ./... && go test -count=1 ./test/unit/trace/ ./test/e2e/trace/ ./internal/application/trace/... ./internal/handler/`
Expected: PASS

- [ ] **Step 8: lint + 提交**

Run: `go run ./cmd/server lint conv ./internal/dto/ ./internal/application/trace/ ./internal/handler/ ./test/unit/trace/ ./test/e2e/trace/ && go run ./cmd/server lint static ./internal/dto/ ./internal/application/trace/ ./internal/handler/`
Expected: PASS

```bash
git add internal/dto/trace.go internal/application/trace internal/handler/trace.go test/unit/trace test/e2e/trace/trace_test.go
git commit -m "refactor(trace): 服务端上报 agent 化，删除 legacy 单事件路径"
```

---

### Task 4: 对话投影分发 + claude 投影

**Files:**
- Create: `internal/domain/trace/conversation.go`（共享类型 + helper）
- Create: `internal/domain/trace/projector.go`（registry + `BuildConversationFor`）
- Create: `internal/domain/trace/claude.go`（claude 投影）
- Modify: `internal/domain/trace/rollout.go`（`BuildConversation`→`buildCodexConversation`，共享件搬走）
- Modify: `internal/application/trace/query/conversation.go`（按 agent 分发）
- Modify: `test/unit/trace/conversation_test.go`（`BuildConversation` → `BuildConversationFor`）
- Create: `test/unit/trace/claude_conversation_test.go`

**Interfaces:**
- Consumes: Task 3 的常量与 `Trace.Agent`。
- Produces:
  ```go
  func BuildConversationFor(agent string, events []*TraceEvent) (*Conversation, error)
  ```

- [ ] **Step 1: 写失败测试 `test/unit/trace/claude_conversation_test.go`**

```go
package trace

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func claudeRecords() []*trace.TraceEvent {
	return []*trace.TraceEvent{
		{ID: 1, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "UserPromptSubmit", TurnID: "p1", Payload: []byte(`{"hook_event_name":"UserPromptSubmit","prompt_id":"p1","prompt":"列一下文件"}`)},
		{ID: 2, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordUser, Event: constant.TraceClaudeEventUserPrompt, Payload: []byte(`{"type":"user","uuid":"u1","promptId":"p1","message":{"role":"user","content":"列一下文件"}}`)},
		{ID: 3, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordAssistant, Event: constant.TraceClaudeEventAssistantMessage, Payload: []byte(`{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"thinking","thinking":"看一下"},{"type":"text","text":"我来列一下"},{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]}}`)},
		{ID: 4, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordUser, Event: constant.TraceClaudeEventToolResult, CallID: "toolu_1", Payload: []byte(`{"type":"user","uuid":"u2","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file.go"}]}}`)},
		{ID: 5, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordAssistant, Event: constant.TraceClaudeEventAssistantMessage, Payload: []byte(`{"type":"assistant","uuid":"a2","message":{"role":"assistant","content":[{"type":"text","text":"目录下只有 file.go"}]}}`)},
		{ID: 6, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "Stop", TurnID: "p1", Payload: []byte(`{"hook_event_name":"Stop","prompt_id":"p1","last_assistant_message":"目录下只有 file.go"}`)},
		{ID: 7, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordUser, Event: constant.TraceClaudeEventUserPrompt, Payload: []byte(`{"type":"user","uuid":"u3","promptId":"p2","isSidechain":false,"message":{"role":"user","content":"第二个问题"}}`)},
		{ID: 8, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordAssistant, Event: constant.TraceClaudeEventAssistantMessage, Payload: []byte(`{"type":"assistant","uuid":"a3","isSidechain":true,"message":{"role":"assistant","content":[{"type":"text","text":"子代理输出"}]}}`)},
		{ID: 9, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "PreToolUse", TurnID: "p1", Payload: []byte(`{"hook_event_name":"PreToolUse","prompt_id":"p1","agent_id":"sub-1","tool_use_id":"toolu_sub","tool_name":"Bash","tool_input":{"command":"pwd"}}`)},
	}
}

func TestBuildClaudeConversation_TurnsMessagesAndTools(t *testing.T) {
	t.Parallel()
	conversation, err := trace.BuildConversationFor(constant.TraceAgentClaude, claudeRecords())
	if err != nil {
		t.Fatal(err)
	}
	if len(conversation.Turns) != 2 {
		t.Fatalf("expected 2 turns, got %d: %+v", len(conversation.Turns), conversation.Turns)
	}
	turn := conversation.Turns[0]
	if turn.TurnID != "u1" {
		t.Fatalf("turn key must be transcript user uuid, got %q", turn.TurnID)
	}
	var userMsgs, assistantMsgs, toolCalls int
	var tool *trace.ConversationItem
	for _, item := range turn.Items {
		switch {
		case item.Kind == constant.TraceConversationKindMessage && item.Role == constant.TraceConversationRoleUser:
			userMsgs++
		case item.Kind == constant.TraceConversationKindMessage && item.Role == constant.TraceConversationRoleAssistant:
			assistantMsgs++
		case item.Kind == constant.TraceConversationKindToolCall:
			toolCalls++
			tool = item
		}
		if item.Content == "看一下" {
			t.Fatal("thinking block must be skipped")
		}
	}
	if userMsgs != 1 || assistantMsgs != 2 || toolCalls != 1 {
		t.Fatalf("user=%d assistant=%d tools=%d, want 1/2/1", userMsgs, assistantMsgs, toolCalls)
	}
	if tool == nil || tool.CallID != "toolu_1" || tool.ToolName != "Bash" {
		t.Fatalf("unexpected tool call: %+v", tool)
	}
	if tool.Output != "file.go" {
		t.Fatalf("tool output = %q, want file.go", tool.Output)
	}
	if tool.Arguments != `{"command":"ls"}` {
		t.Fatalf("tool arguments = %q", tool.Arguments)
	}
}

func TestBuildClaudeConversation_SkipsSidechainAndSubagentHooks(t *testing.T) {
	t.Parallel()
	conversation, err := trace.BuildConversationFor(constant.TraceAgentClaude, claudeRecords())
	if err != nil {
		t.Fatal(err)
	}
	for _, turn := range conversation.Turns {
		for _, item := range turn.Items {
			if item.Content == "子代理输出" || item.CallID == "toolu_sub" {
				t.Fatalf("subagent content must not enter projection: %+v", item)
			}
		}
	}
	if conversation.Turns[1].TurnID != "u3" {
		t.Fatalf("second turn key = %q, want u3", conversation.Turns[1].TurnID)
	}
}

func TestBuildClaudeConversation_HookFallbackWithoutTranscript(t *testing.T) {
	t.Parallel()
	records := []*trace.TraceEvent{
		{ID: 1, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "UserPromptSubmit", TurnID: "p9", Payload: []byte(`{"hook_event_name":"UserPromptSubmit","prompt_id":"p9","prompt":"hi"}`)},
		{ID: 2, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "PreToolUse", TurnID: "p9", Payload: []byte(`{"hook_event_name":"PreToolUse","prompt_id":"p9","tool_name":"Bash","tool_use_id":"toolu_9","tool_input":{"command":"ls"}}`)},
		{ID: 3, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "PostToolUse", TurnID: "p9", Payload: []byte(`{"hook_event_name":"PostToolUse","prompt_id":"p9","tool_use_id":"toolu_9","tool_response":{"stdout":"ok"}}`)},
	}
	conversation, err := trace.BuildConversationFor(constant.TraceAgentClaude, records)
	if err != nil {
		t.Fatal(err)
	}
	if len(conversation.Turns) != 1 || len(conversation.Turns[0].Items) != 2 {
		t.Fatalf("unexpected conversation: %+v", conversation)
	}
	var tool *trace.ConversationItem
	for _, item := range conversation.Turns[0].Items {
		if item.Kind == constant.TraceConversationKindToolCall {
			tool = item
		}
	}
	if tool == nil || tool.Output == "" {
		t.Fatalf("hook tool output must be paired: %+v", tool)
	}
}

func TestBuildConversationFor_UnknownAgent(t *testing.T) {
	t.Parallel()
	if _, err := trace.BuildConversationFor("gemini", nil); err == nil {
		t.Fatal("unknown agent must return error")
	}
}
```

同时把 `test/unit/trace/conversation_test.go` 中所有 `trace.BuildConversation(` 调用替换为：

```go
conversation, err := trace.BuildConversationFor(constant.TraceAgentCodex, records)
if err != nil { t.Fatal(err) }
```

（逐一替换原 `conversation := trace.BuildConversation(records)` 的调用形态。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/trace/`
Expected: FAIL — `BuildConversationFor` 未定义

- [ ] **Step 3: 拆分共享件到 `internal/domain/trace/conversation.go`**

从 `rollout.go` 原样搬移（不改逻辑）：`Conversation` / `ConversationTurn` / `ConversationItem` 类型、`appendToTurn`、`dedupeMessage`、`dedupeToolCall`、`isRolloutUpgrade`、`pairToolOutput`、`hookString`、`hookRaw`。文件头注释 `// 对话投影共享结构：类型、turn 归组与去重 helper`。

`rollout.go` 中 `BuildConversation` 改名为 `buildCodexConversation`（包内私有），`projectConversationItem` 等 codex 专有逻辑原地保留；`RolloutRecord` / `ParseRolloutRecord` 不动。

- [ ] **Step 4: 创建 `internal/domain/trace/projector.go`**

```go
package trace

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// conversationBuilders 按 agent 注册的对话投影构造器。新 agent 接入在此登记。
var conversationBuilders = map[string]func([]*TraceEvent) *Conversation{
	constant.TraceAgentCodex:  buildCodexConversation,
	constant.TraceAgentClaude: buildClaudeConversation,
}

// BuildConversationFor 按 agent 分发对话投影构造。
func BuildConversationFor(agent string, events []*TraceEvent) (*Conversation, error) {
	builder, ok := conversationBuilders[agent]
	if !ok {
		return nil, ierr.New(ierr.ErrValidation, "unknown trace agent")
	}
	return builder(events), nil
}
```

- [ ] **Step 5: 创建 `internal/domain/trace/claude.go`**

```go
package trace

import (
	"bytes"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// claudeTranscriptPayload 仅抽取投影所需字段；未知字段原样留在 TraceEvent.Payload。
type claudeTranscriptPayload struct {
	UUID        string `json:"uuid"`
	PromptID    string `json:"promptId"`
	IsSidechain bool   `json:"isSidechain"`
	Message     *struct {
		Role    string                 `json:"role"`
		Content sonic.NoCopyRawMessage `json:"content"`
	} `json:"message"`
}

type claudeContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text"`
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Input     sonic.NoCopyRawMessage `json:"input"`
	ToolUseID string                 `json:"tool_use_id"`
	Content   sonic.NoCopyRawMessage `json:"content"`
}

// buildClaudeConversation 从 Claude transcript + hook 记录投影对话视图。
// turn 归组：transcript 真实输入记录的 promptId→uuid 建 alias；hook 事件按 prompt_id 归并。
func buildClaudeConversation(records []*TraceEvent) *Conversation {
	alias := map[string]string{}
	for _, record := range records {
		if record.Source != constant.TraceRecordSourceRollout ||
			record.Event != constant.TraceClaudeEventUserPrompt {
			continue
		}
		var payload claudeTranscriptPayload
		if sonic.Unmarshal(record.Payload, &payload) == nil &&
			payload.PromptID != "" && payload.UUID != "" {
			alias[payload.PromptID] = payload.UUID
		}
	}

	conversation := &Conversation{Turns: []*ConversationTurn{}}
	turns := map[string]*ConversationTurn{}
	seenMessages := map[string]*ConversationItem{}
	tools := map[string]*ConversationItem{}
	currentTurn := const_claudeSessionTurn
	for _, record := range records {
		if claudeSidechain(record) || claudeSubagentHook(record) {
			continue
		}
		turnID := resolveClaudeTurn(record, alias, currentTurn)
		items := projectClaudeItems(record, tools)
		for _, item := range items {
			if item == nil {
				continue
			}
			if item.Kind == constant.TraceConversationKindMessage && dedupeMessage(seenMessages, item) {
				continue
			}
			if item.Kind == constant.TraceConversationKindToolCall && item.CallID != "" && dedupeToolCall(tools, item) {
				continue
			}
			appendToTurn(conversation, turns, turnID, item)
		}
		if isClaudeTurnOpener(record) {
			currentTurn = turnID
		}
	}
	return conversation
}

const const_claudeSessionTurn = constant.TraceConversationDefaultTurn
```

实现注意（写码时直接落实，不留二义）：
1. 不要把 `const` 起名为 `const_claudeSessionTurn`（违反命名规范）；直接使用 `constant.TraceConversationDefaultTurn`。
2. `resolveClaudeTurn`：

```go
func resolveClaudeTurn(record *TraceEvent, alias map[string]string, currentTurn string) string {
	if isClaudeTurnOpener(record) {
		var payload claudeTranscriptPayload
		if sonic.Unmarshal(record.Payload, &payload) == nil && payload.UUID != "" {
			return payload.UUID
		}
	}
	if aliasID, ok := alias[record.TurnID]; ok && record.TurnID != "" {
		return aliasID
	}
	if record.TurnID != "" {
		return "prompt:" + record.TurnID // transcript 行未入库时的稳定兜底
	}
	return currentTurn
}

func isClaudeTurnOpener(record *TraceEvent) bool {
	return record.Source == constant.TraceRecordSourceRollout &&
		record.Event == constant.TraceClaudeEventUserPrompt
}
```

3. `claudeSidechain` / `claudeSubagentHook`：

```go
func claudeSidechain(record *TraceEvent) bool {
	if record.Source != constant.TraceRecordSourceRollout {
		return false
	}
	var payload struct {
		IsSidechain bool `json:"isSidechain"`
	}
	return sonic.Unmarshal(record.Payload, &payload) == nil && payload.IsSidechain
}

// claudeSubagentHook 子代理内触发的 hook（payload 携带 agent_id）不进主对话投影。
func claudeSubagentHook(record *TraceEvent) bool {
	if record.Source != constant.TraceRecordSourceHook {
		return false
	}
	return hookString(record, "agent_id") != ""
}
```

4. `projectClaudeItems`（一条 record 可产出多个 item：assistant 记录内的 text + 多个 tool_use）：

```go
func projectClaudeItems(record *TraceEvent, tools map[string]*ConversationItem) []*ConversationItem {
	switch record.Event {
	case constant.TraceClaudeEventUserPrompt:
		return claudeUserPromptItems(record)
	case constant.TraceClaudeEventToolResult:
		claudePairToolResults(record, tools)
		return nil
	case constant.TraceClaudeEventAssistantMessage:
		return claudeAssistantItems(record)
	case constant.TraceConversationEventUserPrompt: // hook UserPromptSubmit
		return []*ConversationItem{hookMessage(record, constant.TraceConversationRoleUser, constant.TracePayloadFieldPrompt)}
	case constant.TraceConversationEventStop: // hook Stop
		return []*ConversationItem{hookMessage(record, constant.TraceConversationRoleAssistant, constant.TracePayloadFieldLastMessage)}
	case constant.TraceConversationEventPreToolUse:
		return []*ConversationItem{hookToolCall(record)}
	case constant.TraceConversationEventPostToolUse:
		pairToolOutput(tools, hookCallID(record), record, hookRaw(record, constant.TracePayloadFieldToolResponse))
		return nil
	default:
		return nil
	}
}

func claudeUserPromptItems(record *TraceEvent) []*ConversationItem {
	var payload claudeTranscriptPayload
	if sonic.Unmarshal(record.Payload, &payload) != nil || payload.Message == nil {
		return nil
	}
	var text string
	if sonic.Unmarshal(payload.Message.Content, &text) != nil || text == "" {
		return nil
	}
	return []*ConversationItem{{
		Kind:      constant.TraceConversationKindMessage,
		Role:      constant.TraceConversationRoleUser,
		Content:   text,
		Source:    record.Source,
		RecordIDs: []uint{record.ID},
	}}
}

func claudeAssistantItems(record *TraceEvent) []*ConversationItem {
	var payload claudeTranscriptPayload
	if sonic.Unmarshal(record.Payload, &payload) != nil || payload.Message == nil {
		return nil
	}
	var blocks []claudeContentBlock
	if sonic.Unmarshal(payload.Message.Content, &blocks) != nil {
		return nil
	}
	items := []*ConversationItem{}
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text == "" {
				continue
			}
			items = append(items, &ConversationItem{
				Kind:      constant.TraceConversationKindMessage,
				Role:      constant.TraceConversationRoleAssistant,
				Content:   block.Text,
				Source:    record.Source,
				RecordIDs: []uint{record.ID},
			})
		case "tool_use":
			arguments := ""
			if len(block.Input) > 0 {
				arguments = string(bytes.TrimSpace(block.Input))
			}
			items = append(items, &ConversationItem{
				Kind:      constant.TraceConversationKindToolCall,
				Role:      constant.TraceConversationRoleAssistant,
				ToolName:  block.Name,
				CallID:    block.ID,
				Arguments: arguments,
				Source:    record.Source,
				RecordIDs: []uint{record.ID},
			})
		default: // thinking / 其他块：投影 v1 跳过
		}
	}
	return items
}

// claudePairToolResults 把 user 记录内的 tool_result 块逐块回填到既有 tool_call。
func claudePairToolResults(record *TraceEvent, tools map[string]*ConversationItem) {
	var payload claudeTranscriptPayload
	if sonic.Unmarshal(record.Payload, &payload) != nil || payload.Message == nil {
		return
	}
	var blocks []claudeContentBlock
	if sonic.Unmarshal(payload.Message.Content, &blocks) != nil {
		return
	}
	for _, block := range blocks {
		if block.Type != "tool_result" || block.ToolUseID == "" {
			continue
		}
		pairToolOutput(tools, block.ToolUseID, record, claudeToolResultText(block.Content))
	}
}

// claudeToolResultText tool_result 的 content：字符串取值，其余类型保留原始 JSON。
func claudeToolResultText(raw sonic.NoCopyRawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if sonic.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(bytes.TrimSpace(raw))
}
```

注意：`buildClaudeConversation` 里消息为空的 item 应跳过（与 codex 一致：`item.Kind == message && item.Content == ""` 时丢弃）——在追加循环里保留既有守卫逻辑（参照 codex `projectConversationItem` 的尾部判断）。

- [ ] **Step 6: 修改 `internal/application/trace/query/conversation.go` 分发**

```go
	conversation, err := trace.BuildConversationFor(item.Agent, events)
	if err != nil {
		return nil, err
	}
```

（替换原 `conversation := trace.BuildConversation(events)`；`BuildConversation` 符号不再导出，全仓 grep 确认无其他调用方。）

- [ ] **Step 7: 运行测试**

Run: `go build ./... && go test -count=1 ./internal/domain/trace/ ./test/unit/trace/ ./test/e2e/trace/`
Expected: PASS（codex 投影测试与 claude 新投影测试均绿）

- [ ] **Step 8: lint + 提交**

Run: `go run ./cmd/server lint conv ./internal/domain/trace/ ./internal/application/trace/ ./test/unit/trace/ && go run ./cmd/server lint static ./internal/domain/trace/ ./internal/application/trace/`
Expected: PASS

```bash
git add internal/domain/trace internal/application/trace/query/conversation.go test/unit/trace
git commit -m "feat(trace): 对话投影按 agent 分发，新增 claude 投影"
```

---

### Task 5: 安装脚本支持 Codex / Claude / Both

**Files:**
- Modify: `internal/handler/install_trace_client.sh.tmpl`
- Modify: `test/e2e/trace/install_script_test.go`

**Interfaces:**
- Consumes: Task 1 的 `--agent` 命令行契约。
- Produces: 安装脚本注册的 hook 命令为 `$aris_bin trace ingest --agent codex|claude`。

- [ ] **Step 1: 改测试 `test/e2e/trace/install_script_test.go`（先红）**

在 `TestInstallScript_ReturnsScriptWithHost` 追加断言：

```go
	for _, want := range []string{
		"trace ingest --agent codex",
		"trace ingest --agent claude",
		".claude/settings.json",
		"Select agent",
		"PostToolUseFailure",
		"SessionEnd",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script must contain %q", want)
		}
	}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/e2e/trace/ -run TestInstallScript_ReturnsScriptWithHost`
Expected: FAIL — 断言缺失

- [ ] **Step 3: 修改 `internal/handler/install_trace_client.sh.tmpl`**

`# --- [2/4] select agent ---` 整段替换为：

```sh
# --- [2/4] select agent ---
echo "[2/4] Select agent"
echo "  1) Codex"
echo "  2) Claude Code"
echo "  3) Both"
while :; do
  printf "Choice [3]: " >&3
  IFS= read -r answer <&3 || exit 1
  case "$answer" in
    ""|3|both|Both)   agent_codex=1; agent_claude=1; break ;;
    1|codex|Codex)    agent_codex=1; agent_claude=0; break ;;
    2|claude|Claude)  agent_codex=0; agent_claude=1; break ;;
    *) echo "Please choose 1, 2, or 3" >&3 ;;
  esac
done
```

`# --- [4/4] configure Codex hooks ---` 整段替换为：

```sh
# --- [4/4] configure hooks ---
echo "[4/4] Configure hooks"

# register_aris_hooks <target-file> <hook-command> <event...>
# 幂等：保留非 Aris hook 组；已含相同 command 的组去重后追加。
register_aris_hooks() {
  target="$1"; hook_cmd="$2"; shift 2
  aris_group='{"matcher":"","hooks":[{"type":"command","command":"'"$hook_cmd"'","timeout":30}]}'
  mkdir -p -m 0700 "$(dirname "$target")"
  if [ -f "$target" ]; then
    cp "$target" "$target.bak"
    chmod 600 "$target.bak"
    config=$(cat "$target")
  else
    config='{}'
  fi
  printf '%s' "$config" | jq . >/dev/null 2>&1 || { echo "Invalid JSON in $target; skipped" >&2; return 0; }
  for event in "$@"; do
    config=$(printf '%s' "$config" | jq \
      --arg event "$event" \
      --arg cmd "$hook_cmd" \
      --argjson group "$aris_group" \
      '.hooks[$event] = ((.hooks[$event] // [])
        | map(select(any(.hooks[]?; .command == $cmd) | not))
        + [$group])')
  done
  tmp_config=$(mktemp "${TMPDIR:-/tmp}/aris-hooks.XXXXXX")
  printf '%s\n' "$config" | jq . > "$tmp_config"
  chmod 600 "$tmp_config"
  mv "$tmp_config" "$target"
}

if [ "$agent_codex" = "1" ]; then
  register_aris_hooks "$HOME/.codex/hooks.json" "$aris_bin trace ingest --agent codex" \
    SessionStart UserPromptSubmit PreToolUse PermissionRequest \
    PostToolUse Stop SubagentStart SubagentStop PreCompact PostCompact
  echo "Codex hooks configured"
fi

if [ "$agent_claude" = "1" ]; then
  register_aris_hooks "$HOME/.claude/settings.json" "$aris_bin trace ingest --agent claude" \
    SessionStart UserPromptSubmit PreToolUse PostToolUse PostToolUseFailure \
    Stop SubagentStart SubagentStop PreCompact PostCompact SessionEnd
  echo "Claude Code hooks configured"
fi
```

config.json 写入段去掉 `agent` 键：

```sh
jq -n --arg host "$host" --arg key "$api_key" \
  '{host:$host, apiKey:$key}' > "$tmp_config"
```

完成提示段替换为：

```sh
echo ""
echo "Trace configuration completed"
echo "Config: $config_file"
if [ "$agent_codex" = "1" ]; then
  echo "In Codex, run /hooks and manually approve the new Aris hooks."
fi
if [ "$agent_claude" = "1" ]; then
  echo "Claude Code picks up ~/.claude/settings.json hooks automatically; review them with /hooks."
fi
```

- [ ] **Step 4: 运行测试**

Run: `go test -count=1 ./test/e2e/trace/`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/handler/install_trace_client.sh.tmpl test/e2e/trace/install_script_test.go
git commit -m "feat(trace): 安装脚本支持 Codex / Claude / Both 选择"
```

---

### Task 6: claude E2E 全链路 + 文档收尾

**Files:**
- Create: `test/e2e/trace/claude_flow_test.go`
- Modify: `CONTEXT.md`（TraceClient / Transcript Ingestion 词汇）

**Interfaces:**
- Consumes: Task 1-5 全部。

- [ ] **Step 1: 写 E2E `test/e2e/trace/claude_flow_test.go`（hook 序列 → handler → usecase → fake repo → 投影断言）**

```go
package trace_e2e

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	tracefake "github.com/hcd233/aris-proxy-api/test/unit/trace"
)

// TestE2E_ClaudeTraceFlow 验证 claude 上报 → 入库 → 投影全链路。
func TestE2E_ClaudeTraceFlow(t *testing.T) {
	t.Parallel()
	repo := tracefake.NewFakeRepo()
	report := command.NewReportTraceEventHandler(repo)
	conversationQuery := query.NewListTraceConversationHandler(repo, nil)
	ctx := context.Background()

	seq := int64(0)
	hook := func(event, turnID, payload string) {
		t.Helper()
		seq++
		_, err := report.Handle(ctx, port.ReportTraceEventCommand{
			SessionID: "claude-e2e", Agent: constant.TraceAgentClaude, UserID: 7, APIKeyName: "k",
			Records: []port.ReportTraceRecord{{
				Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
				HookEventName: event, Event: event, TurnID: turnID,
				ClientSequence: seq, DedupKey: "hook:claude-e2e:" + event + turnID,
				Payload: []byte(payload),
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	transcript := func(recordType, event, payload string) {
		t.Helper()
		seq++
		_, err := report.Handle(ctx, port.ReportTraceEventCommand{
			SessionID: "claude-e2e", Agent: constant.TraceAgentClaude, UserID: 7, APIKeyName: "k",
			Records: []port.ReportTraceRecord{{
				Source: constant.TraceRecordSourceRollout, RecordType: recordType, Event: event,
				ClientSequence: seq, DedupKey: "rollout:claude-e2e:" + event + payload[:16],
				Payload: []byte(payload),
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	hook("SessionStart", "", `{"hook_event_name":"SessionStart","session_id":"claude-e2e","model":"claude-sonnet-5","source":"startup"}`)
	hook("UserPromptSubmit", "p1", `{"hook_event_name":"UserPromptSubmit","prompt_id":"p1","prompt":"列一下文件"}`)
	transcript(constant.TraceClaudeRecordUser, constant.TraceClaudeEventUserPrompt, `{"type":"user","uuid":"u1","promptId":"p1","message":{"role":"user","content":"列一下文件"}}`)
	transcript(constant.TraceClaudeRecordAssistant, constant.TraceClaudeEventAssistantMessage, `{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"text","text":"好的"},{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]}}`)
	transcript(constant.TraceClaudeRecordUser, constant.TraceClaudeEventToolResult, `{"type":"user","uuid":"u2","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file.go"}]}}`)
	hook("Stop", "p1", `{"hook_event_name":"Stop","prompt_id":"p1","last_assistant_message":"完成"}`)

	tr, err := repo.FindBySessionID(ctx, "claude-e2e")
	if err != nil || tr == nil {
		t.Fatalf("trace missing: %v", err)
	}
	if tr.Agent != constant.TraceAgentClaude || tr.Status != constant.TraceStatusActive {
		t.Fatalf("unexpected trace: %+v", tr)
	}

	view, err := conversationQuery.Handle(ctx, port.ListTraceConversationQuery{UserID: 7, TraceID: tr.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Turns) != 1 || view.Turns[0].TurnID != "u1" {
		t.Fatalf("unexpected turns: %+v", view.Turns)
	}
	var tool *port.TraceConversationItemView
	for _, item := range view.Turns[0].Items {
		if item.Kind == constant.TraceConversationKindToolCall {
			tool = item
		}
	}
	if tool == nil || tool.Output != "file.go" {
		t.Fatalf("tool call projection broken: %+v", tool)
	}

	hook("SessionEnd", "", `{"hook_event_name":"SessionEnd","session_id":"claude-e2e","reason":"other"}`)
	tr, _ = repo.FindBySessionID(ctx, "claude-e2e")
	if tr.Status != constant.TraceStatusDone {
		t.Fatalf("SessionEnd must mark trace done, got %s", tr.Status)
	}
}
```

注意：`authorize` 依赖 `apiKeyRepo`；`NewListTraceConversationHandler(repo, nil)` 需要 fake APIKeyRepo——看 `authorize.go` 的实现，fake 包里有私有 `newFakeAPIKeyRepo`。实现时改为：在 `test/unit/trace` 包内写本用例（同一 package 可直接用 `newFakeAPIKeyRepo`），即文件落在 `test/unit/trace/claude_e2e_flow_test.go`，package 为 `trace`（去掉 `trace_e2e` 引用问题）；或导出 fake repo 的 helper。选择：**放进 `test/unit/trace/claude_e2e_flow_test.go`**，userID 7 配上 `owners: map[uint][]string{7: {"k"}}`。