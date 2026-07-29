# Aris 客户端 TUI 重构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 基于 charmbracelet 生态重构 `aris` 客户端：新增 `aris init`（huh 四步配置向导）与 `aris status`（状态面板），install.sh 退化为纯下载器。**服务端零改动**（复用现有 health / client check 端点）。

**Architecture:** huh 表单（Input/Select/Confirm/Spinner，基于 bubbletea inline 模式）承担全部交互，lipgloss 提供主题与静态渲染；不编写自定义 bubbletea 模型。`aris status` 并发收集检查结果后用 lipgloss 一次性静态渲染。

**Tech Stack:** Go 1.25（cobra、huh/bubbletea/bubbles/lipgloss、sonic、golang.org/x/term），POSIX sh（install.sh 模板）。

**Spec:** `docs/superpowers/specs/2026-07-28-aris-client-tui-redesign-design.md`

## Global Constraints

- `trace ingest` 路径（`internal/client/trace/` 的 ingest/spool/rollout/state 逻辑）行为零改动；fail-open 语义不变
- 客户端包边界：`internal/client/**` 不得导入 server / db / router / web embed / dto / application
- 文件权限：`~/.aris/**` 目录 0700；`config.json` / `hooks.json` / `.bak` 0600；全部原子写（沿用 `writePrivateFile`）
- API Key 不出现在 flag、命令行参数、日志中；来源优先级 `ARIS_API_KEY` env > 向导密码输入；status 中脱敏显示（仅末 4 位）
- 10 个 hook 事件：`SessionStart, UserPromptSubmit, PreToolUse, PermissionRequest, PostToolUse, Stop, SubagentStart, SubagentStop, PreCompact, PostCompact`；hook group 格式 `{"matcher":"","hooks":[{"type":"command","command":"<bin> trace ingest","timeout":30}]}`
- 交互非 TTY 时：init 自动打开 `/dev/tty`（stdin 是 curl 管道场景）；无可用 TTY 报错退出
- 服务端零改动：不新增端点；status 复用 `GET /health` 与 `GET /api/v1/trace/client/check`
- Go lint：`go run ./cmd/server lint conv ./...` 必须通过；`go test ./cmd/... ./internal/... ./test/...` 必须通过
- 语言：代码标识符英文；CLI 面向用户的文案英文（与现有 install.sh 输出口径一致）

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `go.mod` / `go.sum` | Modify | 新增 huh/bubbletea/bubbles/lipgloss 依赖 |
| `internal/client/ui/theme.go` | Create | lipgloss 语义色/图标/间距 token + huh 主题 |
| `internal/client/ui/components.go` | Create | StepHeader/SectionTitle/CheckRow/KeyValue/SummaryPanel 渲染 |
| `internal/client/api/client.go` | Create | 控制面 HTTP 客户端：CheckHealth/CheckAPIKey |
| `internal/client/trace/config.go` | Modify | `ConfigStore` 恢复 `Save` 方法 |
| `internal/client/setup/hooks.go` | Create | codex hooks.json 幂等合并（移植 install.sh jq 逻辑） |
| `internal/client/setup/wizard.go` | Create | huh 向导编排 + /dev/tty 处理 + RunInit 入口 |
| `internal/client/status/checks.go` | Create | 五节检查并发收集 |
| `internal/client/status/render.go` | Create | lipgloss 面板渲染 + `--json` 输出 |
| `internal/client/status/status.go` | Create | RunStatus 编排（spinner + 渲染） |
| `cmd/client/root.go` | Modify | 注册 `init`、`status` |
| `cmd/client/init.go` / `status.go` | Create | cobra 命令定义（`--host` / `--json` flag） |
| `internal/handler/install_trace_client.sh.tmpl` | Modify | 纯下载器 + `exec aris init --host` |
| `internal/common/constant/traceclient.go` | Modify | 清理旧向导提示常量 |
| `test/unit/client/trace/command_tree_test.go` | Modify | 断言 init/status/trace 存在 |
| `test/unit/client/setup/*_test.go` | Create | hooks 合并、config 保存单测 |
| `test/unit/client/api/*_test.go` | Create | httptest 客户端单测 |
| `test/unit/client/status/*_test.go` | Create | checks/render 单测（golden） |
| `test/unit/client/ui/*_test.go` | Create | 组件渲染快照 |
| `test/e2e/trace/install_script_test.go` | Modify | 断言脚本含 `init --host`、不含 jq/交互 |
| `CONTEXT.md` | Modify | 更新客户端命令描述 |

