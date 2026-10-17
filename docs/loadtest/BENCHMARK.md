# 接口性能基准

> 本文档持续更新：所有接口的性能测试结果统一记录在此。
> 测试原始输出（wrk stdout）不入库，关键数据已摘录到下方表格。

## 1. 测试环境

### 1.1 被测服务

| 项 | 值 |
|----|-----|
| 环境 | 生产 `api.lvlvko.top`（真实链路：鉴权 → 限流 → 触发词 → 模型解析 → 审计 → 落库） |
| 部署 | k3s 单节点，2 replicas |
| CPU | requests 50m / limits **300m**（基准）；容量测试期间临时调至 limits 2000m |
| 内存 | requests 128Mi / limits 512Mi |
| 服务版本 | git `4985e817` |

### 1.2 宿主机

| 项 | 值 |
|----|-----|
| CPU | **2 核** |
| 内存 | 3913 MB（无 swap） |
| 系统 | Ubuntu 24.04 / kernel 6.8.0-31 / k3s v1.35.5 |
| 同机容器 | postgresql、redis、openresty(1Panel)、xray、pgadmin4、portainer、1Panel-agent 等 |
| 网络参数 | `somaxconn=4096`、`tcp_max_syn_backlog=256` |

### 1.3 Mock 上游（被测模型的行为）

| 参数 | 取值 |
|------|------|
| 输出长度 | `min(max_tokens, U(1, 16384))` 字符 |
| TTFT（首 token 延迟） | `U(0, 1s)` |
| 输出速率（TPS） | `U(20, 200) tok/s` |
| 流时长 | `TTFT + (N-1)/TPS`，期望约 19s，最长约 205s |
| 字符集 | 纯可打印 ASCII |

三个 mock 模型（`upstream_model=mock-llm`）：`mock-chat` / `mock-response` / `mock-messages`。

## 2. 测试方法

| 项 | 值 |
|----|-----|
| 压测工具 | wrk 4.x（Alpine arm64 容器内原生执行） |
| 命令 | `wrk -t4 -c<N> -d120s --timeout 300s --latency -s test/loadtest/<协议>.lua <url>` |
| 鉴权 | `Authorization: <API Key>`（非 `X-API-Key`） |
| 限流 | 100 次/分钟 per API Key；容量测试用 20 Key 轮换（`LT_KEYS`）绕过 |
| 请求标记 | 消息带 `__LOADTEST__` 前缀，保证 checksum 不与真实数据冲突 |

**指标口径**

| 指标 | 来源 | 说明 |
|------|------|------|
| 完成请求数 / req/s | wrk | 长流场景下完成速率低属正常（单请求平均占用约 19s） |
| 延迟分位 | wrk `--latency` | 含整个 SSE 流时长 |
| TTFT / 流持续 | 审计表 `model_call_audits` | `first_token_latency_ms` / `stream_duration_ms` |
| 状态码 | 审计表 | `upstream_status_code` |
| 资源占用 | `kubectl top pods` / 宿主 `top`/`free` | — |

> `--timeout 300s` 必须显式指定：wrk 默认 2s，会把平均 19s 的长流全判为 timeout。

## 3. 接口测试结果

### 3.1 OpenAI Chat Completions

`POST /api/openai/v1/chat/completions`，模型 `mock-chat`。

**基准（单 Key，限流内，2026-10-16）**

| 并发 | 完成请求 | req/s | p50 | p75 | p90 | p99 | 传输量 |
|------|---------|-------|-----|-----|-----|-----|--------|
| 10 | 32 | 0.27 | 20.87s | 33.15s | 40.65s | 1.31m | 14.10MB |
| 20 | 74 | 0.62 | 18.34s | 30.30s | 47.87s | 1.38m | 28.64MB |
| 30 | 114 | 0.95 | 16.98s | 28.57s | 47.18s | 1.69m | 39.01MB |

**容量（多 Key，2026-10-17）**

| 并发 | 完成请求 | req/s | p50 | connect 错误 | pod CPU | pod 内存 | 宿主 load |
|------|---------|-------|-----|-------------|---------|---------|----------|
| 50 | 193 | 1.61 | — | 0 | 407m | 175Mi | — |
| 100 | 281 | 2.34 | — | 2 | 554m | 186Mi | — |
| 200 | 296 | **2.47** | — | 5 | 786m | 297Mi | — |
| 400 | 251 | 2.09 | — | 120 | 824m | 453Mi | — |
| 800 | 210 | 1.75 | — | 371 | — | — | — |

**容量复测（多 Key，停 hermes 释放 704MB 内存后，2026-10-17）**

| 并发 | 完成请求 | req/s | p50 | connect 错误 | 宿主 load |
|------|---------|-------|-----|-------------|----------|
| 100 | 290 | 2.41 | 23.87s | 0 | 8.25 |
| 200 | 212 | 1.77 | 42.56s | 2 | 10.09 |
| 400 | 258 | 2.15 | 40.51s | 60 | 11.34 |
| 800 | 658⚠ | 5.48 | 4.44s | 902 | 23.01 |

> ⚠️ 800 档 p50 仅 4.44s（远低于流期望 19s），该批"完成请求"以快速失败为主，不代表真实吞吐。

**服务端指标（基准三档合计，审计表成功记录）**

| 成功记录 | avg TTFT | p50 TTFT | p90 TTFT | avg 流持续 | avg 输出 token |
|---------|----------|----------|----------|-----------|---------------|
| 223 | 506ms | 527ms | 897ms | 20.7s | 1862 |

### 3.2 OpenAI Responses

