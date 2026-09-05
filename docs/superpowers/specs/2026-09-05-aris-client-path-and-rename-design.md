# ArisClient PATH 写入与 TraceClient 重命名设计

> 日期：2026-09-05
> 状态：已批准
> 范围：install.sh 安装脚本 PATH 写入 + TraceClient → ArisClient 统一重命名

## 背景与目标

1. **PATH 写入**：`GET /install.sh` 渲染的自包含安装脚本（`internal/handler/install_trace_client.sh.tmpl`）目前只把二进制装到 `$HOME/.aris/bin/aris`，装完用户只能全路径调用。目标：安装时将安装目录写入用户 shell 的 PATH，使 `aris` 成为可直接执行的命令。
2. **重命名**：领域概念 `TraceClient（Trace 客户端）` 统一更名为 `ArisClient（Aris 客户端）`，消除"trace client"这一与产品名不一致的旧称。

## 决策记录

| 决策点 | 结论 | 理由 |
|--------|------|------|
| PATH 逻辑放哪 | 安装脚本模板内联（方案 A） | 安装关注点（下载+落盘+进 PATH）集中一处；不向 `aris init` 向导扩散"安装目录"职责 |
| 写哪些 rc 文件 | 按 `$SHELL` 检测写对应 rc | 与 Deno/pnpm 安装器一致，改动面最小 |
| opt-out 机制 | 不加，默认写 | 写入幂等、带标记注释、目录在用户家目录内，风险低；wizard 本身也改 hooks 配置 |
| HTTP 路径 `/api/cli/v1/trace/client/check` | 直接改为 `/api/cli/v1/aris/client/check`，不保留旧路径 | 用户确认接受破坏性变更；旧客户端需重装 |
| Redis 键 `trace:client:ticket:%s` | 标识符与值一并改 | `TraceClientTicketKeyTemplate` 全库无使用点（死常量），改名零影响 |
| 分支策略 | 直接在 master 开发 | 用户既有要求（快速任务绕过 worktree） |

## 第一部分：install.sh PATH 写入

### 行为规格

在 `install_trace_client.sh.tmpl` 的 `echo "Installed to $aris_bin"` 之后、`exec "$aris_bin" init --host "$host"` 之前插入 PATH 段落：

1. **rc 分派**（POSIX sh）：
   - `$SHELL` basename 为 `zsh` → `$HOME/.zshrc`
   - 为 `bash` → `$HOME/.bashrc` 存在则用之；否则 Darwin（macOS bash 登录 shell 不读 `.bashrc`）用 `$HOME/.bash_profile`，Linux 仍用 `$HOME/.bashrc`
   - 其他 → `$HOME/.profile`
   - 目标文件不存在则创建。
2. **幂等守卫**：`grep -qF ".aris/bin"` 命中目标 rc 则跳过写入并打印 `PATH already configured`；用 `if grep` 包裹避免 `set -eu` 下非零退出中断安装。
3. **写入内容**：追加带标记注释的块（供幂等 grep 与人工识别）：
   ```sh
   # aris (added by installer)
   export PATH="$HOME/.aris/bin:$PATH"
   ```
4. **当前会话生效**：脚本内 `export PATH="$aris_bin_dir:$PATH"`（仅影响脚本子 shell，保证 `exec aris init` 上下文一致）。
5. **失败容忍**：rc 不可写等失败只向 stderr 打 warning 继续，不让安装失败。
6. **收尾提示**：`exec` 前打印 `Restart your shell or run: source <rc>`（必须在 exec 前，exec 后脚本不返回）。

### 不做的事（YAGNI）

- 不加 opt-out 参数 / 环境变量
- 不写 `~/.zshenv` / `~/.zprofile`
- 不支持 fish、不支持自定义安装目录
- 不在 `aris status` 里检测 PATH

## 第二部分：TraceClient → ArisClient 重命名

### 改动范围（全库标识符替换，约 28 个 Go 文件）

