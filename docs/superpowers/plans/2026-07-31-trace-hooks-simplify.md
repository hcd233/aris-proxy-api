# Trace Hook 简化与主/子 Agent 关联 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 codex trace hook 从 10 个收敛为 {SessionStart, Stop, SubagentStop}，消除 hook/rollout 双源重复，并通过 `traces.parent_trace_id` 建立主/子 Agent trace 关联，顺带修复 session_meta 重复存储。

**Architecture:** 客户端只保留 3 个 hook：SessionStart（元数据 + resume 兜底）、Stop（主 transcript 增量上报 + 轻量标记）、SubagentStop（子代理 transcript 增量上报，带 parent_session_id）。服务端 `ensureTrace` 按 parent_session_id 建立子 trace 并写入 parent_trace_id；子 trace done 由 transcript 的 `task_complete` 记录触发（现有 doneEvents 已含 TaskComplete）。session_meta 的 dedup key 改用稳定语义 `payload.id`。

**Tech Stack:** Go 1.26+、GORM（AutoMigrate）、bytedance/sonic、Fiber/huma。

## Global Constraints

- 服务端测试只能放 `test/unit/<topic>/`（本计划用 `test/unit/trace/`），禁止在 `internal/` 下放 `*_test.go`。
- 测试统一 `github.com/bytedance/sonic`；禁止 `encoding/json`、`json.RawMessage`、`any`、`interface{}`；只用标准库 `testing`，禁止 testify/gomock、禁止 `time.Sleep` 同步。
- 测试命令：`go test -v -count=1 ./test/unit/trace/`；全量：`make test`。
- 所有 DTO/端口字段遵循 huma 惯例，`*Req`/`*Rsp` 结构体不带 `[]byte` 字段（`dto_convention_test.go` 会校验）。
- JSON 序列化一律 `sonic`；字符串常量复用 `internal/common/constant` 包，不硬编码。
- spec 文档：`docs/superpowers/specs/2026-07-31-trace-hooks-simplify-design.md`（本计划的权威需求来源）。

---

### Task 1: traces 存储层 parent_trace_id

**Files:**
- Modify: `internal/infrastructure/database/model/trace.go`
- Modify: `internal/domain/trace/repository.go`
- Modify: `internal/infrastructure/repository/trace_repository.go`
- Modify: `internal/common/constant/sql.go`
- Modify: `test/unit/trace/fake_repository.go`
- Test: `test/unit/trace/repository_test.go`

**Interfaces:**
- Consumes: `dbmodel.Trace`、`trace.Trace`（现有结构）
- Produces: `trace.Trace.ParentTraceID uint`、`dbmodel.Trace.ParentTraceID uint`、`constant.FieldParentTraceID = "parent_trace_id"`；`FakeRepo` 支持 ParentTraceID 往返

- [ ] **Step 1: 写失败测试**（fake repo 持久化 parent_trace_id）

在 `test/unit/trace/repository_test.go` 追加：

```go
func TestFakeRepo_PersistsParentTraceID(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	ctx := context.Background()

	parent, err := repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "parent-s1", APIKeyName: "key1", Status: "active"})
	if err != nil {
		t.Fatalf("upsert parent: %v", err)
	}
	child, err := repo.UpsertBySessionID(ctx, &trace.Trace{SessionID: "child-s1", APIKeyName: "key1", ParentTraceID: parent.ID, Status: "active", Source: "subagent"})
	if err != nil {
		t.Fatalf("upsert child: %v", err)
	}
	if child.ParentTraceID != parent.ID {
		t.Fatalf("expected child.ParentTraceID=%d, got %d", parent.ID, child.ParentTraceID)
	}
	got, err := repo.FindBySessionID(ctx, "child-s1")
	if err != nil || got == nil {
		t.Fatalf("find child: %v", err)
	}
	if got.ParentTraceID != parent.ID {
		t.Fatalf("expected persisted ParentTraceID=%d, got %d", parent.ID, got.ParentTraceID)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test -v -count=1 -run TestFakeRepo_PersistsParentTraceID ./test/unit/trace/`
Expected: FAIL — `trace.Trace` 无 `ParentTraceID` 字段（编译错误）或 fake 未持久化。

- [ ] **Step 3: 实现**

`internal/domain/trace/repository.go` 的 `Trace` 结构体加字段（`UserID` 行后）：

```go
	UserID     uint
	ParentTraceID uint
	Model      string
```

`internal/infrastructure/database/model/trace.go` 的 `Trace` 结构体加字段（`UserID` 行后）：

```go
	UserID      uint              `json:"user_id" gorm:"column:user_id;not null;default:0;comment:归属用户"`
	ParentTraceID uint            `json:"parent_trace_id" gorm:"column:parent_trace_id;not null;default:0;index:idx_trace_parent;comment:父 trace id，0 表示主 trace"`
	Model       string            `json:"model" gorm:"column:model;not null;default:'';comment:活跃模型 slug"`
```

`internal/common/constant/sql.go` 的 trace 字段常量区（找到 `FieldSessionID` 定义处，在附近加）：

```go
	FieldParentTraceID = "parent_trace_id"
```

`internal/infrastructure/repository/trace_repository.go`：

`toTraceDomain` 增加映射：
```go
		UserID: m.UserID, ParentTraceID: m.ParentTraceID, Model: m.Model, CWD: m.CWD, Source: m.Source,
```

`toTraceRecord` 增加映射：
```go
		Agent: t.Agent, SessionID: t.SessionID, APIKeyName: t.APIKeyName,
		UserID: t.UserID, ParentTraceID: t.ParentTraceID, Model: t.Model, CWD: t.CWD, Source: t.Source,
```

