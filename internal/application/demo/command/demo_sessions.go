package command

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/session"
)

type addDemoSessionsHandler struct {
	demoRepo port.DemoSessionRepository
	readRepo session.SessionReadRepository
}

func NewAddDemoSessionsHandler(demoRepo port.DemoSessionRepository, readRepo session.SessionReadRepository) port.AddDemoSessionsHandler {
	return &addDemoSessionsHandler{demoRepo: demoRepo, readRepo: readRepo}
}

func (h *addDemoSessionsHandler) Handle(ctx context.Context, cmd port.AddDemoSessionsCommand) ([]uint, error) {
	ids := lo.Uniq(cmd.SessionIDs)
	if len(ids) == 0 {
		return nil, ierr.New(ierr.ErrValidation, "sessionIds is required")
	}
	// 校验存在性：仅存在的 session 可加入（fail-closed：查询失败拒绝全部）
	existing, err := existingSessionIDs(ctx, h.readRepo, ids)
	if err != nil {
		return nil, err
	}
	valid := lo.Filter(ids, func(id uint, _ int) bool { return existing[id] })
	if err := h.demoRepo.Add(ctx, valid); err != nil {
		return nil, err
	}
	return valid, nil
}

// existingSessionIDs 返回 ids 中实际存在的会话 ID 集合。
// 查询失败返回 error（fail-closed 拒绝添加），仅"无匹配结果"返回空 map（无 error）。
func existingSessionIDs(ctx context.Context, readRepo session.SessionReadRepository, ids []uint) (map[uint]bool, error) {
	param := model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: len(ids)}}
	projections, _, err := readRepo.ListSessionsByIDs(ctx, ids, param)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "validate demo session ids")
	}
	if len(projections) == 0 {
		return map[uint]bool{}, nil
	}
	return lo.SliceToMap(projections, func(p *session.SessionSummaryProjection) (uint, bool) {
		return p.ID, true
	}), nil
}

type removeDemoSessionsHandler struct {
	demoRepo port.DemoSessionRepository
}

func NewRemoveDemoSessionsHandler(demoRepo port.DemoSessionRepository) port.RemoveDemoSessionsHandler {
	return &removeDemoSessionsHandler{demoRepo: demoRepo}
}

func (h *removeDemoSessionsHandler) Handle(ctx context.Context, cmd port.RemoveDemoSessionsCommand) error {
	if len(cmd.SessionIDs) == 0 {
		return ierr.New(ierr.ErrValidation, "sessionIds is required")
	}
	return h.demoRepo.Remove(ctx, cmd.SessionIDs)
}
