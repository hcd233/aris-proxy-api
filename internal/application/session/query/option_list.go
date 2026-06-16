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

type listSessionOptionHandler struct {
	readRepo session.SessionReadRepository
}

func NewListSessionOptionHandler(readRepo session.SessionReadRepository) sessionport.ListSessionOptionHandler {
	return &listSessionOptionHandler{readRepo: readRepo}
}

func (h *listSessionOptionHandler) Handle(ctx context.Context, q sessionport.ListSessionOptionQuery) ([]string, error) {
	switch q.Field {
	case constant.FieldScore:
		items := []string{constant.SessionOptionScoreValueUnscored}

		scores, err := h.readRepo.ListDistinctScores(ctx, q.StartTime, q.EndTime)
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
		return h.readRepo.ListDistinctModels(ctx, q.Keyword, q.StartTime, q.EndTime)
	default:
		return []string{}, nil
	}
}
