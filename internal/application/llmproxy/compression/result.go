// Package compression 提供 LLM 代理请求体中 tool output 的上下文压缩能力。
//
// 复刻 headroom 项目的确定性压缩算法（跳过 ML 模型），在 marshal body 之后、
// 转发上游之前执行。任何压缩异常回退原始 body，不影响请求正常转发。
package compression

// ItemCompressionResult 单个 tool output 的压缩结果。
type ItemCompressionResult struct {
	Output      string // 压缩后内容（或跳过/失败时的原始内容）
	Strategy    string // 策略名（"smart_crusher"/"log_compressor"/"search_compressor"/"passthrough"）
	Applied     bool   // 是否实际执行了压缩
	BytesBefore int    // len(原始内容)
	BytesAfter  int    // len(Output)
}

// CompressionStats 一个请求的聚合压缩统计。
type CompressionStats struct {
	BytesBefore     int
	BytesAfter      int
	ItemsCompressed int
	ItemsSkipped    int
	StrategiesUsed  []string
}

func (s *CompressionStats) addItem(r ItemCompressionResult) {
	s.BytesBefore += r.BytesBefore
	s.BytesAfter += r.BytesAfter
	if r.Applied {
		s.ItemsCompressed++
		if s.StrategiesUsed == nil {
			s.StrategiesUsed = []string{}
		}
		s.StrategiesUsed = append(s.StrategiesUsed, r.Strategy)
	} else {
		s.ItemsSkipped++
	}
}
