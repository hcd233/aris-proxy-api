// Package repository 领域仓储的基础设施（GORM）实现
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// endpointFields Endpoint 查询的统一字段清单；与原 service.endpointFields 一致
var endpointFields = []string{"id", "model", "api_key", "base_url", "provider"}

// endpointRepository EndpointRepository 的 GORM 实现
type endpointRepository struct {
	dao *dao.ModelEndpointDAO
}

// NewEndpointRepository 构造 EndpointRepository
//
//	@return llmproxy.EndpointRepository
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func NewEndpointRepository() llmproxy.EndpointRepository {
	return &endpointRepository{dao: dao.GetModelEndpointDAO()}
}

// FindByAliasAndProvider 按 alias + provider 查询单个端点
//
//	@receiver r *endpointRepository
//	@param ctx context.Context
//	@param alias vo.EndpointAlias
//	@param provider enum.ProviderType
//	@return *aggregate.Endpoint 未找到返回 nil 且 error 为 nil
//	@return error 真正 DB 错误（非 record not found）
//	@author centonhuang
//	@update 2026-04-22 16:30:00
func (r *endpointRepository) FindByAliasAndProvider(ctx context.Context, alias vo.EndpointAlias, provider enum.ProviderType) (*aggregate.Endpoint, error) {
	db := database.GetDBInstance(ctx)
	ep, err := r.dao.Get(db, &dbmodel.ModelEndpoint{Alias: alias.String(), Provider: provider}, endpointFields)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find endpoint by alias+provider")
	}
	return toAggregate(ep), nil
}

// toAggregate 将 GORM 模型映射为 Endpoint 聚合根
func toAggregate(m *dbmodel.ModelEndpoint) *aggregate.Endpoint {
	return aggregate.NewEndpoint(
		m.ID,
		vo.EndpointAlias(m.Alias),
		m.Provider,
		vo.UpstreamCreds{BaseURL: m.BaseURL, APIKey: m.APIKey, Model: m.Model},
	)
}
