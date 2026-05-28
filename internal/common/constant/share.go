package constant

import "time"

const (
	// ShareTTL 分享链接有效期
	ShareTTL = 24 * time.Hour

	// ShareIDAlphabet shareID 字符集：大小写字母+数字 (62 个字符)
	ShareIDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	// ShareIDMinLen shareID 最短长度
	ShareIDMinLen = 6
	// ShareIDMaxLen shareID 最长长度
	ShareIDMaxLen = 8
	// ShareIDMaxAttemptsPerLen 单一长度内的最大尝试次数；超过后递增长度再试
	ShareIDMaxAttemptsPerLen = 3
)
