package transport_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

func fixedRNG() *rand.Rand { return rand.New(rand.NewPCG(1, 2)) }

// TestNewMockSpec_Ranges 验证随机参数落在设计区间：长度 [1,16384]、TTFT [0,1s]、TPS [20,200]。
func TestNewMockSpec_Ranges(t *testing.T) {
	t.Parallel()
	rng := fixedRNG()
	for range 200 {
		s := transport.NewMockSpec(rng, 0)
		if n := len(s.Chars); n < 1 || n > constant.MockMaxChars {
			t.Fatalf("chars length %d out of [1,%d]", n, constant.MockMaxChars)
		}
		if s.TTFT < 0 || s.TTFT > constant.MockMaxTTFT {
			t.Fatalf("ttft %v out of [0,%v]", s.TTFT, constant.MockMaxTTFT)
		}
		if s.Interval <= 0 {
			t.Fatalf("interval must be > 0, got %v", s.Interval)
		}
		tps := time.Second / s.Interval
		if tps < constant.MockMinTPS || tps > constant.MockMaxTPS {
			t.Fatalf("tps %d out of [%d,%d]", tps, constant.MockMinTPS, constant.MockMaxTPS)
		}
	}
}

// TestNewMockSpec_MaxTokensCapsLength 验证 max_tokens 收紧长度上限（token×4 字符）。
func TestNewMockSpec_MaxTokensCapsLength(t *testing.T) {
	t.Parallel()
	rng := fixedRNG()
	for range 200 {
		s := transport.NewMockSpec(rng, 10) // 10 token → 最多 40 字符
		if len(s.Chars) > 10*constant.MockCharsPerToken {
			t.Fatalf("chars %d exceeds max_tokens*%d", len(s.Chars), constant.MockCharsPerToken)
		}
	}
}

// TestMockSpec_CharsOnlyPrintableASCII 验证字符集为可打印 ASCII 加换行。
func TestMockSpec_CharsOnlyPrintableASCII(t *testing.T) {
	t.Parallel()
	rng := fixedRNG()
	spec := transport.NewMockSpec(rng, 0)
	for i, c := range spec.Chars {
		if c == '\n' {
			continue
		}
		if c < 0x21 || c > 0x7E {
			t.Fatalf("char at %d = %q outside printable ASCII set", i, c)
		}
	}
}

// TestMockSpec_ChunksAndTokens 验证分片为 4 字符一片、token 按 1 token≈4 字符估算。
func TestMockSpec_ChunksAndTokens(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefghijklmno")} // 15 字符
	chunks := spec.Chunks()
	if len(chunks) != 4 { // 4+4+4+3
		t.Fatalf("want 4 chunks, got %d", len(chunks))
	}
	if want := 3; spec.Tokens() != want { // 15/4 = 3
		t.Fatalf("want tokens=%d, got %d", want, spec.Tokens())
	}
	short := transport.MockSpec{Chars: []byte("ab")}
	if got := short.Tokens(); got != 1 {
		t.Fatalf("short input must yield at least 1 token, got %d", got)
	}
}
