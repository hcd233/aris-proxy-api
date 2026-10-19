package common_util

import (
	"testing"
	"unicode/utf8"

	"github.com/hcd233/aris-proxy-api/internal/common/util"
)

// TestTruncateRunes 按字符截断不得产生非法 UTF-8。
//
// 回归（CR I4）：上游错误摘要按字节截断会把多字节字符切成非法序列，
// 写入 Postgres text 列（last_error）直接失败。
func TestTruncateRunes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  string
		max    int
		expect string
	}{
		{name: "短于上限原样返回", input: "abc", max: 5, expect: "abc"},
		{name: "等于上限原样返回", input: "abcde", max: 5, expect: "abcde"},
		{name: "中文按字符截断", input: "中文错误摘要", max: 3, expect: "中文错"},
		{name: "截断点落在多字节字符中间", input: "ab中文", max: 3, expect: "ab中"},
		{name: "上限为 0 返回空串", input: "abc", max: 0, expect: ""},
		{name: "上限为负返回空串", input: "abc", max: -1, expect: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := util.TruncateRunes(tc.input, tc.max)
			if got != tc.expect {
				t.Fatalf("TruncateRunes(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.expect)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("truncated result must stay valid UTF-8, got %q", got)
			}
		})
	}
}
