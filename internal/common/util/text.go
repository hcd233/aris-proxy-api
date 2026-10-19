package util

// TruncateRunes 按字符（rune）截断字符串，最多保留 maxRunes 个字符。
//
// 不能写成 s[:maxRunes]：按字节截断会把多字节字符切成非法 UTF-8 序列，
// 该值落库到 Postgres text 列会直接报错（上游错误摘要含中文/emoji 时必现）。
// maxRunes <= 0 时返回空串（调用方语义：不保留任何内容）。
func TruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}
