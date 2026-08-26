package query

import (
	"context"
	"strconv"
	"strings"

	"github.com/samber/lo"

	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
)

// ownerNameLookup 接口定义见 jwt_session_queries.go（同包共享，由 apikey 仓储实现）。

type listSessionOptionHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo ownerNameLookup
}

func NewListSessionOptionHandler(readRepo session.SessionReadRepository, apiKeyRepo ownerNameLookup) sessionport.ListSessionOptionHandler {
	return &listSessionOptionHandler{readRepo: readRepo, apiKeyRepo: apiKeyRepo}
}

// Handle 执行筛选选项查询。
//
// 视角语义：admin 全量；demo 按 SessionIDs 白名单（handler 层已保证空白名单
// 不进入）；user（非 admin 且 SessionIDs 为 nil）按名下 key owner 过滤——
// 选项接口此前对普通用户返回全平台维度，与列表接口的 owner 隔离语义
// 不一致（2026-08-26 CR 修复）。
func (h *listSessionOptionHandler) Handle(ctx context.Context, q sessionport.ListSessionOptionQuery) ([]string, error) {
	var ownerNames []string
	if !q.IsAdmin && q.SessionIDs == nil {
		var err error
		ownerNames, err = h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, q.UserID)
		if err != nil {
			return nil, err
		}
		if len(ownerNames) == 0 {
			// 名下无 Key：选项恒为空，不得退化为全平台维度
			return []string{}, nil
		}
	}

	switch q.Field {
	case constant.FieldScore:
		items := []string{constant.SessionOptionScoreValueUnscored}

		scores, err := h.readRepo.ListDistinctScores(ctx, ownerNames, q.StartTime, q.EndTime, q.SessionIDs)
		if err != nil {
			return nil, err
		}

		for _, s := range scores {
			if s >= 1 && s <= 5 {
				items = append(items, strconv.Itoa(s))
			}
		}

		if q.Keyword != "" {
			filtered := lo.Filter(items, func(item string, _ int) bool {
				return strings.Contains(item, q.Keyword)
			})
			return filtered, nil
		}

		return items, nil
	case constant.SessionFilterFieldModel:
		return h.readRepo.ListDistinctModels(ctx, ownerNames, q.Keyword, q.StartTime, q.EndTime, q.SessionIDs)
	case constant.SessionFilterFieldMessageCount:
		maxCount, bucketCounts, err := h.readRepo.ListMessageCountStats(ctx, ownerNames, q.StartTime, q.EndTime, q.SessionIDs)
		if err != nil {
			return nil, err
		}
		return BuildMessageCountBuckets(maxCount, bucketCounts), nil
	default:
		return []string{}, nil
	}
}
