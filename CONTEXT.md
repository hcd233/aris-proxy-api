# aris-proxy-api

LLM 代理网关。本文件是项目的领域词汇表（glossary），只定义术语，不含实现细节。

## Metrics（指标）

项目里有两类性质完全不同的指标，必须区分，不可混用 "metrics" 一词笼统指代。

**Business Metrics（业务指标）**:
从持久化的审计数据聚合出的业务维度指标，如 token 用量、请求速率、首 token 延迟、各 model 的调用分布。由后端按时间窗口聚合后返回，dashboard 主页展示。数据可长期留存、可回溯历史。
_Avoid_: usage stats, audit metrics

**Runtime Metrics（运行时指标）**:
反映进程/HTTP 运行状态的瞬时指标，如 goroutine 数、堆内存、CPU、QPS、在途请求数、P95 延迟、SSE 活跃连接数。只反映"此刻集群健康度"，无需长期留存，`/monitor` 页展示。
_Avoid_: monitor metrics, system metrics, ops metrics

**Snapshot（快照）**:
单个 pod 在某一时刻把自己全部 Runtime Metrics 的当前值采下的一份记录，带时间戳与 pod 实例标识。是写入 Redis 的最小单位。
_Avoid_: sample, datapoint, reading

**Aggregation（聚合）**:
把多个 Instance、多个时刻的 Snapshot 合并成一条可展示时序的过程，由后端聚合 API 完成，前端只渲染。三类指标合并规则不同，不可混用：
- **Gauge**（goroutines/heap/in_progress/sse_active）：跨 Instance 直接求和。
- **Counter→Rate**（请求总数→QPS、CPU 累计秒→CPU%）：必须**先**按各 Instance 自身相邻快照求速率（遇负 delta 判为重启、该段清零），**再**把各 Instance 的速率求和（等同 PromQL `sum(rate(...))`）。**严禁先跨 Instance 求和再求速率**——某个 pod 重启会让全局累计值塌陷、污染整段速率。
- **Histogram→Percentile**（请求时延→P95）：跨 Instance 合并同 `le` 的 bucket 计数，在合并后的分布上求分位；不可对各 Instance 的 P95 求和或求平均。
_Avoid_: rollup, merge, reduce

**Instance（pod 实例）**:
集群中一个运行的 aris-proxy-api 进程。每个 Instance 各自持有独立的内存计数器，互不可见，因此跨 Instance 的视图只能通过 Redis 共享的 Snapshot 经 Aggregation 得到。
_Avoid_: node, replica, server

**Bucket（时间桶）**:
聚合时按 granularity 把窗口切成的等宽时间段（如 24h 窗口切成 5min 一桶）。一个 Bucket 内可能落入某 Instance 的多份 Snapshot，需按指标类塌缩成单值：gauge 取桶内均值，rate 取桶首尾累计差除以桶时长，histogram 取桶内 bucket 增量。
_Avoid_: window, slot, interval

**Incremental Refresh（增量刷新）**:
前端 `/monitor` 的刷新策略：切换 range 时全量拉取，之后每 ~15s 带 `since=<最后一个桶时间>` 只拉取尾部新增的 Bucket 并追加、裁掉滑出窗口的旧 Bucket。使每次轮询的后端代价与所选 range 无关。
_Avoid_: polling, repull, full fetch