`UpsertBySessionID` 的 `DoUpdates` 列加 `constant.FieldParentTraceID`：
```go
		DoUpdates: clause.AssignmentColumns([]string{
			constant.FieldModel, constant.FieldCWD, constant.FieldSource, constant.FieldStatus,
			constant.FieldUpdatedAt, constant.FieldMetadata, constant.FieldUserID, constant.FieldAPIKeyName,
			constant.FieldParentTraceID,
		}),
```

`test/unit/trace/fake_repository.go`：`UpsertBySessionID` 的 map 写入与 `FindBySessionID`/`FindByID` 返回中同步 `ParentTraceID`（查找 fake 里 `traces map[string]*trace.Trace` 的构造点，`ParentTraceID: t.ParentTraceID` 与 `UserID` 并列）。

- [ ] **Step 4: 运行确认通过**

Run: `go test -v -count=1 -run TestFakeRepo ./test/unit/trace/`
Expected: PASS（含新增与既有 fake 测试）

- [ ] **Step 5: Commit**

```bash
git add internal/domain/trace/repository.go internal/infrastructure/database/model/trace.go internal/infrastructure/repository/trace_repository.go internal/common/constant/sql.go test/unit/trace/fake_repository.go test/unit/trace/repository_test.go
git commit -m "feat(trace): traces 表新增 parent_trace_id 主/子关联字段"
```

---

### Task 2: 上报传输链路新字段（client → server）

**Files:**
- Modify: `internal/application/trace/port/handler.go`
- Modify: `internal/dto/trace.go`
- Modify: `internal/handler/trace.go`
- Modify: `internal/application/trace/query/list_traces.go`
- Modify: `internal/application/trace/query/get_trace.go`
- Test: `test/unit/trace/usecase_test.go`

**Interfaces:**
- Consumes: Task 1 的 `trace.Trace.ParentTraceID`
- Produces: `port.ReportTraceEventCommand` 增加 `ParentSessionID string`、`AgentID string`、`AgentType string`；`port.TraceSummaryView`/`port.TraceDetailView` 增加 `ParentTraceID uint`；`dto.ReportTraceEventReqBody` 增加 `parent_session_id`/`agent_id`/`agent_type`；`dto.TraceSummary`/`dto.TraceDetail` 增加 `parentTraceId`

- [ ] **Step 1: 写失败测试**（usecase 层面断言 ParentSessionID 透传至 command）

在 `test/unit/trace/usecase_test.go` 追加：

```go
func TestReportTraceEvent_SubagentCommandCarriesParentSession(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "parent-s1", APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:p1:1",
			Payload: []byte(`{"hook_event_name":"SessionStart","session_id":"parent-s1"}`),
		}},
	}); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	// 子代理批次：SessionID 为子代理 id，ParentSessionID 指向父
	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "child-s1", ParentSessionID: "parent-s1", AgentType: "worker",
		APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg,
			Event: "task_complete", TurnID: "t1", DedupKey: "rollout:child-s1:1",
			Payload: []byte(`{"type":"event_msg","payload":{"type":"task_complete"}}`),
		}},
	}); err != nil {
		t.Fatalf("report subagent batch: %v", err)
	}

	child, _ := repo.FindBySessionID(ctx, "child-s1")
	if child == nil || child.ParentTraceID == 0 {
		t.Fatalf("expected child trace linked to parent, got %+v", child)
	}
	parent, _ := repo.FindBySessionID(ctx, "parent-s1")
	if parent == nil || child.ParentTraceID != parent.ID {
		t.Fatalf("expected child.ParentTraceID=%d (parent id), got %d", parent.ID, child.ParentTraceID)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test -v -count=1 -run TestReportTraceEvent_SubagentCommandCarriesParentSession ./test/unit/trace/`
Expected: FAIL — `port.ReportTraceEventCommand` 无 `ParentSessionID`/`AgentType` 字段。

- [ ] **Step 3: 实现**

`internal/application/trace/port/handler.go`：

`ReportTraceEventCommand` 增加：
```go
type ReportTraceEventCommand struct {
	SessionID      string
	ParentSessionID string
	Agent          string
	Model          string
	CWD            string
	Source         string
	AgentID        string
	AgentType      string
	APIKeyName     string
	UserID         uint
	Records        []ReportTraceRecord
}
```

`TraceSummaryView` 与 `TraceDetailView` 各增加 `ParentTraceID uint`（`ID` 之后）。

`internal/dto/trace.go`：

`ReportTraceEventReqBody` 增加：
```go
	SessionID      string `json:"session_id" required:"true" minLength:"1" doc:"agent session id"`
	ParentSessionID string `json:"parent_session_id,omitempty" doc:"父 session id（子代理上报）"`
	Agent          string `json:"agent,omitempty" enum:"codex,claude" doc:"agent 类型（默认 codex）"`
	AgentID        string `json:"agent_id,omitempty" doc:"子代理 id（SubagentStop hook 输入）"`
	AgentType      string `json:"agent_type,omitempty" doc:"子代理类型（SubagentStop hook 输入）"`
	Model          string `json:"model,omitempty" doc:"模型"`
```

`TraceSummary` 与 `TraceDetail` 各增加：
```go
	ParentTraceID uint   `json:"parentTraceId" doc:"父 trace ID，0 表示主 trace"`
```
（放 `SessionID` 字段后）

`internal/handler/trace.go`：

`HandleReportTraceEvent` 的 `cmd := port.ReportTraceEventCommand{...}` 增加：
```go
	cmd := port.ReportTraceEventCommand{
		SessionID:       req.Body.SessionID,
		ParentSessionID: req.Body.ParentSessionID,
		Agent:           req.Body.Agent,
		AgentID:         req.Body.AgentID,
		AgentType:       req.Body.AgentType,
		Model:           req.Body.Model,
		...
```

