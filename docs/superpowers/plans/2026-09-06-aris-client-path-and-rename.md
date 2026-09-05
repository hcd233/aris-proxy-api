# ArisClient PATH 写入与 TraceClient 重命名 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** install.sh 安装后将 `$HOME/.aris/bin` 写入用户 shell PATH；同时把 TraceClient 概念全库统一更名为 ArisClient（含 check 路径破坏性变更）。

**Architecture:** PATH 逻辑内联在 `internal/handler/install_aris_client.sh.tmpl`（POSIX sh，按 `$SHELL` 分派 rc、幂等、失败容忍）；重命名通过常量文件改名 + Serena 全局字面替换完成，路由路径由 `CLIAPIPrefix + ArisClientCheckRoutePath` 同源派生，客户端自动跟随。

**Tech Stack:** Go 1.24+（标准库 testing）、text/template 渲染 POSIX sh、Serena replace_in_files、make lint / make test。

**Spec:** `docs/superpowers/specs/2026-09-05-aris-client-path-and-rename-design.md`

## Global Constraints

- 直接在 master 开发（用户既定偏好，不建 worktree）；**只本地提交，禁止 push**（push master 触发部署）。
- 每次 `git commit` 前必须先跑 `git rev-parse -q --verify MERGE_HEAD`，输出为空才能提交（防止吞掉用户并行 pull 留下的进行中合并）。
- 编辑 Go 文件前先跑 `sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path <目标文件>`，完整读输出，不得截断。
- 测试只用标准库 `testing`，禁止 testify/gomock；`*_test.go` 只能放 `test/unit/<topic>/` 或 `test/e2e/<topic>/`。
- 全项目统一 `github.com/bytedance/sonic`，禁止 `encoding/json`；业务错误统一走 `internal/common/ierr`。
- 常量只放 `internal/common/constant/`；业务包禁止本地 const 块；HTTP 状态码用 `fiber.StatusXxx`。
- `docs/superpowers/` 下的历史 plans/specs 是历史文档，**不参与**重命名替换。
- pre-commit 对暂存的 Go 文件自动跑 gofmt / go mod tidy / vet / test / lint，提交耗时数分钟属正常，不要中断。
- shell 模板必须保持 POSIX sh 兼容（`#!/bin/sh` + `set -eu`），禁用 bashism。

---

### Task 1: install.sh PATH 写入段（TDD）

**Files:**
- Modify: `test/e2e/trace/install_script_test.go`（追加断言 + 新增语法测试 + 补 import）
- Modify: `internal/handler/install_trace_client.sh.tmpl:52-55`（在 `echo "Installed to $aris_bin"` 与 `# --- run interactive setup wizard` 注释之间插入）

**Interfaces:**
- Consumes: 现有模板变量 `aris_bin="$HOME/.aris/bin/aris"`、`host`；现有测试骨架 `TestInstallScript_ReturnsScriptWithHost`（fiber + huma 渲染 `/install.sh`）。
- Produces: 渲染产物包含标记块 `# aris (added by installer)`、rc 分派（`.zshrc` / `.bashrc` / `.bash_profile` / `.profile`）、幂等提示 `PATH already configured`，PATH 段位于 `Installed to` 之后、`exec "$aris_bin" init` 之前。Task 2 的 `git mv` 会改模板文件名，本任务不改名。

- [ ] **Step 1: 编辑 Go 前加载 modern-go 指南**

Run: `sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path test/e2e/trace/install_script_test.go`
Expected: 完整输出指南列表（禁止 head/tail/grep 截断）。

- [ ] **Step 2: 在 `TestInstallScript_ReturnsScriptWithHost` 中追加 PATH 断言**

在 `if !strings.Contains(script, "init --host")` 断言块之后（现有 `for _, removed := range ...` 循环之前）插入：

```go
	// PATH 写入段：带标记注释、按 $SHELL 分派 rc、幂等、位置在安装消息与 exec 之间
	pathMarker := "# aris (added by installer)"
	if !strings.Contains(script, pathMarker) {
		t.Fatalf("script must append a marked PATH block, got:\n%s", script)
	}
	for _, rc := range []string{".zshrc", ".bashrc", ".bash_profile", ".profile"} {
		if !strings.Contains(script, rc) {
			t.Fatalf("script must dispatch PATH setup to %s by $SHELL", rc)
		}
	}
	if !strings.Contains(script, "PATH already configured") {
		t.Fatalf("script must skip PATH setup when already configured (idempotent)")
	}
	installedIdx := strings.Index(script, "Installed to")
	pathIdx := strings.Index(script, pathMarker)
	execIdx := strings.Index(script, `exec "$aris_bin" init`)
	if installedIdx < 0 || pathIdx < 0 || execIdx < 0 || pathIdx < installedIdx || pathIdx > execIdx {
		t.Fatalf("PATH block must sit between install message and exec init, got:\n%s", script)
	}
```

