// Package modelcall ModelCall 域根（仓储接口）
//
// 审计读写双向统一经本包的 AuditRepository 聚合仓储：写路径由 pool.SubmitModelCallAuditTask()
// 构造 ModelCallAudit 聚合后调用 Save()，读路径由 application/audit 查询服务调用 ListAll/聚合统计。
package modelcall

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
)

// AuditRelation 审计列表关联展示信息。
type AuditRelation struct {
	APIKeyID   uint
	APIKeyName string
	UserID     uint
	UserName   string
	UserEmail  string
}

// AuditRepository ModelCallAudit 聚合仓储接口
//
//	@author centonhuang
//	@update 2026-06-09 10:00:00
type AuditRepository interface {
	// Save 持久化审计聚合（首次 Save 后回填 ID）
	Save(ctx context.Context, audit *aggregate.ModelCallAudit) error

	// ListAll 全量分页查询审计记录，支持时间范围过滤、关键词搜索和多字段排序（admin/demo 用）
	ListAll(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, criteria *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)

	// ListByAPIKeyIDs 按 api_key_id IN (...) 分页查询；apiKeyIDs 为空时返回空结果且不打 SQL
	ListByAPIKeyIDs(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time, criteria *filter.FilterCriteria) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)

	// BatchGetRelations 批量查询审计列表所需的 API Key/User 展示信息。
	BatchGetRelations(ctx context.Context, apiKeyIDs []uint) (map[uint]*AuditRelation, error)

	// ListDistinctUserNames 查询去重的用户名列表。apiKeyIDs 为 nil 时查全部（admin/demo），非 nil 时按 key 过滤（空列表返回空结果，名下无 Key 不得越权查全量）。
	ListDistinctUserNames(ctx context.Context, apiKeyIDs []uint, keyword string, startTime, endTime time.Time) ([]string, error)

	// ListDistinctModels 查询去重的模型列表。apiKeyIDs 为 nil 时查全部（admin/demo），非 nil 时按 key 过滤（空列表返回空结果，名下无 Key 不得越权查全量）。
	ListDistinctModels(ctx context.Context, apiKeyIDs []uint, keyword string, startTime, endTime time.Time) ([]string, error)

	// ListDistinctStatusCodes 查询去重的上游状态码列表。apiKeyIDs 为 nil 时查全部（admin/demo），非 nil 时按 key 过滤（空列表返回空结果，名下无 Key 不得越权查全量）。
	ListDistinctStatusCodes(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time) ([]string, error)

	// ListDistinctUserAgents 查询去重的 User-Agent 列表（排除空值）。apiKeyIDs 为 nil 时查全部（admin/demo），非 nil 时按 key 过滤（空列表返回空结果，名下无 Key 不得越权查全量）。
	ListDistinctUserAgents(ctx context.Context, apiKeyIDs []uint, keyword string, startTime, endTime time.Time) ([]string, error)
}
