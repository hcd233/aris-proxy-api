# Install.sh 自包含安装脚本设计

## 1. 背景

当前 Aris 轨迹客户端的安装流程为：

1. Web 前端调用 `POST /api/v1/trace/client/ticket` 签发一次性下载票据。
2. 前端生成命令 `curl -fsSL -H 'Authorization: Bearer <ticket>' '<host>/api/v1/trace/client/install' | bash`。
3. 后端动态生成 install.sh（票据嵌入），脚本下载二进制后 `exec aris trace init --host <host>`。
4. `aris trace init` 在 Go 二进制内执行四步配置向导（健康检查、Agent 选择、API Key 验证、Hook 配置）。

问题：配置和安装流程被委托给 `aris trace init`（Go 二进制），不符合 `curl -fsSL <url>/install.sh | sh` 的标准安装模式（参考 [pi.dev/install.sh](https://pi.dev/install.sh)）。命令携带 auth header 和票据，不够简洁；配置逻辑封闭在二进制内，不可审计。

## 2. 目标与非目标

### 2.1 目标

1. 安装命令简化为 `curl -fsSL <host>/install.sh | sh`，无 auth header、无票据。
2. install.sh 自包含完成全部流程：preflight 检查、平台检测、下载二进制、四步交互式配置（健康检查、Agent 选择、API Key、Hook）。
3. `aris` 二进制只保留 `trace ingest`（Hook 回调），移除 `trace init`。
4. 移除票据系统（签发、存储、中间件、消费），二进制下载改为公开。
5. Web 安装对话框简化：不再签发票据，直接展示 `curl | sh` 命令。
6. 客户端二进制发布到 GitHub Releases，install.sh 从 GitHub 下载，不经过服务端 API（避免流量滥用）。
7. `trace ingest` 逻辑、config.json 格式、hooks.json 结构不变。

### 2.2 非目标

1. 不实现 install.sh 的 reinstall/uninstall 交互（首期只做全新安装）。
2. 不实现 logo 动画等 UI 打磨（首期关注功能正确性）。
3. 不改造 `trace ingest` 的 spool、rollout、上报链路。
4. 不实现二进制校验和（checksum）验证（首期信任 GitHub Releases 传输完整性）。

## 3. 整体架构

```
用户执行:  curl -fsSL <host>/install.sh | sh

install.sh (服务端用 text/template 生成，嵌入 host，无票据):
  ├─ preflight: 检查 curl/jq，检测 OS/arch
  ├─ 下载二进制: https://github.com/hcd233/aris-proxy-api/releases/latest/download/aris-<os>-<arch>
  ├─ 安装到 ~/.aris/bin/aris (0700)
  ├─ [1/4] 连接服务器: curl <host>/health (失败可重试)
  ├─ [2/4] 选择 Agent: 仅 Codex (按 Enter 确认)
  ├─ [3/4] API Key: read -s 隐藏输入 → curl 验证 /api/v1/trace/client/check
  ├─ [4/4] 配置 Hook: jq 修改 ~/.codex/hooks.json (10 事件，幂等，备份)
  ├─ 写 ~/.aris/trace/config.json (0600)
  └─ 提示用户去 Codex /hooks 手动批准

aris 二进制 (cmd/client):
  └─ 只保留 trace ingest (Hook 回调)
```

### 3.1 数据流

1. 用户从 Web UI 复制 `curl -fsSL <host>/install.sh | sh` 并在终端执行。
2. `GET /install.sh` 返回嵌入 host 的 POSIX sh 脚本（`text/plain`，`no-store`）。
3. 脚本从 GitHub Releases 下载二进制：`https://github.com/hcd233/aris-proxy-api/releases/latest/download/aris-<os>-<arch>`。
4. 脚本交互式收集 API Key，通过 `GET /api/v1/trace/client/check`（Bearer auth）验证。
5. 脚本用 `jq` 修改 `~/.codex/hooks.json`，写入 `~/.aris/trace/config.json`。
6. 后续 Codex 触发 hook 时调用 `aris trace ingest`，读取 config 上报事件（不变）。

## 4. install.sh 脚本设计

### 4.1 模板与生成

- 模板文件通过 `//go:embed` 嵌入服务端二进制。
- Handler 用 `text/template` 替换 `{{.Host}}`（从请求 `X-Forwarded-Proto` + `Host` 头推导 origin）。
- 返回 `Content-Type: text/plain`、`Cache-Control: no-store`。
- Host 验证：必须为 `http://` 或 `https://` 且有 host 部分，否则返回错误脚本。

### 4.2 脚本结构

脚本为 POSIX sh（`#!/bin/sh`），`set -eu`，主要阶段：

**Preflight：**
- 检查 `curl` 和 `jq` 是否可用，缺失则报错退出。
- 检测 OS/arch：`uname -s` + `uname -m` → `darwin/amd64`、`darwin/arm64`、`linux/amd64`、`linux/arm64`。不支持的平台报错退出。

**下载二进制：**
- `mktemp` 创建临时文件，`trap 'rm -f "$tmp"' EXIT`。
- GitHub Releases 下载：`curl -fsSL -o "$tmp" "https://github.com/hcd233/aris-proxy-api/releases/latest/download/aris-$os-$arch"`。
- HTTP 非 200 报错退出，不覆盖已有二进制。
- `mkdir -p -m 0700 ~/.aris/bin`，`chmod 0700 "$tmp"`，`mv "$tmp" ~/.aris/bin/aris`，清除 trap。

**[1/4] 连接服务器：**
- `curl -sf --max-time 5 "$host/health"`。
- 失败时提示 `Retry? [Y/n]`，读取 `/dev/tty`，`n`/`no` 退出，其他重试。

**[2/4] 选择 Agent：**
- 提示 `Press Enter to select Codex:`，读取 `/dev/tty`，空输入或 `codex` 通过，其他提示仅支持 Codex。

**[3/4] 配置 API Key：**
- 如果 `~/.aris/trace/config.json` 已存在且含 API Key，提示 `API key (Enter keeps current):`，否则 `API key:`。
- 隐藏输入：`stty -echo; read api_key; stty echo`（从 `/dev/tty` 读取）。
- 验证：`curl -sf --max-time 5 -H "Authorization: Bearer $api_key" "$host/api/v1/trace/client/check"`，期望 204。
- 失败时提示 `Retry API key? [Y/n]`，重试或退出。

**[4/4] 配置 Codex Hook：**
- `codex_hooks="$HOME/.codex/hooks.json"`
- 如果文件存在，先备份到 `$codex_hooks.bak`（`cp`，`chmod 600`）。
- 构建新 Aris hook group：`{"matcher":"","hooks":[{"type":"command","command":"<aris_path> trace ingest","timeout":30}]}`
- 对 10 个事件（`SessionStart`、`UserPromptSubmit`、`PreToolUse`、`PermissionRequest`、`PostToolUse`、`Stop`、`SubagentStart`、`SubagentStop`、`PreCompact`、`PostCompact`），用 `jq` 移除已有 Aris hook（匹配 command），追加新 group，保留非 Aris 配置。
- `mkdir -p -m 0700 ~/.codex`，原子写入（临时文件 + `mv`），`chmod 600`。

**写 config.json：**
- `mkdir -p -m 0700 ~/.aris/trace`
- 写入 `{"host":"<host>","agent":"codex","apiKey":"<api_key>"}`（`cat` heredoc），`chmod 600`。

**完成提示：**
- 打印配置路径。
- 提示用户在 Codex 中运行 `/hooks` 手动批准新增的 Aris hooks。

### 4.3 jq Hook 合并逻辑

```sh
hook_cmd="$HOME/.aris/bin/aris trace ingest"
aris_group='{"matcher":"","hooks":[{"type":"command","command":"'"$hook_cmd"'","timeout":30}]}'

# 读取现有配置或初始化
if [ -f "$codex_hooks" ]; then
  config=$(cat "$codex_hooks")
else
  config='{}'
fi

for event in SessionStart UserPromptSubmit PreToolUse PermissionRequest \
             PostToolUse Stop SubagentStart SubagentStop PreCompact PostCompact; do
  config=$(printf '%s' "$config" | jq \
    --arg event "$event" \
    --arg cmd "$hook_cmd" \
    --argjson group "$aris_group" \
    '.hooks[$event] = ((.hooks[$event] // [])
      | map(select(any(.hooks[]?; .command == $cmd) | not))
      + [$group])')
done

# 原子写入
tmp_config=$(mktemp)
printf '%s\n' "$config" | jq . > "$tmp_config"
chmod 600 "$tmp_config"
mv "$tmp_config" "$codex_hooks"
```

幂等性：重复执行时，`map(select(... | not))` 移除旧 Aris hook，再追加新的，不会重复。

### 4.4 错误处理

所有错误路径输出到 stderr 并 `exit 1`：
- 缺少依赖（curl/jq）
- 不支持的平台
- 下载失败（HTTP 非 200）
- 健康检查失败且用户拒绝重试
- API Key 验证失败且用户拒绝重试
- jq 合并失败
- 文件写入失败

不覆盖已有二进制（下载失败时保留旧版）。

## 5. 后端变更

### 5.1 新增路由

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/install.sh` | 无 | 返回嵌入 host 的 install.sh 脚本 |

Handler `HandleInstallScript`：
- 从 `X-Forwarded-Proto` + `Host` 头推导 origin。
- 验证 scheme（http/https）和 host 非空。
- 用 `text/template` 执行嵌入的模板，替换 `{{.Host}}`。
- 返回 `text/plain`、`no-store`。
- 错误时返回 `#!/bin/sh\necho 'Failed to generate install script.' >&2\nexit 1\n`。

### 5.2 保留路由

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/trace/client/check` | API Key 验证（Bearer auth），install.sh 调用 |

### 5.3 删除路由

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/trace/client` | 二进制下载端点（改为 GitHub Releases） |
| GET | `/api/v1/trace/client/install` | 旧票据 install.sh |
| POST | `/api/v1/trace/client/ticket` | 签发票据 |

### 5.4 删除组件

- `internal/application/trace/command/issue_client_ticket.go` — 票据签发 command
- `TraceClientTicketMiddleware` — 票据验证中间件
- 票据 store（Redis）及相关 port/command
- `HandleInstallTraceClient`、`buildInstallScript`、`writeInstallScriptError` — 旧 install handler
- `HandleIssueTraceClientTicket` — 票据签发 handler
- `HandleDownloadTraceClient` — 二进制下载 handler（改为 GitHub Releases）
- `ArtifactResolver` 及相关 artifact 解析逻辑 — 二进制文件查找
- `DownloadTraceClientReq` DTO — 下载请求
- `IssueTraceClientTicketReq`/`Rsp`、`InstallTraceClientReq` — DTO
- 票据相关常量（`TraceClientTicket*`、`PeriodIssueTraceClientTicket`、`LimitIssueTraceClientTicket`）
- `TraceClientArtifactDir` 配置项及 `TRACE_CLIENT_ARTIFACT_DIR` 环境变量
- `TraceClientArtifactDarwinAMD64` 等产物文件名常量
- `TraceClientInstallErrorMessage` — 旧错误消息
- `TraceClientInit*` 常量 — init 向导消息（移到 install.sh 内联）
- Dockerfile 交叉编译客户端步骤及 `COPY /go/trace-client /app/trace-client`

### 5.5 CI/CD 变更

新增 GitHub Actions workflow（或在现有 `docker-publish.yml` 中添加 job），在 tag push（`v*.*.*`）时：

1. 交叉编译 4 平台客户端二进制（`make build-client-all`）。
2. 创建 GitHub Release（如果不存在）。
3. 上传 4 个产物到 Release assets：
   - `aris-darwin-amd64`
   - `aris-darwin-arm64`
   - `aris-linux-amd64`
   - `aris-linux-arm64`
4. install.sh 使用 `releases/latest/download/` URL 自动指向最新版本。

Dockerfile 移除客户端交叉编译步骤和 `/app/trace-client/` COPY，减小镜像体积。

### 5.6 模板文件

新增 `internal/handler/install_trace_client.sh.tmpl`，通过 `//go:embed` 嵌入。模板内容为第 4 节描述的 POSIX sh 脚本，仅 `{{.Host}}` 占位。

## 6. 客户端二进制变更（`cmd/client`）

### 6.1 删除

| 文件 | 说明 |
|------|------|
| `cmd/client/trace.go` 中 `newTraceInitCommand` | trace init 命令 |
| `internal/tracecli/init.go` | InitRunner 及四步向导 |
| `internal/tracecli/codex.go` | CodexHookInstaller |
| `internal/tracecli/terminal.go` | Terminal（ReadSecret/ReadLine/Interactive） |

### 6.2 修改

| 文件 | 变更 |
|------|------|
| `cmd/client/trace.go` | 只注册 `trace ingest`，移除 `trace init` |
| `internal/tracecli/config.go` | 移除 `ConfigStore.Save`，保留 `Load`（ingest 读配置）；`Config` 结构不变 |
| `internal/tracecli/http_client.go` | 移除 `CheckHealth`、`CheckAPIKey`，保留 POST 上报方法 |

### 6.3 保留（不变）

- `internal/tracecli/ingest.go` — ingest 链路
- `internal/tracecli/spool.go` — 本地 spool
- `internal/tracecli/state.go` — 客户端状态
- `internal/tracecli/rollout.go` — rollout 增量读取
- `internal/tracecli/paths.go` — 路径布局
- `internal/tracecli/lock_unix.go`、`fileid_unix.go` — 文件锁/inode

## 7. Web UI 变更

### 7.1 `trace-install-dialog.tsx`

- 移除 `api.issueTraceClientTicket()` 调用和 `generateInstallCommand` 函数。
- 命令改为 `curl -fsSL <host>/install.sh | sh`（`<host>` = `window.location.origin`）。
- Copy 按钮直接复制命令（同步，无异步票据签发）。
- 移除 `TICKET_PLACEHOLDER`、`shellQuote`、`copying` 状态。
- 步骤从 3 步简化为 2 步：
  1. 下载并配置（脚本自动完成平台检测 → 下载 → API Key → Hook）
  2. 在 Codex `/hooks` 中手动批准

### 7.2 `api-client.ts`

- 移除 `issueTraceClientTicket` 方法。

### 7.3 `types.ts`

- 移除 `IssueTraceClientTicketRsp`。

### 7.4 i18n

| Key | 操作 |
|-----|------|
| `trace.install_terminal_hint` | 更新：移除票据提及 |
| `trace.install_step_download` | 更新：描述改为"下载并自动配置" |
| `trace.install_step_key` | 移除（合并到步骤 1） |
| `trace.install_step_approve` | 保留 |
| `trace.install_ticket_note` | 移除 |
| `trace.install_footer` | 更新：移除票据提及 |
| `trace.install_copying` | 移除（copy 不再异步） |

## 8. 安全考虑

1. **二进制公开下载**：aris 二进制通过 GitHub Releases 分发，不经过服务端 API，不会被刷流量。二进制本身不含密钥或敏感信息，API Key（交互式输入）才是鉴证手段。
2. **API Key 不入脚本**：API Key 通过 `read -s` 隐藏输入收集，不出现在命令行参数、脚本内容或 shell 历史中。
3. **文件权限**：`~/.aris/`（0700）、`~/.aris/bin/`（0700）、`~/.aris/trace/config.json`（0600）、`~/.codex/hooks.json`（0600）、备份文件（0600）。
4. **Host 验证**：服务端验证 origin scheme 和 host，防止注入恶意 URL。
5. **jq 依赖**：preflight 检查 jq 可用性，缺失时报错退出，不静默降级。

## 9. 测试策略

### 9.1 后端单元测试

- `HandleInstallScript`：验证返回的脚本包含正确 host、content-type、cache-control。
- Host 推导：验证 `X-Forwarded-Proto` + `Host` 组合，非法 scheme 返回错误脚本。
- 模板执行失败时返回错误脚本。

### 9.2 install.sh 集成测试

- 在 Docker 容器（linux/amd64）中执行 `curl | sh`，验证：
  - 二进制下载到 `~/.aris/bin/aris` 且权限 0700。
  - `~/.aris/trace/config.json` 正确写入且权限 0600。
  - `~/.codex/hooks.json` 包含 10 个事件且每个有 Aris hook。
  - 重复执行幂等（不重复追加 hook）。
  - 保留已有非 Aris hook。
- jq 缺失时报错退出。
- 不支持平台报错退出。

### 9.3 E2E 测试

- 更新 `test/e2e/trace/client_download_test.go`：
  - 移除票据签发、票据消费和二进制下载测试（端点已删除）。
  - 新增 `GET /install.sh` 测试：验证返回脚本包含 host、content-type 正确。

### 9.4 客户端测试

- 移除 `trace init` 相关测试。
- `trace ingest` 测试不变。

## 10. 迁移与兼容

- 旧版客户端（已有 `aris trace init`）不受影响——已安装的二进制仍可 `trace ingest`。
- 已安装的 `config.json` 和 `hooks.json` 格式不变，无需迁移。
- 旧版 `GET /api/v1/trace/client`、`GET /api/v1/trace/client/install` 和 `POST /api/v1/trace/client/ticket` 删除后，旧版 Web UI 的 copy 按钮会失败——需前后端同步发布。
- 首次发布前需先创建一个 GitHub Release 并上传 4 平台二进制，否则 `releases/latest/download/` URL 不可用。
