package query

import (
	"context"
	"strconv"
	"strings"

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
	if q.Field != constant.FieldScore {
		return []string{}, nil
	}

	items := []string{constant.SessionOptionScoreLabelUnscored}

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
		filtered := make([]string, 0, len(items))
		for _, item := range items {
			if strings.Contains(item, q.Keyword) {
				filtered = append(filtered, item)
			}
		}
		return filtered, nil
	}

	return items, nil
}
