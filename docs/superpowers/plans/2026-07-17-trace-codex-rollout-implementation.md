# Codex Trace 独立客户端与 Rollout 采集实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Codex Trace 的 shell + 后台 `curl` 上报替换为独立 `aris` 客户端，实现安全下载、四步初始化、可靠 Hook/Rollout 采集、幂等存储与 Trace Conversation 展示。

**Architecture:** 服务端命令迁入 `cmd/server`，客户端从 `cmd/client` 独立构建且只暴露 `aris trace init` / `aris trace ingest`。Web 使用 JWT 换取 Redis 中 10 分钟有效的单次下载票据；客户端先把 Hook 和 rollout 增量持久化到本地 spool，再批量上报。服务端保存完整原始记录并生成只读 Conversation projection。

**Tech Stack:** Go 1.25.1、Cobra、Fiber/Huma、GORM/PostgreSQL、Redis、Bytedance Sonic、`golang.org/x/term`、`golang.org/x/sys/unix`、Next.js 16、React 19、TypeScript、Tailwind v4。

## Global Constraints

- 保留当前未提交的 `internal/dto/schema/schema.go`、`internal/dto/trace.go`、`test/unit/trace/dto_convention_test.go`、`test/e2e/trace/hook_test.go`，只在对应任务中演进。
- `cmd/server` 构建 `aris-proxy-api` 并保留 `server`、`database`、`object`、`lint`；`cmd/client` 构建 `aris` 且只含 `trace init`、`trace ingest`。
- 客户端只支持 `darwin/amd64`、`darwin/arm64`、`linux/amd64`、`linux/arm64`，全部 `CGO_ENABLED=0`。
- 下载票据 TTL 为 10 分钟，只能原子消费一次；签发令牌桶容量 3、补充周期 1 分钟、维度为 JWT `userID`。
- API Key 只保存于 `~/.aris/trace/config.json`；目录 `0700`、文件 `0600`，不得进入 Hook 参数、日志、payload 或错误消息。
- `trace ingest` 必须 fail-open：所有运行时错误退出 `0`；非 `Stop` stdout 为空，`Stop` stdout 严格为 `{}`。
- pending 单批最多 500 条且不超过 4 MiB，网络总超时 5 秒；全局硬上限 256 MiB，不删除未确认记录。
- Hook 与 rollout 原始 JSON 必须完整保存；DTO 任意 JSON 使用 `schema.RawJSON` 或 `sonic.NoCopyRawMessage`，禁止 `any`、`interface{}`、`encoding/json`、`json.RawMessage`。
- Go 业务错误使用 `ierr`；Context 从调用方传递；日志不得输出密钥或完整 Trace payload。
- Go 测试只放 `test/unit/<topic>/` 或 `test/e2e/<topic>/`，只用标准库 `testing` 与 Sonic，禁止 testify、gomock、`time.Sleep`。
- 前端请求统一走 `web/src/lib/api-client.ts`；保持 static export 与 `/web` basePath。
- 新领域词汇同步更新 `CONTEXT.md` 与 `web/CONTEXT.md`。
- 不提交、推送或部署，除非用户后续明确要求。

## 文件结构与职责

```text
cmd/server/                          服务端入口与存量 Cobra 命令
cmd/client/                          独立 aris 客户端入口和 trace 命令树
internal/tracecli/                   配置、初始化、Codex Hook、spool、rollout、HTTP 上报
internal/application/trace/          批量 ingest、下载票据、Conversation 查询
internal/domain/trace/               Raw record、rollout parser、Conversation projection
internal/infrastructure/cache/       Redis 单次下载票据
internal/infrastructure/traceclient/ 四平台产物白名单解析
internal/router/trace.go             JWT、API Key、ticket 三类 Trace 路由
web/src/components/trace-install-dialog.tsx
                                     短安装脚本与按需票据签发
web/src/app/(dashboard)/trace/       Conversation / Raw records 展示
```

---

### Task 1: 拆分 Server 命令入口并保持存量行为

**Files:**
- Move: `cmd/root.go` → `cmd/server/root.go`
- Move: `cmd/server.go` → `cmd/server/server.go`
- Move: `cmd/database.go` → `cmd/server/database.go`
- Move: `cmd/object.go` → `cmd/server/object.go`
- Move: `cmd/lint.go` → `cmd/server/lint.go`
- Create: `cmd/server/main.go`
- Delete: `main.go`
- Modify: `Makefile`
- Modify: `docker/dockerfile`
- Modify: `docs/agents/architecture.md`
- Modify: `docs/agents/commands.md`
- Create: `test/unit/tracecli/command_tree_test.go`

**Interfaces:**
- Produces: `go run ./cmd/server <existing command>`.
- Preserves: `server start`、`database migrate`、`object bucket create`、`lint conv`、`lint static`。

- [ ] **Step 1: Write the failing command-tree test**

Create `test/unit/tracecli/command_tree_test.go`:

```go
package tracecli

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil { t.Fatal(err) }
	return root
}

func runGo(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "go", args...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil { t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out) }
	return string(out)
}

func TestServerCommandTreePreservesExistingCommands(t *testing.T) {
	out := runGo(t, "run", "./cmd/server", "--help")
	for _, name := range []string{"server", "database", "object", "lint"} {
		if !strings.Contains(out, name) { t.Errorf("help missing %q:\n%s", name, out) }
	}
}
```

- [ ] **Step 2: Run RED**

Run: `go test -count=1 ./test/unit/tracecli -run TestServerCommandTreePreservesExistingCommands`

Expected: FAIL because `./cmd/server` is not an executable directory.

- [ ] **Step 3: Move commands and create the server entrypoint**

Change moved files to `package main`. Create `cmd/server/main.go`:

```go
package main

import _ "time/tzdata"

func main() { execute() }
```

In `root.go`, rename exported `Execute` to package-local `execute` while retaining logging/exit semantics:

