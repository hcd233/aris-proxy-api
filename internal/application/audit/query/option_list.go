package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// ListAuditOptionQuery 审计筛选选项查询
type ListAuditOptionQuery struct {
	Field     string
	Keyword   string
	StartTime time.Time
	EndTime   time.Time
	// UserID 非零表示 user 视角：先解析名下 API Key，按 key 范围过滤选项；
	// 为零表示 admin/demo 全量视角（demo 的身份脱敏由 service 层处理）。
	UserID uint
	// APIKeyIDs key 范围过滤。nil 表示不过滤（admin/demo 路径）；
	// user 视角由 Handle 解析填充，名下无 Key 时 Handle 直接返回空。
	APIKeyIDs []uint
}

// ListAuditOptionHandler 审计筛选选项查询处理器
type ListAuditOptionHandler interface {
	Handle(ctx context.Context, q ListAuditOptionQuery) ([]string, error)
}

type listAuditOptionHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
}

// NewListAuditOptionHandler 构造审计筛选选项查询处理器
func NewListAuditOptionHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup) ListAuditOptionHandler {
	return &listAuditOptionHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

// Handle 执行筛选选项查询。
//
// user 视角（UserID 非零）选项必须限制在名下 key 的数据范围内——
// 选项接口此前对普通用户返回全平台维度（用户名/邮箱等），与列表接口的
// owner 隔离语义不一致（2026-08-26 CR 修复）。
func (h *listAuditOptionHandler) Handle(ctx context.Context, q ListAuditOptionQuery) ([]string, error) {
	if q.UserID != 0 {
		keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
		if err != nil {
			return nil, ierr.Wrap(ierr.ErrDBQuery, err, "lookup api key ids by user id")
		}
		if len(keyIDs) == 0 {
			// 名下无 Key：选项恒为空，不得退化为全平台维度
			return []string{}, nil
		}
		q.APIKeyIDs = keyIDs
	}

	switch q.Field {
	case constant.AuditFilterFieldUser:
		return h.repo.ListDistinctUserNames(ctx, q.APIKeyIDs, q.Keyword, q.StartTime, q.EndTime)
	case constant.AuditFilterFieldModel:
		return h.repo.ListDistinctModels(ctx, q.APIKeyIDs, q.Keyword, q.StartTime, q.EndTime)
	case constant.AuditFilterFieldStatus:
		return h.repo.ListDistinctStatusCodes(ctx, q.APIKeyIDs, q.StartTime, q.EndTime)
	case constant.AuditFilterFieldUA:
		return h.repo.ListDistinctUserAgents(ctx, q.APIKeyIDs, q.Keyword, q.StartTime, q.EndTime)
	default:
		return []string{}, nil
	}
}
