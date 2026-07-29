# Aris 客户端 TUI 重构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于 charmbracelet 生态重构 `aris` 客户端：新增 `aris init`（huh 四步配置向导）与 `aris status`（状态面板），install.sh 退化为纯下载器，服务端新增 1 个 API Key 鉴权的 trace 摘要端点。

**Architecture:** huh 表单（Input/Select/Confirm/Spinner，基于 bubbletea inline 模式）承担全部交互，lipgloss 提供主题与静态渲染；不编写自定义 bubbletea 模型。`aris status` 并发收集检查结果后用 lipgloss 一次性静态渲染。服务端摘要按 `CtxKeyAPIKeyName`（owner）聚合。

**Tech Stack:** Go 1.25（cobra、huh/bubbletea/bubbles/lipgloss、sonic、golang.org/x/term），huma/fiber（服务端端点），POSIX sh（install.sh 模板）。

**Spec:** `docs/superpowers/specs/2026-07-28-aris-client-tui-redesign-design.md`

## Global Constraints

- `trace ingest` 路径（`internal/client/trace/` 的 ingest/spool/rollout/state 逻辑）行为零改动；fail-open 语义不变
- 客户端包边界：`internal/client/**` 不得导入 server / db / router / web embed / dto / application
- 文件权限：`~/.aris/**` 目录 0700；`config.json` / `hooks.json` / `.bak` 0600；全部原子写（沿用 `writePrivateFile`）
- API Key 不出现在 flag、命令行参数、日志中；来源优先级 `ARIS_API_KEY` env > 向导密码输入；status 中脱敏显示（仅末 4 位）
- 10 个 hook 事件：`SessionStart, UserPromptSubmit, PreToolUse, PermissionRequest, PostToolUse, Stop, SubagentStart, SubagentStop, PreCompact, PostCompact`；hook group 格式 `{"matcher":"","hooks":[{"type":"command","command":"<bin> trace ingest","timeout":30}]}`
- 交互非 TTY 时：init 自动打开 `/dev/tty`（stdin 是 curl 管道场景）；无可用 TTY 报错退出
- DTO 遵守 huma-dto-conventions：禁 any/json.RawMessage/dbmodel 导入；时间用 `time.Time`；GET 无 Body 包装
- Go lint：`go run ./cmd/server lint conv ./...` 必须通过；`go test ./cmd/... ./internal/... ./test/...` 必须通过
- 语言：代码标识符英文；CLI 面向用户的文案英文（与现有 install.sh 输出口径一致）

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `go.mod` / `go.sum` | Modify | 新增 huh/bubbletea/bubbles/lipgloss 依赖 |
| `internal/domain/trace/repository.go` | Modify | 新增 `OwnerTraceSummary` 结构体 + `SummarizeByOwner` 接口方法 |
| `internal/infrastructure/repository/trace_repository.go` | Modify | 实现 `SummarizeByOwner`（traces 聚合 + events join 计数） |
| `internal/application/trace/port/handler.go` | Modify | 新增 `TraceClientSummaryView` / `GetTraceClientSummaryHandler` |
| `internal/application/trace/query/get_client_summary.go` | Create | 摘要读侧 usecase |
| `internal/dto/trace.go` | Modify | 新增 `GetTraceClientSummaryReq/Rsp` |
| `internal/handler/trace.go` | Modify | 新增 `HandleGetTraceClientSummary`；`TraceDependencies` 加 `ClientSummary` |
| `internal/router/trace.go` | Modify | reportGroup 注册 `GET /client/summary` |
| `internal/bootstrap/modules/application.go` | Modify | fx 提供 `NewGetTraceClientSummaryHandler` |
| `internal/bootstrap/modules/handler.go` | Modify | `NewTraceDependencies` 注入 ClientSummary |
| `internal/client/ui/theme.go` | Create | lipgloss 语义色/图标/间距 token + huh 主题 |
| `internal/client/ui/components.go` | Create | StepHeader/SectionTitle/CheckRow/KeyValue/SummaryPanel 渲染 |
| `internal/client/api/client.go` | Create | 控制面 HTTP 客户端：CheckHealth/CheckAPIKey/FetchSummary |
| `internal/client/trace/config.go` | Modify | `ConfigStore` 恢复 `Save` 方法 |
| `internal/client/setup/hooks.go` | Create | codex hooks.json 幂等合并（移植 install.sh jq 逻辑） |
| `internal/client/setup/wizard.go` | Create | huh 向导编排 + /dev/tty 处理 + RunInit 入口 |
| `internal/client/status/checks.go` | Create | 六节检查并发收集 |
| `internal/client/status/render.go` | Create | lipgloss 面板渲染 + `--json` 输出 |
| `internal/client/status/status.go` | Create | RunStatus 编排（spinner + 渲染） |
| `cmd/client/root.go` | Modify | 注册 `init`、`status` |
| `cmd/client/init.go` / `status.go` | Create | cobra 命令定义（`--host` / `--json` flag） |
| `internal/handler/install_trace_client.sh.tmpl` | Modify | 纯下载器 + `exec aris init --host` |
| `internal/common/constant/traceclient.go` | Modify | 清理旧向导提示常量，新增 summary path 等 |
| `test/unit/client/trace/command_tree_test.go` | Modify | 断言 init/status/trace 存在 |
| `test/unit/client/setup/*_test.go` | Create | hooks 合并、config 保存单测 |
| `test/unit/client/api/*_test.go` | Create | httptest 客户端单测 |
| `test/unit/client/status/*_test.go` | Create | checks/render 单测（golden） |
| `test/unit/client/ui/*_test.go` | Create | 组件渲染快照 |
| `test/unit/trace/client_summary_test.go` | Create | usecase 单测 |
| `test/e2e/trace/client_summary_test.go` | Create | 摘要端点 e2e（huma 测试 app + stub port） |
| `test/e2e/trace/install_script_test.go` | Modify | 断言脚本含 `init --host`、不含 jq/交互 |
| `CONTEXT.md` | Modify | 更新客户端命令描述 |

