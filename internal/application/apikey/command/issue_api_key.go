// Package command APIKey 域命令处理器
package command

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/service"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/vo"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// IssueAPIKeyCommand 签发新 API Key 命令
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type IssueAPIKeyCommand struct {
	UserID uint
	Name   string
}

// IssueAPIKeyResult 签发命令结果
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type IssueAPIKeyResult struct {
	KeyID     uint
	Name      string
	Secret    string
	CreatedAt time.Time
}

// UserExistenceChecker 用户存在性校验器（跨域适配接口，避免 apikey 域强依赖 identity 域）
//
// 返回 (true, nil) 表示用户存在；(false, nil) 表示用户不存在；(_, err) 表示查询失败。
//
//	@author centonhuang
//	@update 2026-04-22 20:00:00
type UserExistenceChecker interface {
	Exists(ctx context.Context, userID uint) (bool, error)
}

// IssueAPIKeyHandler 签发命令处理器
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type IssueAPIKeyHandler interface {
	Handle(ctx context.Context, cmd IssueAPIKeyCommand) (*IssueAPIKeyResult, error)
}

type issueAPIKeyHandler struct {
	repo         apikey.APIKeyRepository
	generator    service.APIKeyGenerator
	userExistsCh UserExistenceChecker
}

// NewIssueAPIKeyHandler 构造签发命令处理器
//
//	@param repo apikey.APIKeyRepository
//	@param generator service.APIKeyGenerator
//	@param userExistsCh 用户存在性校验器，若为 nil 则跳过前置存在性校验（用于测试场景）
//	@return IssueAPIKeyHandler
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewIssueAPIKeyHandler(repo apikey.APIKeyRepository, generator service.APIKeyGenerator, userExistsCh UserExistenceChecker) IssueAPIKeyHandler {
	return &issueAPIKeyHandler{repo: repo, generator: generator, userExistsCh: userExistsCh}
}

// Handle 执行签发命令
//
// 流程：生成 Secret → 统计已有数量 → 聚合根 IssueProxyAPIKey 校验配额并记录事件 → 仓储 Save 回填 ID
//
//	@receiver h *issueAPIKeyHandler
//	@param ctx context.Context
//	@param cmd IssueAPIKeyCommand
//	@return *IssueAPIKeyResult
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (h *issueAPIKeyHandler) Handle(ctx context.Context, cmd IssueAPIKeyCommand) (*IssueAPIKeyResult, error) {
	log := logger.WithCtx(ctx)

	// 前置：校验用户存在（与原 service.CreateAPIKey 行为对齐）
	if h.userExistsCh != nil {
		exists, err := h.userExistsCh.Exists(ctx, cmd.UserID)
		if err != nil {
			log.Error("[APIKeyCommand] Check user existence failed", zap.Error(err), zap.Uint("userID", cmd.UserID))
			return nil, err
		}
		if !exists {
			log.Warn("[APIKeyCommand] User not found when issuing api key", zap.Uint("userID", cmd.UserID))
			return nil, ierr.New(ierr.ErrDataNotExists, "user not found")
		}
	}

	existing, err := h.repo.CountByUser(ctx, cmd.UserID)
	if err != nil {
		log.Error("[APIKeyCommand] Count api keys failed", zap.Error(err), zap.Uint("userID", cmd.UserID))
		return nil, err
	}

	secret, err := h.generator.Generate()
	if err != nil {
		log.Error("[APIKeyCommand] Generate secret failed", zap.Error(err))
		return nil, err
	}

	key, err := aggregate.IssueProxyAPIKey(
		cmd.UserID,
		vo.APIKeyName(cmd.Name),
		secret,
		vo.DefaultAPIKeyQuota(),
		existing,
		time.Now(),
	)
	if err != nil {
		log.Warn("[APIKeyCommand] Issue aggregate failed", zap.Error(err),
			zap.Uint("userID", cmd.UserID), zap.String("name", cmd.Name))
		return nil, err
	}

	if err := h.repo.Save(ctx, key); err != nil {
		log.Error("[APIKeyCommand] Save api key failed", zap.Error(err))
		return nil, err
	}

	masked := key.Secret().Masked()
	log.Info("[APIKeyCommand] API key issued",
		zap.Uint("keyID", key.AggregateID()),
		zap.Uint("userID", key.UserID()),
		zap.String("name", key.Name().String()),
		zap.String("masked", masked))

	return &IssueAPIKeyResult{
		KeyID:     key.AggregateID(),
		Name:      key.Name().String(),
		Secret:    key.Secret().Raw(),
		CreatedAt: key.CreatedAt(),
	}, nil
}