`HandleListTraces` 的 `TraceSummary` 构造加 `ParentTraceID: item.ParentTraceID`；`HandleGetTrace` 的 `TraceDetail` 构造加 `ParentTraceID: view.ParentTraceID`。

`internal/application/trace/query/list_traces.go`（`TraceSummaryView` 构造处）与 `get_trace.go`（`TraceDetailView` 构造处）各加 `ParentTraceID: item.ParentTraceID` / `ParentTraceID: item.ParentTraceID`。

- [ ] **Step 4: 运行确认通过**

Run: `go test -v -count=1 ./test/unit/trace/`
Expected: PASS（新增测试通过，`dto_convention_test.go` 兼容新字段）

- [ ] **Step 5: Commit**

```bash
git add internal/application/trace/port/handler.go internal/dto/trace.go internal/handler/trace.go internal/application/trace/query/list_traces.go internal/application/trace/query/get_trace.go test/unit/trace/usecase_test.go
git commit -m "feat(trace): 上报传输链路增加 parent_session_id/agent 字段与 parentTraceId 视图字段"
```

---

### Task 3: 服务端子代理关联与 source/metadata 逻辑

**Files:**
- Modify: `internal/application/trace/command/report_trace_event.go`
- Test: `test/unit/trace/usecase_test.go`

**Interfaces:**
- Consumes: Task 1 `trace.Trace.ParentTraceID`、Task 2 `cmd.ParentSessionID/AgentID/AgentType`
- Produces: 子 trace 创建时 `ParentTraceID` 关联、`Source="subagent"`、`Metadata["agent_type"]`；父 trace 缺失时容错（子 trace 照常创建，parent=0）

- [ ] **Step 1: 写失败测试**

在 `test/unit/trace/usecase_test.go` 追加：

```go
func TestReportTraceEvent_SubagentChildMetadataAndDone(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "parent-s2", APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
			HookEventName: "SessionStart", DedupKey: "hook:p2:1",
			Payload: []byte(`{"hook_event_name":"SessionStart","session_id":"parent-s2"}`),
		}},
	}); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "child-s2", ParentSessionID: "parent-s2", AgentID: "agent-1", AgentType: "worker",
		APIKeyName: "key1", UserID: 1, Model: "gpt-5", CWD: "/work",
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg,
			Event: "task_complete", TurnID: "t1", DedupKey: "rollout:child-s2:1",
			Payload: []byte(`{"type":"event_msg","payload":{"type":"task_complete"}}`),
		}},
	}); err != nil {
		t.Fatalf("report subagent: %v", err)
	}

	child, _ := repo.FindBySessionID(ctx, "child-s2")
	if child == nil {
		t.Fatal("child trace missing")
	}
	if child.Source != "subagent" {
		t.Fatalf("expected child Source=subagent, got %q", child.Source)
	}
	if child.Metadata["agent_type"] != "worker" || child.Metadata["agent_id"] != "agent-1" {
		t.Fatalf("unexpected child metadata: %+v", child.Metadata)
	}
	if child.Model != "gpt-5" || child.CWD != "/work" {
		t.Fatalf("unexpected child model/cwd: %+v", child)
	}
	if child.Status != constant.TraceStatusDone {
		t.Fatalf("expected child done via task_complete, got %s", child.Status)
	}
}

func TestReportTraceEvent_SubagentMissingParentIsTolerant(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	handler := command.NewReportTraceEventHandler(repo)
	ctx := context.Background()

	if _, err := handler.Handle(ctx, port.ReportTraceEventCommand{
		SessionID: "orphan-child", ParentSessionID: "no-such-parent", APIKeyName: "key1", UserID: 1,
		Records: []port.ReportTraceRecord{{
			Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg,
			Event: "task_complete", DedupKey: "rollout:orphan:1",
			Payload: []byte(`{"type":"event_msg","payload":{"type":"task_complete"}}`),
		}},
	}); err != nil {
		t.Fatalf("orphan subagent should not error: %v", err)
	}
	child, _ := repo.FindBySessionID(ctx, "orphan-child")
	if child == nil || child.ParentTraceID != 0 {
		t.Fatalf("expected orphan child with ParentTraceID=0, got %+v", child)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test -v -count=1 -run 'TestReportTraceEvent_Subagent' ./test/unit/trace/`
Expected: FAIL — child 的 `Source`/`Metadata`/`ParentTraceID` 断言不满足。

- [ ] **Step 3: 实现**

`internal/application/trace/command/report_trace_event.go`：

`ensureTrace` 开头解析父 trace：

```go
func (h *reportTraceEventHandler) ensureTrace(
	ctx context.Context,
	cmd port.ReportTraceEventCommand,
	agent string,
) (*trace.Trace, error) {
	parentTraceID := uint(0)
	if cmd.ParentSessionID != "" && cmd.ParentSessionID != cmd.SessionID {
		if parent, err := h.repo.FindBySessionID(ctx, cmd.ParentSessionID); err == nil && parent != nil {
			parentTraceID = parent.ID
		}
	}
	t, err := h.repo.FindBySessionID(ctx, cmd.SessionID)
	...
```

创建分支改为：

```go
	if t == nil {
		source := cmd.Source
		metadata := map[string]string{}
		if cmd.ParentSessionID != "" {
			source = constant.TraceSourceSubagent
			if cmd.AgentID != "" {
				metadata[constant.TraceMetadataAgentID] = cmd.AgentID
			}
			if cmd.AgentType != "" {
				metadata[constant.TraceMetadataAgentType] = cmd.AgentType
			}
		}
		return h.repo.UpsertBySessionID(ctx, &trace.Trace{
			Agent: agent, SessionID: cmd.SessionID, ParentTraceID: parentTraceID,
			APIKeyName: cmd.APIKeyName, UserID: cmd.UserID, Model: cmd.Model, CWD: cmd.CWD,
			Source: source, Status: constant.TraceStatusActive, Metadata: metadata,
		})
	}
```

