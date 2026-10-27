package constant

import "time"

const (
	// ReadCacheTTL 读缓存条目基准 TTL
	//
	// 实际写入值 = 基准 TTL + [0, ReadCacheTTLJitter) 随机抖动，
	// 把同批写入条目的过期时刻打散，防缓存雪崩。
	ReadCacheTTL = 5 * time.Minute
	// ReadCacheTTLJitter 读缓存 TTL 随机抖动上限
	ReadCacheTTLJitter = 1 * time.Minute
	// ReadCacheEmptyTTL 空结果标记（防缓存穿透）基准 TTL
	//
	// 空结果用短 TTL：查询条件一旦开始命中数据（如用户新建模型），
	// 即使失效链路被绕过也能在短周期内自愈。
	ReadCacheEmptyTTL = 30 * time.Second
	// ReadCacheEmptyTTLJitter 空结果标记 TTL 随机抖动上限
	ReadCacheEmptyTTLJitter = 10 * time.Second
	// ReadCacheKeywordIDsTTL 读缓存-关键词命中用户消息 ID 集合基准 TTL
	//
	// 搜索结果新鲜度敏感（新消息应在分钟级可被搜到），显著短于目录类缓存的 5min；
	// 实际写入值 = 基准 TTL + [0, ReadCacheKeywordIDsTTLJitter) 随机抖动。
	ReadCacheKeywordIDsTTL = 60 * time.Second
	// ReadCacheKeywordIDsTTLJitter 读缓存-关键词命中 ID 集合 TTL 随机抖动上限
	ReadCacheKeywordIDsTTLJitter = 10 * time.Second
	// ReadCacheEmptyValue 空结果标记载荷
	//
	// 以 \x00 开头，与任何 sonic JSON 载荷（对象/数组/标量均以可打印字符开头）不冲突，
	// 命中即视为「查无此物」，直接返回零值而不回源。
	ReadCacheEmptyValue = "\x00nil"
	// ReadCacheScanBatch InvalidateAll 逐批 SCAN 的批次大小
	ReadCacheScanBatch = 128
	// ReadCacheModelListScopeAll Web 模型列表缓存键的 admin 全量视角片段
	ReadCacheModelListScopeAll = "all"
	// ReadCacheWindowTTL 读缓存-时间窗查询（统计聚合 / 筛选选项）基准 TTL
	//
	// 这类查询按时间窗取数，结果会随新写入的审计行变化；60s 足够摊薄看板重复刷新，
	// 又不至于让图表明显落后（目录类缓存用 5min）。
	ReadCacheWindowTTL = 60 * time.Second
	// ReadCacheWindowTTLJitter 时间窗查询 TTL 随机抖动上限
	ReadCacheWindowTTLJitter = 10 * time.Second
	// 读缓存-时间窗查询的用途标识（readCacheKey 的 purpose，参与键摘要）。
	// 必须全局唯一：两个不同查询共用同一 purpose + 相同其余参数会互相串数据。
	ReadCachePurposeAuditOptionUser   = "audit:option:user"
	ReadCachePurposeAuditOptionModel  = "audit:option:model"
	ReadCachePurposeAuditOptionStatus = "audit:option:status"
	ReadCachePurposeAuditOptionUA     = "audit:option:ua"
	ReadCachePurposeAuditStatsCost    = "audit:stats:model-cost"
	ReadCachePurposeAuditStatsTrend   = "audit:stats:model-trend"
	// ReadCachePurposeAuditStatsHarness 审计统计-按 harness 的调用量走势
	ReadCachePurposeAuditStatsHarness    = "audit:stats:harness"
	ReadCachePurposeAuditStatsRate       = "audit:stats:request-rate"
	ReadCachePurposeAuditStatsThroughput = "audit:stats:token-throughput"
	ReadCachePurposeAuditStatsLatency    = "audit:stats:first-token-latency"
	ReadCachePurposeCronOptionType       = "cron:option:type"
	ReadCachePurposeCronOptionStatus     = "cron:option:status"
	ReadCachePurposeSessionOptionScore   = "session:option:score"
	ReadCachePurposeSessionOptionModel   = "session:option:model"
	ReadCachePurposeSessionOptionMsgCnt  = "session:option:message-count"
	// ReadCachePurposeAuditPaginateCount 审计列表 total 的计数缓存（仅无 filter/keyword 时启用）。
	ReadCachePurposeAuditPaginateCount = "audit:paginate:count"

	// ReadCachePaginateCountTTL 列表总数字段基准 TTL。
	//
	// 审计表持续追加，但看板 total 落后 30s 无感知；继续沿用时间窗查询的抖动策略，
	// 避免同一批 scope 的计数条目同时过期。
	ReadCachePaginateCountTTL       = 30 * time.Second
	ReadCachePaginateCountTTLJitter = 10 * time.Second

	// ReadCacheWindowBucket 时间窗查询缓存键的时间对齐粒度
	//
	// 前端每次请求的 startTime/endTime 多为「当前时刻 - N 天」，逐请求变化会让缓存永不命中。
	// 键里把时间对齐到本粒度，同一分钟内的请求复用同一条目（聚合窗口最多偏移 59s）。
	ReadCacheWindowBucket = 60 * time.Second
)
