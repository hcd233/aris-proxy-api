# Trace 采集极简重构 实施计划（2026-08-01）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 codex trace 采集只保留有价值数据——event_msg 白名单过滤、turn_context 丢弃、hook 纯触发不记录、status（active/done）全链路移除，并清洗生产存量冗余。

**Architecture:** 过滤与裁剪全部在客户端采集时完成（省传输+存储）：`AgentAdapter` 新增 `IgnoreTranscriptLine` 方法表达各 agent 的丢弃规则（codex 丢弃 turn_context 与非白名单 event_msg）；codex hook 从"记录+触发"改为纯触发，会话级元数据（model/cwd/source）持久化到 state 目录，flush 时从状态文件组装 batch；服务端移除 status 字段与 done 判定链路；前端移除状态列/徽标。claude 分支保持现状不动。

**Tech Stack:** Go（客户端采集器 `internal/client/trace/`、服务端 `internal/application/trace/`）、PostgreSQL（gorm）、Next.js/React 前端、生产 k3s 部署 + Docker PostgreSQL。

## Global Constraints

- **测试契约**：所有任务必须带测试；全量验证 `go test -count=1 ./...`（基准 835 passed / 193 packages）必须全绿；`go vet ./...` 必须零告警。
- **lint 约束**（pre-commit 强制）：LintConv 禁匿名 struct（提取命名类型）；gocritic 要求命名返回值；nestif 复杂度 >8 失败；nilerr 需 `//nolint:nilerr`；unparam 需删除未用参数。
- **前端契约**：`npx tsc --noEmit` 零错误、`npm run build`（next build）exit 0；i18n 键 zh/en/ja 三语同步。
- **生产安全基线**：所有生产写操作（DELETE/发布）先完整展示命令与影响范围，等用户明确授权；禁止生产 DDL（gorm AutoMigrate 不删列，status 列残留无害，代码层移除即可）；不泄露连接凭据。
- **claude 不动**：claude adapter 保持现有 hook 记录行为，仅实现 `IgnoreTranscriptLine` 返回 false 以适配接口。
- **命名**：新增常量放 `internal/common/constant/` 既有分区；代码注释与提交信息用中文。

---

### Task 1: AgentAdapter 扩展 `IgnoreTranscriptLine`（codex 白名单 + claude no-op）

**Files:**
- Modify: `internal/common/constant/sql.go`（新增 `TraceEventTaskStarted` 常量）
- Modify: `internal/client/trace/adapter.go`（接口加方法）
- Modify: `internal/client/trace/codex.go`（实现白名单过滤）
- Modify: `internal/client/trace/claude.go`（实现 no-op）
- Test: `test/unit/client/trace/codex_test.go`（追加测试用例）

**Interfaces:**
- Produces: `AgentAdapter.IgnoreTranscriptLine(meta TranscriptMeta) bool`；codex 白名单 map `codexEventMsgWhitelist`（包内私有）

- [ ] **Step 1: 写失败测试**（追加到 `test/unit/client/trace/codex_test.go` 末尾；该目录为独立测试包，经 import 别名 `trace` 访问被测包导出符号，通过 `trace.LookupAdapter` 拿接口实例）

```go
func TestCodexAdapterIgnoreTranscriptLine(t *testing.T) {
	t.Parallel()
	adapter, err := trace.LookupAdapter("codex")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		meta trace.TranscriptMeta
		want bool
	}{
		{name: "session_meta 保留", meta: trace.TranscriptMeta{RecordType: "session_meta"}, want: false},
		{name: "response_item 保留", meta: trace.TranscriptMeta{RecordType: "response_item", Event: "message"}, want: false},
		{name: "event_msg task_complete 保留", meta: trace.TranscriptMeta{RecordType: "event_msg", Event: "task_complete"}, want: false},
		{name: "event_msg task_started 保留", meta: trace.TranscriptMeta{RecordType: "event_msg", Event: "task_started"}, want: false},
		{name: "event_msg token_count 保留（固定 dedup key 覆盖）", meta: trace.TranscriptMeta{RecordType: "event_msg", Event: "token_count"}, want: false},
		{name: "event_msg agent_message 丢弃", meta: trace.TranscriptMeta{RecordType: "event_msg", Event: "agent_message"}, want: true},
		{name: "event_msg world_state 丢弃（未来类型）", meta: trace.TranscriptMeta{RecordType: "event_msg", Event: "world_state"}, want: true},
		{name: "turn_context 丢弃", meta: trace.TranscriptMeta{RecordType: "turn_context"}, want: true},
		{name: "unknown 保留（服务端告警丢弃）", meta: trace.TranscriptMeta{RecordType: "unknown", Event: "x"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := adapter.IgnoreTranscriptLine(tc.meta); got != tc.want {
				t.Fatalf("IgnoreTranscriptLine(%+v) = %v, want %v", tc.meta, got, tc.want)
			}
		})
	}
}

func TestClaudeAdapterIgnoreTranscriptLineNoop(t *testing.T) {
	t.Parallel()
	adapter, err := trace.LookupAdapter("claude")
	if err != nil {
		t.Fatal(err)
	}
	if adapter.IgnoreTranscriptLine(trace.TranscriptMeta{RecordType: "event_msg", Event: "x"}) {
		t.Fatal("claude 不应忽略任何记录")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/client/trace/ -run 'IgnoreTranscriptLine' -v`
Expected: 编译失败，报 `codexAdapter does not implement AgentAdapter (missing method IgnoreTranscriptLine)` 或 `undefined: TraceEventTaskStarted`（按 Task 顺序先跑会先缺方法）。

- [ ] **Step 3: 实现常量与接口**

`internal/common/constant/sql.go` 在 `TraceEventTaskComplete` 常量行上方新增：