```go
func execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Logger().Error("[Command] failed to execute command", zap.Error(err))
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Update build and documentation references**

Set `SERVER_MAIN := ./cmd/server` in `Makefile` and use it for all server build/run/lint targets. In `docker/dockerfile`, stop copying `main.go` and build:

```dockerfile
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /go/bin/aris-proxy-api ./cmd/server \
    && upx -9 /go/bin/aris-proxy-api
```

Update docs to `cmd/server/main.go → execute()` and `go run ./cmd/server ...`.

- [ ] **Step 5: Run GREEN**

Run:

```bash
go test -count=1 ./test/unit/tracecli -run TestServerCommandTreePreservesExistingCommands
go run ./cmd/server lint conv ./internal/dto/...
go build -o /tmp/aris-proxy-api ./cmd/server
```

Expected: PASS; help includes all four groups and server builds without root `main.go`.

### Task 2: 建立 Raw Record 模型、批量 DTO 与幂等 Ingest

**Files:**
- Modify: `internal/domain/trace/repository.go`
- Modify: `internal/infrastructure/database/model/trace.go`
- Modify: `internal/infrastructure/repository/trace_repository.go`
- Modify: `internal/common/constant/sql.go`
- Modify: `internal/dto/schema/schema.go`
- Modify: `internal/dto/trace.go`
- Modify: `internal/application/trace/port/handler.go`
- Modify: `internal/application/trace/command/report_trace_event.go`
- Modify: `internal/handler/trace.go`
- Modify: `test/unit/trace/fake_repository.go`
- Modify: `test/unit/trace/repository_test.go`
- Modify: `test/unit/trace/dto_convention_test.go`
- Modify: `test/unit/trace/usecase_test.go`
- Modify: `test/e2e/trace/trace_test.go`
- Create: `test/unit/trace/fixtures/raw_records.json`

**Interfaces:**
- Produces `trace.TraceEvent{Source, RecordType, Event, TurnID, CallID, TranscriptLine, ClientSequence, DedupKey, Payload}`.
- Changes repository to `InsertEvent(ctx context.Context, e *TraceEvent) (inserted bool, err error)`.
- Produces `ReportTraceEventHandler.Handle(ctx, cmd) ([]ReportTraceRecordResult, error)` with `accepted|duplicate|rejected`.

- [ ] **Step 1: Add the raw-record fixture**

Create `test/unit/trace/fixtures/raw_records.json`:

```json
{"session_id":"session-1","records":[{"source":"hook","record_type":"hook_event","hook_event_name":"PermissionRequest","client_sequence":1,"dedup_key":"hook:spool-1:1","payload":{"session_id":"session-1","hook_event_name":"PermissionRequest","future":{"nested":true}}},{"source":"rollout","record_type":"response_item","turn_id":"turn-1","call_id":"call-1","transcript_line":42,"dedup_key":"rollout:session-1:42:hash","payload":{"type":"response_item","payload":{"type":"function_call","call_id":"call-1","extra":"keep"}}}]}
```

- [ ] **Step 2: Write failing identity, raw round-trip, and duplicate tests**

Add a repository test that inserts the same `DedupKey` twice, expects `(true,nil)` then `(false,nil)`, and verifies every identity field survives `ListEvents`. Extend `dto_convention_test.go` to unmarshal/marshal the fixture with Sonic and assert `future.nested` and `extra` remain present. Add `TestReportTraceEvent_BatchPersistsAndDeduplicates` that submits the same batch twice and expects two stored rows, not four.

Key assertion:

```go
field, ok := reflect.TypeOf(dto.ReportTraceRecordReq{}).FieldByName("Payload")
if !ok || field.Type.Name() != "RawJSON" {
	t.Fatalf("Payload must be schema.RawJSON, got %v", field.Type)
}
```

- [ ] **Step 3: Run RED**

Run: `go test -count=1 ./test/unit/trace -run 'TestFakeRepo_EventsPreserve|TestReportTrace(Event_Batch|EventReq_)'`

Expected: compile/assertion failure because new fields and batch DTOs do not exist.

- [ ] **Step 4: Extend domain/model and repository idempotency**

Use this domain shape:

```go
type TraceEvent struct {
	ID uint; TraceID uint; SessionID string
	Source string; RecordType string; Event string
	TurnID string; CallID string; TranscriptLine *int64
	ClientSequence int64; DedupKey string; Payload []byte
	CreatedAt time.Time
}
```

Add a nullable `*string` database field with unique `dedup_key` index plus `(trace_id,turn_id,id)` and `(trace_id,call_id)` indexes. Convert an empty domain key to SQL `NULL`, so legacy requests retain insert-every-time behavior; new batch records always carry a non-empty key. Insert non-empty keys with `ON CONFLICT(dedup_key) DO NOTHING`; `RowsAffected==0` returns `(false,nil)`. `FakeRepo` tracks only non-empty keys in `dedup map[string]struct{}`.

Add constants:

```go
TraceRecordSourceHook = "hook"
TraceRecordSourceRollout = "rollout"
TraceRecordTypeHookEvent = "hook_event"
TraceRecordTypeSessionMeta = "session_meta"
TraceRecordTypeTurnContext = "turn_context"
TraceRecordTypeResponseItem = "response_item"
TraceRecordTypeEventMsg = "event_msg"
TraceEventPermissionRequest = "PermissionRequest"
```

- [ ] **Step 5: Define concrete batch DTOs**

Implement:

```go
type ReportTraceRecordReq struct {
	Source string `json:"source" required:"true" enum:"hook,rollout"`
	RecordType string `json:"record_type" required:"true"`
	HookEventName string `json:"hook_event_name,omitempty"`
	TurnID string `json:"turn_id,omitempty"`
	CallID string `json:"call_id,omitempty"`
	TranscriptLine *int64 `json:"transcript_line,omitempty" minimum:"1"`
	ClientSequence int64 `json:"client_sequence,omitempty" minimum:"0"`
	DedupKey string `json:"dedup_key" required:"true" minLength:"1"`
	Payload traceschema.RawJSON `json:"payload" required:"true"`
}

