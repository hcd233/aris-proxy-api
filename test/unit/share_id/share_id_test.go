package share_id

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// TestGenerateShareID_Length 验证生成结果的长度等于入参 length。
//
//	@author centonhuang
//	@update 2026-05-28 20:10:00
func TestGenerateShareID_Length(t *testing.T) {
	t.Parallel()
	for length := constant.ShareIDMinLen; length <= constant.ShareIDMaxLen; length++ {
		got, err := util.GenerateShareID(42, length)
		if err != nil {
			t.Fatalf("length=%d unexpected error: %v", length, err)
		}
		if len(got) != length {
			t.Errorf("length=%d got len=%d (%q)", length, len(got), got)
		}
	}
}

// TestGenerateShareID_Charset 验证生成结果只包含 [0-9A-Za-z]。
//
//	@author centonhuang
//	@update 2026-05-28 20:10:00
func TestGenerateShareID_Charset(t *testing.T) {
	t.Parallel()
	const samples = 1000
	for i := 0; i < samples; i++ {
		got, err := util.GenerateShareID(uint(i+1), constant.ShareIDMinLen)
		if err != nil {
			t.Fatalf("sample %d error: %v", i, err)
		}
		for _, c := range []byte(got) {
			if !util.IsValidShareIDChar(c) {
				t.Errorf("invalid char %q in shareID %q", c, got)
			}
		}
	}
}

// TestGenerateShareID_OutOfRange 验证非法 length 会返回错误。
//
//	@author centonhuang
//	@update 2026-05-28 20:10:00
func TestGenerateShareID_OutOfRange(t *testing.T) {
	t.Parallel()
	cases := []int{0, 1, constant.ShareIDMinLen - 1, constant.ShareIDMaxLen + 1, 32}
	for _, length := range cases {
		_, err := util.GenerateShareID(1, length)
		if err == nil {
			t.Errorf("length=%d expected error, got nil", length)
		}
	}
}

// TestGenerateShareID_DivergentForSameSession 验证同一 sessionID 多次生成会产生不同短码
// （随机熵保证发散，避免与 UUID 不可重复的语义出现回归）。
//
//	@author centonhuang
//	@update 2026-05-28 20:10:00
func TestGenerateShareID_DivergentForSameSession(t *testing.T) {
	t.Parallel()
	const samples = 200
	seen := make(map[string]struct{}, samples)
	for i := 0; i < samples; i++ {
		got, err := util.GenerateShareID(7, constant.ShareIDMinLen)
		if err != nil {
			t.Fatalf("sample %d error: %v", i, err)
		}
		seen[got] = struct{}{}
	}
	// 6 位 62 进制空间 ~568 亿，重复极小概率。如果 200 个样本中重复超过 1 个就明显异常。
	if len(seen) < samples-1 {
		t.Errorf("expected near-unique shareIDs, got %d unique out of %d", len(seen), samples)
	}
}

// TestGenerateShareID_DistributionAcrossSessions 验证不同 sessionID 之间不会出现可观测的偏置。
//
//	@author centonhuang
//	@update 2026-05-28 20:10:00
func TestGenerateShareID_DistributionAcrossSessions(t *testing.T) {
	t.Parallel()
	const samples = 500
	seen := make(map[string]struct{}, samples)
	for i := 0; i < samples; i++ {
		got, err := util.GenerateShareID(uint(i+1), constant.ShareIDMaxLen)
		if err != nil {
			t.Fatalf("sample %d error: %v", i, err)
		}
		seen[got] = struct{}{}
	}
	if len(seen) != samples {
		t.Errorf("expected %d unique 8-char shareIDs, got %d", samples, len(seen))
	}
}

// TestIsValidShareIDChar 验证 shareID 字符集判定函数对边界字符的覆盖。
//
//	@author centonhuang
//	@update 2026-05-28 20:10:00
func TestIsValidShareIDChar(t *testing.T) {
	t.Parallel()
	for _, c := range []byte(constant.ShareIDAlphabet) {
		if !util.IsValidShareIDChar(c) {
			t.Errorf("expected %q to be valid", c)
		}
	}
	invalid := []byte("-_.+/= !@#$%^&*()|?,<>")
	for _, c := range invalid {
		if util.IsValidShareIDChar(c) {
			t.Errorf("expected %q to be invalid", c)
		}
	}
}