```go
	TraceEventTaskStarted             = "task_started"
	TraceEventTaskComplete            = "task_complete"
	TraceEventTokenCount              = "token_count"
	TraceRecordMessageUnknown         = "unknown record type"
```

`internal/client/trace/adapter.go` 接口追加方法（放在 `StdoutAck` 之后）：

```go
	// IgnoreTranscriptLine 返回 true 表示该行 transcript 记录不采集（不进 spool、
	// 不上报、不入库）。codex 用于 event_msg 白名单过滤与 turn_context 丢弃；
	// claude 恒返回 false（保持现状）。
	IgnoreTranscriptLine(meta TranscriptMeta) bool
```

- [ ] **Step 4: 实现 codex/claude 白名单逻辑**

`internal/client/trace/codex.go` 新增（放在 `codexRolloutRecordType` 函数之后）：

```go
// codexEventMsgWhitelist event_msg 记录不丢弃的白名单。task_complete/task_started 为任务
// 生命周期标记；token_count 每条都带累计统计，上报后由服务端按固定 dedup key 覆盖写入
// （D1a），库里只留最后一条。其余 event_msg（agent_message/agent_reasoning/
// user_message/thread_settings_applied/world_state 等）与 response_item 双源重复或纯噪音，
// 客户端直接丢弃。
var codexEventMsgWhitelist = map[string]bool{
	constant.TraceEventTaskStarted:  true,
	constant.TraceEventTaskComplete: true,
	constant.TraceEventTokenCount:   true,
}

func (codexAdapter) IgnoreTranscriptLine(meta TranscriptMeta) bool {
	switch meta.RecordType {
	case constant.TraceRolloutTypeTurnContext:
		return true // 无消费者，model/cwd/turn_id 已被 traces 表与 response_item 覆盖
	case constant.TraceRolloutTypeEventMsg:
		return !codexEventMsgWhitelist[meta.Event]
	default:
		return false
	}
}
```

`internal/client/trace/claude.go` 新增（放在 `StdoutAck` 之后）：

```go
// IgnoreTranscriptLine claude 采集保持现状，不忽略任何记录。
func (claudeAdapter) IgnoreTranscriptLine(TranscriptMeta) bool { return false }
```

- [ ] **Step 5: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/client/trace/ -run 'IgnoreTranscriptLine' -v`
Expected: 两个测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/common/constant/sql.go internal/client/trace/adapter.go internal/client/trace/codex.go internal/client/trace/claude.go test/unit/client/trace/codex_test.go
git commit -m "feat(trace): AgentAdapter 新增 IgnoreTranscriptLine，codex event_msg 白名单过滤"
```

---

### Task 2: RolloutReader 跳过被忽略记录

**Files:**
- Modify: `internal/client/trace/rollout.go`（`parseRolloutLines` + `rolloutRecord` 签名）
- Test: `test/unit/client/trace/rollout_test.go`（追加测试）

**Interfaces:**
- Consumes: `AgentAdapter.IgnoreTranscriptLine(meta TranscriptMeta) bool`（Task 1）
- Produces: `RolloutReader.parseRolloutLines` 对被忽略行不生成 `PendingRecord`、不进 spool

- [ ] **Step 1: 写失败测试**（追加到 `test/unit/client/trace/rollout_test.go` 末尾；复用现有 fixture/构造方式）

```go
func TestRolloutReaderKeepsTokenCountAndSkipsIgnored(t *testing.T) {
	t.Parallel()
	paths := Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	spool := NewSpool(paths, 100)
	reader := NewRolloutReader(paths, spool, codexAdapter{})

	transcript := strings.Join([]string{
		`{"type":"session_meta","payload":{"id":"sid-1","type":"session_meta"}}`,
		`{"type":"turn_context","payload":{"cwd":"/tmp","model":"m1","turn_id":"t1"}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":10}}}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":20}}}}`,
		`{"type":"event_msg","payload":{"type":"task_started"}}`,
		`{"type":"event_msg","payload":{"type":"task_complete"}}`,
		`{"type":"event_msg","payload":{"type":"agent_message","message":"hi"}}`,
		`{"type":"response_item","payload":{"id":"r1","type":"message"}}`,
		`{"type":"weird","payload":{}}`,
	}, "\n") + "\n"

	transcriptPath := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}

	records, err := reader.ReadNew(context.Background(), "sess-1", transcriptPath)
	if err != nil {
		t.Fatalf("ReadNew error: %v", err)
	}
	// 保留：session_meta / token_count×2（服务端按固定 dedup key 覆盖）/ task_started / task_complete / response_item
	if len(records) != 6 {
		t.Fatalf("records = %d, want 6", len(records))
	}
	joined := ""
	for _, r := range records {
		joined += r.RecordType + ":" + r.Event + "|"
	}
	for _, want := range []string{"session_meta:", "event_msg:token_count", "event_msg:task_started", "event_msg:task_complete", "response_item:message"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("records 缺少 %q，实际: %s", want, joined)
		}
	}
	if strings.Contains(joined, "turn_context") || strings.Contains(joined, "agent_message") {
		t.Fatalf("被忽略记录不应出现: %s", joined)
	}
	// token_count 固定 dedup key：两条 token_count 的 DedupKey 相同
	var tokenDedup []string
	for _, r := range records {
		if r.RecordType == "event_msg" && r.Event == "token_count" {
			tokenDedup = append(tokenDedup, r.DedupKey)
		}
	}
	if len(tokenDedup) != 2 || tokenDedup[0] == "" || tokenDedup[0] != tokenDedup[1] {
		t.Fatalf("token_count dedup key 应固定且相同，实际: %v", tokenDedup)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/client/trace/ -run 'TestRolloutReaderKeepsTokenCountAndSkipsIgnored' -v`