type ReportTraceRecordResult struct {
	DedupKey string `json:"dedupKey"`
	Status string `json:"status" enum:"accepted,duplicate,rejected"`
	Message string `json:"message,omitempty"`
}
```

Extend `ReportTraceEventReqBody` with `Records []*ReportTraceRecordReq`, retain current legacy Hook fields, remove the current `required:"true"` tag from legacy `HookEventName`, and keep `SessionID` required. Add a named raw field populated by alias-based custom `UnmarshalJSON` so unknown legacy top-level fields are not lost. Handler validation requires either a non-empty `Records` slice or a legacy `HookEventName`. `ReportTraceEventRsp` contains `Results []*ReportTraceRecordResult`.

- [ ] **Step 6: Normalize legacy/batch input and implement partial success**

Define application `ReportTraceRecord`, `ReportTraceEventCommand{SessionID,Model,CWD,Source,Records,APIKeyName,UserID}` and `ReportTraceRecordResult` with names matching DTO semantics. Legacy input becomes one Hook record with empty `DedupKey` (stored as SQL NULL and therefore not deduplicated); batch payload bytes are copied directly, never rebuilt. Only `aris trace ingest` batch records receive at-least-once dedup guarantees.

In the command handler: ensure Trace once, iterate in order, reject invalid records without aborting later records, insert `Stop` before `MarkDone`, and never reset a done Trace when late rollout arrives. Map repository outcomes to exact statuses.

- [ ] **Step 7: Run GREEN**

Run:

```bash
go test -count=1 ./test/unit/trace
go test -count=1 ./test/e2e/trace -run TestE2E_TraceReportFlow
go run ./cmd/server lint conv ./internal/dto/... ./test/unit/trace/...
```

Expected: PASS; unknown fields survive and repeated batches produce duplicate results without duplicate rows.

### Task 3: 实现 JWT 单次票据、限流与安全客户端下载

**Files:**
- Create: `internal/infrastructure/cache/trace_client_ticket.go`
- Create: `internal/infrastructure/traceclient/artifact.go`
- Create: `internal/application/trace/command/issue_client_ticket.go`
- Modify: `internal/application/trace/port/handler.go`
- Modify: `internal/config/config.go`
- Modify: `internal/common/constant/rediskey.go`
- Modify: `internal/common/constant/ratelimit.go`
- Modify: `internal/common/constant/http.go`
- Modify: `internal/dto/trace.go`
- Create: `internal/middleware/trace_client_ticket.go`
- Modify: `internal/handler/trace.go`
- Modify: `internal/router/trace.go`
- Modify: `internal/bootstrap/modules/repository.go`
- Modify: `internal/bootstrap/modules/application.go`
- Modify: `internal/bootstrap/modules/handler.go`
- Create: `test/unit/trace/client_ticket_test.go`
- Create: `test/e2e/trace/client_download_test.go`

**Interfaces:**
- `TraceClientTicketStore.Issue(ctx,userID,ttl) (ticket string, expiresAt time.Time, err error)`.
- `TraceClientTicketStore.Consume(ctx,ticket) (userID uint, found bool, err error)`.
- Routes: `POST /api/v1/trace/client/ticket`、`GET /api/v1/trace/client`、`GET /api/v1/trace/client/check`.

- [ ] **Step 1: Write failing ticket and whitelist tests**

Use `miniredis`: issue a ticket, assert Redis keys do not contain plaintext, consume once successfully, and second consume returns `found=false`. Create four temporary artifact files and table-test allowed combinations plus `../../etc/passwd` and Windows rejection.

- [ ] **Step 2: Write failing route/limit tests**

Build a Huma/Fiber test API with injected `CtxKeyUserID=7`. Issue four tickets without advancing Redis time and expect `2xx,2xx,2xx,429`. Download with one ticket twice and expect success then 401. Assert body bytes and headers:

```text
Content-Type: application/octet-stream
Content-Disposition: attachment; filename="aris"
Cache-Control: no-store
```

- [ ] **Step 3: Run RED**

Run:

```bash
go test -count=1 ./test/unit/trace -run 'TestTraceClient(Ticket|Artifact)'
go test -count=1 ./test/e2e/trace -run TestTraceClientDownload
```

Expected: compile failure.

- [ ] **Step 4: Implement hashed atomic tickets**

Generate 32 bytes with `crypto/rand`, encode raw URL base64, hash plaintext with SHA-256, and store only `trace:client:ticket:<hex-hash> → userID` for 10 minutes. Consume with one Lua script performing `GET` then `DEL`. Add constants for ticket key, TTL, period one minute and limit three.

- [ ] **Step 5: Implement artifact resolver and configuration**

Add `config.TraceClientArtifactDir`, Viper default `/app/trace-client`, env mapping `TRACE_CLIENT_ARTIFACT_DIR`. Resolve only through:

```go
var artifactNames = map[string]string{
	"darwin/amd64": "aris-darwin-amd64",
	"darwin/arm64": "aris-darwin-arm64",
	"linux/amd64": "aris-linux-amd64",
	"linux/arm64": "aris-linux-arm64",
}
```

Whitelist before `filepath.Join`; return typed `Path`, `Filename:"aris"`, `Size`.

- [ ] **Step 6: Implement DTOs, ticket middleware and handlers**

Add `IssueTraceClientTicketRsp{CommonRsp,Ticket,ExpiresAt}` and `DownloadTraceClientReq{OS,Arch}`. Ticket middleware reads Bearer, atomically consumes, injects user ID, and returns 401 for missing/invalid/reused tickets.

Download handler opens the resolved file inside `huma.StreamResponse.Body`, sets status/headers/content length, `io.Copy`s, and closes it. API Key check is behind `APIKeyMiddleware` and returns `204` with no body.

- [ ] **Step 7: Register route groups and wire DI**

Use separate groups: JWT for queries and `POST /client/ticket`; ticket middleware for `GET /client`; API Key for `GET /client/check` and `POST /event`. Ticket issue middleware order is JWT → request limiter by `CtxKeyUserID` → user permission.

Provide ticket store and artifact resolver in RepositoryModule, issue handler in ApplicationModule, and dependencies in HandlerModule.

- [ ] **Step 8: Run GREEN**

Run:

```bash
go test -count=1 ./test/unit/trace -run 'TestTraceClient(Ticket|Artifact)'
go test -count=1 ./test/e2e/trace -run TestTraceClientDownload
go test -count=1 ./test/unit/token_rate_limiter
```

Expected: PASS; traversal never reaches filesystem and reused tickets fail.

### Task 4: 建立独立 Client 命令与四步 Init 向导

**Files:**
- Create: `cmd/client/main.go`
- Create: `cmd/client/root.go`
- Create: `cmd/client/trace.go`
- Create: `internal/tracecli/paths.go`
- Create: `internal/tracecli/config.go`
- Create: `internal/tracecli/http_client.go`
- Create: `internal/tracecli/terminal.go`
- Create: `internal/tracecli/init.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `CONTEXT.md`
- Modify: `test/unit/tracecli/command_tree_test.go`
- Create: `test/unit/tracecli/config_test.go`
- Create: `test/unit/tracecli/init_test.go`