---

## Task 1: 依赖引入 + 服务端摘要端点

**Files:**
- Modify: `go.mod`（新增依赖）
- Modify: `internal/domain/trace/repository.go`
- Modify: `internal/infrastructure/repository/trace_repository.go`
- Modify: `internal/application/trace/port/handler.go`
- Create: `internal/application/trace/query/get_client_summary.go`
- Modify: `internal/dto/trace.go`
- Modify: `internal/handler/trace.go`、`internal/router/trace.go`
- Modify: `internal/bootstrap/modules/application.go`、`internal/bootstrap/modules/handler.go`
- Test: `test/unit/trace/client_summary_test.go`、`test/e2e/trace/client_summary_test.go`

**Interfaces:**
- Produces: `trace.TraceRepository.SummarizeByOwner(ctx, owner string) (*trace.OwnerTraceSummary, error)`
- Produces: `port.GetTraceClientSummaryHandler.Handle(ctx, port.GetTraceClientSummaryQuery) (*port.TraceClientSummaryView, error)`
- Produces: `TraceHandler.HandleGetTraceClientSummary(ctx, *dto.GetTraceClientSummaryReq) (*dto.HTTPResponse[*dto.GetTraceClientSummaryRsp], error)`

- [ ] **Step 1: 引入 TUI 依赖**

```bash
go get github.com/charmbracelet/huh@latest github.com/charmbracelet/bubbles@latest github.com/charmbracelet/lipgloss@latest
go mod tidy
```
（bubbletea、x/term 由 huh 传递引入；确认 `go build ./...` 通过）

- [ ] **Step 2: domain 层 — `OwnerTraceSummary` + 接口方法**

`internal/domain/trace/repository.go`：

