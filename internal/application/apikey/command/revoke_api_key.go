package command

import (
	"context"

	"go.uber.org/zap"

	commonenum "github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// RevokeAPIKeyCommand 吊销 API Key 命令
//
//	@author centonhuang
//	@update 2026-04-23 10:45:00
type RevokeAPIKeyCommand struct {
	// KeyID 要吊销的 Key ID
	KeyID uint
	// RequesterID 执行吊销操作的用户 ID（来自 ctx）
	RequesterID uint
	// RequesterPermission 执行者权限（判定是否可跨用户吊销）
	RequesterPermission commonenum.Permission
}

// RevokeAPIKeyHandler 吊销命令处理器
//
//	@author centonhuang
//	@update 2026-04-23 10:45:00
type RevokeAPIKeyHandler interface {
	Handle(ctx context.Context, cmd RevokeAPIKeyCommand) error
}

type revokeAPIKeyHandler struct {
	repo apikey.APIKeyRepository
}

// NewRevokeAPIKeyHandler 构造吊销命令处理器
//
//	@param repo apikey.APIKeyRepository
//	@return RevokeAPIKeyHandler
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func NewRevokeAPIKeyHandler(repo apikey.APIKeyRepository) RevokeAPIKeyHandler {
	return &revokeAPIKeyHandler{repo: repo}
}

// Handle 执行吊销
//
// 规则：
//
//   - Key 不存在 → ErrDataNotExists
//
//   - 非 admin 且非严格所有者（UserID != RequesterID 或 UserID==0）→ ErrNoPermission
//     （UserID==0 的 legacy Key 仅允许 admin 操作）
//
//   - 通过校验 → Delete
//
//     @receiver h *revokeAPIKeyHandler
//     @param ctx context.Context
//     @param cmd RevokeAPIKeyCommand
//     @return error
//     @author centonhuang
//     @update 2026-04-23 10:45:00
func (h *revokeAPIKeyHandler) Handle(ctx context.Context, cmd RevokeAPIKeyCommand) error {
	log := logger.WithCtx(ctx)

	key, err := h.repo.FindByID(ctx, cmd.KeyID)
	if err != nil {
		log.Error("[APIKeyCommand] FindByID failed", zap.Error(err), zap.Uint("keyID", cmd.KeyID))
		return err
	}
	if key == nil {
		log.Warn("[APIKeyCommand] API key not found", zap.Uint("keyID", cmd.KeyID))
		return ierr.New(ierr.ErrDataNotExists, "api key not found")
	}

	if cmd.RequesterPermission != commonenum.PermissionAdmin && !key.IsOwnedBy(cmd.RequesterID) {
		log.Warn("[APIKeyCommand] No permission to revoke api key",
			zap.Uint("keyID", cmd.KeyID),
			zap.Uint("keyOwnerID", key.UserID()),
			zap.Uint("requesterID", cmd.RequesterID))
		return ierr.New(ierr.ErrNoPermission, "no permission to revoke api key")
	}

	if err := h.repo.Delete(ctx, cmd.KeyID); err != nil {
		log.Error("[APIKeyCommand] Delete api key failed", zap.Error(err), zap.Uint("keyID", cmd.KeyID))
		return err
	}

	masked := key.Secret().Masked()
	log.Info("[APIKeyCommand] API key revoked",
		zap.Uint("keyID", cmd.KeyID),
		zap.Uint("requesterID", cmd.RequesterID),
		zap.String("keyName", key.Name().String()),
		zap.String("masked", masked))
	return nil
}