**Interfaces:**
- Executable: `go run ./cmd/client trace init --host <origin>`.
- Config: `tracecli.Config{Host,Agent,APIKey string}` at `~/.aris/trace/config.json`.
- Consumes `GET <host>/health` and `GET <host>/api/v1/trace/client/check`.

- [ ] **Step 1: Extend the command-tree test for isolation**

Add:

```go
func TestClientCommandTreeContainsOnlyTrace(t *testing.T) {
	out := runGo(t, "run", "./cmd/client", "--help")
	if !strings.Contains(out, "trace") { t.Fatalf("missing trace:\n%s", out) }
	for _, forbidden := range []string{"server", "database", "object", "lint"} {
		if strings.Contains(out, forbidden) { t.Errorf("contains %q:\n%s", forbidden, out) }
	}
	traceOut := runGo(t, "run", "./cmd/client", "trace", "--help")
	for _, name := range []string{"init", "ingest"} {
		if !strings.Contains(traceOut, name) { t.Errorf("missing %q:\n%s", name, traceOut) }
	}
}
```

- [ ] **Step 2: Write failing private-config tests**

Use injected `Paths{Root:t.TempDir()}`; save, load, and assert directory/file modes `0700`/`0600`. Replace the config and assert no temp file remains. Force rename failure and assert the previous valid config is unchanged.

```go
want := tracecli.Config{Host:"https://example.com", Agent:"codex", APIKey:"secret"}
if err := store.Save(context.Background(), want); err != nil { t.Fatal(err) }
```

- [ ] **Step 3: Write failing four-step wizard tests**

Define testable boundaries:

```go
type Terminal interface {
	Interactive() bool
	ReadLine(prompt string) (string, error)
	ReadSecret(prompt string) (string, error)
	Printf(format string, args ...string)
}

type CodexInstaller interface {
	Install(ctx context.Context, commandPath string) (backupPath string, err error)
}
```

With fakes and `httptest.Server`, assert output contains `[1/4]` through `[4/4]`, health precedes Key check, Key appears only in Authorization, completion mentions `/hooks`, and non-TTY returns validation before network/file calls.

- [ ] **Step 4: Run RED**

Run: `go test -count=1 ./test/unit/tracecli -run 'Test(ClientCommandTree|ConfigStore|InitWizard)'`

Expected: compile failure because client/tracecli packages do not exist.

- [ ] **Step 5: Implement paths and atomic config storage**

Define `Paths` methods for `BinFile`, `TraceDir`, `ConfigFile`, `SpoolDir`, `StateDir`, `RejectedDir`, `LogDir`. Save using Sonic, `MkdirAll(0700)`, destination-directory temp file, `Chmod(0600)`, `Sync`, `Close`, `Rename`. Never print Config or Key.

- [ ] **Step 6: Implement terminal and HTTP adapters**

Make `golang.org/x/term` direct. `Interactive` checks stdin/stdout FDs; `ReadSecret` uses `term.ReadPassword`. Normalize Host with `url.Parse`: only `http|https`, no userinfo/query/fragment, trim trailing slash.

Use injected `http.Client{Timeout:5*time.Second}`. Health calls `/health`; Key check calls `/api/v1/trace/client/check` with Bearer and accepts only `204`.

- [ ] **Step 7: Implement the Init runner**

```go
type InitOptions struct { Host, CommandPath string }
type InitRunner struct {
	Terminal Terminal
	Config ConfigStore
	Codex CodexInstaller
	HTTP *Client
}
func (r *InitRunner) Run(ctx context.Context, opts InitOptions) error
```

Order: normalize/retry health; select only Codex; retain existing Key on empty input or read hidden new Key and validate; install Hook; atomically save final config. Hook failure must not overwrite old config. Success prints paths and manual `/hooks` approval, never Key.

- [ ] **Step 8: Create isolated Cobra entrypoint**

`rootCmd.Use="aris"`, `SilenceUsage=true`, `SilenceErrors=true`. `trace init` requires `--host`, resolves `os.Executable()` absolute path, and invokes runner. Register `trace ingest` with a temporary fail-open no-op returning `nil`; Task 6 replaces it.

- [ ] **Step 9: Run GREEN**

Run:

```bash
go test -count=1 ./test/unit/tracecli -run 'Test(ClientCommandTree|ConfigStore|InitWizard)'
go build -o /tmp/aris ./cmd/client
/tmp/aris --help
```