```go
// OwnerTraceSummary 按 owner（API Key 名称）聚合的 trace 摘要
type OwnerTraceSummary struct {
	TraceCount   int64
	ActiveCount  int64
	EventCount   int64
	LastActiveAt *time.Time
	LastModel    string
}

// 接口新增：
// SummarizeByOwner 按 owner 聚合 trace 统计（client summary 端点用）
SummarizeByOwner(ctx context.Context, owner string) (*OwnerTraceSummary, error)
```

- [ ] **Step 3: repository 实现**

`internal/infrastructure/repository/trace_repository.go` 新增方法。两条查询（值绑定，owner 过滤 + `DBConditionDeletedAtZero`）：

```go
// 1) traces 聚合：COUNT(*)、SUM(status='active')、MAX(updated_at)
// 2) events 计数：JOIN trace_events ON trace_id，同 owner 过滤
// LastModel：取 MAX(updated_at) 对应行的 model（子查询或单独 ORDER BY updated_at DESC LIMIT 1）
```

- [ ] **Step 4: port 视图与 usecase**

`port/handler.go` 新增：

```go
// TraceClientSummaryView 客户端摘要视图
type TraceClientSummaryView struct {
	TraceCount   int64
	ActiveCount  int64
	EventCount   int64
	LastActiveAt *time.Time
	LastModel    string
	APIKeyName   string
}

// GetTraceClientSummaryQuery 客户端摘要查询（owner 取自 API Key middleware context）
type GetTraceClientSummaryQuery struct{ APIKeyName string }

// GetTraceClientSummaryHandler 客户端摘要 handler
type GetTraceClientSummaryHandler interface {
	Handle(ctx context.Context, q GetTraceClientSummaryQuery) (*TraceClientSummaryView, error)
}
```

`query/get_client_summary.go`：调用 `repo.SummarizeByOwner(ctx, q.APIKeyName)` 组装视图；`APIKeyName == ""` 返回 `ierr.ErrValidation`。

- [ ] **Step 5: DTO**

`internal/dto/trace.go`：

```go
// GetTraceClientSummaryReq 客户端摘要请求（API Key 鉴权，无参数）。
type GetTraceClientSummaryReq struct{}

// GetTraceClientSummaryRsp 客户端摘要响应
type GetTraceClientSummaryRsp struct {
	CommonRsp
	TraceCount   int64      `json:"traceCount" doc:"trace 总数"`
	ActiveCount  int64      `json:"activeCount" doc:"进行中 trace 数"`
	EventCount   int64      `json:"eventCount" doc:"事件总数"`
	LastActiveAt *time.Time `json:"lastActiveAt,omitempty" doc:"最近活跃时间"`
	LastModel    string     `json:"lastModel,omitempty" doc:"最近使用模型"`
	APIKeyName   string     `json:"apiKeyName" doc:"API Key 名称"`
}
```

- [ ] **Step 6: handler + 路由 + DI**

- `TraceHandler` 接口与实现新增 `HandleGetTraceClientSummary`：从 ctx 取 `constant.CtxKeyAPIKeyName` 调 usecase，映射到 Rsp（`apiutil.WrapHTTPResponse`）
- `TraceDependencies` 新增 `ClientSummary port.GetTraceClientSummaryHandler` 字段
- `internal/router/trace.go` reportGroup 注册：

```go
huma.Register(reportGroup, huma.Operation{
	OperationID: "getTraceClientSummary", Method: http.MethodGet, Path: "/client/summary",
	Summary: "GetTraceClientSummary", Description: "Aggregate trace stats for the calling API key",
	Tags:     []string{constant.TagTrace},
	Security: []map[string][]string{{constant.SecuritySchemeAPIKey: {}}},
}, deps.TraceHandler.HandleGetTraceClientSummary)
```

- bootstrap：`application.go` fx 提供 `query.NewGetTraceClientSummaryHandler`；`handler.go` `NewTraceDependencies` 注入

- [ ] **Step 7: 写失败测试**

- `test/unit/trace/client_summary_test.go`：mock `TraceRepository` 断言聚合映射、空 owner 报错、repo 错误透传
- `test/e2e/trace/client_summary_test.go`：参照 `install_script_test.go` 模式（fiber + humafiber + stub port handler），断言 200、字段透出、无数据时 `lastActiveAt` 缺省

