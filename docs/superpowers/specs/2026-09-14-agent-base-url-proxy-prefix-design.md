# 各 Agent 配置 base URL 缺协议分区前缀修复设计

> 日期：2026-09-14
> 分支：`bugfix/agent-base-url-proxy-prefix-2026-09-14`
> 状态：已获用户批准（一并收敛 proxy 前缀常量；不新增 cobra 命令）

## 1. 问题

`aris model export` 把 `aris init` 保存的 `host`（站点裸根，如 `https://api.lvlvko.top`）**原样**写进各 harness 的 API base URL 字段，4 个目标全部指向不存在的路径：

| Target | 写入字段 | 现状值 | 缺前缀时的实际请求 | 生产实测 |
|---|---|---|---|---|
| Codex | `[model_providers.aris-proxy].base_url` | `{host}` | `{host}/responses` | 404 |
| Claude Code | `env.ANTHROPIC_BASE_URL` | `{host}` | `{host}/v1/messages` | 404 |
| OpenCode | `provider.aris-proxy.options.baseURL` | `{host}` | `{host}/chat/completions` | 404 |
| Pi | `providers.aris-proxy.baseUrl` | `{host}` | `{host}/chat/completions` | 404 |

网关真实路由是 `/api/openai/v1/*` 与 `/api/anthropic/v1/*`（`POST /api/openai/v1/responses` 实测 401、`POST /responses` 实测 404），根路径没有兼容重写。

`host` 的语义明确是站点根：`setup.NormalizeHost` 只校验 scheme、去尾斜杠；控制面路径由 `{host}/health`、`{host}/api/cli/v1/...` 派生。因此协议分区前缀必须由各 harness 的 writer 自己拼。

## 2. 实测：客户端在 base URL 之后追加什么

用真实客户端 + 本地 mock HTTP server（记录请求路径）逐个确认，**不是按 SDK 文档推断**：

| 客户端（版本） | base URL 设置 | 实际请求路径 | 与网关路由一致 |
|---|---|---|---|
| Codex 0.154.0-alpha.6.2 | `{host}/api/openai/v1` | `POST /api/openai/v1/responses` | ✅ |
| Codex | `{host}` | `POST /responses` | ❌ 404 |
| Claude Code 2.1.207 | `{host}/api/anthropic` | `POST /api/anthropic/v1/messages?beta=true` | ✅ |
| Claude Code | `{host}/api/anthropic/v1` | `POST /api/anthropic/v1/v1/messages` | ❌ 404 |
| Claude Code | `{host}` | `POST /v1/messages` | ❌ 404 |
| OpenCode 1.14.30 | `{host}/api/openai/v1` | `POST /api/openai/v1/chat/completions` | ✅ |
| OpenCode | `{host}` | `POST /chat/completions` | ❌ 404 |
| Pi 0.85.1 | `{host}/api/openai/v1` | `POST /api/openai/v1/chat/completions` | ✅ |
| Pi | `{host}` | `POST /chat/completions` | ❌ 404 |

**关键结论（与直觉相反）**：OpenAI 系（Codex / OpenCode / Pi）的 base URL 与网关路由前缀相同（含 `/v1`），而 Claude Code 的 base URL **必须比网关路由前缀少一段 `/v1`**——Anthropic SDK 惯例是在 base 之后追加完整的 `/v1/messages`。用户此前手写的 `ANTHROPIC_BASE_URL=https://api.lvlvko.top/api/anthropic/v1` 同样会 404（`/api/anthropic/v1/v1/messages`）。

## 3. 设计

### 3.1 前缀常量收敛（`internal/common/constant/string.go`）

```go
// 网关路由前缀（服务端注册）
OpenAIProxyPrefix    = "/api/openai/v1"
AnthropicProxyPrefix = "/api/anthropic/v1"

// 写给 Claude Code 的 ANTHROPIC_BASE_URL 前缀：必须比 AnthropicProxyPrefix 少一段 /v1
ClaudeCodeBaseURLPrefix = "/api/anthropic"
```

`internal/router/router.go` 的 proxy 注册改用前两个常量。该文件顶部注释已声明「所有分区前缀与客户端可见路径常量在此收敛，禁止在任何一侧重新硬编码（#165/#166 两次 404 的根因）」，而这两个 proxy 前缀是当时唯一遗留的硬编码——正是同类 404 的再次复发。

### 3.2 各 target 在自己的 `Write` 内拼前缀

| Target | 拼接 |
|---|---|
| Codex | `host + constant.OpenAIProxyPrefix` |
| Claude Code | `host + constant.ClaudeCodeBaseURLPrefix` |
| OpenCode | `host + constant.OpenAIProxyPrefix` |
| Pi | `host + constant.OpenAIProxyPrefix` |

拼接放在 writer 内部而非 `export.go` 或新增 `Target` 接口方法：走哪个协议分区、追加什么路径是各 harness 侧知识，放 `Write` 内更内聚；代价仅为 4 处字符串相加，不值得为此引入接口维度。

### 3.3 OpenCode / Pi 的 provider 字段改为每次覆盖

原实现只在 provider 不存在时写入 `baseURL` / `baseUrl` / `apiKey`，已存在的 provider 永远保留旧值——**存量用户重跑 `aris model export` 也修不回来**，修复等于无效。

改为：`baseURL`（OpenCode）/ `baseUrl`（Pi）与 `apiKey` 由本工具管理，**每次导出覆盖**；`models` 保持 merge 语义（不删用户手工添加的模型），provider 的 `name` / `npm` 仅在新建时写入，不动用户改动。

Codex 与 Claude Code 的 writer 本就无条件覆盖 `base_url` / `ANTHROPIC_BASE_URL`，无需改动。

## 4. 回归测试（`test/unit/client/model/`）

现有断言只检查输出「包含 `aris.example.com`」，裸 `host` 也能通过，因此测不出本 bug。新增/加强：

- 4 个 target 均断言写出的完整 base URL（Codex `base_url`、Claude Code `ANTHROPIC_BASE_URL`、OpenCode `options.baseURL`、Pi `baseUrl`）。
- `TestOpenCodeWrite_OverwritesStaleBaseURL`（新增）与 `TestPiWrite_MergesExistingModels`（加强）：provider 已存在且 base URL 为旧值时，导出必须覆盖为带前缀的新值，同时保留既有模型与用户字段。

改前验证：4 个 base URL 断言在旧实现下全部失败并各自指出缺失值。

## 5. 不做

- **不加 host 兼容迁移**：带路径的 host 在 `aris init` 的 health check 阶段就会失败，不存在需要兼容的历史合法值。
- **不动 status 的 provider 检查**：该节只校验配置文件存在性，与本 bug 无关。
- **不新增 cobra 命令**。

## 6. 存量配置

`aris` 是独立二进制，修复需打新 tag 才能分发；用户升级后重跑 `aris model export` 才会重写各 harness 配置。Claude Code 因本次实测新发现，其存量配置（含手工写入的 `/api/anthropic/v1`）也需按上面的正确值修正。