Expected: FAIL——结果包含 turn_context / agent_message（当前无过滤），或 token_count dedup key 不相同。

- [ ] **Step 3: 实现过滤 + token_count 固定 dedup key**

`internal/client/trace/rollout.go` 的 `parseRolloutLines` 循环体改为：

```go
		classified := r.adapter.ClassifyTranscriptLine(raw)
		if r.adapter.IgnoreTranscriptLine(classified) {
			continue
		}
		record := r.rolloutRecord(sessionID, state.Line, raw, meta, classified)
		record.CreatedAt = time.Now().UTC()
		if err := r.spool.Append(ctx, record); err != nil {
			return nil, state, err
		}
		records = append(records, record)
```

`rolloutRecord` 签名加 `classified TranscriptMeta` 参数并去掉内部重复分类调用：

```go
func (r *RolloutReader) rolloutRecord(
	sessionID string,
	line int64,
	raw []byte,
	meta PendingRecord,
	classified TranscriptMeta,
) PendingRecord {
	lineCopy := line
	record := PendingRecord{
		SessionID:       sessionID,
		Agent:           r.adapter.Name(),
		Source:          constant.TraceRecordSourceRollout,
		RecordType:      classified.RecordType,
		Event:           classified.Event,
		TurnID:          classified.TurnID,
		CallID:          classified.CallID,
		TranscriptLine:  &lineCopy,
		DedupKey:        RolloutDedupKey(sessionID, classified, line, raw),
		Payload:         append(sonic.NoCopyRawMessage{}, raw...),
		ParentSessionID: meta.ParentSessionID,
		AgentID:         meta.AgentID,
		AgentType:       meta.AgentType,
	}
	return record
}
```

`RolloutDedupKey` 增加 token_count 固定 key 分支（`TraceClientTokenCountDedupFormat` 常量加到 `internal/common/constant/traceclient.go` 现有 dedup 格式常量旁）：

```go
// TraceClientTokenCountDedupFormat token_count 固定 dedup key：同一会话多条 token_count
// 共用同一 key，服务端 ON CONFLICT DO UPDATE 后库里只保留最后一条（会话累计 token 汇总）。
TraceClientTokenCountDedupFormat = "token_count:%s"
```

```go
func RolloutDedupKey(sessionID string, meta TranscriptMeta, line int64, raw []byte) string {
	if meta.RecordType == constant.TraceRolloutTypeSessionMeta && meta.SessionID != "" {
		return fmt.Sprintf(constant.TraceClientSessionMetaDedupFormat, sessionID, meta.SessionID)
	}
	if meta.RecordType == constant.TraceRolloutTypeEventMsg && meta.Event == constant.TraceEventTokenCount {
		return fmt.Sprintf(constant.TraceClientTokenCountDedupFormat, sessionID)
	}
	digest := sha256.Sum256(raw)
	return fmt.Sprintf(constant.TraceClientRolloutDedupFormat, sessionID, line, hex.EncodeToString(digest[:]))
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/client/trace/ -run 'TestRolloutReaderKeepsTokenCountAndSkipsIgnored' -v`
Expected: PASS（6 条保留：session_meta + token_count×2（同 key）+ task_started + task_complete + response_item；turn_context/agent_message 被忽略）。

- [ ] **Step 5: 提交**

```bash
git add internal/client/trace/rollout.go internal/common/constant/traceclient.go test/unit/client/trace/rollout_test.go
git commit -m "feat(trace): RolloutReader 跳过忽略记录，token_count 固定 dedup key"
```

---

### Task 3: codex hook 纯触发 + per-session 元数据持久化

**Files:**
- Modify: `internal/common/constant/traceclient.go`（新增 meta 文件常量）
- Create: `internal/client/trace/meta.go`（sessionMeta 读写）
- Modify: `internal/client/trace/ingest.go`（Ingest 分流、flush/flushSubagent 元数据）
- Test: `test/unit/client/trace/ingest_test.go`（更新 + 新增）

**Interfaces:**
- Consumes: `AgentAdapter.Name()`（分流 codex/claude）、`writePrivateFile`（现有包内函数）、`withFileLock`（现有包内函数）
- Produces: `writeSessionMeta(ctx, paths, sessionID, sessionMeta) error`、`loadSessionMeta(paths, sessionID) sessionMeta`；`Ingestor.Ingest` 对 codex 不再生成 hook 记录

- [ ] **Step 1: 写失败测试**

`test/unit/client/trace/ingest_test.go` 追加（改造 `TestRunIngestCommand_FlushesAcceptedRecord` 之外的新用例——codex hook 触发不产生 hook 记录，但 SessionStart 元数据随 rollout 批次上报）：