更新分支（`t != nil`）保持现状（子 trace 通常一次创建；如需更新 parent 关联，`UpsertBySessionID` 的 OnConflict 已含 `FieldParentTraceID`）。

新增常量（`internal/common/constant/sql.go` 的 trace 常量区）：

```go
	TraceSourceSubagent        = "subagent"
	TraceMetadataAgentID       = "agent_id"
	TraceMetadataAgentType     = "agent_type"
```

- [ ] **Step 4: 运行确认通过**

Run: `go test -v -count=1 -run 'TestReportTraceEvent' ./test/unit/trace/`
Expected: PASS（全部上报用例，含新增两个子代理用例）

- [ ] **Step 5: Commit**

```bash
git add internal/application/trace/command/report_trace_event.go internal/common/constant/sql.go test/unit/trace/usecase_test.go
git commit -m "feat(trace): 子代理 trace 创建时关联父 trace 并写入 source/metadata"
```

---

### Task 4: 客户端 hook 注册收敛

**Files:**
- Modify: `internal/common/constant/traceclient.go`

**Interfaces:**
- Produces: `TraceClientCodexHookEvents = [SessionStart, Stop, SubagentStop]`（已安装机器重跑 install 自动收敛）

- [ ] **Step 1: 实现（纯常量改动，无独立测试；由构建 + 后续任务测试验证）**

`internal/common/constant/traceclient.go`：

```go
// TraceClientCodexHookEvents aris hook 需要注册的 codex 事件
var TraceClientCodexHookEvents = []string{
	TraceEventSessionStart,
	TraceEventStop,
	TraceEventSubagentStop,
}
```

同文件常量区追加（Task 8 使用，一并加）：

```go
	TraceClientRolloutFileSuffix      = ".jsonl"
	TraceClientSessionMetaDedupFormat = "rollout:%s:session_meta:%s"
```

- [ ] **Step 2: 构建验证**

Run: `go build ./cmd/client ./cmd/server`
Expected: 成功，无错误。

- [ ] **Step 3: Commit**

```bash
git add internal/common/constant/traceclient.go
git commit -m "feat(trace): codex hook 收敛为 SessionStart/Stop/SubagentStop"
```

---

### Task 5: 客户端 ParseHook 扩展 + 子代理 id 解析

**Files:**
- Modify: `internal/client/trace/adapter.go`
- Modify: `internal/client/trace/codex.go`
- Test: `test/unit/trace/client_subagent_test.go`（新建）

**Interfaces:**
- Consumes: Task 4 常量
- Produces: `HookInfo.AgentTranscriptPath/AgentID/AgentType`；导出 `SubagentSessionIDFromPath(transcriptPath string) string`（从 rollout 文件名解析子代理 session id，失败返回 ""）

- [ ] **Step 1: 写失败测试**

新建 `test/unit/trace/client_subagent_test.go`：

```go
package trace

import (
	"testing"

	traceclient "github.com/hcd233/aris-proxy-api/internal/client/trace"
)

func TestSubagentSessionIDFromPath_ParsesRolloutFilename(t *testing.T) {
	t.Parallel()
	cases := []struct {
		path string
		want string
	}{
		{"/Users/x/.codex/sessions/2026/07/17/rollout-2026-07-17T15-52-57-019f6f10-7524-7891-96a2-fe7aa659430c.jsonl", "019f6f10-7524-7891-96a2-fe7aa659430c"},
		{"rollout-2026-07-17T15-52-57-abc.jsonl", "abc"},
		{"", ""},
		{"/tmp/plain.txt", ""},
	}
	for _, c := range cases {
		if got := traceclient.SubagentSessionIDFromPath(c.path); got != c.want {
			t.Fatalf("SubagentSessionIDFromPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test -v -count=1 -run TestSubagentSessionIDFromPath ./test/unit/trace/`
Expected: FAIL — `SubagentSessionIDFromPath` 未定义。

- [ ] **Step 3: 实现**

`internal/client/trace/adapter.go` 的 `HookInfo` 增加：

```go
type HookInfo struct {
	SessionID          string
	EventName          string
	Model              string
	CWD                string
	SessionSource      string
	TurnID             string // codex: turn_id；claude: prompt_id
	CallID             string // 工具调用关联 ID（tool_use_id）
	TranscriptPath     string
	AgentTranscriptPath string // SubagentStop: 子代理 transcript 路径
	AgentID            string // SubagentStop: 子代理 id
	AgentType          string // SubagentStop: 子代理类型
}
```

`internal/client/trace/codex.go`：

`codexHookEnvelope` 增加字段：

```go
type codexHookEnvelope struct {
	HookEventName      string `json:"hook_event_name"`
	SessionID          string `json:"session_id"`
	Model              string `json:"model,omitempty"`
	CWD                string `json:"cwd,omitempty"`
	Source             string `json:"source,omitempty"`
	TurnID             string `json:"turn_id,omitempty"`
	ToolUseID          string `json:"tool_use_id,omitempty"`
	TranscriptPath     string `json:"transcript_path,omitempty"`
	AgentTranscriptPath string `json:"agent_transcript_path,omitempty"`
	AgentID            string `json:"agent_id,omitempty"`
	AgentType          string `json:"agent_type,omitempty"`
}
```

`ParseHook` 返回增加：