- [ ] **Step 3: 追加 shell 语法守卫测试（文件末尾）**

import 块补充 `"os"`、`"os/exec"`、`"path/filepath"`（标准库组，与 `"io"` 等同组）。文件末尾追加：

```go
func TestInstallScript_ShellSyntaxValid(t *testing.T) {
	t.Parallel()
	traceHandler := handler.NewTraceHandler(handler.TraceDependencies{})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Install Script Test", "1.0"))
	huma.Register(api, huma.Operation{
		OperationID: "installTraceScript", Method: http.MethodGet, Path: "/install.sh",
		Tags: []string{constant.TagTrace},
	}, traceHandler.HandleInstallScript)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/install.sh", http.NoBody)
	req.Host = "aris.example.com"
	req.Header.Set(constant.HTTPHeaderXForwardedProto, "https")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	scriptPath := filepath.Join(t.TempDir(), "install.sh")
	if err := os.WriteFile(scriptPath, body, 0o700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("sh", "-n", scriptPath).CombinedOutput(); err != nil {
		t.Fatalf("install script has shell syntax errors: %v\n%s", err, out)
	}
}
```

- [ ] **Step 4: 跑测试确认失败（红）**

Run: `go test -count=1 -run 'TestInstallScript' ./test/e2e/trace/ -v`
Expected: `TestInstallScript_ReturnsScriptWithHost` **FAIL**（`script must append a marked PATH block`）；`TestInstallScript_ShellSyntaxValid` PASS（现有脚本本就语法合法，此测试是防回归守卫）。

- [ ] **Step 5: 在模板中实现 PATH 段**

`internal/handler/install_trace_client.sh.tmpl`：在 `echo "Installed to $aris_bin"` 行之后、`# --- run interactive setup wizard ...` 注释之前插入（保持模板其余内容不动）：

```sh

# --- add install dir to PATH (idempotent; per detected login shell) ---
aris_shell="${SHELL:-}"
aris_bin_dir="$(dirname "$aris_bin")"
export PATH="$aris_bin_dir:$PATH"
case "${aris_shell##*/}" in
  zsh) aris_rc="$HOME/.zshrc" ;;
  bash)
    if [ -f "$HOME/.bashrc" ]; then
      aris_rc="$HOME/.bashrc"
    elif [ "$(uname -s)" = "Darwin" ]; then
      aris_rc="$HOME/.bash_profile"
    else
      aris_rc="$HOME/.bashrc"
    fi
    ;;
  *) aris_rc="$HOME/.profile" ;;
esac
if grep -qF ".aris/bin" "$aris_rc" 2>/dev/null; then
  echo "PATH already configured in $aris_rc"
elif touch "$aris_rc" 2>/dev/null && printf '\n# aris (added by installer)\nexport PATH="$HOME/.aris/bin:$PATH"\n' >> "$aris_rc"; then
  echo "Added $aris_bin_dir to PATH via $aris_rc"
else
  echo "Warning: could not update $aris_rc. Add $aris_bin_dir to PATH manually." >&2
fi
echo "Restart your shell or run: source $aris_rc"
```

设计要点（实现者须知）：`${SHELL:-}` + `##*/` 替代 `basename` 子进程（规避 `set -u` 下 unset 报错与空串边界）；幂等 grep 用 `if/elif` 包裹避免 `set -e` 中断；`touch && printf >>` 失败落入 warning 分支不阻塞安装；`export PATH` 使后续 `exec aris init` 与提示一致；提示必须在 `exec` 前输出。

- [ ] **Step 6: 跑测试确认通过（绿）**

Run: `go test -count=1 -run 'TestInstallScript' ./test/e2e/trace/ -v`
Expected: 两个测试全部 PASS。

- [ ] **Step 7: 提交**

```bash
git rev-parse -q --verify MERGE_HEAD   # 必须无输出
git add test/e2e/trace/install_script_test.go internal/handler/install_trace_client.sh.tmpl
git commit -m "feat(client): install.sh 安装后将安装目录写入 shell PATH"
```