Expected: PASS; client help contains no server command/import chain.

### Task 5: 幂等配置 Codex Hook

**Files:**
- Create: `internal/tracecli/codex.go`
- Modify: `internal/tracecli/init.go`
- Create: `test/unit/tracecli/codex_test.go`
- Create: `test/unit/tracecli/fixtures/hooks_existing.json`

**Interfaces:**
- `CodexHookInstaller.Install(ctx,commandPath) (backupPath string, err error)`.
- Registers ten events including `PermissionRequest`.

- [ ] **Step 1: Create a preservation fixture**

```json
{"version":2,"future_top_level":{"keep":true},"hooks":{"PreToolUse":[{"matcher":"Bash","future_group":"keep","hooks":[{"type":"command","command":"/usr/local/bin/other-hook","timeout":9}]}]}}
```

- [ ] **Step 2: Write failing preservation/idempotency tests**

Install twice and assert unknown top-level/group JSON remains, unrelated Hook remains, exactly one Aris group exists under each of ten events, command is `<absolute aris> trace ingest`, timeout is 30, and `.bak` contains the pre-change file. Malformed JSON must leave original/backup untouched.

- [ ] **Step 3: Run RED**

Run: `go test -count=1 ./test/unit/tracecli -run TestCodexHookInstaller`

Expected: FAIL because installer is absent.

- [ ] **Step 4: Implement raw-preserving patching**

Do not use `map[string]any`. Decode root as `map[string]sonic.NoCopyRawMessage`; decode only `hooks` to `map[string][]sonic.NoCopyRawMessage`. For each group, decode a small view containing matcher and command hooks only to identify current Aris entries. Preserve unrelated groups as raw bytes, append one canonical group, marshal only changed hooks, merge into raw root.

Events:

```go
[]string{"SessionStart","UserPromptSubmit","PreToolUse","PermissionRequest","PostToolUse","Stop","SubagentStart","SubagentStop","PreCompact","PostCompact"}
```

Write `<hooks.json>.bak` from original, then private atomic replacement (`0700` parent, `0600` file).

- [ ] **Step 5: Run GREEN**

Run: `go test -count=1 ./test/unit/tracecli -run TestCodexHookInstaller`

Expected: PASS including malformed-file safety.

### Task 6: 实现 Fail-open Hook Spool 与批量重试

**Files:**
- Create: `internal/tracecli/lock_unix.go`
- Create: `internal/tracecli/spool.go`
- Create: `internal/tracecli/ingest.go`
- Modify: `cmd/client/trace.go`
- Modify: `go.mod`
- Modify: `go.sum`
- Replace: `test/e2e/trace/hook_test.go`
- Create: `test/unit/tracecli/spool_test.go`
- Create: `test/unit/tracecli/ingest_test.go`

**Interfaces:**
- `Spool.Append(ctx,PendingRecord) error`.
- `Spool.Batch(maxRecords int,maxBytes int64) ([]PendingRecord,error)` oldest-first.
- `Spool.Acknowledge(results []RecordResult) error`.
- `Ingestor.Ingest(ctx,raw []byte) error`; command wrapper always exits success.

- [ ] **Step 1: Write failing atomicity/capacity tests**

Define `PendingRecord` with session/source/type/event/turn/call/line/sequence/dedup/payload/createdAt. Concurrently append unique records using `sync.WaitGroup`; every file must be valid JSON, `0600`, and no temp remains. Inject a tiny hard limit; next append returns `ErrSpoolFull` and existing pending files remain.

- [ ] **Step 2: Write failing real-process stdout/exit tests**

Build the client and invoke `aris trace ingest` with temporary HOME. Cases: Stop, non-Stop, missing config, full spool, unreachable server, malformed JSON. Every case exits `0`; Stop stdout equals exactly `{}`, all others empty.

```go
cmd.Stdin = strings.NewReader(`{"session_id":"s1","hook_event_name":"Stop"}`)
out, err := cmd.Output()
if err != nil || string(out) != "{}" { t.Fatalf("out=%q err=%v", out, err) }
```

- [ ] **Step 3: Run RED**

Run:

```bash
go test -count=1 ./test/unit/tracecli -run 'TestSpool|TestTraceIngest'
go test -count=1 ./test/e2e/trace -run TestCodexHook
```

Expected: FAIL because ingest is still a stub.

- [ ] **Step 4: Implement locking, spool ID and sequence**

Make `golang.org/x/sys` direct. Add `//go:build darwin || linux` to Unix-specific files and use `unix.Flock` on `state/locks/*.lock`. Persist one random `spool_id` and per-session next sequence in private state. Hash session IDs for state filenames. Dedup keys:

```text
hook:<spool_id>:<client_sequence>
rollout:<session_id>:<transcript_line>:<sha256(raw_line)>
```

Use dedup SHA-256 as pending filename so local append is idempotent.

- [ ] **Step 5: Implement private spool lifecycle**

Layout `spool/pending`、`rejected`、`state`、`logs`; directories `0700`, files `0600`. Batch stops before record 501 or 4 MiB. accepted/duplicate delete pending; rejected atomically moves it. Logs contain timestamp/category/dedup only. Delete logs/rejected older than seven days; never age-delete pending. At 256 MiB, retain existing pending and reject new append.

- [ ] **Step 6: Implement Hook ingestion and 5-second flush**

Parse only a small typed envelope while retaining original bytes. Order: load config → parse → allocate sequence → persist pending → flush global oldest batch. POST Task 2 envelope with child context timeout 5 seconds. Parse only result key/status; do not log response body.

Command wrapper determines Hook name from stdin first, writes `{}` only for Stop, invokes ingestor, writes sanitized failures to private log, ignores returned error, and returns `nil` to Cobra.

- [ ] **Step 7: Replace shell E2E and run GREEN**

Delete `web/src/scripts/codex-hook.sh` only after tests call binary. Replace curl-mocking E2E with `httptest.Server` returning accepted results.

