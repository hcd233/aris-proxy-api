package query

import (
	"context"
	"slices"

	"github.com/samber/lo"
	"go.uber.org/zap"

	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

var validSessionSortFields = map[string]bool{
	constant.FieldID:           true,
	constant.FieldCreatedAt:    true,
	constant.FieldUpdatedAt:    true,
	constant.FieldMessageCount: true,
	constant.FieldToolCount:    true,
}

// sessionFieldConfigs Session filter 字段配置
var sessionFieldConfigs = map[string]filter.FieldConfig{
	constant.FieldScore: {
		SQLColumn: constant.FieldScore,
		ValueMap: map[string]*string{
			constant.SessionOptionScoreValueUnscored: nil,
		},
	},
	constant.SessionFilterFieldModel: {
		SQLColumn:    constant.SessionFilterModelSQLColumn,
		IsJSONBArray: true,
	},
}

type ownerNameLookup interface {
	LookupOwnerNamesByUserID(ctx context.Context, userID uint) ([]string, error)
}

type listSessionsByUserHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo ownerNameLookup
}

func NewListSessionsByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) sessionport.ListSessionsByUserHandler {
	return &listSessionsByUserHandler{readRepo: readRepo, apiKeyRepo: apiKeyRepo}
}

func (h *listSessionsByUserHandler) Handle(ctx context.Context, q sessionport.ListSessionsByUserQuery) ([]*sessionport.SessionSummaryView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	param, err := sanitizeSessionListParam(ctx, q)
	if err != nil {
		return nil, nil, err
	}

	// 解析 filter
	criteria, err := parseSessionFilterCriteria(q.Filter)
	if err != nil {
		return nil, nil, err
	}

	var projections []*session.SessionSummaryProjection
	var pageInfo *model.PageInfo

	if q.IsAdmin {
		projections, pageInfo, err = h.readRepo.ListAllSessions(ctx, param, q.StartTime, q.EndTime, q.Keyword, criteria)
	} else {
		ownerNames, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, q.UserID)
		if lookupErr != nil {
			log.Error("[SessionQuery] Failed to lookup owner names", zap.Error(lookupErr), zap.Uint("userID", q.UserID))
			return nil, nil, lookupErr
		}
		if len(ownerNames) == 0 {
			return []*sessionport.SessionSummaryView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize, Total: 0}, nil
		}
		projections, pageInfo, err = h.readRepo.ListSessionsByOwnerNames(ctx, ownerNames, param, q.StartTime, q.EndTime, q.Keyword, criteria)
	}

	if err != nil {
		log.Error("[SessionQuery] Failed to list sessions by user", zap.Error(err), zap.Uint("userID", q.UserID))
		return nil, nil, err
	}

	lastQuestionIDs := lo.FilterMap(projections, func(p *session.SessionSummaryProjection, _ int) (uint, bool) {
		if len(p.Questions) > 0 {
			return p.Questions[len(p.Questions)-1], true
		}
		return 0, false
	})
	var msgByID map[uint]*session.MessageDetailProjection
	if len(lastQuestionIDs) > 0 {
		msgs, msgErr := h.readRepo.FindMessagesByIDs(ctx, lo.Uniq(lastQuestionIDs))
		if msgErr != nil {
			log.Warn("[SessionQuery] Failed to load questions[last] messages for summary", zap.Error(msgErr))
		} else {
			msgByID = lo.SliceToMap(msgs, func(m *session.MessageDetailProjection) (uint, *session.MessageDetailProjection) {
				return m.ID, m
			})
		}
	}

	views := lo.Map(projections, func(p *session.SessionSummaryProjection, _ int) *sessionport.SessionSummaryView {
		summary := ""
		if len(p.Questions) > 0 {
			if m, ok := msgByID[p.Questions[len(p.Questions)-1]]; ok && m.Message != nil {
				summary = util.ExtractMessageText(m.Message.Content)
			}
		}
		return &sessionport.SessionSummaryView{
			ID:           p.ID,
			CreatedAt:    p.CreatedAt,
			UpdatedAt:    p.UpdatedAt,
			Summary:      summary,
			Score:        p.Score,
			MessageCount: p.MessageCount,
			ToolCount:    p.ToolCount,
			Models:       p.Models,
		}
	})
	return views, pageInfo, nil
}

