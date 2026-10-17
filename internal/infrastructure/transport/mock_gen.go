package transport

import (
	"math/rand/v2"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// MockSpec 一次 mock 响应的随机参数：内容、首 token 延迟、吐字节奏。
//
// 时序模型（每次请求独立抽参）：T=0 收到请求 → T=TTFT 吐首片 →
// 之后每片间隔 1/TPS → T=TTFT+(N-1)/TPS 流结束。
type MockSpec struct {
	Chars       []byte        // 随机字符内容
	TTFT        time.Duration // 首 token 延迟 [0,1s]
	Interval    time.Duration // 相邻分片间隔 = 1/TPS
	InputTokens int           // 输入 token 估算（由调用方按请求 body 填入）
}

// NewMockSpec 抽取一次 mock 响应的随机参数。
// maxTokens 是请求的 max_tokens（token 数）；<=0 表示不限制。
// 随机长度单位为字符，范围 [1, MockMaxChars]；带 max_tokens 时按 token 数×MockCharsPerToken 收紧上限。
//
// maxTokens 来自请求体（用户可控），先按 MockMaxChars/MockCharsPerToken 钳制再乘：
// 直接乘在 int64 上会回绕为负或零（2^61×4 溢出、2^62×4 归零），使 IntN 收到 n<=0 而 panic。
func NewMockSpec(rng *rand.Rand, maxTokens int) MockSpec {
	maxChars := constant.MockMaxChars
	if maxTokens > 0 {
		maxChars = min(maxTokens, constant.MockMaxChars/constant.MockCharsPerToken) * constant.MockCharsPerToken
	}
	n := rng.IntN(maxChars) + 1
	tps := constant.MockMinTPS + rng.IntN(constant.MockMaxTPS-constant.MockMinTPS+1)
	return MockSpec{
		Chars:    randomChars(rng, n),
		TTFT:     time.Duration(rng.IntN(int(constant.MockMaxTTFT) + 1)),
		Interval: time.Second / time.Duration(tps),
	}
}

// randomChars 生成 n 个字符：可打印 ASCII 0x21~0x7E 加换行（不含裸空格与控制字符）。
func randomChars(rng *rand.Rand, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = constant.MockCharAlphabet[rng.IntN(len(constant.MockCharAlphabet))]
	}
	return out
}

// Tokens 估算输出 token 数：1 token ≈ 4 字符，至少 1。
func (s MockSpec) Tokens() int {
	return max(1, len(s.Chars)/constant.MockCharsPerToken)
}

// Chunks 把内容切成 4 字符一片（≈1 token）的序列，末片可短。
func (s MockSpec) Chunks() []string {
	size := constant.MockCharsPerToken
	out := make([]string, 0, (len(s.Chars)+size-1)/size)
	for i := 0; i < len(s.Chars); i += size {
		out = append(out, string(s.Chars[i:min(i+size, len(s.Chars))]))
	}
	return out
}