Run:

```bash
go test -count=1 ./test/unit/tracecli -run 'TestSpool|TestTraceIngest'
go test -count=1 ./test/e2e/trace -run TestCodexHook
```

Expected: PASS; Hook runtime requires no jq/Python/curl.

### Task 7: 增量读取 Rollout 并加入同一 Spool

**Files:**
- Create: `internal/tracecli/fileid_unix.go`
- Create: `internal/tracecli/rollout.go`
- Modify: `internal/tracecli/ingest.go`
- Create: `test/e2e/trace/fixtures/rollout.jsonl`
- Create: `test/unit/tracecli/rollout_test.go`
- Modify: `test/e2e/trace/hook_test.go`

**Interfaces:**
- `RolloutReader.ReadNew(ctx,sessionID,transcriptPath) ([]PendingRecord,error)`.
- Persists `TranscriptState{Identity,Offset,Line}` only after returned records are durable.

- [ ] **Step 1: Create fixture and failing delta tests**

Fixture has three complete JSONL records plus an incomplete fragment without newline. Test first read returns three; append remainder/newline and second returns one; third returns zero; truncate/replace resets; reprocessed old content produces same dedup key. Assert payload bytes equal source line excluding newline.

- [ ] **Step 2: Run RED**

Run: `go test -count=1 ./test/unit/tracecli -run TestRolloutReader`

Expected: compile failure.

- [ ] **Step 3: Implement complete-line reading and file identity**

Add `//go:build darwin || linux` to `fileid_unix.go`. Use device/inode identity from `unix.Stat_t`, stored byte offset and line. Seek, `ReadBytes('\n')`, emit only newline-terminated Sonic-valid JSON. Strip only CRLF/LF. If identity changes or size is below offset, reset to zero.

Extract envelope `type`, nested `payload.type`, `turn_id`, `call_id` with small typed views. Unknown types are stored as `unknown`, never dropped. Dedup is session+line+SHA-256(raw line).

- [ ] **Step 4: Couple state advancement to durable append**

Under transcript lock: read records, append all to spool, then atomically write the new offset/line. If any append fails, do not advance state. Integrate this before network flush in `Ingestor.Ingest`; unreadable transcript logs a sanitized category but Hook record still proceeds.

- [ ] **Step 5: Run GREEN**

Run:

```bash
go test -count=1 ./test/unit/tracecli -run TestRolloutReader
go test -count=1 ./test/e2e/trace -run 'TestCodexHook_(Persists|Retries|ReadsRollout)'
```

Expected: PASS for delta, partial line, replacement and retry.

### Task 8: 解析 Rollout 并提供 Conversation API

**Files:**
- Create: `internal/domain/trace/rollout.go`
- Create: `internal/domain/trace/conversation.go`
- Create: `internal/application/trace/query/get_conversation.go`
- Modify: `internal/application/trace/query/get_trace.go`
- Modify: `internal/application/trace/query/list_trace_events.go`
- Modify: `internal/application/trace/port/handler.go`
- Modify: `internal/handler/trace.go`
- Modify: `internal/router/trace.go`
- Modify: `internal/bootstrap/modules/application.go`
- Modify: `internal/bootstrap/modules/handler.go`
- Create: `internal/dto/trace_conversation.go`
- Modify: `internal/dto/trace.go`
- Create: `test/unit/trace/fixtures/rollout_records.json`
- Create: `test/unit/trace/rollout_test.go`
- Create: `test/unit/trace/conversation_test.go`
- Modify: `test/e2e/trace/trace_test.go`

**Interfaces:**
- `trace.ParseRolloutRecord(payload []byte) (RolloutRecord,error)` retains raw data and tolerates unknown types.
- `trace.BuildConversation(records []*TraceEvent) *Conversation` returns a non-persisted projection.
- Route: `GET /api/v1/trace/conversation?id=<traceID>`.

- [ ] **Step 1: Create comprehensive fixtures and failing parser tests**

Cover `session_meta`、`turn_context`、task start/complete、user/agent message、reasoning、function/custom call+output、MCP end、web search、token、error、malformed arguments and unknown types. Assert envelope type is selected before nested type, raw bytes survive, invalid arguments remain as text, and unknown records return without error.

- [ ] **Step 2: Write failing projection tests**

Build mixed Hook/rollout records. Assert one ordered turn, rollout user/assistant suppress duplicate Hook fallback, a tool call pairs with output by `call_id`, pending/orphaned calls remain, and every item exposes source record IDs. A second case without rollout must include Hook fallback.

- [ ] **Step 3: Write failing owner-isolation tests**

Current Get/Events queries carry user metadata but must prove ownership. Test non-admin access to another user's Trace returns unauthorized while admin succeeds. Apply the same authorization to Get, Events and Conversation.

- [ ] **Step 4: Run RED**

Run: `go test -count=1 ./test/unit/trace -run 'Test(ParseRolloutRecord|BuildConversation|TraceOwnerIsolation)'`

Expected: compile/failing assertions.

- [ ] **Step 5: Implement tolerant typed parser**

Use Sonic raw messages and concrete views, never dynamic interface maps. `RolloutRecord` exposes envelope/nested type, timestamp, turn/call IDs, message/reasoning/tool fields, original bytes, unknown flag and parse diagnostic. Preserve `function_call.arguments` string and additionally expose raw JSON only when valid; custom tool input remains text.

- [ ] **Step 6: Implement projection and shared authorization**

Group by rollout turn ID; derive boundaries from task start/complete/aborted. Message priority is event message → response message → Hook fallback. Pair function/custom/web/MCP records by call ID; Hook uses tool-use ID only as fallback. Never mutate raw rows.

Create one application authorization helper: load Trace; allow admin; otherwise call `APIKeyRepository.LookupOwnerNamesByUserID` and match `Trace.APIKeyName`. Reuse in Get, Events and Conversation constructors.