---

## Task 1: 依赖引入 + `internal/client/ui` 主题与组件

**Files:**
- Modify: `go.mod`（新增依赖）
- Create: `internal/client/ui/theme.go`
- Create: `internal/client/ui/components.go`
- Test: `test/unit/client/ui/components_test.go`

**Interfaces:**
- Produces: `ui.StepHeader(step, total, title string) string`、`ui.SectionTitle(name string) string`、`ui.CheckRow(level Level, label, detail string) string`、`ui.KeyValue(pairs ...[2]string) string`、`ui.SummaryPanel(lines ...string) string`、`ui.HuhTheme() *huh.Theme`
- Produces: `ui.Level`（`LevelOK / LevelFail / LevelWarn / LevelInfo`）

- [ ] **Step 1: 引入 TUI 依赖**

```bash
go get github.com/charmbracelet/huh@latest github.com/charmbracelet/bubbles@latest github.com/charmbracelet/lipgloss@latest
go mod tidy
```
（bubbletea、x/term 由 huh 传递引入；确认 `go build ./...` 通过）

- [ ] **Step 2: theme.go**

- 语义色（`lipgloss.AdaptiveColor`，明暗终端自适应）：Primary（clay 橙 `#CC785C`/`#D97757`）、Success（绿）、Warning（黄）、Error（红）、Muted（灰）
- 图标常量：`IconOK="✓" IconFail="✗" IconWarn="!" IconSection="◆"`（非 emoji）
- `Level` 枚举：`LevelOK / LevelFail / LevelWarn / LevelInfo`
- `HuhTheme()`：基于 `huh.ThemeCharm()` 覆盖主色为 Primary，聚焦/错误样式对齐语义色
- 导出 `Renderer(w io.Writer) *lipgloss.Renderer`；`NO_COLOR`/非 TTY 时 lipgloss 自动降级

- [ ] **Step 3: components.go**

- `StepHeader`：`[1/4] Connect to server`（序号 Muted、标题 Primary Bold）
- `SectionTitle`：`◆ Server`（accent 色）
- `CheckRow`：`✓ label · detail`（图标按 Level 着色，detail Muted）
- `KeyValue`：冒号对齐键值对
- `SummaryPanel`：rounded border 成功面板
- 全部纯函数返回 string，不直接写 stdout（可测试）

- [ ] **Step 4: 快照测试 + 提交**

各组件 golden 测试（固定 lipgloss renderer 强制无色彩，比对字面输出）。

```bash
go test ./test/unit/client/ui/ -v && git add -A && git commit -m "feat(client): 新增 ui 主题与渲染组件包"
```

---

## Task 2: `internal/client/api` 控制面 HTTP 客户端

**Files:**
- Create: `internal/client/api/client.go`
- Test: `test/unit/client/api/client_test.go`

**Interfaces:**
- Produces: `api.New(baseURL, apiKey string, hc *http.Client) *Client`
- Produces: `(c *Client) CheckHealth(ctx) (time.Duration, error)`、`(c *Client) CheckAPIKey(ctx) error`

- [ ] **Step 1: client.go**

- 超时沿用 `constant.TraceClientHTTPTimeout`；baseURL 去尾部 `/`
- `CheckHealth`：`GET {base}/health`，返回 RTT；非 2xx 视为失败
- `CheckAPIKey`：`GET {base}{constant.TraceClientCheckPath}`（Bearer），2xx 通过
- 错误统一 `ierr.Wrap`，不暴露 key

- [ ] **Step 2: httptest 单测 + 提交**

覆盖：health 延迟返回、check 401/200、baseURL 尾斜杠处理、超时。

```bash
go test ./test/unit/client/api/ -v && git add -A && git commit -m "feat(client): 新增控制面 API 客户端（health/check）"
```

---

## Task 3: `aris init` 向导