- [ ] **Step 8: 验证 + 提交**

```bash
go build ./... && go test ./test/unit/trace/ ./test/e2e/trace/ -run Summary -v
go run ./cmd/server lint conv ./...
git add -A && git commit -m "feat(trace): 新增 API Key 鉴权的 client summary 端点"
```

---

## Task 2: `internal/client/ui` 主题与组件

**Files:**
- Create: `internal/client/ui/theme.go`
- Create: `internal/client/ui/components.go`
- Test: `test/unit/client/ui/components_test.go`

**Interfaces:**
- Produces: `ui.Theme`（语义色样式集）、`ui.StepHeader(step, total, title string) string`、`ui.SectionTitle(name string) string`、`ui.CheckRow(level Level, label, detail string) string`、`ui.KeyValue(pairs ...[2]string) string`、`ui.SummaryPanel(lines ...string) string`、`ui.HuhTheme() *huh.Theme`

- [ ] **Step 1: theme.go**

- 语义色（`lipgloss.AdaptiveColor`，明暗终端自适应）：Primary（clay 橙 `#CC785C`/`#D97757`）、Success（绿）、Warning（黄）、Error（红）、Muted（灰）
- 图标常量：`IconOK="✓" IconFail="✗" IconWarn="!" IconSection="◆"`（非 emoji）
- `Level` 枚举：`LevelOK / LevelFail / LevelWarn / LevelInfo`
- `HuhTheme()`：基于 `huh.ThemeCharm()` 覆盖主色为 Primary，聚焦/错误样式对齐语义色
- 导出 `Renderer(w io.Writer) *lipgloss.Renderer`；`NO_COLOR`/非 TTY 时 lipgloss 自动降级

- [ ] **Step 2: components.go**

- `StepHeader`：`[1/4] Connect to server`（序号 Muted、标题 Primary Bold）
- `SectionTitle`：`◆ Server`（accent 色）
- `CheckRow`：`✓ label · detail`（图标按 Level 着色，detail Muted）
- `KeyValue`：冒号对齐键值对
- `SummaryPanel`：rounded border 成功面板
- 全部纯函数返回 string，不直接写 stdout（可测试）

- [ ] **Step 3: 快照测试 + 提交**

各组件 golden 测试（固定 lipgloss renderer 强制无色彩，比对字面输出）。

```bash
go test ./test/unit/client/ui/ -v && git add -A && git commit -m "feat(client): 新增 ui 主题与渲染组件包"
```

---

## Task 3: `internal/client/api` 控制面 HTTP 客户端

**Files:**
- Create: `internal/client/api/client.go`
- Test: `test/unit/client/api/client_test.go`

**Interfaces:**
- Produces: `api.New(baseURL, apiKey string, hc *http.Client) *Client`
- Produces: `(c *Client) CheckHealth(ctx) (time.Duration, error)`、`CheckAPIKey(ctx) error`、`FetchSummary(ctx) (*Summary, error)`
- Produces: `api.Summary`（与服务端 Rsp 对齐的客户端结构体）

- [ ] **Step 1: client.go**

- 超时沿用 `constant.TraceClientHTTPTimeout`；baseURL 去尾部 `/`
- `CheckHealth`：`GET {base}/health`，返回 RTT；非 2xx 视为失败
- `CheckAPIKey`：`GET {base}/api/v1/trace/client/check`（Bearer），2xx 通过
- `FetchSummary`：`GET {base}/api/v1/trace/client/summary`（Bearer），sonic 解码；常量 `TraceClientSummaryPath = "/api/v1/trace/client/summary"` 加入 `constant/traceclient.go`
- 错误统一 `ierr.Wrap`，不暴露 key

- [ ] **Step 2: httptest 单测 + 提交**

覆盖：health 延迟返回、check 401/200、summary 正常解码与空态、baseURL 尾斜杠处理。