- [ ] **Step 7: Define concrete DTOs, handler and route**

Create DTOs for Conversation, Turn and Item with explicit kind, role, content, toolName, callId, arguments, output, status, source, recordIds and timestamps. Raw JSON uses `sonic.NoCopyRawMessage`. Register JWT `GET /conversation` with `LimitUserPermissionMiddleware("getTraceConversation", enum.PermissionUser)` and wire DI.

- [ ] **Step 8: Run GREEN**

Run:

```bash
go test -count=1 ./test/unit/trace -run 'Test(ParseRolloutRecord|BuildConversation|TraceOwnerIsolation)'
go test -count=1 ./test/e2e/trace
```

Expected: PASS; E2E returns user+assistant+completed tool with record IDs.

### Task 9: Web 改为按需签发票据并复制短安装脚本

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api-client.ts`
- Modify: `web/src/components/trace-install-dialog.tsx`
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/zh.json`
- Modify: `web/src/locales/ja.json`
- Modify: `web/CONTEXT.md`

**Interfaces:**
- Consumes `POST /api/v1/trace/client/ticket`.
- Copies a script that downloads `/api/v1/trace/client` and runs `aris trace init --host <origin>`.

- [ ] **Step 1: Add ticket type and API method**

```ts
export interface IssueTraceClientTicketRsp extends CommonRsp {
  ticket?: string;
  expiresAt?: string;
}
```

Add `api.issueTraceClientTicket()` using the existing authenticated request wrapper with POST.

- [ ] **Step 2: Verify the component contract RED**

Reference the new method from the copy handler before Step 1 is complete. Run `cd web && npm run lint`; expected missing-symbol failure, then complete Step 1.

- [ ] **Step 3: Implement safe short-script generation**

Use `window.location.origin`; shell-quote Host and ticket. Script must detect:

```text
Darwin-x86_64 → darwin/amd64
Darwin-arm64  → darwin/arm64
Linux-x86_64 → linux/amd64
Linux-aarch64|Linux-arm64 → linux/arm64
```

It downloads to `mktemp`, sets trap, executes `curl -fsSL -H 'Authorization: Bearer <ticket>' '<host>/api/v1/trace/client?os=...&arch=...'`, creates `~/.aris/bin` as `0700`, chmods temp `0700`, atomically moves to `~/.aris/bin/aris`, clears trap, then `exec`s `aris trace init --host '<host>'`.

Do not store the real ticket script in React state. Preview uses `<single-use-ticket>`; copy handler issues a fresh ticket, generates a local string, and copies only that string.

- [ ] **Step 4: Simplify dialog and handle failures**

Remove URL/API Key inputs. Explain that API Key is entered securely in terminal. Add `copying` state and disable copy while issuing. API or clipboard failure shows `sonner` error, never marks copied, and retains no ticket. Update all three locale files with matching keys and remove obsolete input labels.

- [ ] **Step 5: Run GREEN**

Run: `cd web && npm run lint && npm run build`

Expected: PASS; static export succeeds and no Hook shell source remains.

### Task 10: Web 展示 Conversation 与 Raw Records

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api-client.ts`
- Create: `web/src/components/trace-conversation.tsx`
- Create: `web/src/components/trace-raw-records.tsx`
- Modify: `web/src/app/(dashboard)/trace/page.tsx`
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/zh.json`
- Modify: `web/src/locales/ja.json`

**Interfaces:**
- Consumes `GET /api/v1/trace/conversation?id=<traceID>` and extended event list.
- Produces Conversation / Raw records tabs.

- [ ] **Step 1: Define exact TS contracts and API method**

Add `TraceConversation`、`TraceTurn`、`TraceConversationItem` matching Task 8 DTO exactly. Extend `TraceEventItem` with source, recordType, callId, transcriptLine, clientSequence, dedupKey. Add:

```ts
async getTraceConversation(id: number): Promise<GetTraceConversationRsp> {
  return this.request<GetTraceConversationRsp>(`/api/v1/trace/conversation?id=${id}`);
}
```

- [ ] **Step 2: Establish RED through the page contract**

Reference the new API/type in `page.tsx` before Step 1, run `cd web && npm run lint`, and confirm missing-symbol failure. Then complete Step 1 and load detail+Conversation with `Promise.all`; fetch raw records only when Raw tab/page needs them.

- [ ] **Step 3: Implement `TraceConversation` component**

Render server-ordered turns and distinguish message, reasoning, tool, lifecycle and error using existing Card/Badge styles. Show pending/orphaned/fallback/source and record IDs. Render content as React text; raw objects use escaped `JSON.stringify` inside `<pre>`, never raw HTML.

- [ ] **Step 4: Extract `TraceRawRecords` component**

Move current event disclosure UI from the large page file. Add source/type/call/line/dedup metadata and retain loading/pagination. Payload stays escaped JSON.

- [ ] **Step 5: Add responsive tabs and locales**

Use existing `components/ui/tabs.tsx`; Conversation is default. Preserve mobile dialog scrolling and language-stability rules. Add identical en/zh/ja keys for tabs, item kinds, pending/orphaned, fallback, source, empty/error and rollout diagnostics. No hardcoded user-facing text.

- [ ] **Step 6: Run GREEN and browser interaction verification**

Run: `cd web && npm run lint && npm run build`

Start the normal local server/frontend, then use Chrome DevTools MCP to verify install-copy loading/error states, ticket-free preview, detail tab switching, tool call/output pairing, raw pagination and mobile viewport. Expected: no console errors, no API Key/JWT in copied script, and static routes remain under `/web`.

### Task 11: 构建四平台客户端并随镜像发布

**Files:**
- Modify: `Makefile`
- Modify: `docker/dockerfile`
- Modify: `env/api.env.template`
- Modify: `k8s/configmap.yaml`
- Modify: `.github/workflows/docker-publish.yml`
- Modify: `docs/agents/commands.md`
- Modify: `docs/agents/repo-ci.md`

