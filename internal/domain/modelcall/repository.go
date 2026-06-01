// Package modelcall ModelCall 域根（仓储接口）
//
// TODO: 此域尚未被 use case 层接入。LLM 代理当前通过 pool.SubmitModelCallAuditTask() 直接写入审计记录。
// 计划在后续迭代中将审计写入迁移至 aggregate + repository 模式，届时本包将被激活。
package modelcall

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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
//	@update 2026-05-29 14:00:00
type AuditRepository interface {
	// Save 持久化审计聚合（首次 Save 后回填 ID）
	Save(ctx context.Context, audit *aggregate.ModelCallAudit) error

	// ListAll 全量分页查询审计记录，支持时间范围过滤、关键词搜索和多字段排序（admin 用）
	ListAll(ctx context.Context, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)

	// ListByAPIKeyIDs 按 api_key_id IN (...) 分页查询；apiKeyIDs 为空时返回空结果且不打 SQL
	ListByAPIKeyIDs(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)

	// BatchGetRelations 批量查询审计列表所需的 API Key/User 展示信息。
	BatchGetRelations(ctx context.Context, apiKeyIDs []uint) (map[uint]*AuditRelation, error)

	// QueryModelTrend 按模型 + 时间桶统计调用次数。apiKeyIDs 为 nil 时查全部，非空时按 key 过滤。
	QueryModelTrend(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*ModelTrendPoint, error)

	// QueryRequestRate 按模型 + 时间桶统计请求成功率。apiKeyIDs 为 nil 时查全部，非空时按 key 过滤。
	QueryRequestRate(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*RequestRatePoint, error)

	// QueryTokenThroughput 按模型 + 时间桶统计 Token 吞吐量。apiKeyIDs 为 nil 时查全部，非空时按 key 过滤。
	QueryTokenThroughput(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*TokenThroughputPoint, error)
}

// ModelTrendPoint 模型调用趋势的数据点
type ModelTrendPoint struct {
	Model string
	Time  time.Time
	Count int
}

// RequestRatePoint 请求成功率的数据点
type RequestRatePoint struct {
	Model   string
	Time    time.Time
	Total   int
	Success int
}

// TokenThroughputPoint Token 吞吐量的数据点
type TokenThroughputPoint struct {
	Model                 string
	Time                  time.Time
	InputTokens           int
	OutputTokens          int
	CacheCreationTokens   int
	CacheReadTokens       int
	OutputTokensPerSecond float64
}