---

### Task 2: TraceClient → ArisClient 全局重命名

**Files:**
- Rename: `internal/common/constant/traceclient.go` → `internal/common/constant/arisclient.go`（git mv）
- Rename: `internal/handler/install_trace_client.sh.tmpl` → `internal/handler/install_aris_client.sh.tmpl`（git mv，Task 1 已改其内容）
- Modify: `internal/handler/trace.go`（go:embed 行、`HandleCheckTraceClient`、`CheckTraceClientReq` 引用）
- Modify: `internal/common/constant/string.go:176-177`（`ArisClientCheckRoutePath` 值改为 `"/aris/client/check"`）
- Modify: `internal/common/constant/rediskey.go:7`（键值 `trace:client:ticket:%s` → `aris:client:ticket:%s`，死常量一并统一）
- Modify: `README.md:150`（`/trace/client/check` → `/aris/client/check`，注意 `/trace/event` 与 `/api/cli/v1/model/list` 不动）
- Modify: `internal/`、`test/` 下全部含 `TraceClient` 的 Go 文件（约 28 个，Serena 批量替换）

**Interfaces:**
- Consumes: Task 1 产物（模板已含 PATH 段；文件名在本任务改为 `install_aris_client.sh.tmpl`）。
- Produces: 导出标识符统一为 `ArisClient*` 前缀；check 接口新路径 `/api/cli/v1/aris/client/check`（`constant.ArisClientCheckPath`，由 `CLIAPIPrefix + ArisClientCheckRoutePath` 派生）；`ArisClientTicketKeyTemplate = "aris:client:ticket:%s"`。**破坏性变更（已获用户确认）**：旧已安装 aris 二进制的 init/status API Key 校验将 404，需重装。

- [ ] **Step 1: 提交前状态核查**

```bash
git rev-parse -q --verify MERGE_HEAD   # 必须无输出
git status --short                      # 必须干净
grep -rln "TraceClient" internal cmd test --include="*.go" | wc -l   # 记录基线文件数
grep -rn "install_trace_client" internal test --include="*.go"      # 应只有 internal/handler/trace.go 的 embed 行
```

Expected: 无 MERGE_HEAD、工作区干净；记录 TraceClient 文件清单供 Step 4 核对。

- [ ] **Step 2: git mv 两个文件**

```bash
git mv internal/common/constant/traceclient.go internal/common/constant/arisclient.go
git mv internal/handler/install_trace_client.sh.tmpl internal/handler/install_aris_client.sh.tmpl
```

- [ ] **Step 3: 修 embed 行与三个字面量（精确点替换）**

`internal/handler/trace.go` 的 embed 行：

```go
//go:embed install_aris_client.sh.tmpl
var installScriptTemplate string
```

Serena `replace_in_files` 逐条执行（全部 mode=literal）：

| needle | repl | relative_path | 预期命中 |
|--------|------|---------------|---------|
| `trace client` | `aris client` | `internal` | 10（config.go×3、ingest.go×3、state.go×3、router/cli.go:47 Description×1） |
| `trace/client/check` | `aris/client/check` | `internal` | 1（string.go:177 值） |
| `trace:client:ticket:%s` | `aris:client:ticket:%s` | `internal/common/constant/rediskey.go` | 1 |
| `/trace/client/check` | `/aris/client/check` | `README.md` | 1（第 150 行；`/trace/event` 不受影响） |

- [ ] **Step 4: Serena 批量替换标识符**

Serena `replace_in_files`，mode=literal，needle=`TraceClient`，repl=`ArisClient`：

1. `relative_path: internal`（覆盖常量、DTO `CheckTraceClientReq`→`CheckArisClientReq`、handler `HandleCheckTraceClient`→`HandleCheckArisClient`、router OperationID `checkTraceClientAPIKey`→`checkArisClientAPIKey`、客户端 `internal/client/**` 全部引用）
2. `relative_path: test`（覆盖 `checks_test.go`、`hook_test.go` 的 `buildTraceClient`→`buildArisClient` 等）

先 `dry_run=true` 核对命中文件数与 Step 1 基线一致，再 `dry_run=false` 全量应用。

- [ ] **Step 5: 构建 + 聚焦测试**

```bash
go build ./cmd/server ./cmd/client
go test -count=1 ./test/e2e/trace/ ./test/unit/client/... 
```