1. **常量**：`internal/common/constant/traceclient.go` → `git mv` 为 `arisclient.go`，全部 `TraceClient*` 常量（~100 个）改为 `ArisClient*`；`clientmodel.go`（6 个）、`rediskey.go`（1 个）同步。
2. **HTTP 路径**：`TraceClientCheckPath = "/api/cli/v1/trace/client/check"` → `ArisClientCheckPath = "/api/cli/v1/aris/client/check"`。服务端 `internal/router/cli.go` 路由字面量与客户端 `internal/client/api` 引用两端同步；OpenAPI OperationID（`checkTraceClientAPIKey` → `checkArisClientAPIKey`）、Summary、Description 同步。
   **破坏性变更**：已安装旧 `aris` 二进制硬编码旧路径，新服务端下 init/status 的 API Key 校验将 404，需重装。已确认接受。
3. **DTO/handler**：`CheckTraceClientReq` → `CheckArisClientReq`、`HandleCheckTraceClient` → `HandleCheckArisClient`，`TraceHandler` 接口签名同步。
4. **客户端 `internal/client/**`**：~15 个文件对常量的引用机械替换；用户可见字符串仅改含 "trace client" 字样的（OpenAPI Description 等）；`"Trace configuration completed"` 这类指 trace 功能域的文案不动。
5. **不改的**：`internal/client/trace/**` 包路径与 `aris trace ingest/install` 子命令名（属 trace 功能域命名）；JSON 字段（无相关 tag）；web 前端（无 "trace client" 文案，popover 走 `trace.install` i18n 键）。
6. **测试**：`test/unit/client/status/checks_test.go`、`test/e2e/trace/hook_test.go`（check 路径断言同步新路径）、`test/e2e/trace/install_script_test.go` 同步更新。
7. **文档**：`CONTEXT.md` 词条 `TraceClient（Trace 客户端）` → `ArisClient（Aris 客户端）`，词条行为描述合并 PATH 写入；README / docs 中出现处一并改。

### 实现方式

- 标识符替换用 Serena `replace_in_files` 全局字面替换 `TraceClient` → `ArisClient`（标识符前缀唯一，无 JSON tag / web 波及，安全）；文件改名用 `git mv`。
- 替换后全库 `grep "TraceClient\|trace/client\|trace_client"` 验证归零（CONTEXT.md 历史词条更新后同样归零）。

## 测试与验证

1. **e2e 内容断言**（`test/e2e/trace/install_script_test.go`，沿用现有风格扩展）：
   - 断言脚本包含 `# aris (added by installer)` 标记
   - 断言包含 `.zshrc` / `.bashrc` / `.bash_profile` / `.profile` 分派分支
   - 断言包含 `PATH already configured` 幂等提示
   - 断言 PATH 段位于 `Installed to` 之后、`exec` 之前
2. **语法校验**：渲染产物落临时文件跑 `sh -n`，防止模板改动引入语法错误。
3. **回归**：`go test ./test/e2e/trace/... ./test/unit/...` 全绿；`golangci-lint run` 通过。
4. **冒烟**：本地起服务 `curl -s localhost:<port>/install.sh | sh -n`；渲染产物人工核对 PATH 段。
5. **重命名验证**：全库 grep 归零 + `go build ./...` 通过。

## 风险与缓解

| 风险 | 缓解 |
|------|------|
| `set -eu` 下 grep 无命中导致脚本中断 | 幂等检查用 `if grep -qF` 包裹 |
| macOS bash 写 `~/.bashrc` 不生效 | bash 分支：`.bashrc` 存在则用，否则 Darwin 用 `.bash_profile` |
| rc 写失败导致安装失败 | 失败仅 warning，不阻塞安装与 init |
| 旧客户端 check 404 | 已接受的破坏性变更，重装即恢复 |
| 全局替换误伤 | 替换前全库 grep 确认 `TraceClient` 均为该概念；替换后 build + test + lint 三重验证 |
