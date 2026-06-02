package constant

import "time"

const (
	// ShareTTLDefault 分享链接默认有效期
	ShareTTLDefault = 24 * time.Hour
	// ShareTTL1Day 分享链接有效期：1天
	ShareTTL1Day = 24 * time.Hour
	// ShareTTL1Week 分享链接有效期：1周
	ShareTTL1Week = 7 * 24 * time.Hour
	// ShareTTL1Month 分享链接有效期：1月
	ShareTTL1Month = 30 * 24 * time.Hour
	// ShareTTLNeverExpire 分享链接有效期：永不过期（100年）
	ShareTTLNeverExpire = 100 * 365 * 24 * time.Hour
	// ShareExpiredRetention 分享链接过期后在用户列表中保留的时间
	ShareExpiredRetention = 72 * time.Hour

	// ShareExpireOption 分享链接过期选项
	ShareExpireOption1Day      = "1d"
	ShareExpireOption1Week     = "7d"
	ShareExpireOption1WeekAlt  = "1w"
	ShareExpireOption1Month    = "30d"
	ShareExpireOption1MonthAlt = "1M"
	ShareExpireOptionNever     = "never"
	ShareExpireOptionCustom    = "custom"

	// ShareIDAlphabet shareID 字符集：大小写字母+数字 (62 个字符)
	ShareIDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	// ShareIDMinLen shareID 最短长度
	ShareIDMinLen = 6
	// ShareIDMaxLen shareID 最长长度
	ShareIDMaxLen = 8
	// ShareIDMaxAttemptsPerLen 单一长度内的最大尝试次数；超过后递增长度再试
	ShareIDMaxAttemptsPerLen = 3
	// ShareListScanChunkSize 用户分享列表分页时每批从 Redis sorted set 拉取的最大 member 数
	ShareListScanChunkSize = 100
)
