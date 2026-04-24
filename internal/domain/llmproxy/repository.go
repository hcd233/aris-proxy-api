// Package llmproxy llmproxy 域根（仓储接口 + 聚合引用）
package llmproxy

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

// EndpointRepository 模型端点仓储接口
//
// 实现由 infrastructure/repository 层提供，聚合根的加载与持久化通过本接口完成。
// 领域层只依赖该接口，不依赖 GORM 或 DAO。
//
//	@author centonhuang
//	@update 2026-04-22 16:30:00
type EndpointRepository interface {
	// FindByAliasAndProvider 按别名和上游协议查询单个端点
	//
	//	@param ctx context.Context
	//	@param alias vo.EndpointAlias
	//	@param provider enum.ProviderType
	//	@return *aggregate.Endpoint 未找到返回 nil
	//	@return error
	FindByAliasAndProvider(ctx context.Context, alias vo.EndpointAlias, provider enum.ProviderType) (*aggregate.Endpoint, error)
}