```bash
go test ./test/unit/client/api/ -v && git add -A && git commit -m "feat(client): 新增控制面 API 客户端（health/check/summary）"
```

---

## Task 4: `aris init` 向导

**Files:**
- Modify: `internal/client/trace/config.go`（恢复 Save）
- Create: `internal/client/setup/hooks.go`、`internal/client/setup/wizard.go`
- Create: `cmd/client/init.go`；Modify: `cmd/client/root.go`
- Test: `test/unit/client/setup/hooks_test.go`、`test/unit/client/setup/wizard_test.go`

**Interfaces:**
- Produces: `setup.InstallCodexHooks(paths trace.Paths, binPath string) error`（幂等，返回前备份）
- Produces: `setup.RunInit(ctx, setup.InitOptions) error`；`InitOptions{Host string, Paths trace.Paths, In io.Reader, Out io.Writer, HTTPClient *http.Client}`
- Consumes: `api.Client`（Task 3）、`ui.*`（Task 2）

- [ ] **Step 1: config.go 恢复 Save**

`ConfigStore` 接口加 `Save(ctx, Config) error`：sonic.Marshal + `writePrivateFile`（已存在，0600 原子写）。

- [ ] **Step 2: hooks.go — codex hooks 幂等合并**

```go
type hookSpec struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}
type hookGroup struct {
	Matcher string     `json:"matcher"`
	Hooks   []hookSpec `json:"hooks"`
}
```

- 读取 `~/.codex/hooks.json` 到 `map[string]sonic.NoCopyRawMessage`（保留未知顶层字段）；不存在则从 `{}` 开始
- 解码 `hooks` 字段为 `map[string][]hookGroup`；对 10 个事件逐一：过滤掉含 `command == <binPath> trace ingest` 的 group，再追加 aris group（去重逻辑与 install.sh jq 版一致）
- 写前已存在文件则复制为 `.bak`（0600）；`writePrivateFile` 原子写回（0600，indent 两空格）
- `InstallCodexHooks` 返回注册的事件数（恒 10）供向导展示

- [ ] **Step 3: wizard.go — 向导编排**

```go
// 终端解析：stdin 是 TTY 直接用；否则打开 /dev/tty（O_RDWR）作为 huh 输入输出；
// 打开失败 → ierr（constant.TraceClientInitNonInteractiveMessage）
func terminalIO() (in io.Reader, out io.Writer, cleanup func(), err error)
```

流程（每步先 `ui.StepHeader` 打印，huh 均带 `huh.WithInput/WithOutput` + `ui.HuhTheme()`）：

1. **连接**：`huh.NewSpinner().Title("Connecting to <host>...")` 执行 `api.CheckHealth`；失败打印 `ui.CheckRow(LevelFail, ...)` + `huh.NewConfirm("Retry?")` 循环
2. **Agent**：`huh.NewSelect[string]`，唯一可用项 Codex（其余 disabled 标注 coming soon）
3. **API Key**：先 `config.Load`；`huh.NewInput().EchoMode(huh.EchoModePassword)`，有存量 key 时描述提示"press Enter to keep current"；`ARIS_API_KEY` env 存在则跳过输入直接采用。随后 spinner 执行 `CheckAPIKey`，失败 `CheckRow(LevelFail)` + Confirm 重试循环
4. **Hooks**：spinner 执行 `InstallCodexHooks` + `config.Save({host, agent, apiKey})`

完成后 `ui.SummaryPanel` 输出：config 路径、hooks 注册数、`In Codex, run /hooks and manually approve the new Aris hooks.`

- host 解析优先级：`--host` flag > 已有 config.host > huh.Input 提示输入（带 `https://` 前缀校验）
- 步骤逻辑（重试循环、key 保留判断、host 归一化）抽纯函数便于单测；huh 交互层保持最薄

- [ ] **Step 4: cobra 接线**

`cmd/client/init.go`：`Use: "init"`，`--host` flag，`RunE` 调 `setup.RunInit`（Paths 默认 `trace.DefaultPaths()`）。`root.go` 注册 `init`。

