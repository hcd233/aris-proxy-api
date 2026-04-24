// Package session Session 域根（仓储接口）
package session

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/aggregate"
)

// SessionRepository Session 聚合仓储接口
//
//	@author centonhuang
//	@update 2026-04-22 19:30:00
type SessionRepository interface {
	// Save 持久化聚合（首次 Save 后回填 ID）
	Save(ctx context.Context, session *aggregate.Session) error
	// FindByID 按 ID 查询；未找到返回 (nil, nil)
	FindByID(ctx context.Context, id uint) (*aggregate.Session, error)
	// Paginate 按 owner 分页查询（用于 List 接口）
	Paginate(ctx context.Context, owner string, param PageParam) ([]*aggregate.Session, *model.PageInfo, error)
	// Delete 软删除（标记 deleted_at）
	Delete(ctx context.Context, id uint) error
}

// PageParam 分页查询参数
//
//	@author centonhuang
//	@update 2026-04-22 19:30:00
type PageParam struct {
	Page     int
	PageSize int
}