func sanitizeSessionListParam(ctx context.Context, q sessionport.ListSessionsByUserQuery) (model.CommonParam, error) {
	page := q.Page
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > constant.SessionMaxPageSize {
		pageSize = constant.SessionMaxPageSize
	}
	if page < 1 {
		page = 1
	}
	if q.SortField != "" && !validSessionSortFields[q.SortField] {
		logger.WithCtx(ctx).Warn("[SessionQuery] Invalid sort field", zap.String("sortField", q.SortField))
		return model.CommonParam{}, ierr.New(ierr.ErrValidation, "invalid sort field: "+q.SortField)
	}
	sort := q.Sort
	sortField := q.SortField
	if sort == "" {
		sort = enum.SortAsc
	}
	if sortField == "" {
		sortField = constant.FieldID
	}
	return model.CommonParam{
		PageParam: model.PageParam{Page: page, PageSize: pageSize},
		SortParam: model.SortParam{Sort: sort, SortField: sortField},
	}, nil
}

// parseSessionFilterCriteria 解析 session filter 表达式为 FilterCriteria
func parseSessionFilterCriteria(filterExpr string) (*filter.FilterCriteria, error) {
	if filterExpr == "" {
		return nil, nil
	}
	filters, err := filter.Parse(filterExpr)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "parse filter expression")
	}
	return &filter.FilterCriteria{
		Filters:      filters,
		FieldConfigs: sessionFieldConfigs,
	}, nil
}

type getSessionByUserHandler struct {
	readRepo   session.SessionReadRepository
	apiKeyRepo ownerNameLookup
}

func NewGetSessionByUserHandler(readRepo session.SessionReadRepository, apiKeyRepo apikey.APIKeyRepository) sessionport.GetSessionByUserHandler {
	return &getSessionByUserHandler{readRepo: readRepo, apiKeyRepo: apiKeyRepo}
}

func (h *getSessionByUserHandler) Handle(ctx context.Context, q sessionport.GetSessionByUserQuery) (*sessionport.SessionDetailView, error) {
	log := logger.WithCtx(ctx)

	detail, err := h.readRepo.GetSessionDetail(ctx, q.SessionID)
	if err != nil {
		log.Error("[SessionQuery] Failed to get session detail", zap.Error(err), zap.Uint("sessionID", q.SessionID))
		return nil, err
	}
	if detail == nil {
		log.Warn("[SessionQuery] Session not found", zap.Uint("sessionID", q.SessionID))
		return nil, ierr.New(ierr.ErrDataNotExists, "session not found")
	}

	if !q.IsAdmin && !q.SkipOwnershipCheck {
		ownerNames, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, q.UserID)
		if lookupErr != nil {
			log.Error("[SessionQuery] Failed to lookup owner names", zap.Error(lookupErr), zap.Uint("userID", q.UserID))
			return nil, lookupErr
		}
		allowed := slices.Contains(ownerNames, detail.APIKeyName)
		if !allowed {
			log.Warn("[SessionQuery] No permission to access session",
				zap.Uint("sessionID", q.SessionID),
				zap.String("owner", detail.APIKeyName),
				zap.Uint("userID", q.UserID))
			return nil, ierr.New(ierr.ErrNoPermission, "no permission to access session")
		}
	}

	messages := lo.Map(detail.Messages, func(m *session.MessageDetailProjection, _ int) *sessionport.MessageView {
		return &sessionport.MessageView{
			ID:        m.ID,
			Model:     m.Model,
			Message:   m.Message,
			CreatedAt: m.CreatedAt,
		}
	})

	tools := lo.Map(detail.Tools, func(t *session.ToolDetailProjection, _ int) *sessionport.ToolView {
		return &sessionport.ToolView{
			ID:        t.ID,
			Tool:      t.Tool,
			CreatedAt: t.CreatedAt,
		}
	})

	return &sessionport.SessionDetailView{
		ID:         detail.ID,
		APIKeyName: detail.APIKeyName,
		CreatedAt:  detail.CreatedAt,
		UpdatedAt:  detail.UpdatedAt,
		Metadata:   detail.Metadata,
		Score:      detail.Score,
		ScoredAt:   detail.ScoredAt,
		MessageIDs: detail.MessageIDs,
		ToolIDs:    detail.ToolIDs,
		Messages:   messages,
		Tools:      tools,
	}, nil
}
