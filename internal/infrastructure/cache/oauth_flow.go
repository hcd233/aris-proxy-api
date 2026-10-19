package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// oauthFlowCache OAuthFlowStore 的 Redis 实现
type oauthFlowCache struct {
	rdb *redis.Client
}

// NewOAuthFlowCache 构造订阅账号授权流存储（TTL = constant.OAuthFlowTTL）
func NewOAuthFlowCache(rdb *redis.Client) service.OAuthFlowStore {
	return &oauthFlowCache{rdb: rdb}
}

// Put 写入授权流记录；失败必须向上报（flow 未落库则授权链接不可用）
func (c *oauthFlowCache) Put(ctx context.Context, state string, flow vo.OAuthFlow) error {
	payload, err := sonic.Marshal(flow)
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "marshal oauth flow")
	}
	if err := c.rdb.Set(ctx, fmt.Sprintf(constant.OAuthFlowKeyTemplate, state), payload, constant.OAuthFlowTTL).Err(); err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "store oauth flow")
	}
	return nil
}

// Take 原子取出并删除（GETDEL）；未命中返回 (零值, false, nil)
func (c *oauthFlowCache) Take(ctx context.Context, state string) (vo.OAuthFlow, bool, error) {
	raw, err := c.rdb.GetDel(ctx, fmt.Sprintf(constant.OAuthFlowKeyTemplate, state)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return vo.OAuthFlow{}, false, nil
		}
		return vo.OAuthFlow{}, false, ierr.Wrap(ierr.ErrInternal, err, "take oauth flow")
	}
	var flow vo.OAuthFlow
	if err := sonic.Unmarshal(raw, &flow); err != nil {
		return vo.OAuthFlow{}, false, ierr.Wrap(ierr.ErrInternal, err, "unmarshal oauth flow")
	}
	return flow, true, nil
}