```go
func TestRunIngestCommand_CodexHookTriggersWithoutHookRecord(t *testing.T) {
	t.Parallel()
	var gotRequest struct {
		SessionID string `json:"session_id"`
		Model     string `json:"model"`
		CWD       string `json:"cwd"`
		Source    string `json:"source"`
		Records   []struct {
			Source     string `json:"source"`
			RecordType string `json:"record_type"`
			Event      string `json:"event"`
		} `json:"records"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatal(err)
		}
		results := lo.Map(gotRequest.Records, func(_ struct {
			Source     string `json:"source"`
			RecordType string `json:"record_type"`
			Event      string `json:"event"`
		}, _ int) client.RecordResult {
			return client.RecordResult{DedupKey: "k", Status: "accepted"}
		})
		data, _ := sonic.Marshal(struct {
			Results []client.RecordResult `json:"results"`
		}{Results: results})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer server.Close()

	paths := client.Paths{Root: filepath.Join(t.TempDir(), ".aris")}
	if err := os.MkdirAll(paths.TraceDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	configJSON := `{"host":"` + server.URL + `","agent":"codex","apiKey":"proxy-key"}`
	if err := os.WriteFile(paths.ConfigFile(), []byte(configJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	// 预置 transcript：一次 SessionStart hook 触发后应上报 rollout 记录且 batch 携带 hook 元数据
	transcriptPath := filepath.Join(t.TempDir(), "rollout.jsonl")
	transcript := `{"type":"session_meta","payload":{"id":"sid-1","type":"session_meta"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"token_count"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"task_complete"}}` + "\n"
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o600); err != nil {
		t.Fatal(err)
	}
	hookInput := `{"hook_event_name":"SessionStart","session_id":"sess-1","model":"glm-5.2","cwd":"/tmp/x","source":"startup","transcript_path":"` + transcriptPath + `"}`

	var out bytes.Buffer
	if err := client.RunIngestCommand(context.Background(), client.IngestCommandOptions{
		Paths:      paths,
		In:         bytes.NewBufferString(hookInput),
		Out:        &out,
		HTTPClient: server.Client(),
		AgentName:  "codex",
	}); err != nil {
		t.Fatal(err)
	}
	if gotRequest.SessionID != "sess-1" {
		t.Fatalf("session_id = %q", gotRequest.SessionID)
	}
	if gotRequest.Model != "glm-5.2" || gotRequest.CWD != "/tmp/x" || gotRequest.Source != "startup" {
		t.Fatalf("batch 元数据 = %+v，期望 model=glm-5.2 cwd=/tmp/x source=startup", gotRequest)
	}
	for _, rec := range gotRequest.Records {
		if rec.Source != "rollout" {
			t.Fatalf("出现非 rollout 记录: %+v", rec)
		}
	}
	// event_msg 白名单生效：token_count 不出现，task_complete 保留
	events := lo.Map(gotRequest.Records, func(r struct {
		Source     string `json:"source"`
		RecordType string `json:"record_type"`
		Event      string `json:"event"`
	}, _ int) string { return r.RecordType + ":" + r.Event })
	joined := strings.Join(events, "|")
	if strings.Contains(joined, "token_count") {
		t.Fatalf("token_count 不应上报: %s", joined)
	}
	if !strings.Contains(joined, "event_msg:task_complete") {
		t.Fatalf("task_complete 应上报: %s", joined)
	}
}
```

> 注意：`client.RecordResult` 为外部包导出类型，该测试文件 import 了 `client "github.com/hcd233/aris-proxy-api/internal/client/trace"` 且使用 `client.Paths` 等导出符号；`RecordResult` 字段需确认导出名（`DedupKey`/`Status`，见现有测试用法）。`lo` 与 `strings` 需补 import。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/client/trace/ -run 'TestRunIngestCommand_CodexHookTriggersWithoutHookRecord' -v`
Expected: FAIL——当前 codex hook 会生成 hook 记录，batch 出现 `hook` source 记录，或元数据断言失败（Model 为空）。

- [ ] **Step 3: 新增常量与 meta.go**

`internal/common/constant/traceclient.go` 在现有 trace 客户端常量区新增：

```go
	// TraceClientSessionMetaDir/Suffix per-session hook 元数据状态文件（codex hook 纯触发后，
	// model/cwd/source 不再随 hook 记录上报，改为持久化后由 flush 读取）
	TraceClientSessionMetaDir      = "sessions"
	TraceClientSessionMetaSuffix   = ".meta"
	TraceClientSessionMetaLockSuffix = ".lock"
```

新建 `internal/client/trace/meta.go`：

```go
package trace

import (
	"context"
	"os"
	"path/filepath"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// sessionMeta codex hook 触发时记录的会话级元数据，flush 时随批次上报。
type sessionMeta struct {
	Model  string `json:"model,omitempty"`
	CWD    string `json:"cwd,omitempty"`
	Source string `json:"source,omitempty"`
}

func sessionMetaPath(paths Paths, sessionID string) string {
	return filepath.Join(paths.StateDir(), constant.TraceClientSessionMetaDir, sessionID+constant.TraceClientSessionMetaSuffix)
}

// writeSessionMeta 覆盖写入 per-session 元数据（codex SessionStart/Stop hook 触发时调用）。
func writeSessionMeta(ctx context.Context, paths Paths, sessionID string, meta sessionMeta) error {
	data, err := sonic.Marshal(meta)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode session meta")
	}
	path := sessionMetaPath(paths, sessionID)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "mkdir session meta dir")
	}
	return withFileLock(path+constant.TraceClientSessionMetaLockSuffix, func() error {
		return writePrivateFile(path, data)
	})
}

// loadSessionMeta 读取 per-session 元数据；不存在或损坏时返回零值（元数据缺失不阻塞上报）。
func loadSessionMeta(paths Paths, sessionID string) sessionMeta {
	var meta sessionMeta
	data, err := os.ReadFile(sessionMetaPath(paths, sessionID))
	if err != nil {
		return meta
	}
	_ = sonic.Unmarshal(data, &meta) //nolint:errcheck // best-effort read
	return meta
}
```

- [ ] **Step 4: 改造 ingest.go（codex 分流 + flush 元数据）**

`internal/client/trace/ingest.go` 的 `Ingest` 中，在 SubagentStop 分支后插入 codex 分流，并提取公共尾部：

```go
	if info.EventName == constant.TraceEventSubagentStop && i.adapter.Name() == constant.TraceAgentCodex {
		// codex 子代理是独立 session（独立 transcript），SubagentStop 只读子代理 transcript
		return i.ingestSubagentStop(ctx, info)
	}
	if i.adapter.Name() == constant.TraceAgentCodex {
		// codex hook 纯触发：不生成 hook 记录，仅写会话元数据 + 触发 transcript 增量读取
		return i.ingestCodexHookTrigger(ctx, info)
	}
	// ── claude（保持现状：生成 hook 记录）──
	spoolID, sequence, err := nextSequence(ctx, i.paths)
```

在 `Ingest` 之后新增：

```go
// ingestCodexHookTrigger codex hook 纯触发：写 per-session 元数据并读取 transcript 增量，
// 不生成任何 hook 记录。
func (i *Ingestor) ingestCodexHookTrigger(ctx context.Context, info HookInfo) error {
	if err := writeSessionMeta(ctx, i.paths, info.SessionID, sessionMeta{
		Model:  info.Model,
		CWD:    info.CWD,
		Source: info.SessionSource,
	}); err != nil {
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

`flush` 中 batch 元数据来源改为状态文件（替换原 `batch[0].Model` 补逻辑）：

```go
func (i *Ingestor) flush(ctx context.Context, config Config) error {
	batch, err := i.spool.Batch(
		ctx,
		constant.TraceClientBatchMaxRecords,
		constant.TraceClientBatchMaxBytes,
	)
	if err != nil || len(batch) == 0 {
		return err
	}
	meta := loadSessionMeta(i.paths, batch[0].SessionID)
	request := ingestBatch{
		SessionID:       batch[0].SessionID,
		ParentSessionID: batch[0].ParentSessionID,
		Agent:           batch[0].Agent,
		AgentID:         batch[0].AgentID,
		AgentType:       batch[0].AgentType,
		Model:           meta.Model,
		CWD:             meta.CWD,
		Source:          meta.Source,
		Records:         make([]ingestRecord, 0, len(batch)),
	}
	for _, record := range batch {
		request.Records = append(request.Records, ingestRecord{
			Source:         record.Source,
			RecordType:     record.RecordType,
			HookEventName:  record.Event,
			TurnID:         record.TurnID,
			CallID:         record.CallID,
			TranscriptLine: record.TranscriptLine,
			ClientSequence: record.ClientSequence,
			DedupKey:       record.DedupKey,
			Payload:        record.Payload,
		})
	}
	// 原 `if request.Model == "" && record.Model != ""` 补逻辑删除（rollout 记录恒无元数据）
	body, err := sonic.Marshal(request)
	// ...其余不变
```

`flushSubagent` 同理（用 `childID` 读取）：

```go
	meta := loadSessionMeta(i.paths, childID)
	request := ingestBatch{
		SessionID:       childID,
		ParentSessionID: info.SessionID,
		Agent:           i.adapter.Name(),
		AgentID:         info.AgentID,
		AgentType:       info.AgentType,
		Model:           meta.Model,
		CWD:             meta.CWD,
		Records:         make([]ingestRecord, 0, len(batch)),
	}
	for _, record := range batch {
		request.Records = append(request.Records, ingestRecord{
			Source:         record.Source,
			RecordType:     record.RecordType,
			HookEventName:  record.Event,
			TurnID:         record.TurnID,
			CallID:         record.CallID,
			TranscriptLine: record.TranscriptLine,
			ClientSequence: record.ClientSequence,
			DedupKey:       record.DedupKey,
			Payload:        record.Payload,
		})
	}
	if len(request.Records) == 0 {
		return nil // 无子代理记录可发，避免空批次 400
	}
	// 原 `if request.Model == "" && record.Model != ""` 补逻辑删除（子代理 rollout 记录恒无元数据）
	body, err := sonic.Marshal(request)
	// ...其余不变（postBatch）
```

> 注意：`ingestSubagentStop` 现有实现已经处理 codex 子代理（`flushSubagent`），无需改动其触发路径；`flushSubagent` 中 `len(request.Records) == 0` 早退逻辑保留。

- [ ] **Step 5: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/client/trace/`
Expected: 全部 PASS（含新测试与既有 `TestRunIngestCommand_*` 同步后的用例）。

> 同步既有测试：`TestRunIngestCommand_FlushesAcceptedRecord` 使用 `hook_event_name: UserPromptSubmit`（codex 下已不再注册），改造后 codex 分支不生成记录、无 transcript_path 时不产生批次 → 该用例会因"无请求发出"导致 `authorization` 断言失败。将其改为使用 claude adapter（`AgentName: "claude"`）验证 claude 分支原逻辑，或改用 SessionStart + transcript 断言 codex 新行为（二选一，推荐前者：claude 分支保真）。

- [ ] **Step 6: 提交**

```bash
git add internal/common/constant/traceclient.go internal/client/trace/meta.go internal/client/trace/ingest.go test/unit/client/trace/ingest_test.go
git commit -m "feat(trace): codex hook 纯触发，元数据持久化后随批次上报"
```

---

### Task 4: 服务端移除 status 全链路

**Files:**
- Modify: `internal/common/constant/sql.go`（删 `TraceStatusActive/Done`）
- Modify: `internal/infrastructure/database/model/trace.go`（删 `Trace.Status`）
- Modify: `internal/infrastructure/repository/trace_repository.go`（toTraceDomain/toTraceRecord/AssignmentColumns/MarkDone）
- Modify: `internal/domain/trace/repository.go`（删 `Trace.Status`、`MarkDone`）
- Modify: `internal/application/trace/command/report_trace_event.go`（删 done 链路）
- Modify: `internal/application/trace/query/get_trace.go`、`list_traces.go`（删 Status）
- Modify: `internal/application/trace/port/handler.go`（视图删 Status）
- Modify: `internal/handler/trace.go`（构造删 Status）
- Modify: `internal/dto/trace.go`（`TraceSummary`/`TraceDetail` 删 Status）
- Test: `test/unit/trace/usecase_test.go`、`repository_test.go`、`fake_repository.go` 等同步

**Interfaces:**
- Consumes: Task 1-3（客户端行为变化）
- Produces: `TraceRepository` 无 `MarkDone`；`insertRecords` 签名去掉 doneEvents/isComplete

- [ ] **Step 1: 写失败测试（编译级红）**

先改 `test/unit/trace/fake_repository.go` 中 mock 实现，删除 `MarkDone` 方法并确认编译失败：

Run: `go build ./...`
Expected: FAIL——`traceRepository`/fake 未实现 `TraceRepository`（多了 MarkDone 或少了方法）。

> 本任务以编译红为第一测试信号（接口变更），随后逐文件删除 status 引用并保持全量测试绿。

- [ ] **Step 2: 删除常量与模型字段**

`internal/common/constant/sql.go`：

```go
	TraceListPageSize  = 20
	TraceEventPageSize = 50
	TraceAgentCodex    = "codex"
	TraceAgentClaude   = "claude"
```
（删除 `TraceStatusActive`/`TraceStatusDone` 两行；`FieldStatus` 常量保留——cron/audit 等仍在用）

`internal/infrastructure/database/model/trace.go` `Trace` 结构删除：

```go
	Status        string            `json:"status" gorm:"column:status;not null;default:'active';comment:active/done"`
```

`internal/domain/trace/repository.go`：`Trace` 结构删除 `Status string` 行；接口删除：

```go
	// MarkDone 将 trace 标记为 done
	MarkDone(ctx context.Context, sessionID string) error
```

- [ ] **Step 3: 改造 repository**

`internal/infrastructure/repository/trace_repository.go`：

```go
func toTraceDomain(m *dbmodel.Trace) *trace.Trace {
	return &trace.Trace{
		ID: m.ID, Agent: m.Agent, SessionID: m.SessionID, APIKeyName: m.APIKeyName,
		UserID: m.UserID, ParentTraceID: m.ParentTraceID, Model: m.Model, CWD: m.CWD, Source: m.Source,
		Metadata: m.Metadata, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		DeletedAt: m.DeletedAt,
	}
}

func toTraceRecord(t *trace.Trace) *dbmodel.Trace {
	return &dbmodel.Trace{
		Agent: t.Agent, SessionID: t.SessionID, APIKeyName: t.APIKeyName,
		UserID: t.UserID, ParentTraceID: t.ParentTraceID, Model: t.Model, CWD: t.CWD, Source: t.Source,
		Metadata: t.Metadata,
	}
}
```

`UpsertBySessionID` 的 `AssignmentColumns` 删除 `constant.FieldStatus`：

```go
		DoUpdates: clause.AssignmentColumns([]string{
			constant.FieldModel, constant.FieldCWD, constant.FieldSource,
			constant.FieldUpdatedAt, constant.FieldMetadata, constant.FieldUserID, constant.FieldAPIKeyName,
			constant.FieldParentTraceID,
		}),
```

删除整个 `MarkDone` 方法：

```go
func (r *traceRepository) MarkDone(ctx context.Context, sessionID string) error {
	...
}
```

`InsertEvent` 增加 token_count 覆盖写入（其余记录保持 `DO NOTHING`）：

```go
func (r *traceRepository) InsertEvent(ctx context.Context, e *trace.TraceEvent) (bool, error) {
	db := r.db.WithContext(ctx)
	rec := &dbmodel.TraceEvent{
		TraceID:        e.TraceID,
		SessionID:      e.SessionID,
		Source:         e.Source,
		RecordType:     e.RecordType,
		Event:          e.Event,
		TurnID:         e.TurnID,
		CallID:         e.CallID,
		TranscriptLine: e.TranscriptLine,
		ClientSequence: e.ClientSequence,
		DedupKey:       e.DedupKey,
		Payload:        e.Payload,
	}
	query := db
	if e.DedupKey != "" {
		if e.RecordType == constant.TraceRecordTypeEventMsg && e.Event == constant.TraceEventTokenCount {
			// token_count 固定 dedup key（客户端 D1a）：同 key 冲突时覆盖 payload，最终保留最后一条
			query = query.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: constant.FieldDedupKey}},
				DoUpdates: clause.AssignmentColumns([]string{
					constant.FieldTraceID, constant.FieldPayload, constant.FieldUpdatedAt,
				}),
			})
		} else {
			query = query.Clauses(clause.OnConflict{DoNothing: true})
		}
	}
	result := query.Create(rec)
	if result.Error != nil {
		return false, ierr.Wrap(ierr.ErrDBCreate, result.Error, "insert trace event")
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	e.ID = rec.ID
	return true, nil
}
```

> 需要新增常量：`internal/common/constant/sql.go` 加 `FieldDedupKey = "dedup_key"`（Task 1 已加 `TraceEventTokenCount`）；`FieldTraceID`/`FieldPayload`/`FieldUpdatedAt` 若不存在需一并补齐（grep 确认，`FieldPayload` 当前无定义需新增）。

- [ ] **Step 4: 改造 command**

`internal/application/trace/command/report_trace_event.go`：

删除 `traceDoneEvents` var：

```go
// traceDoneEvents 按 agent 注册的完成事件集：命中即把 trace 置为 done。
// 新 agent 接入在此登记。
var traceDoneEvents = map[string][]string{
	constant.TraceAgentCodex:  {constant.TraceEventStop, constant.TraceEventTaskComplete},
	constant.TraceAgentClaude: {constant.TraceEventSessionEnd},
}
```

`Handle` 中：

```go
	agent := cmd.Agent
	if agent == "" {
		agent = constant.TraceAgentCodex
	}
	if agent != constant.TraceAgentCodex && agent != constant.TraceAgentClaude {
		return nil, ierr.New(ierr.ErrValidation, "unknown trace agent")
	}
```

（原 `doneEvents, ok := traceDoneEvents[agent]` + `if !ok` 删除）

`Handle` 尾部：

```go
	results := insertRecords(ctx, h.repo, t.ID, cmd.SessionID, records)
	return results, nil
```

（原 `results, isComplete := ...` + `if isComplete { MarkDone }` 删除）

`ensureTrace` 中删除两处 Status 赋值：

```go
		return h.repo.UpsertBySessionID(ctx, &trace.Trace{
			Agent:         agent,
			SessionID:     cmd.SessionID,
			ParentTraceID: parentTraceID,
			APIKeyName:    cmd.APIKeyName,
			UserID:        cmd.UserID,
			Model:         cmd.Model,
			CWD:           cmd.CWD,
			Source:        source,
			Metadata:      metadata,
		})
```
（删除 `Status: constant.TraceStatusActive,`）

```go
	return h.repo.UpsertBySessionID(ctx, &trace.Trace{
		ID:            existing.ID,
		Agent:         agent,
		SessionID:     cmd.SessionID,
		ParentTraceID: existing.ParentTraceID,
		APIKeyName:    existing.APIKeyName,
		UserID:        existing.UserID,
		Model:         modelName,
		CWD:           cwd,
		Source:        source,
		Metadata:      existing.Metadata,
	})
```
（删除 `Status: existing.Status,`）

`insertRecords` 签名与尾部，并新增 unknown 告警丢弃（在 `validRecord` 通过后、`InsertEvent` 之前）：

```go
func insertRecords(
	ctx context.Context,
	repo trace.TraceRepository,
	traceID uint,
	sessionID string,
	records []port.ReportTraceRecord,
) []port.ReportTraceRecordResult {
	results := make([]port.ReportTraceRecordResult, 0, len(records))
	for _, record := range records {
		result := port.ReportTraceRecordResult{DedupKey: record.DedupKey}
		if !validRecord(record) {
			result.Status = constant.TraceRecordStatusRejected
			result.Message = constant.TraceRecordMessageInvalid
			results = append(results, result)
			continue
		}
		if record.RecordType == constant.TraceRolloutTypeUnknown {
			// 未知记录类型：打 warning 便于发现 codex 新类型，不入库
			logger.WithCtx(ctx).Warn("[Trace] unknown record dropped",
				zap.String("sessionID", sessionID),
				zap.String("event", record.Event),
				zap.Int("payloadBytes", len(record.Payload)),
			)
			result.Status = constant.TraceRecordStatusRejected
			result.Message = constant.TraceRecordMessageUnknown
			results = append(results, result)
			continue
		}
		// ...中间不变（InsertEvent 等）...
		results = append(results, result)
	}
	return results
}
```
（删除 `doneEvents []string` 参数、`isComplete` 返回值及其判定 `if result.Status != ... && lo.Contains(doneEvents, record.Event) { isComplete = true }`；新增 import `"github.com/hcd233/aris-proxy-api/internal/logger"` 与 `"go.uber.org/zap"`）

> `lo` import 若不再被本文件使用需删除（`normalizeRecords` 未用 lo，检查后处理）。

- [ ] **Step 5: 改造查询/port/handler/dto**

`internal/application/trace/query/get_trace.go`：`TraceDetailView{...}` 删除 `Status: item.Status,`。

`internal/application/trace/query/list_traces.go`：`TraceSummaryView{...}` 删除 `Status: item.Status,`。

`internal/application/trace/port/handler.go`：`TraceSummaryView` 与 `TraceDetailView` 各删除 `Status string` 行。

`internal/handler/trace.go`：`TraceSummary` 构造删除 `Status: item.Status,`；`TraceDetail` 构造删除 `Status: view.Status,`。

`internal/dto/trace.go`：`TraceSummary` 删除 `Status string \`json:"status" doc:"active/done"\``；`TraceDetail` 删除 `Status string \`json:"status" doc:"active/done"\``。`ReportTraceRecordResult.Status` 保留。

- [ ] **Step 6: 同步服务端测试**

- `test/unit/trace/fake_repository.go`：删除 `MarkDone` mock。
- `test/unit/trace/usecase_test.go`、`repository_test.go`、`delete_test.go`、`owner_isolation_test.go`、`report_deleted_test.go` 等：删除 `Status:`/`TraceStatus*`/`MarkDone` 相关断言与构造。
- 用 grep 确认无残留：

Run: `grep -rn "MarkDone\|TraceStatus\|\.Status" internal/ test/unit/trace/ --include="*.go" | grep -i trace`
Expected: 仅剩 `TraceRecordStatus*`（上报结果状态）与 cron/audit 等无关 status。

- [ ] **Step 7: 全量验证 + 提交**

Run: `go build ./... && go vet ./... && go test -count=1 ./...`
Expected: 全部 PASS（基准 835 用例，删除 status 后用例数略减）。

```bash
git add internal/ test/unit/trace/
git commit -m "refactor(trace): 移除 trace status(active/done) 全链路（字段/判定/前端）"
```

---

### Task 5: 前端移除状态展示

**Files:**
- Modify: `web/src/lib/types.ts`（`TraceSummary.status`/`TraceDetail.status`）
- Modify: `web/src/app/(dashboard)/trace/page.tsx`（statusBadge + 状态列）
- Modify: `web/src/components/trace-detail/trace-detail-client.tsx`（statusBadge）
- Modify: `web/src/locales/zh.json` / `en.json` / `ja.json`（status 键）

**Interfaces:**
- Consumes: Task 4 服务端 DTO 变更（响应不再含 status）

- [ ] **Step 1: 删除 types 字段**

`web/src/lib/types.ts`：

```ts
export interface TraceSummary {
  id: number;
  sessionId: string;
  agent: string;
  apiKeyName: string;
  model: string;
  source: string;
  createdAt: string;
  updatedAt: string;
}
```
（删除 `status: string;`）

```ts
export interface TraceDetail {
  id: number;
  sessionId: string;
  agent: string;
  apiKeyName: string;
  model: string;
  cwd: string;
  source: string;
  metadata?: Record<string, string>;
  eventCount: number;
  createdAt: string;
  updatedAt: string;
}
```
（删除 `status: string;`）

- [ ] **Step 2: 删除列表页状态列**

`web/src/app/(dashboard)/trace/page.tsx`：

删除函数：

```tsx
function statusBadge(status: string, t: (k: string, f?: string) => string) {
  if (status === "active") {
    return <Badge variant="secondary">{t("trace.status_active")}</Badge>;
  }
  if (status === "done") {
    return <Badge variant="outline">{t("trace.status_done")}</Badge>;
  }
  return <Badge variant="outline">{status}</Badge>;
}
```

卡片视图删除 `{statusBadge(tr.status, t)}` 行；表格视图删除：

```tsx
<TableHead className="w-24">{t("trace.status")}</TableHead>
```
和
```tsx
<TableCell>{statusBadge(tr.status, t)}</TableCell>
```

- [ ] **Step 3: 删除详情页徽标**

`web/src/components/trace-detail/trace-detail-client.tsx`：删除 `statusBadge` 函数与 `{statusBadge(detail.status, t)}` 行。检查 `Badge` import 是否仍被使用（若仅 statusBadge 使用则删除 import）。

- [ ] **Step 4: 删除 i18n 键**

`web/src/locales/zh.json` / `en.json` / `ja.json` 各删除：

```json
  "trace.status_active": ...,
  "trace.status_done": ...,
  "trace.status": ...,
```

（`trace.status` 若仅列表表头使用一并删除；用 `grep -n '"trace.status' web/src/locales/*.json` 确认三语齐全）

- [ ] **Step 5: 前端验证**

Run: `cd web && npx tsc --noEmit && npm run build`
Expected: exit 0，无 status 相关类型错误。

- [ ] **Step 6: 提交**

```bash
git add web/src/lib/types.ts "web/src/app/(dashboard)/trace/page.tsx" web/src/components/trace-detail/trace-detail-client.tsx web/src/locales/
git commit -m "refactor(web): 移除 trace 状态列与状态徽标展示"
```

---

### Task 6: 全量回归与 lint 同步

**Files:** 无新增（修正所有残留）

- [ ] **Step 1: 全量 Go 测试 + lint**

Run: `go test -count=1 ./... && go vet ./... && gofmt -l internal/ test/`
Expected: 全部 PASS；vet 零告警；gofmt 无输出。若 pre-commit lint（golangci-lint）报 LintConv/gocritic/nestif/unparam，按 Global Constraints 修复。

- [ ] **Step 2: 前端 lint**

Run: `cd web && npx eslint src/components/trace-detail src/app/\(dashboard\)/trace --no-ignore 2>/dev/null || npx eslint src --no-ignore`
Expected: 零 error（react-hooks/set-state-in-effect 等既有注释约束保留）。

- [ ] **Step 3: 全量提交（若 Step 1/2 有修复）**

```bash
git add -A
git commit -m "fix(trace): 全量回归修复 status 移除残留"
```

---

### Task 7: 部署与生产存量清洗（需逐次授权）

**Files:** 无（生产操作）

- [ ] **Step 1: 构建并发布客户端 hook 二进制与服务端**

按 `docs/agents/repo-ci.md` 现有发布流程：构建含新采集逻辑的 `tracecli` 二进制与服务端镜像，部署到 k3s（namespace `aris-proxy-api`）；在已安装 aris hook 的机器上执行 `aris trace install` 刷新 hook（hook 注册列表未变，仅需替换二进制；无安装机器的会话由下次新会话自动使用新二进制）。

> 生产操作需先展示命令并获用户授权（登录方式见 `login-prod-server` skill：`ssh ubuntu@api.lvlvko.top`，环境在 `/home/ubuntu/code/aris-proxy-api/`）。

- [ ] **Step 2: 展示并等待授权——清洗存量 hook_event**

待执行 SQL（生产库 `trace_events`，事务内先 SELECT 后 DELETE）：

```sql
BEGIN;
SELECT count(*) AS cnt FROM trace_events WHERE record_type = 'hook_event';
-- 预期: 313（SessionStart 8 + Stop 12 + 收敛前旧 hook 293）
DELETE FROM trace_events WHERE record_type = 'hook_event';
COMMIT;
```

影响：删除 313 行，仅 `trace_events` 表，不动 `traces`；不可逆。**必须等用户明确授权后执行**（执行形态：`docker exec -i -e PGPASSWORD="$POSTGRES_PASSWORD" postgresql psql ...`，不泄露凭据）。

- [ ] **Step 3: 部署后手工验证**

1. 新采集 trace：`SELECT record_type, event, count(*) FROM trace_events GROUP BY 1,2` 无 `hook_event`/`turn_context`，`event_msg` 仅 task_started/task_complete；
2. `SELECT model, cwd, source FROM traces WHERE id=<新trace>` 元数据正确（来自 per-session 状态文件）；
3. 前端 `/web/trace/detail/?id=<新trace>`：无状态徽标、消息无重复卡片、时间线无 hook/token_count 噪音卡片。

- [ ] **Step 4: 沉淀工程经验**

用 `serena_write_memory` 记录：本重构的决策（event_msg 白名单/turn_context 丢弃/hook 纯触发/status 移除）、per-session 元数据机制、生产清洗 SQL 与验证方式、踩坑（claude 分支分流、AutoMigrate 不删列、`TestRunIngestCommand_FlushesAcceptedRecord` 需改 claude 分支）。
