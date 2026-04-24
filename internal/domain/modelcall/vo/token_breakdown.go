package vo

// TokenBreakdown Token 统计值对象
//
// 覆盖 OpenAI/Anthropic/Response API 三种上游的 token 字段。
// CacheCreation 仅 Anthropic 有；CacheRead 两边均可能有。
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type TokenBreakdown struct {
	Input         int
	Output        int
	CacheCreation int
	CacheRead     int
}

// IsZero 判断是否全零（缺失或未初始化）
//
//	@receiver t TokenBreakdown
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (t TokenBreakdown) IsZero() bool {
	return t.Input == 0 && t.Output == 0 && t.CacheCreation == 0 && t.CacheRead == 0
}
