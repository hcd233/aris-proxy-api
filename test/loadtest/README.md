# 三接口压测脚本（wrk）

对 chat completion / response / message 三个接口做 SSE 长连接容量压测。
被测模型须为 `mock-llm`（见 `docs/superpowers/specs/2026-10-14-loadtest-mock-model-design.md`）。

## 前置

1. 生产已建 mock Endpoint + 3 个 Model 别名（`mock-chat` / `mock-response` / `mock-messages`，`upstream_model=mock-llm`）
2. 服务已开 `MOCK_UPSTREAM_ENABLED=true`（默认关闭）
3. 已创建**压测专用 API Key**（记录 `api_key_id`，清理时用）

## 执行

```bash
export TARGET_URL=https://api.lvlvko.top
export API_KEY=<压测专用 Key>
export MODEL=mock-chat          # 或 mock-response / mock-messages
# export MAX_TOKENS=2048        # 可选：约束输出长度（默认 0=不限制，长度 1~16384 字符）
```

```bash
wrk -t4 -c100 -d5m --timeout 300s --latency -s test/loadtest/chat_completions.lua "$TARGET_URL/api/openai/v1/chat/completions"
wrk -t4 -c100 -d5m --timeout 300s --latency -s test/loadtest/responses.lua        "$TARGET_URL/api/openai/v1/responses"
wrk -t4 -c100 -d5m --timeout 300s --latency -s test/loadtest/messages.lua         "$TARGET_URL/api/anthropic/v1/messages"
```

> **`--timeout 300s` 必加**：wrk 默认超时 2 秒，而 mock 流平均 19 秒、最长 205 秒。
> 不加会把绝大多数长流判成 `Socket errors: timeout`，测不出真实数据（实测 20 并发 120s
> 窗口下漏 79 个连接）。

## 指标口径

- **核心指标 = 并发 SSE 长连接数**（即 wrk `-c`），不是「每秒完成请求数」。
  SSE 长连接场景下一个请求可持续 205 秒，完成速率不反映承载能力。
- **加压方式**：逐档递增并发找容量拐点——50 → 100 → 200 → 400 → …，每档
  warmup 60s + 正式 5 分钟 × 3 轮取中位数，直到出现拐点（错误率上升 / 延迟陡增 / 连接被拒）。
  **能稳定维持的最大并发流数即核心结论。**
- **TTFT 与流持续时间不在 wrk 侧测**（Lua 拿不到精确首字节时间戳），压测后查审计表：
  `model_call_audits.first_token_latency_ms` / `stream_duration_ms`，按 `api_key_id` 过滤。
- **状态码分布也从审计表取**（`upstream_status_code`）：脚本里的 `response()` 回调在部分
  wrk 构建（如 Alpine 包）中不会被调用，实测 `response called: 0`，故不依赖它。
- 窗口须长于最长流；连接超时 `--timeout` 须 > 205 秒（见上）。

## 压测后必做

1. **清理数据**：`psql -v api_key_id=<ID> -f scripts/loadtest-cleanup.sql`（幂等，可重跑）
2. **建档**：复制 `docs/loadtest/TEMPLATE.md` 为 `docs/loadtest/YYYY-MM-DD-<topic>.md`，
   原始 wrk 输出存 `docs/loadtest/raw/YYYY-MM-DD-<topic>/`
3. 关闭 `MOCK_UPSTREAM_ENABLED`（或保留配置但确认开关为 false）
