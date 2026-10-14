# 压测档案：<主题>

> 日期：YYYY-MM-DD ｜ 执行人：xxx ｜ 分支：`xxx`

## 1. 压测参数

| 项 | 值 |
|----|----|
| 目标 URL | |
| 模型别名 | `mock-chat` / `mock-response` / `mock-messages` |
| 脚本 | `test/loadtest/xxx.lua`（git commit: `xxxxxxx`） |
| wrk 命令 | `wrk -t4 -c<并发> -d5m --latency -s ... <url>` |
| 并发档 | 50 / 100 / 200 / 400 / ... |
| stream | true |
| max_tokens | （默认 0=不限制，长度 1~16384 字符） |
| MOCK_UPSTREAM_ENABLED | true |

## 2. 环境信息

- git commit：
- 被测镜像 / 版本：
- pod 数 × CPU 配额：
- 执行时间（低峰期证据）：

## 3. 原始数据

- wrk 输出：`docs/loadtest/raw/<日期-主题>/`

## 4. 服务侧数据（审计表）

| 指标 | 值 | 取数 SQL |
|------|----|---------|
| TTFT p50/p90/p99 | | `SELECT percentile_cont(...) ... FROM model_call_audits WHERE api_key_id=<ID>` |
| 流持续 p50/p90/p99 | | 同上（`stream_duration_ms`） |
| 状态码分布 | | `SELECT upstream_status_code, count(*) ... GROUP BY 1` |
| token 总量 | | `SELECT sum(input_tokens), sum(output_tokens) ...` |

## 5. 结论

- **并发 SSE 长连接数（稳定拐点）**：
- 延迟分位（p50/p90/p99）：
- 流时长分层（短流 <30s / 中流 30~120s / 长流 >120s 各自的完成数与延迟）：
- 错误率与状态码：
- 与历史轮次对比：

## 6. 清理记录

- 清理 SQL：`scripts/loadtest-cleanup.sql`
- 清理前行数（预览）：
- 清理后复核（应为 0）：
- 清理执行时间：
