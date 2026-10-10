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
	// ReadCacheEmptyValue 空结果标记载荷
	//
	// 以 \x00 开头，与任何 sonic JSON 载荷（对象/数组/标量均以可打印字符开头）不冲突，
	// 命中即视为「查无此物」，直接返回零值而不回源。
	ReadCacheEmptyValue = "\x00nil"
	// ReadCacheScanBatch InvalidateAll 逐批 SCAN 的批次大小
	ReadCacheScanBatch = 128
	// ReadCacheModelListScopeAll Web 模型列表缓存键的 admin 全量视角片段
	ReadCacheModelListScopeAll = "all"
)
