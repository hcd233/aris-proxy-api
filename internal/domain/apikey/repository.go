// Package apikey APIKey 域根（仓储接口）
package apikey

import (
	"context"

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
	// FindByID 按 ID 查询聚合；未找到返回 (nil, nil)
	FindByID(ctx context.Context, id uint) (*aggregate.ProxyAPIKey, error)
	// ListByUser 查询指定用户的 Key 列表
	ListByUser(ctx context.Context, userID uint) ([]*aggregate.ProxyAPIKey, error)
	// ListAll 查询所有 Key（admin 视图）
	ListAll(ctx context.Context) ([]*aggregate.ProxyAPIKey, error)
	// CountByUser 统计指定用户的 Key 总数（含 UserID==0 的历史 key）
	CountByUser(ctx context.Context, userID uint) (int64, error)
	// Delete 删除 Key（软删除）
	Delete(ctx context.Context, id uint) error
}