`POST /api/openai/v1/responses`，模型 `mock-response`。

**基准（单 Key，限流内，2026-10-16）**

| 并发 | 完成请求 | req/s | p50 | p75 | p90 | p99 | 传输量 |
|------|---------|-------|-----|-----|-----|-----|--------|
| 10 | 34 | 0.28 | 18.34s | 28.60s | 42.46s | 1.14m | 7.87MB |
| 20 | 92 | 0.77 | 19.06s | 32.07s | 43.03s | 1.26m | 21.99MB |
| 30 | 119 | 0.99 | 18.58s | 34.15s | 43.27s | 1.19m | 29.92MB |

**服务端指标（审计表成功记录）**

| 成功记录 | avg TTFT | p50 TTFT | p90 TTFT | avg 流持续 | avg 输出 token |
|---------|----------|----------|----------|-----------|---------------|
| 246 | 549ms | 568ms | 939ms | 20.7s | 1976 |

### 3.3 Anthropic Messages

`POST /api/anthropic/v1/messages`，模型 `mock-messages`。

**基准（单 Key，限流内，2026-10-16）**

| 并发 | 完成请求 | req/s | p50 | p75 | p90 | p99 | 传输量 |
|------|---------|-------|-----|-----|-----|-----|--------|
| 10 | 37 | 0.31 | 15.23s | 27.48s | 42.65s | 50.93s | 10.74MB |
| 20 | 85 | 0.71 | 19.07s | 28.65s | 42.00s | 1.02m | 24.10MB |
| 30 | 109 | 0.91 | 19.78s | 32.20s | 0.94m | 1.32m | 31.59MB |

**服务端指标（审计表成功记录）**

| 成功记录 | avg TTFT | p50 TTFT | p90 TTFT | avg 流持续 | avg 输出 token |
|---------|----------|----------|----------|-----------|---------------|
| 232 | 524ms | 536ms | 942ms | 21.4s | 1921 |

### 3.4 OpenAI Decision

`POST /api/openai/v1/decisions` — 非流式接口，**未测**。

### 3.5 Anthropic Count Tokens

`POST /api/anthropic/v1/messages/count_tokens` — **未测**。

### 3.6 其他接口（预留）

（待补充：管理后台接口、session 查询接口等）

## 4. 附录

### 4.1 容量测试的 CPU 分布

400 并发压测进行中采样（宿主 `id=0%`，完全饱和）：

```
%Cpu(s): 37.0 us, 33.3 sy, 0.0 id, 29.6 si
```

| 容器 | CPU% |
|------|------|
| 1Panel-openresty | 38.8% |
| postgresql | 35.9% |
| xray | 26.0% |
| aris-proxy-api pod×2 | ~40%（合计） |
| redis / 其他 | <1% |

### 4.2 关键约束

- 限流：100 次/分钟 per API Key → 单 Key 并发上限约 30（流均 19s）
- 宿主 2 核被 openresty / postgresql / xray / 软中断瓜分，服务 pod 实际可用约 0.4~0.8 核
- 服务 pod CPU/内存均未饱和（400 并发时 41% / 88%），限制来自宿主

### 4.3 数据附注

原始 wrk 输出体积较大，不入库；各档的完成请求数、req/s、延迟分位、Socket errors、
pod CPU/内存与宿主 load 均已摘录到 §3 与 §4.1 的表格中。

复现方式见 §2，脚本在 `test/loadtest/`。

## 5. 压测前后必做

### 5.1 压测前

1. **新建专用 API Key**，命名 `loadtest-<date>`（`name` 必须以 `loadtest-` 开头，清理脚本按此前缀做守卫）。**严禁使用主 Key（id=1）或任何真实用户 Key 压测**——清理脚本会按 `api_key_id` 软删 + 硬删该 Key 的全部会话与消息，用错 Key 就是不可逆的误删。
2. 开启 mock 上游开关：`MOCK_UPSTREAM_ENABLED=true`（`api.env` + ConfigMap + 滚动重启三件套，见 `update-prod-config`）。
3. 确认压测脚本参数：`LT_KEYS` 多 Key 轮换、`--timeout 300s` 必加（§2）。

### 5.2 压测后（缺一不可）

1. **关闭 mock 开关**：`MOCK_UPSTREAM_ENABLED=false`（同样三件套）。这是本设计唯一的生产配置改动，长期开启会让误配到 `mock-llm` 的真实模型别名返回假响应且照常计费、照常落库。
2. **软删压测 Key**（写操作，需授权）。
3. **清理压测数据**：

   ```sh
   # 先预览：-v dry_run=1 会打印完整删除量（与真实删除同源），不写任何数据
   psql "$PROD_DATABASE_URL" \
     -v api_key_id=<压测 Key ID> -v confirm=delete-loadtest-data -v dry_run=1 \
     -f scripts/loadtest-cleanup.sql
   # 确认无误后执行（去掉 dry_run）
   ```

   脚本内置守卫：Key 必须存在且未软删、`name` 以 `loadtest-` 开头、必须带 `confirm`，任一不满足即非 0 退出。
4. **复核残留为 0**（脚本 Step 7 输出的 `audits_left` / `sessions_left`）。

> 压测会话以普通 assistant 文本结尾，既不会被 `SoftDeletePurge`（只处理已软删 session）也不会被 `SessionTerminalCleanup`（只处理末条为 assistant + tool_calls 的会话）自动回收——清理必须手工执行，没有 cron 兜底。`__LOADTEST__` 前缀（保证 checksum 不与真实消息冲突）只保护「不误删真实数据」，不提供自动回收。
