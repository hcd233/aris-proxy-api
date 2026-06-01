// Package enum provides common enums for the application.
package enum

// PromptCacheRetention 提示缓存保留策略
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type PromptCacheRetention = string

const (

	// PromptCacheRetentionInMemory 内存中保留
	//
	//	@author centonhuang
	//	@update 2026-04-25 10:00:00
	PromptCacheRetentionInMemory PromptCacheRetention = "in_memory"

	// PromptCacheRetention24h 保留24小时
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	PromptCacheRetention24h PromptCacheRetention = "24h"
)
