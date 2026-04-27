package vo

// TokenBreakdown Token 统计值对象
//
// 覆盖 OpenAI/Anthropic/Response API 三种上游的 token 字段。
// CacheCreation 仅 Anthropic 有；CacheRead 两边均可能有。
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type TokenBreakdown struct {
	input         int
	output        int
	cacheCreation int
	cacheRead     int
}

// NewTokenBreakdown 构造 Token 统计值对象
//
//	@param input int
//	@param output int
//	@param cacheCreation int
//	@param cacheRead int
//	@return TokenBreakdown
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewTokenBreakdown(input, output, cacheCreation, cacheRead int) TokenBreakdown {
	return TokenBreakdown{input: input, output: output, cacheCreation: cacheCreation, cacheRead: cacheRead}
}

// Input 返回输入 token 数
//
//	@receiver t TokenBreakdown
//	@return int
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (t TokenBreakdown) Input() int { return t.input }

// Output 返回输出 token 数
//
//	@receiver t TokenBreakdown
//	@return int
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (t TokenBreakdown) Output() int { return t.output }

// CacheCreation 返回缓存创建 token 数
//
//	@receiver t TokenBreakdown
//	@return int
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (t TokenBreakdown) CacheCreation() int { return t.cacheCreation }

// CacheRead 返回缓存读取 token 数
//
//	@receiver t TokenBreakdown
//	@return int
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (t TokenBreakdown) CacheRead() int { return t.cacheRead }

// IsZero 判断是否全零（缺失或未初始化）
//
//	@receiver t TokenBreakdown
//	@return bool
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (t TokenBreakdown) IsZero() bool {
	return t.input == 0 && t.output == 0 && t.cacheCreation == 0 && t.cacheRead == 0
}
