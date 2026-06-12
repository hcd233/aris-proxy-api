// Package apikey APIKey 域根（仓储接口）
package apikey

import (
	"context"

	"github.com/samber/mo"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
)

// APIKeyRepository API Key 仓储接口
//
// 具体实现由 infrastructure/repository 提供；领域层通过接口倒置 GORM 依赖。
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type APIKeyRepository interface {
	// Save 新建或更新聚合根（首次 Save 后回填 ID）
	Save(ctx context.Context, key *aggregate.ProxyAPIKey) error
	// FindByID 按 ID 查询聚合；未找到返回 ErrDataNotExists
	FindByID(ctx context.Context, id uint) mo.Result[*aggregate.ProxyAPIKey]
	// ListByUser 查询指定用户的 Key 列表
	ListByUser(ctx context.Context, userID uint) ([]*aggregate.ProxyAPIKey, error)
	// ListAll 查询所有 Key（admin 视图）
	ListAll(ctx context.Context) ([]*aggregate.ProxyAPIKey, error)
	// PaginateByUser 分页查询指定用户的 Key 列表
	PaginateByUser(ctx context.Context, userID uint, param model.CommonParam) ([]*aggregate.ProxyAPIKey, *model.PageInfo, error)
	// PaginateAll 分页查询所有 Key（admin 视图）
	PaginateAll(ctx context.Context, param model.CommonParam) ([]*aggregate.ProxyAPIKey, *model.PageInfo, error)
	// CountByUser 统计指定用户的 Key 总数（含 UserID==0 的历史 key）
	CountByUser(ctx context.Context, userID uint) (int64, error)
	// Delete 删除 Key（软删除）
	Delete(ctx context.Context, id uint) error
	// LookupOwnerNamesByUserID 查询指定用户的所有 API Key 名称
	LookupOwnerNamesByUserID(ctx context.Context, userID uint) ([]string, error)
	// LookupIDsByUserID 查询指定用户的所有 API Key ID
	LookupIDsByUserID(ctx context.Context, userID uint) ([]uint, error)
}
