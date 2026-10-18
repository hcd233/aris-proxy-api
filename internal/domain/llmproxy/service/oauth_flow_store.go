package service

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// OAuthFlowStore 订阅账号授权流临时记录存储（实现于 infrastructure/cache）。
//
// Take 即取即删（Redis GetDel）：同一 state 只能消费一次，天然防重放。
type OAuthFlowStore interface {
	// Put 写入授权流记录（TTL 由实现绑定 constant.OAuthFlowTTL）
	Put(ctx context.Context, state string, flow vo.OAuthFlow) error
	// Take 取出并删除授权流记录；未命中返回 (零值, false, nil)
	Take(ctx context.Context, state string) (vo.OAuthFlow, bool, error)
}