```go
	return HookInfo{
		SessionID: env.SessionID, EventName: env.HookEventName, Model: env.Model,
		CWD: env.CWD, SessionSource: env.Source, TurnID: env.TurnID,
		CallID: env.ToolUseID, TranscriptPath: env.TranscriptPath,
		AgentTranscriptPath: env.AgentTranscriptPath, AgentID: env.AgentID, AgentType: env.AgentType,
	}, nil
```

新增导出函数（`internal/client/trace/codex.go` 文件内）：

```go
// SubagentSessionIDFromPath 从子代理 transcript 文件名 rollout-<ts>-<session_id>.jsonl 解析 session id；
// 路径为空或格式不匹配返回 ""。
func SubagentSessionIDFromPath(transcriptPath string) string {
	if transcriptPath == "" {
		return ""
	}
	base := filepath.Base(transcriptPath)
	base = strings.TrimSuffix(base, constant.TraceClientRolloutFileSuffix)
	idx := strings.LastIndex(base, "-")
	if idx < 0 || idx == len(base)-1 {
		return ""
	}
	return base[idx+1:]
}
```

codex.go 增加 import：`"path/filepath"`、`"strings"`。

- [ ] **Step 4: 运行确认通过**

Run: `go test -v -count=1 ./test/unit/trace/ && go build ./cmd/client`
Expected: PASS + 构建成功。

- [ ] **Step 5: Commit**

```bash
git add internal/client/trace/adapter.go internal/client/trace/codex.go test/unit/trace/client_subagent_test.go
git commit -m "feat(trace): ParseHook 支持 agent_transcript_path 与子代理 id 解析"
```

---

### Task 6: 客户端 Stop 记录 payload 裁剪

**Files:**
- Modify: `internal/client/trace/ingest.go`
- Test: `test/unit/trace/client_stop_test.go`（新建）

**Interfaces:**
- Produces: 导出 `TrimStopHookPayload(raw []byte) []byte`（删除 `last_assistant_message` 键；解析失败原样返回）

- [ ] **Step 1: 写失败测试**

新建 `test/unit/trace/client_stop_test.go`：

```go
package trace

import (
	"bytes"
	"testing"

	"github.com/bytedance/sonic"
	traceclient "github.com/hcd233/aris-proxy-api/internal/client/trace"
)

func TestTrimStopHookPayload_RemovesLastAssistantMessage(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"session_id":"s1","hook_event_name":"Stop","turn_id":"t1","stop_hook_active":false,"last_assistant_message":"long text..."}`)
	trimmed := traceclient.TrimStopHookPayload(raw)

	var m map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(trimmed, &m); err != nil {
		t.Fatalf("trimmed payload invalid json: %v", err)
	}
	if len(m[constant.TracePayloadFieldLastMessage]) != 0 {
		t.Fatal("last_assistant_message must be removed")
	}
	if len(m["session_id"]) == 0 || len(m["hook_event_name"]) == 0 {
		t.Fatal("identity fields must be kept")
	}
}

func TestTrimStopHookPayload_InvalidJSONPassesThrough(t *testing.T) {
	t.Parallel()
	raw := []byte(`not-json`)
	if !bytes.Equal(traceclient.TrimStopHookPayload(raw), raw) {
		t.Fatal("invalid json must pass through unchanged")
	}
}
```

（测试需要 `constant` import：`"github.com/hcd233/aris-proxy-api/internal/common/constant"`）

- [ ] **Step 2: 运行确认失败**

Run: `go test -v -count=1 -run TestTrimStopHookPayload ./test/unit/trace/`
Expected: FAIL — `TrimStopHookPayload` 未定义。

- [ ] **Step 3: 实现**

`internal/client/trace/ingest.go` 新增导出函数：

```go
// TrimStopHookPayload 裁剪 Stop hook 的 payload：删除 last_assistant_message 全文
// （与 rollout agent_message 重复），保留身份与状态小字段。解析失败原样返回。
func TrimStopHookPayload(raw []byte) []byte {
	var envelope map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(raw, &envelope); err != nil {
		return raw
	}
	delete(envelope, constant.TracePayloadFieldLastMessage)
	trimmed, err := sonic.Marshal(envelope)
	if err != nil {
		return raw
	}
	return trimmed
}
```

`Ingest()` 构造 `record` 前对 Stop 事件裁剪（`if info.SessionID == "" ...` 校验之后）：

```go
	payload := raw
	if info.EventName == constant.TraceEventStop {
		payload = TrimStopHookPayload(raw)
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
		Payload:        append(sonic.NoCopyRawMessage{}, payload...),
	}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test -v -count=1 ./test/unit/trace/ && go build ./cmd/client`
Expected: PASS + 构建成功。

- [ ] **Step 5: Commit**

```bash
git add internal/client/trace/ingest.go test/unit/trace/client_stop_test.go
git commit -m "feat(trace): Stop hook 记录裁剪 last_assistant_message 消除双源重复"
```

---

### Task 7: 客户端 SubagentStop 上报流程

**Files:**
- Modify: `internal/client/trace/spool.go`
- Modify: `internal/client/trace/ingest.go`
- Test: `test/unit/trace/client_subagent_ingest_test.go`（新建）

**Interfaces:**
- Consumes: Task 5 `HookInfo.AgentTranscriptPath/AgentID/AgentType`、`SubagentSessionIDFromPath`
- Produces: `PendingRecord.ParentSessionID`；`ingestBatch.ParentSessionID/AgentID/AgentType`；SubagentStop 触发时：不生成 hook 记录，读取子代理 transcript（SessionID=子代理 id），每条记录携带父 session_id

- [ ] **Step 1: 写失败测试**（集成：真实 Ingestor + httptest 服务端）

新建 `test/unit/trace/client_subagent_ingest_test.go`：

```go
package trace

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/sonic"
	traceclient "github.com/hcd233/aris-proxy-api/internal/client/trace"
)