**Interfaces:**
- `make build-server` produces `aris-proxy-api`.
- `make build-client` produces host `aris`.
- `make build-client-all` produces four files under `build/trace-client/`.
- Runtime image contains `/app/trace-client/aris-{os}-{arch}`.

- [ ] **Step 1: Add build target assertions**

Extend `test/unit/tracecli/command_tree_test.go` with a test that builds `./cmd/client` under each environment into `t.TempDir()` and asserts non-empty files:

```go
for _, target := range []struct{ os, arch string }{{"darwin","amd64"},{"darwin","arm64"},{"linux","amd64"},{"linux","arm64"}} {
	cmd := exec.CommandContext(context.Background(), "go", "build", "-trimpath", "-ldflags=-s -w", "-o", out, "./cmd/client")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+target.os, "GOARCH="+target.arch)
	if data, err := cmd.CombinedOutput(); err != nil { t.Fatalf("%s/%s: %v\n%s", target.os, target.arch, err, data) }
}
```

- [ ] **Step 2: Run RED for Makefile targets**

Run: `make build-client-all`

Expected: FAIL because the target does not exist.

- [ ] **Step 3: Implement Makefile targets**

Add `SERVER_MAIN:=./cmd/server`, `CLIENT_MAIN:=./cmd/client`, `CLIENT_OUTPUT_DIR:=build/trace-client`. `build-server` depends on `web-build`; `build-client` builds host; `build-client-all` explicitly builds all four with `CGO_ENABLED=0` and strip flags. `build` depends on `build-server build-client-all`. `clean` removes server, host client and `build/trace-client`.

- [ ] **Step 4: Compile all clients in Docker builder**

After dependencies/source copy, explicitly run four cross-build commands into `/go/trace-client`. Do not UPX Darwin clients; keep `-trimpath -ldflags="-s -w"`. Final stage copies:

```dockerfile
COPY --from=builder /go/bin/aris-proxy-api /app/aris-proxy-api
COPY --from=builder /go/trace-client /app/trace-client
```

Keep runtime `nonroot`; artifact directory is read-only. Set/document `TRACE_CLIENT_ARTIFACT_DIR=/app/trace-client` in env template and ConfigMap. In both push and pull-request path filters of `.github/workflows/docker-publish.yml`, remove the deleted root `main.go` entry; retain `cmd/**`, `internal/**`, `web/**`, `docker/**`, `go.mod` and `go.sum`, which fully cover both executables.

- [ ] **Step 5: Verify client dependency isolation and artifacts**

Run:

```bash
go test -count=1 ./test/unit/tracecli -run TestClientBuildsForSupportedPlatforms
make build-client-all
find build/trace-client -maxdepth 1 -type f -print
if go list -deps ./cmd/client | grep -E 'internal/(bootstrap|infrastructure/database|router|web)'; then exit 1; fi
```

Expected: four non-empty files; dependency grep is empty.

- [ ] **Step 6: Verify Docker build**

Run a local Docker build for the host platform with the repository's normal frontend source. Add a builder-stage assertion that runs `test -s` for all four `/go/trace-client/aris-*` files before the final stage. Expected: final image contains executable server plus four readable client files; `aris-proxy-api server start` remains the entry command. The CI continues publishing one multi-architecture service image rather than separate client artifacts.

### Task 12: 领域文档、完整回归与安全验收

**Files:**
- Modify: `CONTEXT.md`
- Modify: `web/CONTEXT.md`

**Interfaces:**
- Final deliverable satisfies all design acceptance criteria without committing or deploying.

- [ ] **Step 1: Update glossary with exact terms**

Add concise definitions for `TraceClient`（独立 `aris`）、`TraceDownloadTicket`（JWT 换取的 10-minute one-time ticket）、`TraceSpool`（本地未确认记录）、`TraceConversation`（非持久化投影）。In `web/CONTEXT.md`, document that Trace install export contains a short, single-use-ticket download script and never embeds API Key/JWT.

- [ ] **Step 2: Run focused backend/client suites**

Run:

```bash
go test -count=1 ./test/unit/trace ./test/unit/tracecli ./test/e2e/trace
```

Expected: PASS for ticket, batch dedup, init, Hook, rollout and Conversation.

- [ ] **Step 3: Run project lint and full Go tests**

Run:

```bash
make lint
make test
```

Expected: PASS; convention lint confirms DTOs do not depend on DB models and contain no prohibited dynamic types.

- [ ] **Step 4: Run frontend and build validation**

Run:

```bash
cd web && npm run lint && npm run build
cd .. && make build-client-all && make build-server
```

Expected: PASS; frontend static export, server and all clients build.

- [ ] **Step 5: Execute the final acceptance scenario**

Using test server/fixtures:

1. JWT user issues three immediate tickets; fourth before refill is 429.
2. One ticket downloads correct bytes once; reuse is 401.
3. Install script detects host platform and atomically installs `aris`.
4. `aris trace init --host` verifies health/Key, writes `0600` config, preserves existing Codex Hook, registers ten events, and prints `/hooks` approval.
5. Simulated user prompt → tool call/output → assistant → Stop plus rollout records survives a failed POST, retries, deduplicates, and marks Trace done.
6. Non-Stop stdout is empty; Stop is exactly `{}`; all failure paths exit 0.
7. Conversation contains one turn, user+assistant, completed tool call with same call ID; Raw records point back through record IDs.
8. Search logs, spool, Hook config and generated script for API Key/JWT/ticket leakage; none is present except the intentionally copied short-lived ticket in the one-time install script.

- [ ] **Step 6: Review complexity and verify workspace state**

Run `ponytail-review` on the final diff, then:

```bash
git diff --check
git status --short
git --no-pager diff --stat
```

Expected: no whitespace errors; no `web/out`, `.next`, binaries, secrets or temporary files tracked; only intended source/tests/docs changed. Do not commit.

