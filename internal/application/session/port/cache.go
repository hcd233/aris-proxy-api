package port

import (
	"context"
	"time"
)

// ShareCreator 创建分享缓存端口（由 infrastructure cache 实现）
type ShareCreator interface {
	CreateShare(ctx context.Context, userID, sessionID uint, ttl time.Duration) (string, time.Time, error)
}
