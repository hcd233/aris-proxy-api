# 三接口 Mock 压测 Benchmark（2026-10-16 基线）

> 目标：量化 chat completion / response / message 三个 LLM 转发接口在生产链路下的
> 并发承载、延迟分布与流式时序特征。
> 设计依据：`docs/superpowers/specs/2026-10-14-loadtest-mock-model-design.md`

## 1. 结论摘要

1. **三接口在 30 并发 SSE 长连接下无饱和迹象**：完成请求数随并发线性增长
   （chat 10→20→30 并发对应 32→74→114 完成请求），p50 稳定在 15~21 秒，
   服务端无错误。
2. **真实反映流式长连接特征**：p50 约 18 秒、p90 约 43 秒、p99 达 1.1~1.7 分钟，
   呈明显长尾——这是 mock 模型设计的时序（输出 1~16384 字符、TPS 20~200）所致，
   而非服务端抖动。
3. **服务端埋点与 mock 设计精确吻合**：TTFT p50 527~568ms（设计 U(0,1s) 期望 500ms）、
   平均流持续 20.7~21.4 秒（设计期望约 19.5 秒）、平均输出 1862~1976 token
   （设计期望 2048）。证明随机流时序生成与网关的 `first_token_latency_ms` /
   `stream_duration_ms` 埋点都准确。
4. **压测的硬天花板是限流，不是服务承载能力**：LLM 转发路由有
   `TokenBucketRateLimiterMiddleware`，限额 **100 次/分钟 per API Key**
   （`constant.LimitCallProxyLLM`）。因 mock 流平均 19 秒，**并发超过约 30 即触发限流**；
   100 并发时 95% 以上请求被 429 快速拒绝，测不到服务真实上限。
5. 三接口之间**无显著性能差异**（同一套链路，仅在协议序列化层不同）。

## 2. 测试环境

| 项 | 值 |
|----|----|
| 被测环境 | 生产 `api.lvlvko.top`（真实链路：鉴权 → 限流 → 触发词 → 模型解析 → 审计 → 落库） |
| 服务版本 | git `4de0f0bb`（含 mock 模型能力 + 周 CR 的 mock 随机源竞态修复） |

> **版本可信度**：压测于 2026-10-16 15:21Z 执行，而 `4de0f0bb`（修复共享 `*rand.Rand`
> 的并发 data race，改为 per-request 实例）的 CI 部署完成于 09:04Z、pod 启动于 09:04:52Z。
> 即**压测跑在已修复竞态的镜像上**，数据不受该竞态影响。
| 部署 | k3s 单节点（Ubuntu 24.04，kernel 6.8.0-31），2 replicas |
| 资源配额 | CPU requests 50m / limits **300m**；Memory requests 128Mi / **512Mi** |
| Mock 上游 | 进程内生成，不发外部 HTTP（`mock.upstream.enabled=true`） |
| 压测客户端 | wrk 4.x（Alpine arm64 容器内原生执行，避免 amd64 模拟段错误） |
| 执行时间 | 2026-10-16 15:21:41Z ~ 15:48:54Z（UTC，低峰期） |
| 压测 Key | 专用 `proxy_api_keys.id=8` |

**被测模型**（`upstream_model='mock-llm'`）：`mock-chat` / `mock-response` / `mock-messages`。

## 3. 测试方法

**Mock 随机流参数**（每次请求独立抽样）：

| 参数 | 取值 |
|------|------|
| 输出长度 | `min(max_tokens, U(1, 16384))` 字符 |
| TTFT | `U(0, 1s)` |
| 输出速率 | `U(20, 200) tok/s`（分片 4 字符 ≈ 1 token） |
| 流时长 | `TTFT + (N-1)/TPS`，期望约 19 秒，最长约 205 秒 |

**执行参数**：

- 窗口 `-d120s`，线程 `-t4`，**超时 `--timeout 300s`（必须，否则默认 2 秒会把长流全判成 timeout）**
- 并发档位：**10 / 20 / 30**（均在 100 req/min 限额内）
- 档间冷却 60 秒（等令牌桶恢复，避免前档余波污染后档）
- 每档记录 wrk 原始输出（`raw/2026-10-16-baseline/`）

**指标来源分两侧**：

- 客户端侧（wrk）：完成请求数、req/s、延迟分位、传输量、Socket errors
- 服务端侧（审计表 `model_call_audits`）：TTFT、流持续、token、状态码

## 4. 结果

### 4.1 客户端侧（wrk，120 秒窗口，限流内）

