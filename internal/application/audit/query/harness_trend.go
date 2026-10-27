package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/agentharness"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// HarnessTrendQuery admin/demo 视角的 harness 调用量走势查询
type HarnessTrendQuery struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

// HarnessTrendByUserQuery user 视角的 harness 调用量走势查询
type HarnessTrendByUserQuery struct {
	UserID      uint
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

// HarnessTrendHandler admin/demo 视角
type HarnessTrendHandler interface {
	Handle(ctx context.Context, q HarnessTrendQuery) ([]*modelcall.HarnessTrendPoint, error)
}

// HarnessTrendByUserHandler user 视角
type HarnessTrendByUserHandler interface {
	Handle(ctx context.Context, q HarnessTrendByUserQuery) ([]*modelcall.HarnessTrendPoint, error)
}

type harnessTrendHandler struct {
	repo  modelcall.AuditRepository
	rules agentharness.RuleRepository
}

type harnessTrendByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
	rules     agentharness.RuleRepository
}

// NewHarnessTrendHandler 构造 admin/demo 视角处理器
func NewHarnessTrendHandler(repo modelcall.AuditRepository, rules agentharness.RuleRepository) HarnessTrendHandler {
	return &harnessTrendHandler{repo: repo, rules: rules}
}

// NewHarnessTrendByUserHandler 构造 user 视角处理器
func NewHarnessTrendByUserHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup, rules agentharness.RuleRepository) HarnessTrendByUserHandler {
	return &harnessTrendByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs, rules: rules}
}

func (h *harnessTrendHandler) Handle(ctx context.Context, q HarnessTrendQuery) ([]*modelcall.HarnessTrendPoint, error) {
	return h.repo.QueryHarnessTrend(ctx, nil, q.StartTime, q.EndTime, q.Granularity, harnessRulesFingerprint(ctx, h.rules))
}

func (h *harnessTrendByUserHandler) Handle(ctx context.Context, q HarnessTrendByUserQuery) ([]*modelcall.HarnessTrendPoint, error) {
	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, err
	}
	// 名下无 API Key：直接返回空，防止空列表在仓储层退化为全量查询（越权）
	if len(keyIDs) == 0 {
		return []*modelcall.HarnessTrendPoint{}, nil
	}
	return h.repo.QueryHarnessTrend(ctx, keyIDs, q.StartTime, q.EndTime, q.Granularity, harnessRulesFingerprint(ctx, h.rules))
}

// harnessRulesFingerprint 取规则指纹供读缓存键使用；读取失败返回空串（退化为不带指纹的键）。
//
// 指纹只影响缓存新鲜度，不影响正确性：失败时宁可复用缓存，也不要让统计接口 500。
func harnessRulesFingerprint(ctx context.Context, rules agentharness.RuleRepository) string {
	if rules == nil {
		return ""
	}
	fingerprint, err := rules.Fingerprint(ctx)
	if err != nil {
		return ""
	}
	return fingerprint
}
