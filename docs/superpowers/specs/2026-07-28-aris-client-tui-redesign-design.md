# Aris 客户端 TUI 重构设计文档

> 日期：2026-07-28
> 分支：feature/aris-client-tui-2026-07-28
> 状态：设计已确认，待实现

## 1. 背景与目标

`aris` 客户端（`cmd/client`）当前只有 `trace ingest` 一个非交互子命令（Codex hook 回调，fail-open）。配置向导（健康检查 → 选 agent → API Key → 配 hooks）由服务端下发的自包含 `install.sh`（`internal/handler/install_trace_client.sh.tmpl`）以 bash 交互实现，体验简陋：`stty -echo` 隐藏输入、无步骤指示、无状态反馈、错误恢复粗糙。

**目标**：基于 charmbracelet 生态（bubbletea / huh / bubbles / lipgloss）重构客户端交互：

1. 新增 `aris init`：四步配置向导搬回 Go 二进制，huh 表单 + 异步任务反馈
2. 新增 `aris status`：状态面板，展示本地状态 + 连通性（复用现有 health / check 端点，服务端零改动）
3. `install.sh` 退化为纯下载器，末尾 `exec aris init --host <origin>`
4. `trace ingest` 行为完全不变

**决策记录**（brainstorming 澄清结论）：

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | 命令范围 | `init` 向导 + `status` 状态面板；不做全屏 trace 浏览 Dashboard |
| 2 | install.sh 归宿 | 纯下载器；四步交互全部搬入 `aris init` |
| 3 | status 信息源 | 本地状态 + 连通性（health + 现有 check 端点）；**不新增服务端端点**——投递失败经 spool rejected/本地日志已可观测，聚合计数 Web UI 已有，YAGNI |
| 4 | TUI 方案 | 方案 A：huh 表单向导 + bubbletea inline 渲染，非全屏、非 alt-screen |

## 2. 架构

### 2.1 命令树与包结构

```
aris
├── init            # 新增：交互式配置向导（顶层命令）
├── status          # 新增：状态面板（--json 输出机器可读）
└── trace
    └── ingest      # 不变：非交互 hook 回调，fail-open
```

```
internal/client/
├── trace/          # 现有，不改动
├── ui/             # 新增：lipgloss 主题 token + 共享组件
│   ├── theme.go        # AdaptiveColor 语义色、字号节奏、图标常量
│   └── components.go   # CheckRow / SectionTitle / KeyValue / SummaryPanel
├── api/            # 新增：控制面 HTTP 客户端（health / client check）
│   └── client.go       # CheckHealth / CheckAPIKey，setup 与 status 共用
├── setup/          # 新增：init 向导（避免包名 init 与 Go 保留字混淆）
│   ├── wizard.go       # huh 多步表单编排 + /dev/tty 处理
│   └── hooks.go        # codex hooks.json 幂等写入 + 安装状态检测（从 install.sh jq 逻辑移植）
└── status/         # 新增：status 面板
    ├── status.go       # huh spinner 编排（并发检查 → 一次性渲染 → 退出）
    ├── checks.go       # 本地扫描 + 网络检查任务
    └── render.go       # 五节渲染 + --json 输出
```

边界约束沿用 2026-07-17 spec：`internal/client/**` 不得导入 server / db / router / web embed；`cmd/client` 只注册 `init`、`status`、`trace ingest`。构建方式不变（`make build-client` / `build-client-all`，`CGO_ENABLED=0` 交叉编译 darwin/linux × amd64/arm64；bubbletea 系纯 Go，兼容）。

新增依赖（仅客户端链接）：`charmbracelet/huh`、`charmbracelet/bubbletea`、`charmbracelet/bubbles`、`charmbracelet/lipgloss`。

## 3. `aris init` 向导设计

### 3.1 流程

| 步骤 | 交互（huh） | 异步动作（bubbles/spinner） |
|------|------------|---------------------------|
| `[1/4]` 连接服务器 | spinner → ✓/✗ 结果行 | `GET {host}/health`（5s 超时）；失败 → Confirm「重试/放弃」 |
| `[2/4]` 选择 Agent | Select：Codex（唯一可用；预留选项 disabled 并标注"即将支持"） | — |
| `[3/4]` 配置 API Key | Input（`EchoModePassword`；已有 key 时提示"Enter 保留当前"） | `GET /api/v1/trace/client/check`（Bearer）；失败就近显示错误，可重试 |
| `[4/4]` 配置 Hooks | spinner 逐步反馈 | 备份并幂等写入 `~/.codex/hooks.json`（10 个事件）→ 原子写 `config.json`（0600） |

完成后渲染成功摘要面板：配置文件路径、hooks 注册数、下一步提示（在 Codex 中运行 `/hooks` 手动批准 Aris hooks）。

### 3.2 关键行为

- **TTY 处理**：`curl | sh` 场景 stdin 是管道；当 stdin 非 TTY 时自动打开 `/dev/tty` 作为向导的输入输出（替代旧脚本 `exec 3<>/dev/tty`）。无可用 TTY 时报错退出（沿用常量 `TraceClientInitNonInteractiveMessage`）。
- **`--host` flag**：install.sh 末尾 `exec "$aris_bin" init --host "$host"` 传入；手动运行未传时在向导内提示输入。
- **`ARIS_API_KEY` 环境变量**：非交互/脚本场景的 key 来源（secrets env-only 原则）；key 不出现在 flag、shell history、日志中。
- **hooks 幂等写入**：移植 install.sh 的 jq 逻辑为 Go——对 10 个事件（`SessionStart`、`UserPromptSubmit`、`PreToolUse`、`PermissionRequest`、`PostToolUse`、`Stop`、`SubagentStart`、`SubagentStop`、`PreCompact`、`PostCompact`），先按命令串去重再追加 hook group（`{"matcher":"","hooks":[{"type":"command","command":"<bin> trace ingest","timeout":30}]}`）；写入前备份 `.bak`（0600），原子替换。
- **config.json**：`{host, agent, apiKey}`，沿用现有 `writePrivateFile`（0600 原子写）。
- 已初始化场景：重复运行 `aris init` 幂等，已有 key 可回车保留。

