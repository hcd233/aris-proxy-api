package service

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// OAuthQuotaStore 额度探测结果的落库能力（窄接口）。
//
// 只暴露「额度列 / 错误列」两组更新，刻意不暴露整行保存：凭据表按 endpoint_id upsert
// 是全列语义，用聚合快照整行回写会把并发刷新出的新 access/refresh token 覆盖回旧值
// （刷新可能发生在探测取 token 之前）——refresh token 轮换后旧值已失效，凭据随即报废。
type OAuthQuotaStore interface {
	// UpdateQuota 更新额度快照与采集时间（不触碰 token/状态列）
	UpdateQuota(ctx context.Context, endpointID uint, snapshot vo.QuotaSnapshot, at time.Time) error
	// UpdateLastError 更新最近错误摘要（不触碰 token/状态列）
	UpdateLastError(ctx context.Context, endpointID uint, lastError string) error
}

// ProbeOAuthQuota 探测一次订阅窗口用量并落库，返回最新快照。
//
// 手动探测接口与 cron 维护任务共用本实现：同一逻辑曾分居两处并漂移（cron 侧用聚合
// 快照整行回写，覆盖了刷新后的 token）。
// 探测失败写 last_error 供管理端展示，但不改凭据状态机——额度探测失败 ≠ 凭据失效。
func ProbeOAuthQuota(
	ctx context.Context,
	endpointID uint,
	provider enum.OAuthProvider,
	tokens OAuthTokenProvider,
	clients map[enum.OAuthProvider]OAuthProviderClient,
	store OAuthQuotaStore,
) (*vo.QuotaSnapshot, error) {
	client := clients[provider]
	if client == nil {
		return nil, persistOAuthProbeError(ctx, store, endpointID,
			ierr.Newf(ierr.ErrInternal, "oauth provider client %q not wired", string(provider)))
	}

	access, err := tokens.GetAccessToken(ctx, endpointID)
	if err != nil {
		return nil, persistOAuthProbeError(ctx, store, endpointID, err)
	}
	snapshot, err := client.ProbeQuota(ctx, access.Token, access.AccountRef)
	if err != nil {
		return nil, persistOAuthProbeError(ctx, store, endpointID, err)
	}
	if err := store.UpdateQuota(ctx, endpointID, *snapshot, time.Now()); err != nil {
		return nil, err
	}
	return snapshot, nil
}

// persistOAuthProbeError 记录探测失败原因并原样返回错误（调用方据此上抛或记日志）。
//
// 摘要按字符截断后落库：上游错误体可能含中文，按字节截断会产生非法 UTF-8 导致写入失败。
func persistOAuthProbeError(ctx context.Context, store OAuthQuotaStore, endpointID uint, probeErr error) error {
	reason := util.TruncateRunes(constant.OAuthReasonQuotaProbeFailed+probeErr.Error(), constant.OAuthErrorReasonMaxLength)
	if err := store.UpdateLastError(ctx, endpointID, reason); err != nil {
		logger.WithCtx(ctx).Error("[OAuthQuota] Persist probe error failed",
			zap.Uint("endpointID", endpointID), zap.Error(err))
	}
	return probeErr
}
