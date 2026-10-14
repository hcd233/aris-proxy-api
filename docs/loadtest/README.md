# 三接口压测档案

对 chat completion / response / message 三个接口的压测记录沉淀。设计见
`docs/superpowers/specs/2026-10-14-loadtest-mock-model-design.md`。

## 沉淀规则

- **每次压测建档一份** `YYYY-MM-DD-<topic>.md`（从 `TEMPLATE.md` 复制），只增不改。
- 原始 wrk 输出存 `raw/YYYY-MM-DD-<topic>/`。
- 同一主题多轮用 `-v2`、`-v3` 后缀。
- **档案必备字段**：压测参数、环境信息（git commit / pod 数 / CPU 配额 / 低峰期证据）、
  脚本版本引用、原始数据、服务侧统计、结论、清理记录。缺一不可，保证可复现。

## 核心指标口径

**并发 SSE 长连接数** = 服务器同时维持的流式连接数（即 wrk `-c`），
**不是**「每秒完成请求数」。SSE 长连接场景下一个请求可持续 205 秒，完成速率不反映承载能力。

- **加压方式**：逐档递增并发找容量拐点（50→100→200→400→…），每档 warmup 60s
  + 正式 5 分钟 × 3 轮取中位数，直到出现拐点（错误率上升 / 延迟陡增 / 连接被拒）。
- **TTFT 与流持续**从审计表取真值（`model_call_audits.first_token_latency_ms` /
  `stream_duration_ms`），不在 wrk 侧测。

## 压测后必做

1. 清理数据：`psql -v api_key_id=<ID> -f scripts/loadtest-cleanup.sql`（幂等）
2. 建档（本目录）+ 原始输出存 `raw/`
3. 关闭 `MOCK_UPSTREAM_ENABLED`
