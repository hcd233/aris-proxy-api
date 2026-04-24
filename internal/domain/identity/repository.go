// Package identity Identity 域根（仓储接口）
package identity

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
)

// UserRepository User 聚合仓储接口
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type UserRepository interface {
	// Save 持久化聚合（首次 Save 后回填 ID）
	Save(ctx context.Context, user *aggregate.User) error
	// FindByID 按 ID 查询；未找到返回 (nil, nil)
	FindByID(ctx context.Context, id uint) (*aggregate.User, error)
	// FindByGithubBindID 按 github 绑定 ID 查询
	FindByGithubBindID(ctx context.Context, bindID string) (*aggregate.User, error)
	// FindByGoogleBindID 按 google 绑定 ID 查询
	FindByGoogleBindID(ctx context.Context, bindID string) (*aggregate.User, error)
	// TouchLastLogin 仅更新指定用户的 last_login 字段为当前时间
	// 提供此方法的原因：OAuth2 回调登录只需更新登录时间，避免全字段 Save
	// 导致 name/email/avatar/permission 的意外覆盖。
	TouchLastLogin(ctx context.Context, userID uint) error
}