| 接口 | 并发 | 完成请求 | req/s | p50 | p75 | p90 | p99 | 传输量 |
|------|-----|---------|-------|-----|-----|-----|-----|--------|
| chat | 10 | 32 | 0.27 | 20.87s | 33.15s | 40.65s | 1.31m | 14.10MB |
| chat | 20 | 74 | 0.62 | 18.34s | 30.30s | 47.87s | 1.38m | 28.64MB |
| chat | 30 | 114 | 0.95 | 16.98s | 28.57s | 47.18s | 1.69m | 39.01MB |
| response | 10 | 34 | 0.28 | 18.34s | 28.60s | 42.46s | 1.14m | 7.87MB |
| response | 20 | 92 | 0.77 | 19.06s | 32.07s | 43.03s | 1.26m | 21.99MB |
| response | 30 | 119 | 0.99 | 18.58s | 34.15s | 43.27s | 1.19m | 29.92MB |
| messages | 10 | 37 | 0.31 | 15.23s | 27.48s | 42.65s | 50.93s | 10.74MB |
| messages | 20 | 85 | 0.71 | 19.07s | 28.65s | 42.00s | 1.02m | 24.10MB |
| messages | 30 | 109 | 0.91 | 19.78s | 32.20s | 0.94m | 1.32m | 31.59MB |

**读法**：这里的 `req/s` 是**完成速率**（长流场景下很低是正常的，一个请求平均占用
19 秒），**不是**承载能力指标。承载能力看「完成请求数随并发的增长是否线性」——
三接口均线性增长，说明 30 并发远未到饱和。

**Socket errors**：全部档位为 0（加了 `--timeout 300s` 后）。

### 4.2 服务端侧（审计表，仅 `upstream_status_code=200`）

| 接口 | 成功记录 | avg TTFT | p50 TTFT | p90 TTFT | avg 流持续 | avg 输出 token | 累计 token |
|------|---------|----------|----------|----------|-----------|---------------|-----------|
| chat | 223 | 506ms | 527ms | 897ms | 20.7s | 1862 | 415,315 |
| messages | 232 | 524ms | 536ms | 942ms | 21.4s | 1921 | 445,680 |
| response | 246 | 549ms | 568ms | 939ms | 20.7s | 1976 | 486,163 |

**与 mock 设计的对照**（验证埋点与生成器一致性）：

| 指标 | 实测 | 设计值 | 判定 |
|------|------|--------|------|
| TTFT 均值 | 506~549ms | `U(0,1s)` 期望 500ms | ✅ 吻合 |
| TTFT p90 | 897~942ms | 接近上限 1s | ✅ 吻合 |
| 流持续均值 | 20.7~21.4s | 期望 ≈ 19.5s（19s 输出 + 0.5s TTFT） | ✅ 长尾略拉高，合理 |
| 输出 token | 1862~1976 | 期望 2048 | ✅ 吻合 |

### 4.3 限流天花板（第一轮，超出限额的档位）

第一轮曾以 20/50/100 档位试压，结果暴露了限流边界：

| 接口 | 并发 | wrk 报告请求 | 现象 | 判定 |
|------|-----|-------------|------|------|
| chat | 20 | 90 | p50 17.10s，正常 | ✅ 有效 |
| chat | 50 | 165 | p50 23.04s，接近限额（82/min） | ⚠️ 临界 |
| chat | 100 | 13542 | p50 **256.83ms**，req/s 112.76 | ❌ 95%+ 被 429 |
| response | 100 | 15042 | p50 297.75ms | ❌ 同上 |
| messages | 100 | 14999 | p50 269.58ms | ❌ 同上 |

**判定依据**：审计表里对应时段只有约 600~800 条成功记录/接口，而 wrk 报告 1.3~1.5 万，
两者相差 95%+；且 p50 从 ~18s 骤降到 ~270ms（429 为快速拒绝，无流式过程）。

**限流配置**：`internal/router/openai.go` 与 `anthropic.go` 的 chat/responses/messages
路由均挂 `TokenBucketRateLimiterMiddleware(cache, "callProxyLLM", CtxKeyAPIKeyID,
PeriodCallProxyLLM=1min, LimitCallProxyLLM=100)`（`internal/common/constant/ratelimit.go`）。

**由此推出的并发上限**：`100 req/min ÷ 60s × 平均流时长 19s ≈ 31.6` ——
**在默认限额下，能稳定压的并发上限约为 30**，与实测（c=30 干净、c=50 临界）一致。

## 5. 关键发现