**Files:**
- Modify: `internal/client/trace/config.go`（恢复 Save）
- Create: `internal/client/setup/hooks.go`、`internal/client/setup/wizard.go`
- Create: `cmd/client/init.go`；Modify: `cmd/client/root.go`
- Test: `test/unit/client/setup/hooks_test.go`、`test/unit/client/setup/wizard_test.go`

**Interfaces:**
- Produces: `setup.InstallCodexHooks(paths trace.Paths, binPath string) error`（幂等，写前备份）
- Produces: `setup.InspectCodexHooks(paths trace.Paths, binPath string) (found int, missing []string)`（status 复用）
- Produces: `setup.RunInit(ctx, setup.InitOptions) error`；`InitOptions{Host string, Paths trace.Paths, In io.Reader, Out io.Writer, HTTPClient *http.Client}`
- Consumes: `api.Client`（Task 2）、`ui.*`（Task 1）

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
- `InspectCodexHooks` 复用同一解析：统计 10 个事件中已注册 aris 命令的数量与缺失列表

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

## Task 4: `aris status` 面板

**Files:**
- Create: `internal/client/status/checks.go`、`render.go`、`status.go`
- Create: `cmd/client/status.go`；Modify: `cmd/client/root.go`
- Test: `test/unit/client/status/checks_test.go`、`render_test.go`

**Interfaces:**
- Produces: `status.Report`（五节结果聚合结构体）、`status.Collect(ctx, paths, apiClient) *Report`（并发）、`status.Render(w, report)`、`status.RenderJSON(w, report)`
- Produces: `status.RunStatus(ctx, StatusOptions{Paths, Out, JSON bool, HTTPClient}) error`

- [ ] **Step 1: checks.go — 并发收集**

```go
type Report struct {
	ConfigFound   bool
	Host          string
	Agent         string
	ServerOK      bool
	ServerLatency time.Duration
	ServerErr     string
	AuthOK        bool
	AuthDetail    string   // key 脱敏（末 4 位）或失败原因
	HooksFound    int      // ~/.codex/hooks.json 中 aris hook 注册的事件数（满分 10）
	HooksMissing  []string
	PendingCount  int
	PendingBytes  int64
	RejectedCount int
	RecentErrors  int      // 当日日志条目数
}
```

- `Collect`：`sync.WaitGroup` 三路并发——本地文件扫描（config/spool/rejected/logs/hooks.json）、health、（有 config 才发）key check
- 无 config：仅本地节 + 引导提示，不发网络请求
- hooks 检测复用 `setup.InspectCodexHooks`（Task 3）

- [ ] **Step 2: render.go — 面板渲染**

- 五节 `SectionTitle + CheckRow`（布局同 spec §4.2 示意）；字节 human-readable（`12.4 KB`，自实现短 helper）
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

## Task 5: install.sh 瘦身 + 常量清理 + 收尾

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

- ✅ spec §2.1 命令树/包结构：Task 1/2/3/4 + `cmd/client` 接线（Task 3/4）
- ✅ spec §3 init 向导（四步、/dev/tty、--host、ARIS_API_KEY、hooks 幂等、config 0600）：Task 3
- ✅ spec §4 status 面板（五节、并发、--json、降级；复用现有 check 端点，服务端零改动）：Task 2 + Task 4
- ✅ spec §5 install.sh 瘦身（去 jq、exec init）：Task 5
- ✅ spec §6 视觉系统（语义色、图标、组件、降级）：Task 1
- ✅ spec §7 测试计划：各 Task 测试步骤 + Task 5 收尾验证
- ✅ spec §8 边界（ingest 零改动、包边界、交叉编译）：Global Constraints + Task 5 Step 5

**Placeholder scan:** 无 TBD/TODO。

**Type consistency:**

- `api.Client`（Task 2）被 setup（Task 3）与 status（Task 4）共用，签名一致
- `setup.InspectCodexHooks`（Task 3）与 status 的 hooks 节（Task 4）签名一致
- `InitOptions.Paths` 复用 `trace.Paths`；hooks 事件清单与 install.sh / 常量一致（10 个）

**简化说明（不偏离 spec）：** spec §3/§4 中"自定义 bubbletea 模型 + bubbles/spinner"统一落地为 huh 原生 Spinner/表单（huh 即 bubbletea inline 程序），交互与视觉等价，代码量更小；spec §2.1 包结构不变。