// TestIngestSubagentStop_ReportsChildTranscriptWithParentSession 验证 SubagentStop
// hook 触发时：只上报子代理 transcript（SessionID=子代理 id），batch 携带父 session_id。
func TestIngestSubagentStop_ReportsChildTranscriptWithParentSession(t *testing.T) {
	t.Parallel()
	paths := traceclient.Paths{Root: t.TempDir()}
	childID := "019f6f10-7524-7891-96a2-fe7aa659430c"
	transcriptDir := filepath.Join(t.TempDir(), "sessions")
	transcriptPath := filepath.Join(transcriptDir, "rollout-2026-07-17T15-52-57-"+childID+".jsonl")
	if err := os.MkdirAll(transcriptDir, 0o700); err != nil {
		t.Fatal(err)
	}
	meta := `{"type":"session_meta","payload":{"id":"` + childID + `","session_id":"` + childID + `","source":"vscode"}}` + "\n"
	complete := `{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t1"}}` + "\n"
	if err := os.WriteFile(transcriptPath, []byte(meta+complete), 0o600); err != nil {
		t.Fatal(err)
	}

	adapter, err := traceclient.LookupAdapter("codex")
	if err != nil {
		t.Fatal(err)
	}
	var gotBody traceclient.IngestBatchJSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := sonic.Unmarshal(body, &gotBody); err != nil {
			t.Errorf("bad ingest body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[{"dedupKey":"rollout:` + childID + `:session_meta:` + childID + `","status":"accepted"}]}`))
	}))
	defer server.Close()

	cfg := traceclient.Config{Host: server.URL, APIKey: "k"}
	configStore := traceclient.NewConfigStore(paths)
	if err := configStore.Save(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	ing := traceclient.NewIngestor(paths, server.Client(), adapter)
	hookInput := []byte(`{"session_id":"parent-s1","hook_event_name":"SubagentStop","turn_id":"t1","agent_id":"agent-1","agent_type":"worker","agent_transcript_path":"` + transcriptPath + `"}`)
	if err := ing.Ingest(context.Background(), hookInput); err != nil {
		t.Fatalf("ingest subagent stop: %v", err)
	}

	if gotBody.ParentSessionID != "parent-s1" {
		t.Fatalf("expected ParentSessionID=parent-s1, got %q", gotBody.ParentSessionID)
	}
	if gotBody.SessionID != childID {
		t.Fatalf("expected SessionID=%s, got %q", childID, gotBody.SessionID)
	}
	if len(gotBody.Records) != 2 {
		t.Fatalf("expected 2 rollout records, got %d", len(gotBody.Records))
	}
	for _, rec := range gotBody.Records {
		if rec.Source != "rollout" {
			t.Fatalf("expected all records source=rollout, got %q", rec.Source)
		}
	}
}
```

> 说明：`traceclient.IngestBatchJSON` 是计划要求客户端导出的测试辅助结构，见 Step 3；`Config` 已导出（`internal/client/trace/config.go`）。

- [ ] **Step 2: 运行确认失败**

Run: `go test -v -count=1 -run TestIngestSubagentStop ./test/unit/trace/`
Expected: FAIL — `IngestBatchJSON` 未定义 / `ParentSessionID` 行为未实现。

- [ ] **Step 3: 实现**

`internal/client/trace/spool.go` 的 `PendingRecord` 增加：

```go
type PendingRecord struct {
	SessionID      string                 `json:"session_id"`
	ParentSessionID string                `json:"parent_session_id,omitempty"`
	Agent          string                 `json:"agent,omitempty"`
	AgentID        string                 `json:"agent_id,omitempty"`
	AgentType      string                 `json:"agent_type,omitempty"`
	...
```

`internal/client/trace/ingest.go`：

`ingestBatch` 增加字段：

```go
type ingestBatch struct {
	SessionID       string         `json:"session_id"`
	ParentSessionID string         `json:"parent_session_id,omitempty"`
	Agent           string         `json:"agent,omitempty"`
	AgentID         string         `json:"agent_id,omitempty"`
	AgentType       string         `json:"agent_type,omitempty"`
	Model           string         `json:"model,omitempty"`
	CWD             string         `json:"cwd,omitempty"`
	Source          string         `json:"source,omitempty"`
	Records         []ingestRecord `json:"records"`
}

// IngestBatchJSON 导出视图，供外部测试断言上报请求体。
type IngestBatchJSON struct {
	SessionID       string         `json:"session_id"`
	ParentSessionID string         `json:"parent_session_id,omitempty"`
	Agent           string         `json:"agent,omitempty"`
	AgentID         string         `json:"agent_id,omitempty"`
	AgentType       string         `json:"agent_type,omitempty"`
	Records         []ingestRecord `json:"records"`
}
```

> 注意：`ingestRecord` 未导出，测试只能断言数量与 `Source` 等 JSON 字段。为让测试断言 records 内容，将 `ingestRecord` 的字段保持 JSON tag 不变，测试断言 `gotBody.Records[i].Source` 即可（同包不同目录无法访问未导出字段——因此 `IngestBatchJSON.Records` 类型改为导出视图）。改为：

```go
// IngestBatchJSON 导出视图，供外部测试断言上报请求体。
type IngestBatchJSON struct {
	SessionID       string         `json:"session_id"`
	ParentSessionID string         `json:"parent_session_id,omitempty"`
	Agent           string         `json:"agent,omitempty"`
	AgentID         string         `json:"agent_id,omitempty"`
	AgentType       string         `json:"agent_type,omitempty"`
	Records         []struct {
		Source     string `json:"source"`
		RecordType string `json:"record_type"`
		Event      string `json:"event"`
		SessionID  string `json:"session_id"`
	} `json:"records"`
}
```

`Ingest()` 的 SubagentStop 分支（在 `nextSequence` 之后、spool 追加 hook 记录**之前**插入；注意 Stop 裁剪逻辑保持）：

```go
	spoolID, sequence, err := nextSequence(ctx, i.paths)
	if err != nil {
		return err
	}
	if info.EventName == constant.TraceEventSubagentStop {
		return i.ingestSubagentStop(ctx, info)
	}
	payload := raw
	if info.EventName == constant.TraceEventStop {
		payload = TrimStopHookPayload(raw)
	}
	record := PendingRecord{... 原逻辑 ...}
```

新增方法：

```go
// ingestSubagentStop 处理 SubagentStop hook：不生成 hook 记录，读取子代理
// transcript 增量（SessionID=子代理 id），每条记录携带父 session_id。
func (i *Ingestor) ingestSubagentStop(ctx context.Context, info HookInfo) error {
	if info.AgentTranscriptPath == "" {
		return nil // 无子代理 transcript，无数据可上报
	}
	childID := SubagentSessionIDFromPath(info.AgentTranscriptPath)
	if childID == "" {
		return nil
	}
	if _, err := i.rollout.ReadNew(ctx, childID, info.AgentTranscriptPath); err != nil {
		writeLocalError(i.paths, constant.TraceClientLogCategoryRollout)
		return nil // fail-open
	}
	config, err := i.config.Load(ctx)
	if err != nil {
		return err
	}
	if config.Host == "" || config.APIKey == "" {
		return ierr.New(ierr.ErrValidation, "trace client is not initialized")
	}
	return i.flushSubagent(ctx, config, childID, info)
}

// flushSubagent 上报子代理批次：从 spool 取 childID 对应记录，batch 携带父 session 与 agent 元数据。
func (i *Ingestor) flushSubagent(ctx context.Context, config Config, childID string, info HookInfo) error {
	batch, err := i.spool.Batch(ctx, constant.TraceClientBatchMaxRecords, constant.TraceClientBatchMaxBytes)
	if err != nil || len(batch) == 0 {
		return err
	}
	// spool.Batch 按首条 SessionID 分组；子代理记录 SessionID=childID，直接构造请求。
	request := ingestBatch{
		SessionID:       childID,
		ParentSessionID: info.SessionID,
		Agent:           i.adapter.Name(),
		AgentID:         info.AgentID,
		AgentType:       info.AgentType,
		Records:         make([]ingestRecord, 0, len(batch)),
	}
	for _, record := range batch {
		if record.SessionID != childID {
			continue // spool 可能残留其他会话记录，只发 childID 对应的子代理记录
		}
		if request.Model == "" && record.Model != "" {
			request.Model = record.Model
		}
		if request.CWD == "" && record.CWD != "" {
			request.CWD = record.CWD
		}
		request.Records = append(request.Records, ingestRecord{
			Source: record.Source, RecordType: record.RecordType, HookEventName: record.Event,
			TurnID: record.TurnID, CallID: record.CallID, TranscriptLine: record.TranscriptLine,
			ClientSequence: record.ClientSequence, DedupKey: record.DedupKey, Payload: record.Payload,
		})
	}
	return i.postBatch(ctx, config, request)
}

// postBatch 发送 ingest 请求并确认 spool（抽取自原 flush 的公共尾部）。
func (i *Ingestor) postBatch(ctx context.Context, config Config, request ingestBatch) error {
	body, err := sonic.Marshal(request)
	if err != nil {
		return ierr.Wrap(ierr.ErrDTOMarshal, err, "encode trace ingest request")
	}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, config.Host+constant.TraceClientIngestPath, bytes.NewReader(body),
	)
	if err != nil {
		return ierr.Wrap(ierr.ErrBadRequest, err, "create trace ingest request")
	}
	req.Header.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
	req.Header.Set(constant.HTTPHeaderAuthorization, constant.HTTPAuthBearerPrefix+config.APIKey)
	resp, err := i.client.Do(req)
	if err != nil {
		return ierr.Wrap(ierr.ErrProxySend, err, "send trace ingest request")
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ierr.New(ierr.ErrBadRequest, "trace ingest request rejected")
	}
	var response ingestResultEnvelope
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&response); err != nil {
		return ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode trace ingest response")
	}
	if len(response.Results) == 0 {
		return ierr.New(ierr.ErrBadRequest, "trace ingest response has no results")
	}
	return i.spool.Acknowledge(ctx, response.Results)
}
```

重构原 `flush`：其发送/确认尾部替换为 `return i.postBatch(ctx, config, request)`（request 构造逻辑保留原样）。

- [ ] **Step 4: 运行确认通过**

Run: `go test -v -count=1 ./test/unit/trace/ && go build ./cmd/client ./cmd/server`
Expected: PASS + 构建成功。

- [ ] **Step 5: Commit**

```bash
git add internal/client/trace/spool.go internal/client/trace/ingest.go test/unit/trace/client_subagent_ingest_test.go
git commit -m "feat(trace): SubagentStop 上报子代理 transcript 并携带父 session 关联"
```

---

### Task 8: session_meta dedup key 稳定化

**Files:**
- Modify: `internal/client/trace/adapter.go`
- Modify: `internal/client/trace/codex.go`
- Modify: `internal/client/trace/rollout.go`
- Test: `test/unit/trace/client_rollout_dedup_test.go`（新建）

**Interfaces:**
- Consumes: Task 4 `TraceClientSessionMetaDedupFormat`
- Produces: `TranscriptMeta.SessionID`（session_meta 的 payload.id）；导出 `RolloutDedupKey(sessionID string, meta TranscriptMeta, line int64, raw []byte) string`（session_meta 用稳定键，其余保持 line:hash）

- [ ] **Step 1: 写失败测试**

新建 `test/unit/trace/client_rollout_dedup_test.go`：

```go
package trace

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	traceclient "github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestRolloutDedupKey_SessionMetaUsesStableID(t *testing.T) {
	t.Parallel()
	childID := "019f6f10-7524-7891-96a2-fe7aa659430c"
	raw := []byte(`{"type":"session_meta","payload":{"id":"` + childID + `","source":"vscode"}}`)
	meta := traceclient.TranscriptMeta{RecordType: constant.TraceRolloutTypeSessionMeta, SessionID: childID}

	key1 := traceclient.RolloutDedupKey("s1", meta, 1, raw)
	key2 := traceclient.RolloutDedupKey("s1", meta, 205, raw) // 压缩重写后行号变化
	if key1 != key2 {
		t.Fatalf("session_meta dedup key must be line-independent: %q vs %q", key1, key2)
	}
	want := "rollout:s1:session_meta:" + childID
	if key1 != want {
		t.Fatalf("expected %q, got %q", want, key1)
	}
}

