package query

import (
	"context"
	"strings"

	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

type listSessionOptionHandler struct{}

func NewListSessionOptionHandler() sessionport.ListSessionOptionHandler {
	return &listSessionOptionHandler{}
}

func (h *listSessionOptionHandler) Handle(ctx context.Context, q sessionport.ListSessionOptionQuery) ([]sessionport.OptionItem, error) {
	if q.Field != constant.FieldScore {
		return []sessionport.OptionItem{}, nil
	}

	items := []sessionport.OptionItem{
		{Value: constant.SessionOptionScoreValue1, Label: constant.SessionOptionScoreLabel1},
		{Value: constant.SessionOptionScoreValue2, Label: constant.SessionOptionScoreLabel2},
		{Value: constant.SessionOptionScoreValue3, Label: constant.SessionOptionScoreLabel3},
		{Value: constant.SessionOptionScoreValue4, Label: constant.SessionOptionScoreLabel4},
		{Value: constant.SessionOptionScoreValue5, Label: constant.SessionOptionScoreLabel5},
		{Value: constant.SessionOptionScoreValueNone, Label: constant.SessionOptionScoreLabelNone},
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
