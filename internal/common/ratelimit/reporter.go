// Package ratelimit 限流相关公共契约。
package ratelimit

import "context"

// TokenUsageReporter 用于在 LLM 调用完成后按实际 token 用量扣减限流桶。
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type TokenUsageReporter interface {
	Report(ctx context.Context, tokens int64)
}