func TestRolloutDedupKey_OtherRecordsKeepLineAndHash(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"type":"response_item","payload":{"type":"function_call"}}`)
	meta := traceclient.TranscriptMeta{RecordType: constant.TraceRolloutTypeResponseItem, Event: "function_call"}
	digest := sha256.Sum256(raw)
	want := "rollout:s1:12:" + hex.EncodeToString(digest[:])
	got := traceclient.RolloutDedupKey("s1", meta, 12, raw)
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test -v -count=1 -run TestRolloutDedupKey ./test/unit/trace/`
Expected: FAIL — `RolloutDedupKey` 未定义。

- [ ] **Step 3: 实现**

`internal/client/trace/adapter.go` 的 `TranscriptMeta` 增加：

```go
type TranscriptMeta struct {
	RecordType string
	Event      string
	TurnID     string
	CallID     string
	SessionID  string // session_meta 的 payload.id（用于稳定 dedup key）
}
```

`internal/client/trace/codex.go`：

`codexRolloutPayload` 增加 `ID string`：

```go
type codexRolloutPayload struct {
	Type   string `json:"type"`
	ID     string `json:"id,omitempty"`
	TurnID string `json:"turn_id,omitempty"`
	CallID string `json:"call_id,omitempty"`
	...
```

`ClassifyTranscriptLine` 返回填充：