- [ ] **Step 5: 单测 + 提交**

- hooks 合并：空文件 / 无 aris 项 / 含旧 aris 项（路径不同去重）/ 未知顶层字段保留 / .bak 生成与权限 / 写回权限 0600
- config Save/Load 往返；host 归一化纯函数；key 保留逻辑纯函数
- wizard 端到交互不测（huh 需要 TTY），逻辑层全覆盖

```bash
go test ./test/unit/client/... -v && go run ./cmd/server lint conv ./...
git add -A && git commit -m "feat(client): 新增 aris init huh 配置向导"
```

---

## Task 5: `aris status` 面板

**Files:**
- Create: `internal/client/status/checks.go`、`render.go`、`status.go`
- Create: `cmd/client/status.go`；Modify: `cmd/client/root.go`
- Test: `test/unit/client/status/checks_test.go`、`render_test.go`

**Interfaces:**
- Produces: `status.Report`（六节结果聚合结构体）、`status.Collect(ctx, paths, apiClient) *Report`（并发）、`status.Render(w, report, now time.Time)`、`status.RenderJSON(w, report)`
- Produces: `status.RunStatus(ctx, StatusOptions{Paths, Out, JSON bool, HTTPClient}) error`

- [ ] **Step 1: checks.go — 并发收集**

```go
type Report struct {
	ConfigFound bool
	Host        string
	Agent       string
	ServerOK    bool
	ServerLatency time.Duration
	ServerErr   string
	AuthOK      bool
	AuthDetail  string        // key 脱敏（末 4 位）或失败原因
	HooksFound  int           // ~/.codex/hooks.json 中 aris hook 命令出现的事件数（满分 10）
	HooksMissing []string
	PendingCount int
	PendingBytes int64
	RejectedCount int
	Summary     *api.Summary  // 失败时 nil + SummaryErr
	SummaryErr  string
	RecentErrors int           // 当日日志条目数
}
```

- `Collect`：`sync.WaitGroup` 三路并发——本地文件扫描（config/spool/rejected/logs/hooks.json）、health、（有 config 才发）check+summary
- 无 config：仅本地节 + 引导提示，不发网络请求
- hooks 检测复用 setup 包的解析函数（`setup.InspectCodexHooks(paths, binPath) (found int, missing []string)`，Task 4 同步抽出）

- [ ] **Step 2: render.go — 面板渲染**

- TTY：六节 `SectionTitle + CheckRow`（布局同 spec §4.2 示意）；时间用相对格式（`5m ago`，自实现短小 helper）；字节 human-readable（`12.4 KB`）
- 非 TTY/`NO_COLOR`：同样结构，lipgloss 自动无色
- `RenderJSON`：`Report` 的 JSON 投影（`--json`，sonic.MarshalIndent，key 脱敏保持一致）

- [ ] **Step 3: status.go — 编排**

`huh.NewSpinner().Title("Checking status...").Action(...)` 包裹 `Collect`，结束后 `Render` 到 Out。`--json` 时跳过 spinner 直接 `RenderJSON`。

- [ ] **Step 4: cobra 接线**

`cmd/client/status.go`：`Use: "status"`，`--json` flag。`root.go` 注册。

- [ ] **Step 5: 单测 + 提交**

- checks：fixture 目录（构造 pending/rejected/logs/hooks.json）+ httptest 服务端，覆盖无 config / 全绿 / 部分失败降级
- render：golden 测试（强制无色 renderer）；`--json` schema 断言（json.Unmarshal 到 map 验证键）

```bash
go test ./test/unit/client/... -v && go run ./cmd/server lint conv ./...
git add -A && git commit -m "feat(client): 新增 aris status 状态面板"
```

---

## Task 6: install.sh 瘦身 + 常量清理 + 收尾

**Files:**
- Modify: `internal/handler/install_trace_client.sh.tmpl`
- Modify: `internal/common/constant/traceclient.go`
- Modify: `test/unit/client/trace/command_tree_test.go`
- Modify: `test/e2e/trace/install_script_test.go`
- Modify: `CONTEXT.md`

