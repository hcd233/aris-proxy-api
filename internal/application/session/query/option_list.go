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

func (h *listSessionOptionHandler) Handle(ctx context.Context, q sessionport.ListSessionOptionQuery) ([]sessionport.OptionItem, error) {
	if q.Field != constant.FieldScore {
		return []sessionport.OptionItem{}, nil
	}

	items := []sessionport.OptionItem{
		{Value: constant.SessionOptionScoreValueNone, Label: constant.SessionOptionScoreLabelNone},
	}

	scores, err := h.readRepo.ListDistinctScores(ctx, q.StartTime, q.EndTime)
	if err != nil {
		return nil, err
	}

	scoreLabels := map[int]string{
		1: constant.SessionOptionScoreLabel1,
		2: constant.SessionOptionScoreLabel2,
		3: constant.SessionOptionScoreLabel3,
		4: constant.SessionOptionScoreLabel4,
		5: constant.SessionOptionScoreLabel5,
	}

	for _, s := range scores {
		if label, ok := scoreLabels[s]; ok {
			items = append(items, sessionport.OptionItem{
				Value: strconv.Itoa(s),
				Label: label,
			})
		}
	}

	if q.Keyword != "" {
		filtered := make([]sessionport.OptionItem, 0, len(items))
		for _, item := range items {
			if strings.Contains(item.Label, q.Keyword) || strings.Contains(item.Value, q.Keyword) {
				filtered = append(filtered, item)
			}
		}
		return filtered, nil
	}

	return items, nil
}