```go
	return TranscriptMeta{
		RecordType: codexRolloutRecordType(envelope.Type),
		Event:      payload.Type,
		TurnID:     payload.turnID(),
		CallID:     payload.CallID,
		SessionID:  payload.ID,
	}
```

`internal/client/trace/rollout.go`：

新增导出函数并让 `rolloutRecord` 使用：

```go
// RolloutDedupKey 生成 rollout 记录 dedup key：session_meta 用稳定语义键
// （payload.id，压缩重写后行号变化不产生重复），其余记录保持 line:hash。
func RolloutDedupKey(sessionID string, meta TranscriptMeta, line int64, raw []byte) string {
	if meta.RecordType == constant.TraceRolloutTypeSessionMeta && meta.SessionID != "" {
		return fmt.Sprintf(constant.TraceClientSessionMetaDedupFormat, sessionID, meta.SessionID)
	}
	digest := sha256.Sum256(raw)
	return fmt.Sprintf(constant.TraceClientRolloutDedupFormat, sessionID, line, hex.EncodeToString(digest[:]))
}
```

`rolloutRecord` 改用：

```go
func (r *RolloutReader) rolloutRecord(sessionID string, line int64, raw []byte) PendingRecord {
	meta := r.adapter.ClassifyTranscriptLine(raw)
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
		DedupKey:       RolloutDedupKey(sessionID, meta, line, raw),
		Payload:        append(sonic.NoCopyRawMessage{}, raw...),
	}
}
```

（`sha256`/`hex` import 已在 rollout.go 中，若 `RolloutDedupKey` 使用后原 `rolloutRecord` 不再直接引用则无需改动 import。）

- [ ] **Step 4: 运行确认通过**

Run: `go test -v -count=1 ./test/unit/trace/ && go build ./cmd/client ./cmd/server`
Expected: PASS + 构建成功。

- [ ] **Step 5: Commit**

```bash
git add internal/client/trace/adapter.go internal/client/trace/codex.go internal/client/trace/rollout.go test/unit/trace/client_rollout_dedup_test.go
git commit -m "fix(trace): session_meta dedup key 改用 payload.id 稳定键，消除压缩重写重复存储"
```

---

### Task 9: 全量验证

**Files:** 无代码改动

- [ ] **Step 1: 全量测试**

Run: `make test`
Expected: 全部 PASS。

- [ ] **Step 2: 构建**

Run: `make build-client && make build-server`
Expected: 成功。

- [ ] **Step 3: 部署后手工验证（可选）**

本机 codex 会话跑一轮（含真实子代理场景），确认：
- `traces` 出现主 trace 与 `parent_trace_id` 非 0 的子 trace，子 trace `source='subagent'`、`metadata.agent_type` 有值；
- 无 hook 来源的 PreToolUse/PostToolUse/UserPromptSubmit 事件；Stop 事件 payload 无 `last_assistant_message`；
- 同一会话的 `session_meta` 记录 ≤ 1 条（压缩重写后不重复）。

- [ ] **Step 4: 更新记忆（serena）**

沉淀工程经验：hook 收敛方案、session_meta dedup key 根因与修复、主/子 trace 关联链路。
