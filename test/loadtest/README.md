# 接口压测

## 文档

- **[BENCHMARK.md](BENCHMARK.md)** —— 接口性能基准主文档。各接口的测试结果统一记录于此，
  新增接口测试请直接追加（已预留章节）。
- 原始数据：`raw/<日期>-<主题>/`（wrk 完整输出 + 采样元数据）

## 脚本

| 脚本 | 目标接口 |
|------|---------|
| `chat_completions.lua` | `POST /api/openai/v1/chat/completions` |
| `responses.lua` | `POST /api/openai/v1/responses` |
| `messages.lua` | `POST /api/anthropic/v1/messages` |

## 执行

```bash
export TARGET_URL=https://api.lvlvko.top
export API_KEY=<压测专用 Key>       # 单 Key 模式
# export LT_KEYS="$K1,$K2,..."      # 多 Key 轮换，优先于 API_KEY；容量压测必须用
export MODEL=mock-chat              # 或 mock-response / mock-messages

wrk -t4 -c100 -d120s --timeout 300s --latency \
  -s test/loadtest/chat_completions.lua "$TARGET_URL/api/openai/v1/chat/completions"
```

## 注意事项

- **`--timeout 300s` 必加**：wrk 默认 2 秒，mock 流平均 19 秒、最长 205 秒，不加会全判 timeout。
- **限流**：100 次/分钟 per API Key，流均 19 秒 → 单 Key 并发上限约 30；
  容量压测必须用 `LT_KEYS` 多 Key 分摊。
- **鉴权头**：`Authorization: <key>`（不是 `X-API-Key`）。
- **状态码分布**从审计表取（`upstream_status_code`）：脚本的 `response()` 回调在部分 wrk 构建中不工作。

## 压测前置（生产）

1. mock Endpoint（三能力标记全开）+ 三个 Model 别名（`upstream_model=mock-llm`）
2. `MOCK_UPSTREAM_ENABLED=true`（api.env + ConfigMap + 滚动重启）
3. 压测专用 API Key（按 `api_key_id` 清理）

## 压测后

```bash
psql -v api_key_id=<ID> -f scripts/loadtest-cleanup.sql   # 幂等
```
