package compression

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// Compressor 压缩单个 tool output 的文本内容。永不返回 error——
// 压缩失败时返回原始内容，Applied=false。
type Compressor interface {
	Compress(content string) ItemCompressionResult
}

// Dispatcher 按 ContentType 路由到具体压缩器。
type Dispatcher struct {
	detector    *ContentDetector
	compressors map[enum.ContentType]Compressor
}

// NewDispatcher 构造默认 Dispatcher，注册一期所有压缩器。
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		detector: NewContentDetector(),
		compressors: map[enum.ContentType]Compressor{
			enum.ContentTypeJsonArray:     NewSmartCrusher(),
			enum.ContentTypeBuildOutput:   NewLogCompressor(),
			enum.ContentTypeSearchResults: NewSearchCompressor(),
		},
	}
}

// Compress 检测内容类型并路由到对应压缩器。未注册的类型走 Passthrough。
func (d *Dispatcher) Compress(content string) ItemCompressionResult {
	ct := d.detector.Detect(content)
	compressor, ok := d.compressors[ct]
	if !ok {
		result := NewPassthroughCompressor().Compress(content)
		result.Strategy = ct.String() + ":" + constant.CompressionStrategyPassthrough
		return result
	}
	return compressor.Compress(content)
}