- [ ] **Step 1: 重写模板**

保留：shebang/`set -eu`/host 嵌入/preflight（**删除 jq 检查**）/平台探测/下载/sha256 校验/原子安装。删除：`/dev/tty` 重定向、四步交互、config/hooks 写入。末尾：

```sh
echo "Installed to $aris_bin"
exec "$aris_bin" init --host "$host"
```

- [ ] **Step 2: 常量清理**

`traceclient.go`：删除旧向导提示常量（`TraceClientInitStep*`/`TraceClientInitConnected`/`TraceClientInitAgentPrompt`/`TraceClientInitAPIKeyPrompt`/`TraceClientInitKeepAPIKeyPrompt`/`TraceClientInitMissingAPIKeyMessage`/`TraceClientInitInvalidAgentMessage`/`TraceClientInitAPIKeyFailed`/`TraceClientInitRetryPrompt`/`TraceClientInitAPIKeyRetryPrompt`/`TraceClientInitDone`/`TraceClientInitConfigFormat`/`TraceClientNegative*`/`TraceClientJSONIndent`，以实际引用为准，`grep -rn` 确认零引用后删）；保留 `TraceClientInitNonInteractiveMessage`、`TraceClientInitApprovalHint`（向导复用）、`TraceClientCheckPath`。

- [ ] **Step 3: 测试更新**

- `command_tree_test.go`：`aris --help` 断言含 `init`、`status`、`trace`；禁止项不变
- `install_script_test.go`：断言脚本含 `init --host`、不含 `jq`、不含 `stty`、不含 `[1/4]`

- [ ] **Step 4: CONTEXT.md 更新**

客户端段落更新为：`aris` 二进制包含 `init`（huh 配置向导）、`status`（状态面板）、`trace ingest` 三个命令；install.sh 为纯下载器。

- [ ] **Step 5: 全量验证**

```bash
go build ./... && make build-client-all
go test -count=1 ./cmd/... ./internal/... ./test/...
go run ./cmd/server lint conv ./...
```

- [ ] **Step 6: 提交**

```bash
git add -A && git commit -m "refactor(client): install.sh 退化为纯下载器，向导迁入 aris init"
```

---

## Self-Review

**Spec coverage:**

- ✅ spec §2.1 命令树/包结构：Task 2/3/4/5 + `cmd/client` 接线（Task 4/5）
- ✅ spec §3 init 向导（四步、/dev/tty、--host、ARIS_API_KEY、hooks 幂等、config 0600）：Task 4
- ✅ spec §4 status 面板（六节、并发、--json、降级）：Task 5
- ✅ spec §5 摘要端点（reportGroup、owner 聚合、DTO 规范）：Task 1
- ✅ spec §6 install.sh 瘦身（去 jq、exec init）：Task 6
- ✅ spec §7 视觉系统（语义色、图标、组件、降级）：Task 2
- ✅ spec §8 测试计划：各 Task 测试步骤 + Task 6 收尾验证
- ✅ spec §9 边界（ingest 零改动、包边界、交叉编译）：Global Constraints + Task 6 Step 5

**Placeholder scan:** 无 TBD/TODO。

**Type consistency:**

- `SummarizeByOwner` 签名在 domain 接口（Task 1 Step 2）、repository 实现（Step 3）、usecase（Step 4）一致
- `TraceClientSummaryView` ↔ `GetTraceClientSummaryRsp` 字段一一对应（含 `*time.Time` 空态）
- `api.Summary`（Task 3）与服务端 Rsp JSON 键一致
- `InitOptions.Paths` 复用 `trace.Paths`；hooks 事件清单与 install.sh / 常量一致（10 个）

**简化说明（不偏离 spec）：** spec §3/§4 中"自定义 bubbletea 模型 + bubbles/spinner"统一落地为 huh 原生 Spinner/表单（huh 即 bubbletea inline 程序），交互与视觉等价，代码量更小；spec §2.1 包结构不变。
