package command

import (
	"context"

	"github.com/samber/mo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// UpdateModelHandler 更新命令处理器
type UpdateModelHandler interface {
	Handle(ctx context.Context, cmd port.UpdateModelCommand) error
}

type updateModelHandler struct {
	repo llmproxy.ModelRepository
}

// NewUpdateModelHandler 构造更新命令处理器
func NewUpdateModelHandler(repo llmproxy.ModelRepository) UpdateModelHandler {
	return &updateModelHandler{repo: repo}
}

func ptrToOption[T any](ptr *T) mo.Option[T] {
	if ptr == nil {
		return mo.None[T]()
	}
	return mo.Some(*ptr)
}

// Handle 执行更新命令
func (h *updateModelHandler) Handle(ctx context.Context, cmd port.UpdateModelCommand) error {
	log := logger.WithCtx(ctx)

	mResult := h.repo.FindByID(ctx, cmd.ModelID)
	if mResult.IsError() {
		log.Error("[ModelCommand] Find model for update failed", zap.Error(mResult.Error()))
		return mResult.Error()
	}
	m := mResult.MustGet()

	aliasOpt := mo.None[vo.EndpointAlias]()
	if cmd.Alias != nil {
		aliasOpt = mo.Some(vo.EndpointAlias(*cmd.Alias))
	}

	m.Update(aliasOpt, ptrToOption(cmd.ModelName), ptrToOption(cmd.EndpointID), ptrToOption(cmd.Enabled))

	if err := h.repo.Update(ctx, m); err != nil {
		log.Error("[ModelCommand] Update model failed", zap.Error(err))
		return err
	}

	log.Info("[ModelCommand] Update model success", zap.Uint("id", cmd.ModelID))
	return nil
}