1. **`--timeout` 必须显式加大**：wrk 默认超时 2 秒，mock 流平均 19 秒、最长 205 秒。
   首次未加时 20 并发/120 秒窗口漏掉 79 个连接（`Socket errors: timeout`），
   延迟与状态码全部失真。**任何长流压测都必须先确认客户端超时 > 最长流时长**。

2. **限流是长流场景下的真实瓶颈**：请求维度限流按「次数/分钟」计，而长流让单个请求
   占用数十秒，两者叠加使**有效压测并发被压到约 30**。若要测服务真实承载上限，
   需先提高 `LimitCallProxyLLM`（改常量需发版）或多 Key 分摊。

3. **服务端时序埋点可信**：审计表的 `first_token_latency_ms` 与 `stream_duration_ms`
   与 mock 注入的已知分布高度吻合，可作为后续性能回归的稳定基准。

4. **三协议无性能差异**：chat / responses / messages 在同等并发下完成数、延迟分位、
   服务端 TTFT 与流持续均在噪声范围内一致。

5. **wrk 的 `response()` 回调在部分构建中不工作**：Alpine 打包的 wrk 实测
   `response called: 0`，故状态码分布改从审计表 `upstream_status_code` 取（更权威）。

## 6. 复现方法

```bash
# 1. 取压测专用 Key（id=8）
LTKEY=$(ssh ubuntu@api.lvlvko.top 'source /home/ubuntu/code/aris-proxy-api/env/api.env; \
  docker exec -e PGPASSWORD="$POSTGRES_PASSWORD" postgresql psql -U lvlvko -d aris-proxy-prod \
  -tAc "SELECT key FROM proxy_api_keys WHERE id=8 AND deleted_at=0"')

# 2. 单档压测（并发 30、窗口 120s、超时 300s）
docker run --rm --platform linux/arm64 -v "$PWD/test/loadtest:/scripts:ro" \
  -e API_KEY="$LTKEY" -e MODEL=mock-chat \
  wrk-arm64 -t4 -c30 -d120s --timeout 300s --latency \
  -s /scripts/chat_completions.lua \
  https://api.lvlvko.top/api/openai/v1/chat/completions

# 3. 服务端指标
#    SELECT ... FROM model_call_audits WHERE api_key_id=8 AND upstream_status_code=200
```

`wrk-arm64` 镜像构建（本机 wrk 因 CLT 不兼容 macOS 26 无法安装；amd64 镜像在
Rosetta 模拟下段错误，故用 Alpine arm64 原生包）：

```bash
printf 'FROM alpine:3.20\nRUN apk add --no-cache wrk\nENTRYPOINT ["wrk"]\n' > Dockerfile
docker build --platform linux/arm64 -t wrk-arm64 .
```

**压测后清理**：

```bash
psql -v api_key_id=8 -f scripts/loadtest-cleanup.sql   # 幂等
```

## 7. 原始数据

`raw/2026-10-16-baseline/` 组织：

| 文件 | 内容 |
|------|------|
| `mock-<model>-c<并发>.txt` | 单档 wrk 完整输出（含 `--latency` 分位分布）。c=10/20/30 为第二轮干净档位；c=50/100 为第一轮超限档位 |
| `round1-ratelimit-sweep.log` | 第一轮（20/50/100）逐档汇总日志——**第一轮的 c=20 原始输出已被第二轮同名文件覆盖**，其数据（90 请求 / p50 17.10s）以本日志为准 |
| `round2-clean-sweep.log` | 第二轮（10/20/30）逐档汇总日志 |
| `run-meta.txt` | 两轮各档位的起止时间戳 |

> 教训：压测脚本按 `<model>-c<并发>.txt` 命名会在多轮同档位时互相覆盖。
> 多轮压测应把轮次编进路径（如 `raw/<日期>-<轮次主题>/`）。

## 8. 未覆盖与已知限制

- **服务真实承载上限未测出**：被 100 req/min 限流挡住。需提高限额后重压。
- **长流未跑满**：120 秒窗口 < 最长流 205 秒，p99 与 req/s 受窗口截断影响；
  要测极端长流需更长的窗口。
- **Responses 非流式响应形态偏差**：`output[].content` 序列化为字符串而非
  OpenAI 规范要求的数组（`[{"type":"output_text","text":...}]`）。网关自身解析兼容、
  对本次压测无影响，但严格按规范解析的第三方 SDK 可能失败。
- **仅压单 Key**：多 Key 并发下的聚合承载能力未测。
