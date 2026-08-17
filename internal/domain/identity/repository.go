// Package identity Identity 域根（仓储接口）
package identity

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
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
	// FindByPermission 按权限精确查询（用于定位全局单例 Demo 账户）；未找到返回 (nil, nil)
	FindByPermission(ctx context.Context, permission enum.Permission) (*aggregate.User, error)
	// ReplaceDemoUser 在一个事务中将目标用户提升为 Demo，并将旧 Demo 用户降为 pending。
	// 返回被替换的 Demo 用户 ID；不存在旧 Demo 时返回 0。
	ReplaceDemoUser(ctx context.Context, targetID uint) (uint, error)
	// TouchLastLogin 仅更新指定用户的 last_login 字段为当前时间
	// 提供此方法的原因：OAuth2 回调登录只需更新登录时间，避免全字段 Save
	// 导致 name/email/avatar/permission 的意外覆盖。
	TouchLastLogin(ctx context.Context, userID uint) error
	// ListUsers 分页查询用户（管理员视图）。permission 非空时按权限精确过滤；
	// param.Query 非空时对 name/email 做模糊匹配。
	ListUsers(ctx context.Context, param model.CommonParam, permission enum.Permission) ([]*aggregate.User, *model.PageInfo, error)
	// DeleteCascade 软删除用户及其全部 API Keys（事务保护）
	DeleteCascade(ctx context.Context, id uint) error
}