## 4. `aris status` 面板设计

### 4.1 交互形态

bubbletea **inline 程序**（非 alt-screen）：启动后并发执行全部检查（本地扫描无依赖、网络检查并行），spinner 显示进行期，全部完成后一次性静态渲染并退出，输出保留在终端 scrollback（参照 `gh auth status`）。`--json` flag 输出机器可读 JSON 供脚本消费。

### 4.2 信息架构（五节）

```
aris status
─────────────────────────────────────────────
◆ Server       ✓ https://aris.example.com · reachable (42ms)
◆ Auth         ✓ API key valid · ••••ab12
◆ Agent        codex · hooks 10/10 registered
◆ Local queue  3 pending (12.4 KB) · 0 rejected
◆ Diagnostics  no recent errors
```

各节数据来源与降级：

| 节 | 来源 | 失败/缺失降级 |
|----|------|--------------|
| Server | config.host + `GET /health`（含延迟） | 无 config → 引导运行 `aris init`；不可达 → ✗ + 错误 |
| Auth | `GET /api/v1/trace/client/check`（现有端点） | key 无效/缺失 → ✗ + 引导重跑 `aris init`；key 脱敏显示 |
| Agent | config.agent + 扫描 `~/.codex/hooks.json` 中 aris hook 数 | 缺失事件列出具体事件名 |
| Local queue | 扫描 spool pending 目录（条数 + 字节数）、rejected 目录条数 | 目录不存在视为 0 |
| Diagnostics | 读取最近日志文件（`trace-YYYY-MM-DD.log`）尾部条目计数 | 无日志 → "no recent errors" |

**不新增服务端端点**：投递失败（网络错误 → 本地日志；服务端拒绝 → rejected 目录）在本地已可观测，trace 聚合计数 Web UI 已提供，终端重复展示收益过低。

## 5. install.sh 瘦身

`internal/handler/install_trace_client.sh.tmpl` 改为：

1. preflight：`curl` + `sha256sum`/`shasum`（**移除 jq 依赖**）
2. 平台探测（darwin/linux × amd64/arm64，现有逻辑保留）
3. 下载二进制 + sha256 校验 + 原子安装到 `~/.aris/bin/aris`（现有逻辑保留）
4. `exec "$aris_bin" init --host "$host"`

删除：四步 bash 交互、`/dev/tty` 重定向、config.json 写入、hooks.json 配置。Web 端安装对话框文案保持不变（"下载二进制后引导完成 API Key 与 Hook 配置"的描述仍成立）。

## 6. 视觉设计系统（ui/ 包）

遵循 ui-ux-pro-max 原则（语义色 token、状态图标 + 文字双通道、loading 反馈 >300ms 显示 spinner、错误就近 + 恢复路径、多步进度指示）：

- **色板**：lipgloss `AdaptiveColor`（适配明暗终端背景）；Primary 取 Anthropic clay 橙（呼应 Web 端主题），Success/Warning/Error/Muted 语义色
- **图标**：`✓ ✗ ! ◆` unicode 符号（非 emoji，不假设 Nerd Font）
- **组件**：`CheckRow`（图标 + 标签 + muted 详情）、`SectionTitle`（◆ + accent 色）、`KeyValue`（冒号对齐）、`SummaryPanel`（rounded border）
- **节奏**：节间 1 空行，节内 4 格缩进；spinner 用 bubbles/spinner
- **降级**：`NO_COLOR` 环境变量或非 TTY 输出时 lipgloss 自动降级无色

## 7. 测试计划

- **单元**（`test/unit/client/`）：
  - `setup/`：hooks.json 幂等写入（新增/去重/备份/权限）、config 原子写、key 校验状态机（mock HTTP）；向导步骤逻辑与 huh 表单解耦为纯函数
  - `status/`：本地扫描（构造 spool/rejected/logs fixture）、渲染 golden 测试（strip ANSI 比对）、`--json` 输出 schema
  - `ui/`：组件渲染快照
  - 更新 `command_tree_test.go`（新增 init/status 注册断言）
- **E2E**（`test/e2e/trace/`）：现有 `hook_test.go`、`install_script_test.go` 适配（后者断言脚本瘦身后行为）；服务端零改动，无新增 e2e
- **lint**：`make lint`（lint-conv + lint-static）；`go build ./cmd/client` + 四平台交叉编译验证
- **手工验证**：`go run ./cmd/client init --host <本地服务>` 走通四步；`curl | sh` 管道场景验证 `/dev/tty` 打开

## 8. 风险与边界

- **`trace ingest` 零改动**：hook 回调路径（fail-open、spool、批量上报）不受任何影响；新增依赖不会链入该路径的热代码
- **兼容性**：已通过旧 install.sh 安装的用户，其二进制无 `init`/`status`，重新运行新 install.sh 即可升级；config.json / hooks.json 格式不变
- **依赖体积**：huh/bubbletea 约增加二进制 2-3MB，可接受（当前客户端为独立分发）
- **非 TTY 环境**：`aris init` 报错退出并提示；`aris status` 在非 TTY 下降级为纯文本输出（无色、无 spinner）
