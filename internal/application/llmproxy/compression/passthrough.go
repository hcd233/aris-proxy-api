package compression

import "github.com/hcd233/aris-proxy-api/internal/common/constant"

// PassthroughCompressor 不压缩，原样返回内容。
type PassthroughCompressor struct{}

// Compress 返回原始内容，Applied=false。
func (p *PassthroughCompressor) Compress(content string) ItemCompressionResult {
	return ItemCompressionResult{
		Output:      content,
		Strategy:    constant.CompressionStrategyPassthrough,
		Applied:     false,
		BytesBefore: len(content),
		BytesAfter:  len(content),
	}
}

// NewPassthroughCompressor 构造 PassthroughCompressor。
func NewPassthroughCompressor() *PassthroughCompressor {
	return &PassthroughCompressor{}
}