Expected: 构建通过；测试全绿（hook_test 会真实构建并执行新二进制，check 路径两端同源）。

- [ ] **Step 6: 归零验证**

```bash
grep -rn "TraceClient\|trace/client\|trace_client\|trace client" internal cmd test README.md
```

Expected: **零命中**（CONTEXT.md 的词条在 Task 3 处理，此处未含）。若有命中逐条判断：属历史 spec 的不改，属活代码的补替换。

- [ ] **Step 7: lint + 提交**

```bash
make lint
git rev-parse -q --verify MERGE_HEAD   # 必须无输出
git add -A
git commit -m "refactor(client): TraceClient 标识与 check 路径统一更名为 ArisClient"
```

Expected: lint 通过；提交含改名文件、约 28 个 Go 文件替换、README 一行。pre-commit 会自动跑 gofmt/vet/test/lint，耗时属正常。

---

### Task 3: CONTEXT.md 词条更新 + 收尾验证

**Files:**
- Modify: `CONTEXT.md:225-227`（TraceClient 词条 → ArisClient，合并 PATH 行为描述）
- Create: serena memory `client/arisclient-rename-2026-09-06`（提交前沉淀）

**Interfaces:**
- Consumes: Task 1/2 全部产物。
- Produces: 领域词汇表与代码一致；全量测试与 lint 绿。

- [ ] **Step 1: 重写 CONTEXT.md 词条**

将现有 `**TraceClient（Trace 客户端）**` 词条（225-227 行）整体替换为：

```markdown
**ArisClient（Aris 客户端）**:
独立编译的 `aris` 二进制，从 `cmd/client` 构建，包含 `init`（huh 交互式配置向导：健康检查 → 选 agent（Codex / Claude Code / Both）→ API Key → 注册对应 hooks：codex 写 `~/.codex/hooks.json`，claude 写 `~/.claude/settings.json`，均幂等去重、保留既有配置、写前 .bak 备份）、`status`（状态面板：连通性、API Key 校验、hooks 注册状态、本地 spool/日志，支持 `--json`）、`trace ingest`（非交互 hook 回调，fail-open）三个命令，不链接数据库、Server、lint 或 Web 静态资源。支持 `darwin/amd64`、`darwin/arm64`、`linux/amd64`、`linux/arm64` 四个平台，产物发布到 GitHub Releases；`GET /install.sh` 返回的自包含脚本负责下载、校验、原子安装，并按 `$SHELL` 将安装目录 `$HOME/.aris/bin` 幂等写入 shell rc（zsh→`~/.zshrc`；bash→`~/.bashrc`，macOS 无 `.bashrc` 时用 `~/.bash_profile`；其他→`~/.profile`，写入失败仅告警不阻塞），末尾 `exec aris init --host <origin>` 进入配置向导。API Key 校验接口为 `GET /api/cli/v1/aris/client/check`（2026-09-06 由 `/trace/client/check` 更名，破坏性变更）。
_Avoid_: TraceClient, trace client, trace cli, codex hook script, install script
```

- [ ] **Step 2: 全量测试 + lint + 归零复核**

```bash
make test
make lint
grep -rn "TraceClient\|trace/client\|trace_client" internal cmd test README.md CONTEXT.md
```

Expected: `make test` 等价 `go test -count=1 ./cmd/... ./internal/... ./test/...` 全绿；lint 通过；grep 零命中（`docs/superpowers/` 历史文档除外，不在扫描范围）。

- [ ] **Step 3: 沉淀 serena memory**

写 memory `client/arisclient-rename-2026-09-06`，要点：check 路径现为 `/api/cli/v1/aris/client/check`（破坏性，旧客户端需重装）；标识符前缀统一 `ArisClient*`；模板改名 `install_aris_client.sh.tmpl` 且含幂等 PATH 段（`${SHELL:-}` + `##*/` 规避 set -u、if/elif 包裹 grep 防 set -e 中断）；`ArisClientTicketKeyTemplate` 为无使用点死常量仅统一字面。

- [ ] **Step 4: 提交**

```bash
git rev-parse -q --verify MERGE_HEAD   # 必须无输出
git add CONTEXT.md
git commit -m "docs: CONTEXT.md TraceClient 词条更名 ArisClient 并同步 install.sh PATH 行为"
```

- [ ] **Step 5: 汇报**

向用户汇报：改动清单、`make test`/`make lint` 证据、破坏性变更提示（旧客户端需重装）、未 push（等用户明确要求）。
